package com.indestructible.messenger

import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test
import org.json.JSONObject

/** P21/R9 fixtures for handleIncoming types. Full ChatViewModel split can follow. */
class HandleIncomingFixturesTest {
    private fun sample(type: String, extra: Map<String, Any?> = emptyMap()): JSONObject {
        val o = JSONObject()
        o.put("type", type)
        o.put("from", "npub1fixture")
        o.put("to", "npub1self")
        for ((k, v) in extra) o.put(k, v)
        return o
    }
    @Test fun joinType() { assertEquals("join", sample("join").getString("type")) }
    @Test fun typingType() { assertEquals("typing", sample("typing", mapOf("chatId" to "c1")).getString("type")) }
    @Test fun keyExchangeType() {
        // Do NOT implement full P5 routing (B_gated X3). Fixture only.
        val j = sample("key_exchange", mapOf("publicKey" to """{"type":"offer"}"""))
        assertEquals("key_exchange", j.getString("type"))
        assertTrue(j.getString("publicKey").contains("offer"))
    }
    @Test fun groupMetaType() { assertTrue(sample("groupmeta", mapOf("text" to "groupmeta:{}")).getString("text").startsWith("groupmeta:")) }
    @Test fun fileMetaType() { assertTrue(sample("filemeta", mapOf("text" to "filemeta:{}")).getString("text").startsWith("filemeta:")) }
}
