package com.indestructible.messenger.vpn

import org.junit.Assert.*
import org.junit.Test

class PacketGateTest {
    @Test
    fun closed_by_default() {
        val g = PacketGate()
        assertFalse(g.isOpen())
    }

    @Test
    fun open_when_channel_up() {
        val g = PacketGate()
        g.setChannel(true)
        assertTrue(g.isOpen())
    }

    @Test
    fun kill_switch_blocks_when_channel_drops() {
        val g = PacketGate(killSwitch = true)
        g.setChannel(true)
        g.setChannel(false)
        assertFalse(g.isOpen())
        try {
            g.checkOrThrow()
            fail("expected ChannelDownException")
        } catch (e: ChannelDownException) {
            assertTrue(e.message!!.contains("kill switch"))
        }
    }

    @Test
    fun kill_switch_off_still_blocks_dial_api_when_down() {
        // even if OS kill-switch disabled, button Dial must not leak
        val g = PacketGate(killSwitch = false)
        g.setChannel(false)
        assertFalse(g.isOpen())
    }
}
