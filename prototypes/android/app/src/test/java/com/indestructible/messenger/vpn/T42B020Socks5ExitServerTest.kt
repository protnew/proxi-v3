package com.indestructible.messenger.vpn

import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test
import java.io.OutputStream
import java.net.InetAddress
import java.net.ServerSocket
import java.net.Socket
import java.nio.charset.StandardCharsets
import java.util.concurrent.CountDownLatch
import java.util.concurrent.TimeUnit

/**
 * T42B-020: Socks5ExitServer serves Socks5Client end-to-end on JVM.
 * Echo backend behind the exit proves the full CONNECT chain.
 */
class T42B020Socks5ExitServerTest {

    @Test
    fun exitServerAcceptsSocks5ClientConnectAndRelays() {
        val backend = ServerSocket(0, 1, InetAddress.getByName("127.0.0.1"))
        val payload = "T42B-020-exit-echo"
        val latch = CountDownLatch(1)
        var received: String? = null

        Thread({
            backend.accept().use { s ->
                s.tcpNoDelay = true
                val buf = ByteArray(256)
                val n = s.getInputStream().read(buf)
                received = String(buf, 0, n, StandardCharsets.UTF_8)
                val out: OutputStream = s.getOutputStream()
                out.write(payload.toByteArray(StandardCharsets.UTF_8))
                out.flush()
                latch.countDown()
            }
        }, "t42b020-backend").apply { isDaemon = true }.start()

        val exitPort = 11808
        assertTrue(Socks5ExitServer.start("127.0.0.1", exitPort))
        try {
            val sock = Socks5Client.connect(
                proxyHost = "127.0.0.1",
                proxyPort = exitPort,
                destHost = "127.0.0.1",
                destPort = backend.localPort,
                timeoutMs = 5000
            )
            sock.use { s ->
                s.tcpNoDelay = true
                val out: OutputStream = s.getOutputStream()
                out.write(payload.toByteArray(StandardCharsets.UTF_8))
                out.flush()
                val buf = ByteArray(256)
                val n = s.getInputStream().read(buf)
                assertEquals(payload, String(buf, 0, n, StandardCharsets.UTF_8))
            }
            assertTrue(latch.await(5, TimeUnit.SECONDS))
            assertEquals(payload, received)
            assertTrue(Socks5ExitServer.bytesRelayed.get() >= payload.length * 2)
        } finally {
            Socks5ExitServer.stop()
            backend.close()
        }
    }

    @Test
    fun stopIsIdempotentAndSecondStartWorks() {
        Socks5ExitServer.stop()
        val port = 11809
        assertTrue(Socks5ExitServer.start("127.0.0.1", port))
        assertTrue(Socks5ExitServer.isRunning())
        Socks5ExitServer.stop()
        assertEquals(false, Socks5ExitServer.isRunning())
        Socks5ExitServer.stop()
        assertTrue(Socks5ExitServer.start("127.0.0.1", port))
        Socks5ExitServer.stop()
    }

    @Test
    fun doubleStartOnSamePortDoesNotCrash() {
        assertTrue(Socks5ExitServer.start("127.0.0.1", 11810))
        val second = Socket()
        try {
            assertTrue(Socks5ExitServer.isRunning())
        } finally {
            second.close()
            Socks5ExitServer.stop()
        }
    }
}
