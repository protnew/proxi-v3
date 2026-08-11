package com.indestructible.messenger

import androidx.test.ext.junit.runners.AndroidJUnit4
import okhttp3.*
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.RequestBody.Companion.toRequestBody
import org.json.JSONObject
import org.junit.Assert.*
import org.junit.Test
import org.junit.runner.RunWith
import java.util.concurrent.CountDownLatch
import java.util.concurrent.TimeUnit

@RunWith(AndroidJUnit4::class)
class WSIntegrationTest {

    private val client = OkHttpClient.Builder()
        .readTimeout(0, TimeUnit.MILLISECONDS)
        .pingInterval(30, TimeUnit.SECONDS)
        .connectTimeout(10, TimeUnit.SECONDS)
        .build()

    private val baseUrl = "http://10.0.2.2:8090"
    private val wsUrl = "ws://10.0.2.2:8090/ws"

    private fun getToken(npub: String): String {
        val mediaType = "application/json".toMediaType()
        val body = """{"npub":"$npub"}""".toRequestBody(mediaType)
        val req = Request.Builder().url("$baseUrl/api/auth/signup").post(body).build()
        val resp = client.newCall(req).execute()
        assertEquals(200, resp.code)
        return JSONObject(resp.body!!.string()).getString("access_token")
    }

    @Test
    fun ws_connectWithToken_succeeds() {
        val token = getToken("ws_conn_" + System.currentTimeMillis())
        val latch = CountDownLatch(1)
        var connected = false

        val req = Request.Builder().url("$wsUrl?token=$token").build()
        val ws = client.newWebSocket(req, object : WebSocketListener() {
            override fun onOpen(ws: WebSocket, response: Response) {
                connected = true
                latch.countDown()
            }
            override fun onFailure(ws: WebSocket, t: Throwable, response: Response?) {
                latch.countDown()
            }
        })
        latch.await(10, TimeUnit.SECONDS)
        assertTrue("WS should connect with valid JWT", connected)
        ws.close(1000, "done")
    }

    @Test
    fun ws_sendMessage_delivered() {
        val npubAlice = "alice_" + System.currentTimeMillis()
        val npubBob = "bob_" + System.currentTimeMillis()
        val tokenAlice = getToken(npubAlice)
        val tokenBob = getToken(npubBob)

        val latch = CountDownLatch(1)
        var receivedText: String? = null

        // Bob connects and listens — filter out system messages
        val reqB = Request.Builder().url("$wsUrl?token=$tokenBob").build()
        val wsBob = client.newWebSocket(reqB, object : WebSocketListener() {
            override fun onMessage(ws: WebSocket, text: String) {
                try {
                    val msg = JSONObject(text)
                    val msgText = msg.optString("text", "")
                    val msgType = msg.optString("type", "")
                    // Skip system messages (connected, join, etc)
                    if (msgText == "connected" || msgType == "join") return
                    if (msgText.isNotEmpty()) {
                        receivedText = msgText
                        latch.countDown()
                    }
                } catch (e: Exception) { /* ignore */ }
            }
        })

        Thread.sleep(2000)

        val reqA = Request.Builder().url("$wsUrl?token=$tokenAlice").build()
        val wsAlice = client.newWebSocket(reqA, object : WebSocketListener() {})
        Thread.sleep(1000)

        // Alice sends DM to Bob
        val msg = JSONObject()
            .put("type", "chat")
            .put("to", npubBob)
            .put("text", "Hello from Alice!")
            .put("ts", System.currentTimeMillis() / 1000)
            .toString()

        wsAlice.send(msg)

        latch.await(15, TimeUnit.SECONDS)

        assertNotNull("Bob should receive Alice's message (got: $receivedText)", receivedText)
        assertEquals("Hello from Alice!", receivedText)

        wsAlice.close(1000, "done")
        wsBob.close(1000, "done")
    }
}
