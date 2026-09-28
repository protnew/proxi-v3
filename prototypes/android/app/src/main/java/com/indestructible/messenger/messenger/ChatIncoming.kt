package com.indestructible.messenger.messenger

import android.util.Log
import org.json.JSONObject
import com.indestructible.messenger.crypto.Nip44
import com.indestructible.messenger.data.ChatEntity
import com.indestructible.messenger.data.MessageEntity

internal fun ChatViewModel.handleIncoming(raw: String) {
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

