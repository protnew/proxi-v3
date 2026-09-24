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
    internal val appContext: Context,
    var serverUrl: String = "ws://10.0.2.2:8090/ws",
    var httpUrl: String = "http://10.0.2.2:8090",
) {
    val chats = mutableStateListOf<Chat>()
    // Messages of the ACTIVE chat only (loaded from Room on setActiveChat).
    val messages = mutableStateListOf<Message>()
    val connectionStatus = mutableStateOf("disconnected")
    var activeChatId: String? = null
        private set

    internal val db by lazy { ProxiDatabase.get(appContext) }
    internal var webSocket: WebSocket? = null
    internal var jwtToken: String = ""
    @Volatile private var connecting = false
    internal var myPubKey: String = ""
    // P24: held for E2E only, never sent anywhere.
    internal var myPrivKeyHex: String = ""

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

    internal fun noteTyping(from: String) {
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

    internal fun ChatEntity.toModel() = Chat(
        id = id, name = name, avatar = avatar,
        type = if (type == "GROUP") Chat.Type.GROUP else Chat.Type.DM,
        messages = emptyList(),
        unread = unread,
        lastActivity = lastActivity,
        lastMessageText = lastMessageText,
        peerPubKey = peerPubKey,
        members = members?.split(",")?.filter { it.isNotBlank() },
    )

    internal fun MessageEntity.toModel() = Message(
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

    internal fun persistMessage(m: MessageEntity) {
        Thread { db.messages().upsert(m) }.start()
    }

    internal fun upsertChatEntity(e: ChatEntity) {
        Thread { db.chats().upsert(e) }.start()
    }

    /** Incoming message: ensure a chat row exists (DM or group) and bump unread. */
    internal fun ensureIncomingChat(fromPub: String, preview: String, ts: Long, chatId: String? = null) {
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

    internal val client = OkHttpClient.Builder()
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

    internal fun markDeliveredLocal(msgId: String, ok: Boolean) {
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
    internal fun sendEncryptedDm(
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
