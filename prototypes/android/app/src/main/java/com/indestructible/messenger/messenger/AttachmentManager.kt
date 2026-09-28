package com.indestructible.messenger.messenger

import android.util.Log
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.MultipartBody
import okhttp3.Request
import okhttp3.RequestBody.Companion.toRequestBody
import org.json.JSONObject
import com.indestructible.messenger.crypto.Nip44
import com.indestructible.messenger.data.ChatEntity
import com.indestructible.messenger.data.MessageEntity

fun ChatViewModel.sendAttachment(
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

internal fun ChatViewModel.sanitizeFileUrl(raw: String?): String? {
    if (raw.isNullOrBlank()) return null
    val u = raw.trim()
    if (!u.startsWith("/api/files/")) return null
    if (u.contains("..") || u.contains('@') || u.contains("://") || u.contains('\\')) return null
    return u
}


internal fun ChatViewModel.sanitizeFileName(raw: String?): String =
    raw?.substringAfterLast('/')?.substringAfterLast('\\')?.takeIf { it.isNotBlank() && it != "." && it != ".." }
        ?: "file"


/** Download an attachment (JWT-authed) into app files dir. */
fun ChatViewModel.downloadAttachment(fileUrl: String, fileName: String, onDone: (java.io.File?) -> Unit) {
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
