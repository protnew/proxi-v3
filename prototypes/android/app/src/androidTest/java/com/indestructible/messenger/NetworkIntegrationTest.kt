package com.indestructible.messenger

import androidx.test.ext.junit.runners.AndroidJUnit4
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.RequestBody.Companion.toRequestBody
import org.json.JSONObject
import org.junit.Assert.*
import org.junit.Test
import org.junit.runner.RunWith
import java.util.concurrent.TimeUnit

@RunWith(AndroidJUnit4::class)
class NetworkIntegrationTest {

    private val client = OkHttpClient.Builder()
        .connectTimeout(10, TimeUnit.SECONDS)
        .readTimeout(10, TimeUnit.SECONDS)
        .build()

    private val baseUrl = "http://10.0.2.2:8090"

    /** INET-001: Go server health check from emulator */
    @Test
    fun server_healthCheck_returns200() {
        val req = Request.Builder().url("$baseUrl/api/health").build()
        val resp = client.newCall(req).execute()
        assertEquals("Health check failed", 200, resp.code)
        val body = resp.body?.string() ?: ""
        assertTrue("Health body should not be empty", body.isNotEmpty())
    }

    /** INET-002: Signup returns JWT token */
    @Test
    fun auth_signup_returnsJwtToken() {
        val mediaType = "application/json; charset=utf-8".toMediaType()
        val npub = "test_npub_" + System.currentTimeMillis()
        val body = """{"npub":"$npub"}""".toRequestBody(mediaType)

        val req = Request.Builder()
            .url("$baseUrl/api/auth/signup")
            .post(body)
            .build()
        val resp = client.newCall(req).execute()
        assertEquals("Signup should return 200", 200, resp.code)

        val respBody = resp.body?.string() ?: ""
        val json = JSONObject(respBody)
        assertTrue("Should have access_token", json.has("access_token"))
        assertTrue("Token not empty", json.getString("access_token").isNotEmpty())
        assertTrue("Should have user_id", json.has("user_id"))
    }

    /** INET-003: WS without token returns 401 */
    @Test
    fun ws_withoutToken_returns401() {
        val req = Request.Builder().url("$baseUrl/ws").build()
        val resp = client.newCall(req).execute()
        assertEquals("WS without token should be 401", 401, resp.code)
    }
}
