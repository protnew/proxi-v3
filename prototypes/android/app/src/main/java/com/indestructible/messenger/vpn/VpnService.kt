package com.indestructible.messenger.vpn

import android.content.Intent
import android.net.VpnService as AndroidVpnService
import android.os.ParcelFileDescriptor
import android.util.Log
import com.wireguard.android.backend.GoBackend
import com.wireguard.android.backend.Tunnel
import com.wireguard.config.Config
import com.wireguard.config.InetEndpoint
import com.wireguard.config.InetNetwork
import com.wireguard.config.Peer
import com.wireguard.config.Interface
import com.wireguard.crypto.Key
import com.wireguard.crypto.KeyPair
import java.util.concurrent.atomic.AtomicBoolean

/**
 * VPN Service — real WireGuard tunnel via com.wireguard.android:tunnel
 * MOB-101b v2: Uses GoBackend (wireguard-go userspace).
 */
class VpnService : AndroidVpnService() {

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

    private var backend: GoBackend? = null
    private var tunnel: Tunnel? = null
    private val isRunning = AtomicBoolean(false)

    fun startVpn(config: WireGuardConfig) {
        if (isRunning.get()) return

        try {
            if (backend == null) {
                backend = GoBackend(this)
            }

            // Build interface
            val privKeyStr = config.privateKey.ifEmpty { KeyPair().privateKey.toBase64() }
            val ifaceBuilder = Interface.Builder()
            ifaceBuilder.parsePrivateKey(privKeyStr)
            ifaceBuilder.addAddress(InetNetwork.parse(config.address + "/" + config.prefixLength))
            ifaceBuilder.parseDnsServers(config.dnsServer)
            ifaceBuilder.setMtu(1420)

            val configBuilder = Config.Builder()
            configBuilder.setInterface(ifaceBuilder.build())

            // Add peer if endpoint configured
            if (config.endpoint.isNotEmpty() && config.publicKey.isNotEmpty()) {
                val peerBuilder = Peer.Builder()
                peerBuilder.parsePublicKey(config.publicKey)
                peerBuilder.parseEndpoint(config.endpoint)
                peerBuilder.parseAllowedIPs("0.0.0.0/0")
                configBuilder.addPeer(peerBuilder.build())
            }

            val wgConfig = configBuilder.build()

            // Create tunnel
            tunnel = object : Tunnel {
                override fun getName(): String = "IndestructibleVPN"
                override fun onStateChange(state: Tunnel.State) {
                    Log.i(TAG, "Tunnel state: $state")
                    if (state == Tunnel.State.UP) {
                        status = "connected"
                    } else if (state == Tunnel.State.DOWN) {
                        status = "disconnected"
                    }
                    onStatusChange?.invoke(status)
                }
            }

            val state = backend!!.setState(tunnel!!, Tunnel.State.UP, wgConfig)

            if (state == Tunnel.State.UP) {
                isRunning.set(true)
                status = "connected"
                onStatusChange?.invoke(status)
                Log.i(TAG, "WireGuard tunnel UP")
            } else {
                status = "error: state=$state"
                onStatusChange?.invoke(status)
            }

        } catch (e: Exception) {
            status = "error: ${e.message}"
            onStatusChange?.invoke(status)
            Log.e(TAG, "startVpn error", e)
        }
    }

    fun stopVpn() {
        if (!isRunning.get()) return
        try {
            tunnel?.let { t ->
                backend?.setState(t, Tunnel.State.DOWN, null)
            }
        } catch (e: Exception) {
            Log.e(TAG, "stopVpn error", e)
        }
        isRunning.set(false)
        tunnel = null
        status = "disconnected"
        onStatusChange?.invoke(status)
        Log.i(TAG, "WireGuard tunnel DOWN")
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
