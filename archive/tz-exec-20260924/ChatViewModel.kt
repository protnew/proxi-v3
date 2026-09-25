package com.indestructible.messenger.messenger

import android.content.Context
import android.util.Log
import androidx.compose.runtime.mutableStateListOf
import androidx.compose.runtime.mutableStateOf
import okhttp3.*
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.RequestBody.Companion.toRequestBody
import org.json.JSONObject
import java.util.concurrent.TimeUnit
import com.indestructible.messenger.crypto.Nip44
import com.indestructible.messenger.data.ChatEntity
import com.indestructible.messenger.data.MessageEntity
import com.indestructible.messenger.data.ProxiDatabase

// P4: no public Nostr relays — DM/presence via local authenticated /nostr only.
class ChatViewModel(
    private val appContext: Context,
    var serverUrl: String = "ws://10.0.2.2:8090/ws",
    var httpUrl: String = "http://10.0.2.2:8090",
) {
    val chats = mutableStateListOf<Chat>()
    // Messages of the ACTIVE chat only (loaded from Room on setActiveChat).
    val messages = mutableStateListOf<Message>()
    val connectionStatus = mutableStateOf("disconnected")
    var activeChatId: String? = null
        private set

    private val db by lazy { ProxiDatabase.get(appContext) }
    private var webSocket: WebSocket? = null
    private var jwtToken: String = ""
    @Volatile private var connecting = false
    private var myPubKey: String = ""
    // P24: held for E2E only, never sent anywhere.
    private var myPrivKeyHex: String = ""

    // Presence: peers observed via WS join events.
    val onlinePeers = mutableStateOf<Set<String>>(emptySet())

    // Typing: peers currently composing in a DM with us.
    val typingPeers = mutableStateOf<Set<String>>(emptySet())
    private var lastTypingSent = 0L

    /** Throttled typing event — wire-compatible with PWA ("type":"typing"). */
    fun sendTyping(to: String) {
        val now = System.currentTimeMillis()
        if (now - lastTypingSent < 3000) return
        lastTypingSent = now
        webSocket?.send(JSONObject().apply {
            put("type", "typing"); put("to", to); put("ts", now / 1000)
        }.toString())
    }

    private fun noteTyping(from: String) {
        if (from == myPubKey || from.isEmpty()) return
        typingPeers.value = typingPeers.value + from
        // Auto-expire after 4s of silence.
        android.os.Handler(android.os.Looper.getMainLooper()).postDelayed({
            typingPeers.value = typingPeers.value - from
        }, 4000)
    }

    fun setPrivateKey(hex: String) { myPrivKeyHex = hex }

    fun isOnline(pubkey: String) = onlinePeers.value.contains(pubkey)

    // ---- persistence ----

    fun loadChats() {
        Thread {
            val rows = db.chats().all()
            val mapped = rows.map { it.toModel() }
            android.os.Handler(android.os.Looper.getMainLooper()).post {
                chats.clear(); chats.addAll(mapped)
            }
        }.start()
    }

    private fun ChatEntity.toModel() = Chat(
        id = id, name = name, avatar = avatar,
        type = if (type == "GROUP") Chat.Type.GROUP else Chat.Type.DM,
        messages = emptyList(),
        unread = unread,
        lastActivity = lastActivity,
        lastMessageText = lastMessageText,
        peerPubKey = peerPubKey,
        members = members?.split(",")?.filter { it.isNotBlank() },
    )

    private fun MessageEntity.toModel() = Message(
        id = id, from = fromPub, to = toPub, text = text,
        timestamp = timestamp, read = read, replyTo = replyTo,
        edited = edited, delivered = delivered,
        type = when (type) {
            "VOICE" -> Message.Type.VOICE
            "FILE" -> Message.Type.FILE
            "SYSTEM" -> Message.Type.SYSTEM
            else -> Message.Type.TEXT
        },
        fileUrl = fileUrl, fileName = fileName,
        fileSize = fileSize, voiceDuration = voiceDuration,
    )

    private fun persistMessage(m: MessageEntity) {
        Thread { db.messages().upsert(m) }.start()
    }

    private fun upsertChatEntity(e: ChatEntity) {
        Thread { db.chats().upsert(e) }.start()
    }

    /** Incoming message: ensure a chat row exists (DM or group) and bump unread. */
    private fun ensureIncomingChat(fromPub: String, preview: String, ts: Long, chatId: String? = null) {
        val cid = chatId ?: "dm:$fromPub"
        Thread {
            val existing = db.chats().byId(cid)
            val displayName = db.contacts().byPubkey(fromPub)?.name
                ?: "${fromPub.take(8)}…${fromPub.takeLast(4)}"
            if (existing == null) {
                val isGroup = cid.startsWith("group:")
                db.chats().upsert(ChatEntity(
                    id = cid,
                    name = if (isGroup) "Группа ${cid.removePrefix("group:").take(12)}" else displayName,
                    avatar = if (isGroup) "👥" else displayName.take(1),
                    type = if (isGroup) "GROUP" else "DM",
                    lastActivity = ts, unread = 1,
                    lastMessageText = preview,
                    peerPubKey = if (isGroup) null else fromPub,
                ))
            } else {
                db.chats().bumpIncoming(cid, ts, preview)
            }
            val fresh = db.chats().all().map { it.toModel() }
            android.os.Handler(android.os.Looper.getMainLooper()).post {
                chats.clear(); chats.addAll(fresh)
            }
        }.start()
    }

    private val client = OkHttpClient.Builder()
        .readTimeout(0, TimeUnit.MILLISECONDS)
        .pingInterval(30, TimeUnit.SECONDS)
        .connectTimeout(10, TimeUnit.SECONDS)
        .build()

    init {
        // Call signaling rides key_exchange messages (PWA wire format).
        CallManager.sendSignal = { peer, signalJson ->
            webSocket?.send(JSONObject().apply {
                put("type", "key_exchange")
                put("to", peer)
                put("publicKey", signalJson.toString())
                put("ts", System.currentTimeMillis() / 1000)
            }.toString())
        }
    }

    fun startAudioCall(peer: String) = CallManager.startCall(appContext, peer, video = false)
    fun startVideoCall(peer: String) = CallManager.startCall(appContext, peer, video = true)

    fun connect(myPubKey: String) {
        if (connecting) return // WS-failure retry loop + NavHost re-entry must not stack auth threads
        connecting = true
        this.myPubKey = myPubKey
        connectionStatus.value = "authenticating"

        // Step 1 (P1): challenge-response signup — bare npub is rejected.
        // challenge → signed kind:22242 event → JWT.
        val mediaType = "application/json".toMediaType()

        Thread {
            try {
                if (myPrivKeyHex.isEmpty()) {
                    connectionStatus.value = "error: no key"
                    return@Thread
                }
                // 1. request challenge
                val chReq = Request.Builder()
                    .url("$httpUrl/api/auth/challenge")
                    .post("""{"npub":"$myPubKey"}""".toRequestBody(mediaType))
                    .build()
                val chResp = client.newCall(chReq).execute()
                val chBody = chResp.body?.string() ?: ""
                if (chResp.code != 200) {
                    connectionStatus.value = "error: challenge ${chResp.code}"
                    Log.e("ChatVM", "Challenge failed: ${chResp.code} $chBody")
                    return@Thread
                }
                val challenge = JSONObject(chBody).getString("challenge")

                // 2. build + sign kind:22242 event (NIP-01 serialization)
                val createdAt = System.currentTimeMillis() / 1000
                val tagsJson = """[["challenge","$challenge"]]"""
                val serialized = """[0,"$myPubKey",$createdAt,22242,$tagsJson,"$challenge"]"""
                val idBytes = java.security.MessageDigest.getInstance("SHA-256")
                    .digest(serialized.toByteArray(Charsets.UTF_8))
                val idHex = idBytes.joinToString("") { "%02x".format(it) }
                val skBytes = myPrivKeyHex.chunked(2).map { it.toInt(16).toByte() }.toByteArray()
                val sig = com.indestructible.messenger.crypto.Schnorr.sign(idBytes, skBytes)
                    .joinToString("") { "%02x".format(it) }

                val eventJson = JSONObject().apply {
                    put("id", idHex); put("pubkey", myPubKey)
                    put("created_at", createdAt); put("kind", 22242)
                    put("tags", org.json.JSONArray(tagsJson))
                    put("content", challenge); put("sig", sig)
                }
                val body = JSONObject().apply {
                    put("npub", myPubKey)
                    put("username", "android_${myPubKey.take(8)}")
                    put("challenge", challenge)
                    put("event", eventJson)
                }.toString().toRequestBody(mediaType)

                val req = Request.Builder()
                    .url("$httpUrl/api/auth/signup")
                    .post(body)
                    .build()
                val resp = client.newCall(req).execute()
                val respBody = resp.body?.string() ?: ""

                if (resp.code == 200) {
                    val json = JSONObject(respBody)
                    jwtToken = json.optString("access_token", "")
                    if (jwtToken.isNotEmpty()) {
                        Log.i("ChatVM", "Auth OK, token received")
                        NostrRelayService.jwtToken = jwtToken
                        NostrRelayService.relayBase =
                            httpUrl.replaceFirst("http", "ws") + "/nostr"
                        connectWebSocket(myPubKey)
                    } else {
                        connectionStatus.value = "error: no token"
                    }
                } else {
                    Log.e("ChatVM", "Signup failed: ${resp.code} $respBody")
                    connectionStatus.value = "error: signup ${resp.code}"
                }
            } catch (e: Exception) {
                Log.e("ChatVM", "Auth error", e)
                connectionStatus.value = "error: ${e.message}"
            } finally {
                // connectWebSocket resets the flag on open/failure; every
                // other exit path leaves here and must release the guard.
                if (connectionStatus.value != "connecting" && connectionStatus.value != "connected") {
                    connecting = false
                }
            }
        }.start()
    }

    private fun connectWebSocket(pubKey: String) {
        connectionStatus.value = "connecting"

        // Step 2: Connect WS with JWT token
        val req = Request.Builder()
            .url("$serverUrl?token=$jwtToken")
            .build()

        webSocket = client.newWebSocket(req, object : WebSocketListener() {
            override fun onOpen(ws: WebSocket, response: Response) {
                connecting = false
                connectionStatus.value = "connected"
                Log.i("ChatVM", "WS connected as $pubKey")
            }

            override fun onMessage(ws: WebSocket, text: String) {
                Log.i("ChatVM", "WS message: ${text.take(100)}")
                handleIncoming(text)
            }

            override fun onClosed(ws: WebSocket, code: Int, reason: String) {
                connectionStatus.value = "disconnected"
                Log.i("ChatVM", "WS closed: $code $reason")
            }

            override fun onFailure(ws: WebSocket, t: Throwable, response: Response?) {
                connecting = false
                connectionStatus.value = "error: ${t.message}"
                Log.e("ChatVM", "WS failure", t)
                Thread { Thread.sleep(3000); connect(pubKey) }.start()
            }
        })
    }

    fun debugPubKey(): String = myPubKey

    fun sendMessage(from: String, to: String, text: String, replyTo: String? = null) {
        val isGroup = to.startsWith("group:")
        val chatId = if (isGroup) to else "dm:$to"
        val msg = Message(
            id = "msg_${System.currentTimeMillis()}",
            from = from, to = to, text = text,
            timestamp = System.currentTimeMillis(),
            replyTo = replyTo,
        )
        messages.add(msg)

        persistMessage(MessageEntity(
            id = msg.id, chatId = chatId, fromPub = from, toPub = to,
            text = text, timestamp = msg.timestamp,
            outgoing = true, replyTo = replyTo,
        ))
        Thread {
            db.chats().byId(chatId)?.let {
                db.chats().upsert(it.copy(lastActivity = msg.timestamp, lastMessageText = text))
            }
        }.start()

        if (isGroup) {
            // E2E group fanout: NIP-44 per member, `group` field marks the room.
            Thread {
                val members = db.chats().byId(chatId)?.members
                    ?.split(",")?.filter { it.isNotBlank() && it != myPubKey } ?: emptyList()
                var allOk = true
                for (m in members) {
                    if (!sendEncryptedDm(m, text, group = to, replyTo = replyTo, msgId = msg.id)) {
                        allOk = false
                    }
                }
                markDeliveredLocal(msg.id, allOk)
            }.start()
            return
        }

        val sent = sendEncryptedDm(to, text, replyTo = replyTo, msgId = msg.id)
        if (sent) markDeliveredLocal(msg.id, true)
    }

    private fun markDeliveredLocal(msgId: String, ok: Boolean) {
        if (!ok) return
        android.os.Handler(android.os.Looper.getMainLooper()).post {
            val idx = messages.indexOfFirst { it.id == msgId }
            if (idx >= 0) messages[idx] = messages[idx].copy(delivered = true)
        }
        Thread { db.messages().markDelivered(msgId) }.start()
    }

    /**
     * P24: E2E-encrypt `text` for `to` and send over WS. Fails CLOSED —
     * returns false instead of ever downgrading to plaintext.
     * `group` marks group-fanout copies so the peer files them under the room.
     */
    private fun sendEncryptedDm(
        to: String, text: String,
        group: String? = null, replyTo: String? = null, msgId: String? = null,
    ): Boolean {
        var payload = text
        var isE2E = false
        if (to.isNotEmpty() && to != "broadcast" && myPrivKeyHex.isNotEmpty()) {
            try {
                val theirPubHex = if (to.startsWith("npub1")) {
                    com.indestructible.messenger.crypto.Bech32.decodeNpub(to)
                        .joinToString("") { "%02x".format(it) }
                } else to
                payload = "nip44:" + Nip44.encryptFor(text, myPrivKeyHex, theirPubHex)
                isE2E = true
            } catch (e: Exception) {
                Log.e("ChatVM", "E2E encrypt failed — NOT sending plaintext", e)
                return false // fail closed: never downgrade to plaintext
            }
        }
        val json = JSONObject().apply {
            put("type", "chat")
            put("to", to)
            put("text", payload)
            if (isE2E) { put("encrypted", true); put("is_e2e", true) }
            if (group != null) put("group", group)
            if (replyTo != null) put("replyTo", replyTo)
            put("ts", System.currentTimeMillis() / 1000)
            put("id", msgId ?: "msg_${System.currentTimeMillis()}")
        }
        val sent = webSocket?.send(json.toString()) ?: false
        Log.i("ChatVM", "Send: ${json.optString("id")} to=${to.take(8)} sent=$sent")
        return sent
    }

    /**
     * Create a group chat locally and invite members via E2E `groupmeta` DM.
     * Signal-style client fanout — no server-side group routing needed.
     */
    fun createGroupChat(name: String, memberPubs: List<String>) {
        val gid = "grp-${System.currentTimeMillis()}"
        val chatId = "group:$gid"
        val all = (memberPubs + myPubKey).distinct()
        upsertChatEntity(ChatEntity(
            id = chatId, name = name, avatar = "👥", type = "GROUP",
            lastActivity = System.currentTimeMillis(),
            members = all.joinToString(","),
        ))
        val meta = "groupmeta:" + JSONObject().apply {
            put("id", gid); put("name", name)
            put("members", org.json.JSONArray(all))
        }.toString()
        Thread {
            for (m in memberPubs.filter { it != myPubKey }) {
                sendEncryptedDm(m, meta)
            }
            val fresh = db.chats().all().map { it.toModel() }
            android.os.Handler(android.os.Looper.getMainLooper()).post {
                chats.clear(); chats.addAll(fresh)
            }
        }.start()
    }

    /**
     * Upload bytes to /api/files (JWT) then send an E2E file descriptor.
     * The ciphertext carries `filemeta:{json}` — file metadata never leaves
     * the E2E channel in plaintext.
     */
    fun sendAttachment(
        to: String, fileName: String, bytes: ByteArray,
        mime: String, voiceDurationSec: Int? = null,
    ) {
        Thread {
            try {
                val body = MultipartBody.Builder().setType(MultipartBody.FORM)
                    .addFormDataPart(
                        "file", fileName,
                        bytes.toRequestBody(mime.toMediaType())
                    ).build()
                val req = Request.Builder()
                    .url("$httpUrl/api/files/upload")
                    .header("Authorization", "Bearer $jwtToken")
                    .post(body).build()
                val resp = client.newCall(req).execute()
                val respBody = resp.body?.string() ?: ""
                if (resp.code != 201 && resp.code != 200) {
                    Log.e("ChatVM", "upload failed: ${resp.code} $respBody")
                    return@Thread
                }
                val url = JSONObject(respBody).getString("url")
                val meta = JSONObject().apply {
                    put("url", url); put("name", fileName); put("size", bytes.size)
                    put("mime", mime)
                    if (voiceDurationSec != null) {
                        put("kind", "voice"); put("dur", voiceDurationSec)
                    } else put("kind", "file")
                }
                val descr = "filemeta:" + meta.toString()
                val kind = if (voiceDurationSec != null) Message.Type.VOICE else Message.Type.FILE

                val msg = Message(
                    id = "msg_${System.currentTimeMillis()}",
                    from = myPubKey, to = to, text = fileName,
                    timestamp = System.currentTimeMillis(),
                    type = kind, fileName = fileName,
                    fileSize = bytes.size.toLong(), fileUrl = url,
                    voiceDuration = voiceDurationSec,
                )
                android.os.Handler(android.os.Looper.getMainLooper()).post {
                    messages.add(msg)
                }
                persistMessage(MessageEntity(
                    id = msg.id, chatId = "dm:$to", fromPub = myPubKey, toPub = to,
                    text = fileName, timestamp = msg.timestamp, type = kind.name,
                    outgoing = true, fileUrl = url, fileName = fileName,
                    fileSize = bytes.size.toLong(), voiceDuration = voiceDurationSec,
                ))

                // E2E-wrap the descriptor (fail closed like sendMessage).
                if (myPrivKeyHex.isEmpty()) return@Thread
                val theirPubHex = if (to.startsWith("npub1")) {
                    com.indestructible.messenger.crypto.Bech32.decodeNpub(to)
                        .joinToString("") { "%02x".format(it) }
                } else to
                val payload = "nip44:" + Nip44.encryptFor(descr, myPrivKeyHex, theirPubHex)
                val json = JSONObject().apply {
                    put("type", "chat"); put("to", to); put("text", payload)
                    put("encrypted", true); put("is_e2e", true)
                    put("ts", System.currentTimeMillis() / 1000)
                    put("id", msg.id)
                }
                webSocket?.send(json.toString())
            } catch (e: Exception) {
                Log.e("ChatVM", "sendAttachment error", e)
            }
        }.start()
    }

    // P11: filemeta comes from the peer and is attacker-controlled. Only a
    // relative same-origin /api/files/... path may ever be fetched with the
    // JWT; file names are reduced to their basename (no traversal).
    private fun sanitizeFileUrl(raw: String?): String? {
        if (raw.isNullOrBlank()) return null
        val u = raw.trim()
        if (!u.startsWith("/api/files/")) return null
        if (u.contains("..") || u.contains('@') || u.contains("://") || u.contains('\\')) return null
        return u
    }

    private fun sanitizeFileName(raw: String?): String =
        raw?.substringAfterLast('/')?.substringAfterLast('\\')?.takeIf { it.isNotBlank() && it != "." && it != ".." }
            ?: "file"

    /** Download an attachment (JWT-authed) into app files dir. */
    fun downloadAttachment(fileUrl: String, fileName: String, onDone: (java.io.File?) -> Unit) {
        val safeUrl = sanitizeFileUrl(fileUrl) ?: run {
            Log.e("ChatVM", "blocked non-relative attachment url")
            onDone(null); return
        }
        val safeName = sanitizeFileName(fileName)
        Thread {
            try {
                val req = Request.Builder()
                    .url(httpUrl + safeUrl)
                    .header("Authorization", "Bearer $jwtToken")
                    .build()
                val resp = client.newCall(req).execute()
                if (resp.code != 200) { onDone(null); return@Thread }
                val dir = java.io.File(appContext.filesDir, "attachments").apply { mkdirs() }
                val out = java.io.File(dir, safeName)
                resp.body?.byteStream()?.use { inp ->
                    out.outputStream().use { inp.copyTo(it) }
                }
                android.os.Handler(android.os.Looper.getMainLooper()).post { onDone(out) }
            } catch (e: Exception) {
                Log.e("ChatVM", "downloadAttachment error", e)
                android.os.Handler(android.os.Looper.getMainLooper()).post { onDone(null) }
            }
        }.start()
    }

    private fun handleIncoming(raw: String) {
        try {
            val json = JSONObject(raw)
            val type = json.optString("type", "")

            if (type == "join") {
                val joined = json.optString("from", "")
                if (joined.isNotEmpty() && joined != "system" && joined != myPubKey) {
                    onlinePeers.value = onlinePeers.value + joined
                }
            }
            if (type == "leave") {
                val left = json.optString("from", "")
                if (left.isNotEmpty()) onlinePeers.value = onlinePeers.value - left
            }
            if (type == "typing") {
                noteTyping(json.optString("from", ""))
            }
            if (type == "key_exchange") {
                // Call signaling envelope (PWA compat): signal JSON in publicKey.
                val sig = json.optString("publicKey", "")
                val from = json.optString("from", "")
                if (sig.isNotEmpty() && from.isNotEmpty() && from != myPubKey) {
                    CallManager.handleSignal(from, sig)
                }
            }
            if (type == "message_deleted") {
                val delId = json.optString("id", "")
                if (delId.isNotEmpty()) {
                    android.os.Handler(android.os.Looper.getMainLooper()).post {
                        messages.removeAll { it.id == delId }
                    }
                    Thread { db.messages().delete(delId) }.start()
                }
            }

            if ((type == "message" || type == "chat") && json.optString("text", "") != "connected") {
                val from = json.optString("from", json.optString("sender", ""))
                val to = json.optString("to", json.optString("recipient", ""))
                val text = json.optString("text", json.optString("content", ""))

                // Self-echo (server sends our own copies back for multi-device
                // sync): skip early — E2E-decrypting our own outbound copy is
                // keyed to the recipient, not us, and always fails MAC.
                if (from == myPubKey) return

                // P24: real NIP-44 decrypt. "nip44:" prefix marks client ciphertext.
                val rawText = text
                val finalText = if (rawText.startsWith("nip44:") && myPrivKeyHex.isNotEmpty()) {
                    try {
                        val theirPubHex = if (from.startsWith("npub1")) {
                            com.indestructible.messenger.crypto.Bech32.decodeNpub(from)
                                .joinToString("") { "%02x".format(it) }
                        } else from
                        Nip44.decryptFrom(rawText.removePrefix("nip44:"), myPrivKeyHex, theirPubHex)
                    } catch (e: Exception) {
                        Log.e("ChatVM", "E2E decrypt failed", e)
                        "[не удалось расшифровать]"
                    }
                } else {
                    rawText
                }

                if (finalText.isNotEmpty() && from != myPubKey) {
                    val ts = json.optLong("timestamp", json.optLong("ts", System.currentTimeMillis()))
                    val msgId = json.optString("id", "msg_${ts}")
                    val replyToId = json.optString("replyTo", "").ifEmpty { null }
                    val groupField = json.optString("group", "")

                    // groupmeta:{json} — E2E group invite descriptor.
                    if (finalText.startsWith("groupmeta:")) {
                        try {
                            val gm = JSONObject(finalText.removePrefix("groupmeta:"))
                            val gid = gm.getString("id")
                            val gname = gm.optString("name", "Группа")
                            val arr = gm.optJSONArray("members")
                            val members = mutableListOf<String>()
                            if (arr != null) for (i in 0 until arr.length()) {
                                members.add(arr.getString(i))
                            }
                            if (myPubKey.isNotEmpty() && !members.contains(myPubKey)) {
                                members.add(myPubKey)
                            }
                            val gChatId = "group:$gid"
                            Thread {
                                db.chats().upsert(ChatEntity(
                                    id = gChatId, name = gname, avatar = "👥",
                                    type = "GROUP", lastActivity = ts,
                                    members = members.joinToString(","),
                                ))
                                val fresh = db.chats().all().map { it.toModel() }
                                android.os.Handler(android.os.Looper.getMainLooper()).post {
                                    chats.clear(); chats.addAll(fresh)
                                }
                            }.start()
                        } catch (e: Exception) {
                            Log.e("ChatVM", "bad groupmeta", e)
                        }
                        return
                    }

                    val chatId = if (groupField.isNotEmpty()) groupField else "dm:$from"

                    // filemeta:{json} — decrypted attachment descriptor.
                    var mType = Message.Type.TEXT
                    var mText = finalText
                    var fileUrl: String? = null
                    var fileName: String? = null
                    var fileSize: Long? = null
                    var voiceDur: Int? = null
                    if (finalText.startsWith("filemeta:")) {
                        try {
                            val meta = JSONObject(finalText.removePrefix("filemeta:"))
                            // P11: accept only relative same-origin file paths;
                            // a hostile url/name degrades to a plain text bubble.
                            fileUrl = sanitizeFileUrl(meta.getString("url"))
                            fileName = sanitizeFileName(meta.optString("name", "file"))
                            fileSize = meta.optLong("size")
                            voiceDur = if (meta.has("dur")) meta.getInt("dur") else null
                            if (fileUrl != null) {
                                mType = if (meta.optString("kind") == "voice")
                                    Message.Type.VOICE else Message.Type.FILE
                                mText = fileName ?: "file"
                            } else {
                                mText = fileName ?: "file"
                            }
                        } catch (e: Exception) {
                            Log.e("ChatVM", "bad filemeta", e)
                        }
                    }

                    persistMessage(MessageEntity(
                        id = msgId, chatId = chatId, fromPub = from, toPub = to,
                        text = mText, timestamp = ts, outgoing = false,
                        type = mType.name, fileUrl = fileUrl, fileName = fileName,
                        fileSize = fileSize, voiceDuration = voiceDur,
                        replyTo = replyToId,
                    ))
                    ensureIncomingChat(from, mText, ts, chatId)
                    Thread {
                        val senderName = db.contacts().byPubkey(from)?.name
                            ?: "${from.take(8)}…${from.takeLast(4)}"
                        Notifier.notifyIncoming(appContext, senderName, msgId)
                    }.start()
                    if (activeChatId == chatId) {
                        messages.add(Message(
                            id = msgId, from = from, to = to,
                            text = mText, timestamp = ts, type = mType,
                            fileUrl = fileUrl, fileName = fileName,
                            fileSize = fileSize, voiceDuration = voiceDur,
                            replyTo = replyToId,
                        ))
                        Thread { db.messages().markIncomingRead(chatId); db.chats().markRead(chatId) }.start()
                    }
                }
            }
        } catch (e: Exception) {
            Log.e("ChatVM", "Parse error: ${raw.take(100)}", e)
        }
    }

    fun setActiveChat(chatId: String) {
        activeChatId = chatId
        Thread {
            val rows = db.messages().forChat(chatId)
            db.messages().markIncomingRead(chatId)
            db.chats().markRead(chatId)
            val mapped = rows.map { it.toModel() }
            val freshChats = db.chats().all().map { it.toModel() }
            android.os.Handler(android.os.Looper.getMainLooper()).post {
                messages.clear(); messages.addAll(mapped)
                chats.clear(); chats.addAll(freshChats)
            }
        }.start()
    }

    fun closeActiveChat() {
        activeChatId = null
        messages.clear()
    }

    fun createDmChat(peerPubKey: String, name: String) {
        val chatId = "dm:$peerPubKey"
        upsertChatEntity(ChatEntity(
            id = chatId, name = name, avatar = name.take(1),
            type = "DM", lastActivity = System.currentTimeMillis(),
            peerPubKey = peerPubKey,
        ))
        Thread {
            db.contacts().upsert(com.indestructible.messenger.data.ContactEntity(
                pubkey = peerPubKey, name = name,
            ))
            val fresh = db.chats().all().map { it.toModel() }
            android.os.Handler(android.os.Looper.getMainLooper()).post {
                chats.clear(); chats.addAll(fresh)
            }
        }.start()
    }

    fun deleteMessage(msgId: String) {
        messages.removeAll { it.id == msgId }
        Thread { db.messages().delete(msgId) }.start()
    }

    // Live message search (sidebar search field).
    val searchResults = mutableStateListOf<Message>()

    fun searchMessages(query: String) {
        Thread {
            val rows = db.messages().search(query)
            val mapped = rows.map { it.toModel() }
            android.os.Handler(android.os.Looper.getMainLooper()).post {
                searchResults.clear(); searchResults.addAll(mapped)
            }
        }.start()
    }

    fun clearSearch() = searchResults.clear()

    /** Logout wipe: local history + chat list, then caller clears IdentityStore. */
    fun wipeAll() {
        disconnect()
        chats.clear(); messages.clear(); searchResults.clear()
        Thread { db.clearAllTables() }.start()
    }

    fun disconnect() {
        connecting = false
        webSocket?.close(1000, "disconnect")
        webSocket = null
        connectionStatus.value = "disconnected"
    }
}

