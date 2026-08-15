package com.indestructible.messenger.vpn

import java.util.concurrent.atomic.AtomicBoolean

class ChannelDownException(msg: String) : RuntimeException(msg)

/**
 * Packet-level kill switch for T42-B.
 * When the DataChannel is down, no packet may leave toward the operator network.
 */
class PacketGate(val killSwitch: Boolean = true) {
    private val channelUp = AtomicBoolean(false)

    fun setChannel(up: Boolean) {
        channelUp.set(up)
    }

    fun isOpen(): Boolean = channelUp.get()

    fun checkOrThrow() {
        if (!channelUp.get()) {
            throw ChannelDownException("kill switch: channel down")
        }
    }
}
