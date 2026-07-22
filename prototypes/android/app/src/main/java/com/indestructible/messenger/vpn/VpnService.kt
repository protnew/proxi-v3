package com.indestructible.messenger.vpn

import android.content.Intent
import android.net.VpnService
import android.os.ParcelFileDescriptor

/**
 * VPN Service — routes traffic through WireGuard tunnel
 * Uses Android's built-in VpnService API
 */
class VpnService : VpnService() {

    private var vpnInterface: ParcelFileDescriptor? = null
    private var isRunning = false

    companion object {
        var status = "disconnected"
            private set
        var onStatusChange: ((String) -> Unit)? = null
    }

    fun startVpn(config: WireGuardConfig) {
        if (isRunning) return

        try {
            vpnInterface = Builder()
                .setSession("IndestructibleVPN")
                .addAddress(config.address, config.prefixLength)
                .addRoute("0.0.0.0", 0)  // Route all traffic
                .addDnsServer(config.dnsServer)
                .setBlocking(true)
                .setMtu(1420)
                .establish()

            if (vpnInterface == null) {
                status = "error"
                onStatusChange?.invoke(status)
                return
            }

            isRunning = true
            status = "connected"
            onStatusChange?.invoke(status)

            // TODO: Start WireGuard tunnel thread
            // WireGuard uses Go library (wireguard-android)
            // The tunnel processes packets from vpnInterface file descriptor

        } catch (e: Exception) {
            status = "error: ${e.message}"
            onStatusChange?.invoke(status)
        }
    }

    fun stopVpn() {
        try {
            vpnInterface?.close()
        } catch (_: Exception) {}
        vpnInterface = null
        isRunning = false
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
