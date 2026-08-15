package com.indestructible.messenger.vpn

import java.io.DataInputStream
import java.io.DataOutputStream
import java.net.InetSocketAddress
import java.net.Socket

/**
 * SOCKS5 CONNECT client (no-auth). Used by TUN→tun2socks path and by JVM tests
 * against an in-process mock — no human peer channel required.
 */
object Socks5Client {
    fun connect(
        proxyHost: String,
        proxyPort: Int,
        destHost: String,
        destPort: Int,
        timeoutMs: Int = 8000
    ): Socket {
        val sock = Socket()
        sock.connect(InetSocketAddress(proxyHost, proxyPort), timeoutMs)
        sock.soTimeout = timeoutMs
        try {
            val out = DataOutputStream(sock.getOutputStream())
            val inp = DataInputStream(sock.getInputStream())
            out.write(byteArrayOf(0x05, 0x01, 0x00))
            out.flush()
            val greet = ByteArray(2)
            inp.readFully(greet)
            if (greet[0] != 0x05.toByte() || greet[1] != 0x00.toByte()) {
                throw IllegalStateException("socks5 auth rejected")
            }
            val hostBytes = destHost.toByteArray(Charsets.US_ASCII)
            if (hostBytes.size > 255) throw IllegalArgumentException("host too long")
            out.write(byteArrayOf(0x05, 0x01, 0x00, 0x03, hostBytes.size.toByte()))
            out.write(hostBytes)
            out.writeByte((destPort shr 8) and 0xFF)
            out.writeByte(destPort and 0xFF)
            out.flush()
            val hdr = ByteArray(4)
            inp.readFully(hdr)
            if (hdr[1] != 0x00.toByte()) {
                throw IllegalStateException("socks5 connect failed code=${hdr[1].toInt() and 0xFF}")
            }
            when (hdr[3].toInt() and 0xFF) {
                0x01 -> inp.skipBytes(4 + 2)
                0x03 -> {
                    val n = inp.readUnsignedByte()
                    inp.skipBytes(n + 2)
                }
                0x04 -> inp.skipBytes(16 + 2)
                else -> { /* leave socket; handshake already succeeded or not */ }
            }
            sock.soTimeout = 0
            return sock
        } catch (e: Exception) {
            try { sock.close() } catch (_: Exception) {}
            throw e
        }
    }

}
