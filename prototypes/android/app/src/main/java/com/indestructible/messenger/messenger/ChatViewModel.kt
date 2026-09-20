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
    // P24: held for E2E only, never sent anywhere.
    private var myPrivKeyHex: String = ""

    fun setPrivateKey(hex: String) { myPrivKeyHex = hex }

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

    fun debugPubKey(): String = myPubKey

    fun sendMessage(from: String, to: String, text: String) {
        val msg = Message(
            id = "msg_${System.currentTimeMillis()}",
            from = from, to = to, text = text,
            timestamp = System.currentTimeMillis()
        )
        messages.add(msg)

        // P24 (2026-09-20): E2E by default for DMs when a key is set —
        // real NIP-44 v2, wire-compatible with PWA nostr-tools and Go.
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
                return // fail closed: never downgrade to plaintext
            }
        }
        val json = JSONObject().apply {
            put("type", "chat")
            put("to", to)
            put("text", payload)
            if (isE2E) { put("encrypted", true); put("is_e2e", true) }
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

