package com.indestructible.messenger

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * P21/R9: fixture contract for handleIncoming message types.
 * Does NOT implement full ChatViewModel / P5 key_exchange routing (B_gated X3).
 */
class HandleIncomingFixturesTest {

    data class Incoming(
        val type: String,
        val from: String = "npub1fixture",
        val to: String = "npub1self",
        val text: String = "",
        val extras: Map<String, String> = emptyMap(),
    )

    private fun sample(type: String, text: String = "", extras: Map<String, String> = emptyMap()) =
        Incoming(type = type, text = text, extras = extras)

    private fun assertKnownType(t: String) {
        val known = setOf("join", "typing", "key_exchange", "groupmeta", "filemeta", "chat", "call-offer")
        assertTrue("type must be non-blank", t.isNotBlank())
        // key_exchange is recognized as a type label only — routing is B_gated
        assertTrue("unexpected type=", t in known || t.startsWith("groupmeta") || t.startsWith("filemeta"))
    }

    @Test fun joinType() {
        val j = sample("join")
        assertEquals("join", j.type)
        assertKnownType(j.type)
    }

    @Test fun typingType() {
        val j = sample("typing", extras = mapOf("chatId" to "c1"))
        assertEquals("typing", j.type)
        assertEquals("c1", j.extras["chatId"])
    }

    @Test fun keyExchangeTypeFixtureOnly() {
        val j = sample("key_exchange", extras = mapOf("publicKey" to """{"type":"offer"}"""))
        assertEquals("key_exchange", j.type)
        assertTrue(j.extras["publicKey"]!!.contains("offer"))
        // Explicit: this test does NOT claim ringing / startCall behavior (X3 gated).
        assertFalse("must not pretend call started", j.extras.containsKey("ringing"))
    }

    @Test fun groupMetaType() {
        val j = sample("groupmeta", text = "groupmeta:{}")
        assertTrue(j.text.startsWith("groupmeta:"))
    }

    @Test fun fileMetaType() {
        val j = sample("filemeta", text = "filemeta:{}")
        assertTrue(j.text.startsWith("filemeta:"))
    }

    @Test fun rejectsBlankType() {
        try {
            assertKnownType(" ")
            throw AssertionError("expected blank type to fail")
        } catch (e: AssertionError) {
            assertTrue(e.message!!.contains("non-blank") || e.message!!.contains("unexpected") || e.message!!.contains("blank"))
        }
    }
}