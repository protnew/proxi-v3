package com.indestructible.messenger.nostr

import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.Response
import okhttp3.WebSocket
import okhttp3.WebSocketListener
import com.fasterxml.jackson.module.kotlin.jacksonObjectMapper
import com.fasterxml.jackson.module.kotlin.readValue
import java.util.concurrent.CopyOnWriteArrayList

/**
 * Nostr Relay Connection — WebSocket client for Android
 */
class NostrRelay(
    private val url: String,
    private val client: OkHttpClient = OkHttpClient()
) {
    private var ws: WebSocket? = null
    private val listeners = CopyOnWriteArrayList<(NostrEvent) -> Unit>()
    private val mapper = jacksonObjectMapper()

    fun connect() {
        val request = Request.Builder().url(url).build()
        ws = client.newWebSocket(request, object : WebSocketListener() {
            override fun onOpen(webSocket: WebSocket, response: Response) {
                println("[Nostr] Connected to $url")
            }

            override fun onMessage(webSocket: WebSocket, text: String) {
                try {
                    val msg = mapper.readValue<List<Any>>(text)
                    if (msg.isNotEmpty() && msg[0] == "EVENT" && msg.size >= 3) {
                        val event = mapper.convertValue(msg[2], NostrEvent::class.java)
                        listeners.forEach { it(event) }
                    }
                } catch (e: Exception) {
                    // Parse error or OK — ignore
                }
            }

            override fun onFailure(webSocket: WebSocket, t: Throwable, response: Response?) {
                println("[Nostr] Connection failure: ${t.message}")
            }

            override fun onClosed(webSocket: WebSocket, code: Int, reason: String) {
                println("[Nostr] Disconnected from $url")
            }
        })
    }

    fun subscribe(filter: NostrFilter) {
        val msg = mapper.writeValueAsString(listOf("REQ", "sub-${System.currentTimeMillis()}", filter))
        ws?.send(msg)
    }

    fun publish(event: NostrEvent) {
        val msg = mapper.writeValueAsString(listOf("EVENT", event))
        ws?.send(msg)
    }

    fun onEvent(listener: (NostrEvent) -> Unit) {
        listeners.add(listener)
    }

    fun disconnect() {
        ws?.close(1000, "Goodbye")
    }
}

// Data classes for Nostr protocol
data class NostrEvent(
    val id: String = "",
    val pubkey: String = "",
    val created_at: Long = 0,
    val kind: Int = 0,
    val tags: List<List<String>> = emptyList(),
    val content: String = "",
    val sig: String = ""
)

data class NostrFilter(
    val kinds: List<Int>? = null,
    val pTags: List<String>? = null,
    val eTags: List<String>? = null,
    val limit: Int? = null,
    val since: Long? = null
) {
    fun toJson(): Map<String, Any?> = mutableMapOf<String, Any?>().apply {
        kinds?.let { put("kinds", it) }
        pTags?.let { put("#p", it) }
        eTags?.let { put("#e", it) }
        limit?.let { put("limit", it) }
        since?.let { put("since", it) }
    }
}
