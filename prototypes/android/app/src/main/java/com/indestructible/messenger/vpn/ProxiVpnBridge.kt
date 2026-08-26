package com.indestructible.messenger.vpn

import android.content.Context
import android.content.Intent
import android.webkit.JavascriptInterface

/**
 * T42B-016: JS bridge — startVpn/stopVpn from the WebView shell.
 * Uses Android's official @JavascriptInterface (no external deps).
 *
 * Register from MainActivity:
 *   webView.addJavascriptInterface(ProxiVpnBridge(this), "ProxiVpn")
 *
 * JS usage:
 *   ProxiVpn.startVpn()  // → {"ok":true,"running":true}
 *   ProxiVpn.stopVpn()
 *   ProxiVpn.isRunning()
 */
class ProxiVpnBridge(private val ctx: Context) {

    @JavascriptInterface
    fun startVpn(): String {
        return try {
            val intent = Intent(ctx, TunVpnService::class.java)
            intent.action = "START_VPN"
            ctx.startService(intent)
            running = true
            """{"ok":true,"running":true}"""
        } catch (e: Exception) {
            """{"ok":false,"error":"${e.message}"}"""
        }
    }

    @JavascriptInterface
    fun stopVpn(): String {
        return try {
            val intent = Intent(ctx, TunVpnService::class.java)
            intent.action = "STOP_VPN"
            ctx.startService(intent)
            running = false
            """{"ok":true,"running":false}"""
        } catch (e: Exception) {
            """{"ok":false,"error":"${e.message}"}"""
        }
    }

    @JavascriptInterface
    fun isRunning(): String = """{"running":$running}"""

    companion object {
        @Volatile
        var running: Boolean = false
            private set
    }
}
