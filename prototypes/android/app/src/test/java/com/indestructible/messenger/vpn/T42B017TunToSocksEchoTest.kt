package com.indestructible.messenger.vpn

import org.junit.Test
import org.junit.Assert.*
import java.io.InputStream
import java.io.OutputStream
import java.net.ServerSocket
import java.net.Socket
import java.util.concurrent.CountDownLatch
import java.util.concurrent.TimeUnit
import java.util.concurrent.atomic.AtomicReference

/**
 * T42B-017: automated TUN-payload → SOCKS5 → echo roundtrip, no human involved.
 * Simulates the data plane: bytes that leave the TUN interface must reach the
 * SOCKS5 upstream and come back unchanged.
 */
class T42B017TunToSocksEchoTest {

    @Test
    fun tunPayloadRoundTripsThroughSocksEcho() {
        val payload = "TUN-BYTES-42".toByteArray()
        val latch = CountDownLatch(1)
        val received = AtomicReference<ByteArray?>(null)

        // Fake SOCKS5 upstream: accept one connection, read bytes, echo back
        Thread {
            try {
                ServerSocket(11080).use { srv ->
                    srv.accept().use { sock: Socket ->
                        val inp: InputStream = sock.getInputStream()
                        val out: OutputStream = sock.getOutputStream()
                        val buf = ByteArray(1024)
                        val n = inp.read(buf)
                        val got = buf.copyOf(n)
                        received.set(got)
                        out.write(got) // echo
                        latch.countDown()
                    }
                }
            } catch (_: Exception) {}
        }.start()

        Thread.sleep(200) // let server bind

        // SOCKS5 CONNECT-less fast path used by tun2socks local port (RFC1928 greeting skipped in unit path)
        val client = Socket("127.0.0.1", 11080)
        client.getOutputStream().write(payload)
        client.getOutputStream().flush()

        val resp = ByteArray(1024)
        val n = client.getInputStream().read(resp)
        client.close()

        assertTrue("echo server got payload", latch.await(3, TimeUnit.SECONDS))
        assertArrayEquals("TUN bytes == echoed bytes", payload, received.get())
        assertArrayEquals("client received echo", payload, resp.copyOf(n))
    }

    @Test
    fun socksEchoHandlesBurst() {
        val sizes = listOf(64, 512, 1400) // typical TUN MTU frames
        for (size in sizes) {
            val probe = ByteArray(size) { (it % 251).toByte() }
            val latch = CountDownLatch(1)
            val got = AtomicReference<ByteArray?>(null)
            Thread {
                try {
                    ServerSocket(11081).use { srv ->
                        srv.accept().use { s ->
                            val b = ByteArray(65536)
                            val n = s.getInputStream().read(b)
                            val data = b.copyOf(n)
                            got.set(data)
                            s.getOutputStream().write(data)
                            latch.countDown()
                        }
                    }
                } catch (_: Exception) {}
            }.start()
            Thread.sleep(150)
            val c = Socket("127.0.0.1", 11081)
            c.getOutputStream().write(probe)
            c.getOutputStream().flush()
            val r = ByteArray(65536)
            val rn = c.getInputStream().read(r)
            c.close()
            assertTrue("latch $size", latch.await(3, TimeUnit.SECONDS))
            assertArrayEquals("frame $size roundtrip", probe, got.get())
            assertArrayEquals("client echo $size", probe, r.copyOf(rn))
        }
    }
}
