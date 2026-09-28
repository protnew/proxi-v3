package com.indestructible.messenger.vpn

import org.junit.Assert.*
import org.junit.Test

class T42BConfigTest {
    @Test
    fun default_is_system_vpn_full_tunnel() {
        val cfg = T42BConfig()
        assertEquals("0.0.0.0/0", cfg.defaultRoute)
        assertTrue(cfg.isSystemVpn())
        assertTrue(cfg.killSwitch)
        assertEquals("IndestructibleVPN", cfg.session)
        assertTrue(cfg.validate().isEmpty())
    }

    @Test
    fun rejects_partial_route() {
        val cfg = T42BConfig(defaultRoute = "10.0.0.0/8")
        assertFalse(cfg.isSystemVpn())
        assertTrue(cfg.validate().any { it.contains("0.0.0.0/0") })
    }

    @Test
    fun rejects_missing_socks() {
        val cfg = T42BConfig(socksPort = 0)
        assertTrue(cfg.validate().any { it.contains("socks") })
    }
}
