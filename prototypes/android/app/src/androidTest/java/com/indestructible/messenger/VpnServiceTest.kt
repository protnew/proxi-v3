package com.indestructible.messenger

import androidx.test.ext.junit.runners.AndroidJUnit4
import org.junit.Assert.*
import org.junit.Test
import org.junit.runner.RunWith
import com.indestructible.messenger.vpn.WireGuardConfig

@RunWith(AndroidJUnit4::class)
class VpnServiceTest {

    @Test
    fun vpn_config_defaultValues() {
        val config = WireGuardConfig()
        assertEquals("10.0.0.2", config.address)
        assertEquals(24, config.prefixLength)
        assertEquals("1.1.1.1", config.dnsServer)
        assertEquals("", config.endpoint)
        assertEquals("", config.publicKey)
        assertEquals("", config.privateKey)
    }

    @Test
    fun vpn_config_customValues() {
        val config = WireGuardConfig(
            address = "10.66.66.2",
            prefixLength = 32,
            dnsServer = "8.8.8.8",
            endpoint = "203.0.113.1:51820",
            publicKey = "xTxKpNp+RMfnPjz2x5T/v3kD8I2vAA==",
            privateKey = "aNp+RMfnPjz2x5T/v3kD8I2vAA=="
        )
        assertEquals("10.66.66.2", config.address)
        assertEquals(32, config.prefixLength)
        assertEquals("8.8.8.8", config.dnsServer)
        assertEquals("203.0.113.1:51820", config.endpoint)
        assertTrue(config.publicKey.isNotEmpty())
        assertTrue(config.privateKey.isNotEmpty())
    }

    @Test
    fun vpn_service_status_initial() {
        // Service starts as disconnected
        assertEquals("disconnected", com.indestructible.messenger.vpn.VpnService.status)
    }

    @Test
    fun vpn_service_hasCallbacks() {
        // onStatusChange callback should be null initially
        assertNull(com.indestructible.messenger.vpn.VpnService.onStatusChange)
    }

    @Test
    fun vpn_service_byteCounters() {
        assertEquals(0L, com.indestructible.messenger.vpn.VpnService.tunnelBytesIn)
        assertEquals(0L, com.indestructible.messenger.vpn.VpnService.tunnelBytesOut)
    }
}
