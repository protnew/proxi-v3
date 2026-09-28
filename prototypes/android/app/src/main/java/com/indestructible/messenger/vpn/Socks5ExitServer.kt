package com.indestructible.messenger.vpn

import java.io.InputStream
import java.io.OutputStream
import java.net.InetAddress
import java.net.InetSocketAddress
import java.net.ServerSocket
import java.net.Socket
import java.util.concurrent.ConcurrentHashMap
import java.util.concurrent.atomic.AtomicBoolean
import java.util.concurrent.atomic.AtomicLong

/**
 * T42-B exit node: SOCKS5 server (no-auth, CONNECT only) on the phone acting as exit.
 * The client phone points tun2socks at this host:port; traffic exits via this device's network.
 */
object Socks5ExitServer {

    private var server: ServerSocket? = null
    private var acceptThread: Thread? = null
    private val running = AtomicBoolean(false)
    private val sessions = ConcurrentHashMap<Int, Socket>()
    val activeSessions = AtomicLong(0)
    val bytesRelayed = AtomicLong(0)

    var bindHost: String = "0.0.0.0"
    var bindPort: Int = 10808

    fun isRunning(): Boolean = running.get()

    fun start(host: String = bindHost, port: Int = bindPort): Boolean {
        if (running.get()) return true
        return try {
            val ss = ServerSocket()
            ss.reuseAddress = true
            ss.bind(InetSocketAddress(InetAddress.getByName(host), port), 16)
            server = ss
            running.set(true)
            acceptThread = Thread({ acceptLoop(ss) }, "socks5-exit-accept").apply {
                isDaemon = true
                start()
            }
            true
        } catch (e: Exception) {
            server = null
            running.set(false)
            false
        }
    }

    fun stop() {
        running.set(false)
        try { server?.close() } catch (_: Exception) {}
        server = null
        acceptThread?.interrupt()
        acceptThread = null
        for ((_, s) in sessions) {
            try { s.close() } catch (_: Exception) {}
        }
        sessions.clear()
        activeSessions.set(0)
    }

    private fun acceptLoop(ss: ServerSocket) {
        while (running.get()) {
            val client = try {
                ss.accept()
            } catch (e: Exception) {
                if (running.get()) continue else break
            }
            Thread({ serve(client) }, "socks5-exit-session").apply {
                isDaemon = true
                start()
            }
        }
    }

    private fun serve(client: Socket) {
        val id = System.identityHashCode(client)
        sessions[id] = client
        activeSessions.incrementAndGet()
        try {
            client.soTimeout = 15000
            val input = client.getInputStream()
            val output = client.getOutputStream()

            if (!handshake(input, output)) return
            val request = readRequest(input, output) ?: return

            val remote = try {
                Socket()
            } catch (e: Exception) {
                writeReply(output, 0x01)
                return
            }
            try {
                remote.tcpNoDelay = true
                remote.connect(InetSocketAddress(request.host, request.port), 10000)
                writeReply(output, 0x00)
            } catch (e: Exception) {
                writeReply(output, 0x05)
                remote.close()
                return
            }

            relay(client, remote)
        } catch (_: Exception) {
        } finally {
            sessions.remove(id)
            activeSessions.decrementAndGet()
            try { client.close() } catch (_: Exception) {}
        }
    }

    private fun handshake(input: InputStream, output: OutputStream): Boolean {
        val header = try {
            input.readNBytesCompat(2)
        } catch (e: Exception) {
            return false
        }
        if (header.size < 2 || header[0] != 0x05.toByte()) return false
        val nMethods = header[1].toInt() and 0xFF
        val methods = try {
            input.readNBytesCompat(nMethods)
        } catch (e: Exception) {
            return false
        }
        if (methods.isEmpty()) return false
        val noAuth = methods.contains(0x00.toByte())
        output.write(byteArrayOf(0x05, if (noAuth) 0x00 else 0xFF.toByte()))
        output.flush()
        return noAuth
    }

    private fun readRequest(input: InputStream, output: OutputStream): SocksRequest? {
        val head = try {
            input.readNBytesCompat(4)
        } catch (e: Exception) {
            return null
        }
        if (head.size < 4 || head[1] != 0x01.toByte()) return null
        val host: String
        when (head[3]) {
            0x01.toByte() -> {
                val b = input.readNBytesCompat(4)
                if (b.size < 4) return null
                host = "${b[0].toInt() and 0xFF}.${b[1].toInt() and 0xFF}.${b[2].toInt() and 0xFF}.${b[3].toInt() and 0xFF}"
            }
            0x03.toByte() -> {
                val len = input.readNBytesCompat(1)
                if (len.isEmpty()) return null
                val name = input.readNBytesCompat(len[0].toInt() and 0xFF)
                if (name.size != (len[0].toInt() and 0xFF)) return null
                host = String(name, Charsets.US_ASCII)
            }
            0x04.toByte() -> {
                val b = input.readNBytesCompat(16)
                if (b.size < 16) return null
                host = ipv6ToString(b)
            }
            else -> return null
        }
        val portBytes = input.readNBytesCompat(2)
        if (portBytes.size < 2) return null
        val port = ((portBytes[0].toInt() and 0xFF) shl 8) or (portBytes[1].toInt() and 0xFF)
        return SocksRequest(host, port)
    }

    private fun writeReply(output: OutputStream, statusByte: Int) {
        val reply = byteArrayOf(
            0x05, statusByte.toByte(), 0x00, 0x01,
            0, 0, 0, 0,
            0, 0
        )
        output.write(reply)
        output.flush()
    }

    private fun relay(client: Socket, remote: Socket) {
        val up = Thread({
            pump(client.getInputStream(), remote.getOutputStream())
            try { remote.shutdownOutput() } catch (_: Exception) {}
        }, "socks5-exit-up").apply { isDaemon = true }
        val down = Thread({
            pump(remote.getInputStream(), client.getOutputStream())
            try { client.shutdownOutput() } catch (_: Exception) {}
        }, "socks5-exit-down").apply { isDaemon = true }
        up.start()
        down.start()
        up.join()
        down.join()
    }

    private fun pump(from: InputStream, to: OutputStream) {
        val buf = ByteArray(16 * 1024)
        try {
            while (true) {
                val n = from.read(buf)
                if (n < 0) break
                if (n == 0) continue
                to.write(buf, 0, n)
                to.flush()
                bytesRelayed.addAndGet(n.toLong())
            }
        } catch (_: Exception) {
        }
    }

    private fun ipv6ToString(b: ByteArray): String {
        val sb = StringBuilder()
        for (i in 0 until 16 step 2) {
            if (i > 0) sb.append(':')
            sb.append(String.format("%x", ((b[i].toInt() and 0xFF) shl 8) or (b[i + 1].toInt() and 0xFF)))
        }
        return sb.toString()
    }

    private data class SocksRequest(val host: String, val port: Int)
}

private fun InputStream.readNBytesCompat(n: Int): ByteArray {
    val out = ByteArray(n)
    var off = 0
    while (off < n) {
        val read = this.read(out, off, n - off)
        if (read < 0) break
        off += read
    }
    return if (off == n) out else out.copyOf(off)
}
