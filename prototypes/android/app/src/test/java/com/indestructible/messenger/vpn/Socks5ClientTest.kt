package com.indestructible.messenger.vpn

import org.junit.Assert.*
import org.junit.Test
import java.io.DataInputStream
import java.io.DataOutputStream
import java.net.InetAddress
import java.net.ServerSocket
import java.net.Socket
import kotlin.concurrent.thread

class Socks5ClientTest {
    @Test
    fun connect_through_in_process_socks_and_echo() {
        val backend = ServerSocket(0)
        val backendPort = backend.localPort
        val backendThread = thread(isDaemon = true) {
            backend.use { ss ->
                val c = ss.accept()
                c.use {
                    val buf = ByteArray(64)
                    val n = it.getInputStream().read(buf)
                    it.getOutputStream().write("echo:".toByteArray())
                    if (n > 0) it.getOutputStream().write(buf, 0, n)
                    it.getOutputStream().flush()
                }
            }
        }

        val proxy = ServerSocket(0)
        val proxyPort = proxy.localPort
        val proxyThread = thread(isDaemon = true) {
            proxy.use { ss ->
                val client = ss.accept()
                client.use { serveTinySocks5(it) }
            }
        }

        try {
            val sock = Socks5Client.connect(
                proxyHost = "127.0.0.1",
                proxyPort = proxyPort,
                destHost = "127.0.0.1",
                destPort = backendPort,
                timeoutMs = 3000
            )
            sock.getOutputStream().write("ping".toByteArray())
            sock.getOutputStream().flush()
            val got = sock.getInputStream().readBytes().toString(Charsets.UTF_8)
            sock.close()
            assertEquals("echo:ping", got)
        } finally {
            backendThread.join(2000)
            proxyThread.join(2000)
        }
    }

    /** Minimal SOCKS5 CONNECT (no-auth) for the unit test only. */
    private fun serveTinySocks5(client: Socket) {
        val inp = DataInputStream(client.getInputStream())
        val out = DataOutputStream(client.getOutputStream())
        inp.readByte() // ver
        val nMethods = inp.readUnsignedByte()
        inp.skipBytes(nMethods)
        out.write(byteArrayOf(0x05, 0x00))
        out.flush()
        inp.readByte() // ver
        inp.readByte() // cmd
        inp.readByte() // rsv
        val atyp = inp.readUnsignedByte()
        val host = when (atyp) {
            0x01 -> InetAddress.getByAddress(inp.readNBytes(4)).hostAddress
            0x03 -> {
                val len = inp.readUnsignedByte()
                String(inp.readNBytes(len))
            }
            else -> throw IllegalStateException("atyp=$atyp")
        }
        val port = inp.readUnsignedShort()
        val remote = Socket(host, port)
        out.write(byteArrayOf(0x05, 0x00, 0x00, 0x01, 0, 0, 0, 0, 0, 0))
        out.flush()
        val t1 = thread(isDaemon = true) {
            try { client.getInputStream().copyTo(remote.getOutputStream()) } catch (_: Exception) {}
        }
        try { remote.getInputStream().copyTo(client.getOutputStream()) } catch (_: Exception) {}
        t1.join(1000)
        try { remote.close() } catch (_: Exception) {}
    }
}
