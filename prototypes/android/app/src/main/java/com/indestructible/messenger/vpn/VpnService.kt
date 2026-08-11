package com.indestructible.messenger.vpn

import android.content.Intent
import android.net.VpnService as AndroidVpnService
import android.os.ParcelFileDescriptor
import android.util.Log
import java.io.FileInputStream
import java.io.FileOutputStream
import java.net.InetSocketAddress
import java.nio.ByteBuffer
import java.util.concurrent.atomic.AtomicBoolean

/**
 * VPN Service — routes traffic through WireGuard tunnel
 * MOB-101b: Packet processing loop — reads from VPN interface,
 * forwards to WireGuard tunnel, writes responses back.
 */
class VpnService : AndroidVpnService() {

    private var vpnInterface: ParcelFileDescriptor? = null
    private var tunnelThread: Thread? = null
    private val isRunning = AtomicBoolean(false)

    companion object {
        private const val TAG = "VpnService"
        var status = "disconnected"
            private set
        var onStatusChange: ((String) -> Unit)? = null
        var tunnelBytesIn: Long = 0
            private set
        var tunnelBytesOut: Long = 0
            private set
    }

    fun startVpn(config: WireGuardConfig) {
        if (isRunning.get()) return

        try {
            vpnInterface = Builder()
                .setSession("IndestructibleVPN")
                .addAddress(config.address, config.prefixLength)
                .addRoute("0.0.0.0", 0)
                .addDnsServer(config.dnsServer)
                .setBlocking(true)
                .setMtu(1420)
                .establish()

            if (vpnInterface == null) {
                status = "error: VPN interface null"
                onStatusChange?.invoke(status)
                return
            }

            isRunning.set(true)
            status = "connected"
            onStatusChange?.invoke(status)

            // MOB-101b: Start packet processing loop
            startTunnelLoop()

        } catch (e: Exception) {
            status = "error: ${e.message}"
            onStatusChange?.invoke(status)
            Log.e(TAG, "startVpn error", e)
        }
    }

    /**
     * Packet processing loop:
     * Read packets from VPN interface -> process/forward -> write back
     *
     * In production this connects to WireGuard Go library (wireguard-android).
     * For now: simple echo + byte counter to prove packets flow.
     */
    private fun startTunnelLoop() {
        tunnelThread = Thread {
            val pfd = vpnInterface ?: return@Thread
            val input = FileInputStream(pfd.fileDescriptor)
            val output = FileOutputStream(pfd.fileDescriptor)
            val buffer = ByteBuffer.allocate(32767)

            Log.i(TAG, "Tunnel loop started")
            while (isRunning.get() && !Thread.interrupted()) {
                try {
                    val length = input.read(buffer.array())
                    if (length > 0) {
                        tunnelBytesIn += length

                        // TODO: Replace with WireGuard Go library processing
                        // For now: packet is read and counted.
                        // Real WireGuard would:
                        //   1. Decrypt packet with session key
                        //   2. Send via UDP to peer endpoint
                        //   3. Receive response
                        //   4. Write response to output

                        // Simple pass-through for loopback test
                        buffer.limit(length)
                        output.write(buffer.array(), 0, length)
                        tunnelBytesOut += length
                        buffer.clear()
                    }
                } catch (e: Exception) {
                    if (isRunning.get()) {
                        Log.e(TAG, "Tunnel loop error", e)
                    }
                    break
                }
            }
            Log.i(TAG, "Tunnel loop stopped")
        }.also { it.start() }
    }

    fun stopVpn() {
        isRunning.set(false)
        tunnelThread?.interrupt()
        try {
            vpnInterface?.close()
        } catch (_: Exception) {}
        vpnInterface = null
        tunnelBytesIn = 0
        tunnelBytesOut = 0
        status = "disconnected"
        onStatusChange?.invoke(status)
    }

    override fun onDestroy() {
        stopVpn()
        super.onDestroy()
    }

    override fun onRevoke() {
        stopVpn()
        super.onRevoke()
    }
}

data class WireGuardConfig(
    val address: String = "10.0.0.2",
    val prefixLength: Int = 24,
    val dnsServer: String = "1.1.1.1",
    val endpoint: String = "",
    val publicKey: String = "",
    val privateKey: String = "",
)
