package com.indestructible.messenger.vpn

/**
 * T42-B control-plane config: system VPN = default route 0.0.0.0/0.
 * Pure Kotlin — unit-tested on JVM, no emulator.
 */
data class T42BConfig(
    val tunAddress: String = "10.8.0.2",
    val tunPrefix: Int = 24,
    val defaultRoute: String = "0.0.0.0/0",
    val dns: String = "1.1.1.1",
    val session: String = "IndestructibleVPN",
    val socksHost: String = "127.0.0.1",
    val socksPort: Int = 10808,
    val killSwitch: Boolean = true,
) {
    fun isSystemVpn(): Boolean = defaultRoute == "0.0.0.0/0"

    fun validate(): List<String> {
        val errors = mutableListOf<String>()
        if (defaultRoute != "0.0.0.0/0") {
            errors.add("defaultRoute must be 0.0.0.0/0 for system VPN")
        }
        if (socksPort <= 0 || socksHost.isBlank()) {
            errors.add("socks host:port required (tun2socks exit)")
        }
        if (session.isBlank()) {
            errors.add("session name required")
        }
        return errors
    }
}
