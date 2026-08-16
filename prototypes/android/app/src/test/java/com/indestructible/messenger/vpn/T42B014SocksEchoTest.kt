package com.indestructible.messenger.vpn

import org.junit.Assert.*
import org.junit.Test
import java.io.DataInputStream
import java.io.DataOutputStream
import java.net.InetAddress
import java.net.ServerSocket
import java.net.Socket
import kotlin.concurrent.thread

/** T42B-014: echo must use T42BConfig.socksPort, not an ephemeral port. */
class T42B014SocksEchoTest {
    @Test
    fun hev_yaml_uses_same_port_as_config() {
        val cfg = T42BConfig()
        assertEquals(10808, cfg.socksPort)
        assertEquals("127.0.0.1", cfg.socksHost)
        val yaml = HevSocks5Engine.yamlConfig(cfg)
        assertTrue(yaml.contains("port: ${cfg.socksPort}"))
        assertTrue(yaml.contains("address: ${cfg.socksHost}"))
    }

    @Test
    fun echo_through_t42b_canon_port() {
        val cfg = T42BConfig()
        val backend = ServerSocket(0)
        val backendPort = backend.localPort
        val backendThread = thread(isDaemon = true) {
            backend.use { ss ->
                ss.accept().use { c ->
                    val buf = ByteArray(64)
                    val n = c.getInputStream().read(buf)
                    c.getOutputStream().write("echo:".toByteArray())
                    if (n > 0) c.getOutputStream().write(buf, 0, n)
                    c.getOutputStream().flush()
                }
            }
        }

        val proxy = try {
            ServerSocket(cfg.socksPort, 1, InetAddress.getByName(cfg.socksHost))
        } catch (e: Exception) {
            backend.close()
            throw AssertionError("canon port ${cfg.socksHost}:${cfg.socksPort} busy: $e")
        }
        val proxyThread = thread(isDaemon = true) {
            proxy.use { ss ->
                ss.accept().use { serveTinySocks5(it) }
            }
        }
        try {
            val sock = Socks5Client.connect(
                proxyHost = cfg.socksHost,
                proxyPort = cfg.socksPort,
                destHost = "127.0.0.1",
                destPort = backendPort,
                timeoutMs = 3000,
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

    private fun serveTinySocks5(client: Socket) {
        val inp = DataInputStream(client.getInputStream())
        val out = DataOutputStream(client.getOutputStream())
        inp.readByte()
        val nMethods = inp.readUnsignedByte()
        inp.skipBytes(nMethods)
        out.write(byteArrayOf(0x05, 0x00))
        out.flush()
        inp.readByte()
        inp.readByte()
        inp.readByte()
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
