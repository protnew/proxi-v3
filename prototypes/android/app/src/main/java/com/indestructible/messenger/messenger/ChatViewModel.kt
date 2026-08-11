package com.indestructible.messenger.messenger

import android.util.Log
import androidx.compose.runtime.mutableStateListOf
import androidx.compose.runtime.mutableStateOf
import okhttp3.*
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.RequestBody.Companion.toRequestBody
import org.json.JSONObject
import java.util.concurrent.TimeUnit
import com.indestructible.messenger.crypto.Nip44

class ChatViewModel(
    var serverUrl: String = "ws://10.0.2.2:8090/ws",
    var httpUrl: String = "http://10.0.2.2:8090",
    val nostrRelays: List<String> = listOf("wss://relay.damus.io", "wss://nos.lol", "wss://relay.nostr.band")
) {
    val chats = mutableStateListOf<Chat>()
    val messages = mutableStateListOf<Message>()
    val connectionStatus = mutableStateOf("disconnected")

    private var webSocket: WebSocket? = null
    private var jwtToken: String = ""
    private var myPubKey: String = ""

    private val client = OkHttpClient.Builder()
        .readTimeout(0, TimeUnit.MILLISECONDS)
        .pingInterval(30, TimeUnit.SECONDS)
        .connectTimeout(10, TimeUnit.SECONDS)
        .build()

    fun connect(myPubKey: String) {
        this.myPubKey = myPubKey
        connectionStatus.value = "authenticating"

        // Step 1: Signup to get JWT
        val mediaType = "application/json".toMediaType()
        val body = """{"npub":"$myPubKey"}""".toRequestBody(mediaType)

        Thread {
            try {
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
                connectionStatus.value = "error: ${t.message}"
                Log.e("ChatVM", "WS failure", t)
                Thread { Thread.sleep(3000); connect(pubKey) }.start()
            }
        })
    }

    fun sendMessage(from: String, to: String, text: String) {
        val msg = Message(
            id = "msg_${System.currentTimeMillis()}",
            from = from, to = to, text = text,
            timestamp = System.currentTimeMillis()
        )
        messages.add(msg)

        // Send plaintext via WS — server handles relay/storage
        // MOB-103: Client-side encryption is optional (Nip44 available but disabled
        // until secp256k1 ECDH is implemented for PWA compatibility)
        val json = JSONObject().apply {
            put("type", "chat")
            put("to", to)
            put("text", text)
            put("ts", System.currentTimeMillis() / 1000)
            put("id", msg.id)
        }
        val sent = webSocket?.send(json.toString()) ?: false
        Log.i("ChatVM", "Send: ${json.optString("id")} text='${text.take(20)}' sent=$sent")
    }

    private fun handleIncoming(raw: String) {
        try {
            val json = JSONObject(raw)
            val type = json.optString("type", "")

            if ((type == "message" || type == "chat") && json.optString("text", "") != "connected") {
                val from = json.optString("from", json.optString("sender", ""))
                val to = json.optString("to", json.optString("recipient", ""))
                val text = json.optString("text", json.optString("content", ""))

                // MOB-103: Decrypt if encrypted payload
                val encryptedText = json.optString("encrypted", "")
                val finalText = if (encryptedText.isNotEmpty()) {
                    try {
                        val convKey = Nip44.deriveConversationKey(from.toByteArray())
                        val decrypted = Nip44.decrypt(encryptedText, convKey)
                        Nip44.zeroBuffer(convKey)
                        decrypted
                    } catch (e: Exception) {
                        text // fallback to plaintext
                    }
                } else {
                    text
                }

                if (finalText.isNotEmpty()) {
                    messages.add(Message(
                        id = json.optString("id", "msg_${System.currentTimeMillis()}"),
                        from = from,
                        to = to,
                        text = finalText,
                        timestamp = json.optLong("timestamp", System.currentTimeMillis())
                    ))
                }
            }
        } catch (e: Exception) {
            Log.e("ChatVM", "Parse error: ${raw.take(100)}", e)
        }
    }

    fun setActiveChat(chatId: String) {
        messages.clear()
    }

    fun disconnect() {
        webSocket?.close(1000, "disconnect")
        webSocket = null
        connectionStatus.value = "disconnected"
    }
}
