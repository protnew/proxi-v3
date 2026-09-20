package com.indestructible.messenger.vpn

import android.app.Notification
import android.app.PendingIntent
import android.content.Intent
import android.net.VpnService as AndroidVpnService
import android.os.ParcelFileDescriptor
import android.util.Log
import java.io.FileInputStream
import java.io.FileOutputStream
import java.net.Socket
import java.nio.ByteBuffer
import java.util.concurrent.ConcurrentHashMap
import java.util.concurrent.atomic.AtomicBoolean
import java.util.concurrent.atomic.AtomicLong

/**
 * TUN VPN Service — captures ALL device traffic via Android VpnService API.
 *
 * Creates a TUN interface, reads raw IP packets, and forwards TCP connections
 * through a tunnel (WebRTC DataChannel in P2P mode, or direct proxy).
 *
 * This is a REAL VPN: all app traffic is captured by the TUN interface.
 */
class TunVpnService : AndroidVpnService() {
    private val NOTIF_ID = 42
    private fun buildNotification(): Notification {
        val channelId = "proxi_vpn"
        val nm = getSystemService(NOTIFICATION_SERVICE) as android.app.NotificationManager
        if (android.os.Build.VERSION.SDK_INT >= 26) {
            nm.createNotificationChannel(
                android.app.NotificationChannel(channelId, "Proxi VPN", android.app.NotificationManager.IMPORTANCE_LOW)
            )
        }
        return Notification.Builder(this, channelId)
            .setContentTitle("Proxi VPN активен")
            .setContentText("Туннель работает в фоне")
            .setSmallIcon(android.R.drawable.ic_lock_lock)
            .build()
    }

    // MOB-005: promote to foreground so Android keeps VPN + WS alive in background
    fun enterForeground() {
        startForeground(NOTIF_ID, buildNotification())
    }

    fun leaveForeground() {
        stopForeground(STOP_FOREGROUND_REMOVE)
    }


    companion object {
        const val ACTION_START = "START_VPN"
        const val ACTION_STOP = "STOP_VPN"
        private const val TAG = "TunVpnService"
        private const val TUN_MTU = 1500
        private const val TUN_ADDRESS = "10.8.0.2"
        private const val TUN_PREFIX = 24
        private const val TUN_DNS = "1.1.1.1"

        var status = "disconnected"
            private set
        var onStatusChange: ((String) -> Unit)? = null

        val bytesIn = AtomicLong(0)
        val bytesOut = AtomicLong(0)
        val activeConnections = AtomicLong(0)

        /** Set this before starting — the exit proxy host:port */
        var exitProxyHost: String = ""
        var exitProxyPort: Int = 0

        /** T42-B config + packet gate (kill switch). Tests run on JVM. */
        var t42: T42BConfig = T42BConfig()
        val gate: PacketGate = PacketGate(killSwitch = true)
    }

    private var tunFd: ParcelFileDescriptor? = null
    private var tunInput: FileInputStream? = null
    private var tunOutput: FileOutputStream? = null
    private val isRunning = AtomicBoolean(false)
    private var readerThread: Thread? = null

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        when (intent?.action) {
            ACTION_START -> start()
            ACTION_STOP -> stopVpn()
        }
        return START_NOT_STICKY
    }

    fun start() {
        if (isRunning.get()) return
        val prepared = AndroidVpnService.prepare(this)
        if (prepared != null) {
            status = "awaiting_permission"
            onStatusChange?.invoke(status)
            Log.w(TAG, "VPN permission not granted yet; caller must fire the prepare intent")
            return
        }
        if (exitProxyHost.isEmpty() || exitProxyPort <= 0) {
            Socks5ExitServer.bindHost = "127.0.0.1"
            Socks5ExitServer.bindPort = t42.socksPort
            Socks5ExitServer.start()
            exitProxyHost = t42.socksHost
            exitProxyPort = t42.socksPort
            status = "self-exit 127.0.0.1:${t42.socksPort}"
            onStatusChange?.invoke(status)
        }
        startVpn()
    }

    // Active TCP connections: source port → TcpConn
    private val connections = ConcurrentHashMap<Int, TcpConn>()

    fun startVpn() {
        if (isRunning.get()) return

        try {
            // 1. Build TUN interface — capture ALL traffic
            val builder = Builder()
            builder.setSession("Proxi VPN")
            builder.setMtu(TUN_MTU)
            builder.addAddress(TUN_ADDRESS, TUN_PREFIX)
            builder.addRoute("0.0.0.0", 0)   // Route EVERYTHING through TUN
            builder.addDnsServer(TUN_DNS)
            builder.addSearchDomain(".")
            try {
                builder.addDisallowedApplication("com.indestructible.messenger")
            } catch (e: android.content.pm.PackageManager.NameNotFoundException) {
                Log.w(TAG, "self-exclude failed; SOCKS traffic may loop into TUN")
            }

            tunFd = builder.establish()
            if (tunFd == null) {
                status = "error: TUN establish() returned null (VPN permission denied?)"
                onStatusChange?.invoke(status)
                Log.e(TAG, status)
                return
            }

            tunInput = FileInputStream(tunFd!!.fileDescriptor)
            tunOutput = FileOutputStream(tunFd!!.fileDescriptor)

            // T42-B: TUN fd → hev JNI tun2socks → SOCKS5. No Kotlin TCP/IP.
            val yaml = HevSocks5Engine.writeYaml(cacheDir, t42)
            val fd = tunFd!!.fd
            val hevOk = HevSocks5Engine.start(yaml.absolutePath, fd)
            if (!hevOk) {
                status = "error: hev JNI start failed (${HevSocks5Engine.lastLoadError})"
                onStatusChange?.invoke(status)
                Log.e(TAG, status)
                try { tunFd?.close() } catch (_: Exception) {}
                tunFd = null
                return
            }

            isRunning.set(true)
            gate.setChannel(true)
            status = "connected"
            onStatusChange?.invoke(status)
            Log.i(TAG, "TUN+hev: $TUN_ADDRESS/$TUN_PREFIX fd=$fd yaml=${yaml.absolutePath}")

        } catch (e: Exception) {
            status = "error: ${e.message}"
            onStatusChange?.invoke(status)
            Log.e(TAG, "startVpn error", e)
        }
        enterForeground() // MOB-005
    }

    /**
     * Read raw IP packets from TUN interface, parse, and forward.
     */
    private fun readPackets() {
        val buffer = ByteBuffer.allocate(TUN_MTU)
        Log.i(TAG, "TUN reader thread started")

        while (isRunning.get()) {
            try {
                buffer.clear()
                val length = tunInput?.read(buffer.array()) ?: -1
                if (length <= 0) continue

                bytesIn.addAndGet(length.toLong())

                // Parse IP header
                val data = buffer.array()
                if (length < 20) continue

                val version = (data[0].toInt() shr 4) and 0x0F
                if (version != 4) continue  // IPv4 only for now

                val protocol = data[9].toInt() and 0xFF
                val srcIp = formatIp(data, 12)
                val dstIp = formatIp(data, 16)

                when (protocol) {
                    6 -> handleTcp(data, length, srcIp, dstIp)   // TCP
                    17 -> handleUdp(data, length, srcIp, dstIp)  // UDP (DNS)
                    else -> {
                        // Other protocols — drop
                    }
                }

            } catch (e: Exception) {
                if (isRunning.get()) {
                    Log.e(TAG, "read error", e)
                }
            }
        }
        Log.i(TAG, "TUN reader thread stopped")
    }

    /**
     * Handle TCP packet — forward connection to exit proxy.
     */
    private fun handleTcp(data: ByteArray, length: Int, srcIp: String, dstIp: String) {
        val srcPort = ((data[20].toInt() and 0xFF) shl 8) or (data[21].toInt() and 0xFF)
        val dstPort = ((data[22].toInt() and 0xFF) shl 8) or (data[23].toInt() and 0xFF)
        val tcpFlags = data[33].toInt() and 0xFF

        // SYN — new connection
        if ((tcpFlags and 0x02) != 0 && (tcpFlags and 0x10) == 0) {
            if (!gate.isOpen()) {
                Log.w(TAG, "kill switch: drop SYN $dstIp:$dstPort")
                return
            }
            Log.i(TAG, "TCP SYN: $srcIp:$srcPort → $dstIp:$dstPort")

            // If exit proxy configured, forward through it
            if (exitProxyHost.isNotEmpty() && exitProxyPort > 0) {
                val conn = TcpConn(srcPort, dstIp, dstPort)
                connections[srcPort] = conn
                activeConnections.incrementAndGet()
                Thread({ conn.forwardThroughProxy() }, "TCP-$srcPort").start()
            }
        }

        // FIN — close connection
        if ((tcpFlags and 0x01) != 0) {
            connections.remove(srcPort)?.close()
            activeConnections.decrementAndGet()
        }
    }

    /**
     * Handle UDP packet — DNS queries and other UDP.
     */
    private fun handleUdp(data: ByteArray, length: Int, srcIp: String, dstIp: String) {
        val srcPort = ((data[20].toInt() and 0xFF) shl 8) or (data[21].toInt() and 0xFF)
        val dstPort = ((data[22].toInt() and 0xFF) shl 8) or (data[23].toInt() and 0xFF)

        if (dstPort == 53) {
            // DNS query — forward to real DNS server
            Thread({
                try {
                    val queryPayload = data.copyOfRange(28, length)
                    val dnsSocket = java.net.DatagramSocket()
                    val dnsAddr = java.net.InetAddress.getByName(TUN_DNS)
                    val request = java.net.DatagramPacket(queryPayload, queryPayload.size, dnsAddr, 53)
                    dnsSocket.send(request)

                    val response = java.net.DatagramPacket(ByteArray(1024), 1024)
                    dnsSocket.soTimeout = 5000
                    dnsSocket.receive(response)

                    // Build response IP/UDP packet and write to TUN
                    val respPacket = buildUdpResponse(
                        response.data, response.length,
                        dstIp, srcIp, dstPort, srcPort
                    )
                    tunOutput?.write(respPacket)
                    tunOutput?.flush()
                    bytesOut.addAndGet(respPacket.size.toLong())
                    dnsSocket.close()
                    Log.d(TAG, "DNS resolved for $srcIp:$srcPort")
                } catch (e: Exception) {
                    Log.e(TAG, "DNS forward error", e)
                }
            }, "DNS-$srcPort").start()
        }
    }

    /**
     * Build a minimal UDP response packet to write back to TUN.
     */
    private fun buildUdpResponse(
        payload: ByteArray, payloadLen: Int,
        srcIp: String, dstIp: String,
        srcPort: Int, dstPort: Int
    ): ByteArray {
        val packet = ByteArray(20 + 8 + payloadLen)

        // IP header
        packet[0] = 0x45  // Version 4, IHL 5
        packet[1] = 0     // DSCP/ECN
        val totalLen = 20 + 8 + payloadLen
        packet[2] = ((totalLen shr 8) and 0xFF).toByte()
        packet[3] = (totalLen and 0xFF).toByte()
        packet[4] = 0; packet[5] = 1  // ID
        packet[6] = 0; packet[7] = 0  // Flags/Fragment
        packet[8] = 64    // TTL
        packet[9] = 17    // Protocol: UDP

        val src = parseIp(srcIp)
        val dst = parseIp(dstIp)
        System.arraycopy(src, 0, packet, 12, 4)
        System.arraycopy(dst, 0, packet, 16, 4)

        // IP checksum
        val cksum = ipChecksum(packet, 20)
        packet[10] = ((cksum shr 8) and 0xFF).toByte()
        packet[11] = (cksum and 0xFF).toByte()

        // UDP header
        packet[20] = ((srcPort shr 8) and 0xFF).toByte()
        packet[21] = (srcPort and 0xFF).toByte()
        packet[22] = ((dstPort shr 8) and 0xFF).toByte()
        packet[23] = (dstPort and 0xFF).toByte()
        val udpLen = 8 + payloadLen
        packet[24] = ((udpLen shr 8) and 0xFF).toByte()
        packet[25] = (udpLen and 0xFF).toByte()
        packet[26] = 0; packet[27] = 0  // UDP checksum (optional)

        // Payload
        System.arraycopy(payload, 0, packet, 28, payloadLen)
        return packet
    }

    fun stopVpn() {
        if (!isRunning.get()) return
        isRunning.set(false)
        gate.setChannel(false)
        HevSocks5Engine.stop()
        Socks5ExitServer.stop()

        // Close all connections
        for ((_, conn) in connections) {
            conn.close()
        }
        connections.clear()
        activeConnections.set(0)

        readerThread?.interrupt()
        readerThread = null

        try { tunInput?.close() } catch (_: Exception) {}
        try { tunOutput?.close() } catch (_: Exception) {}
        try { tunFd?.close() } catch (_: Exception) {}

        tunFd = null
        tunInput = null
        tunOutput = null

        status = "disconnected"
        onStatusChange?.invoke(status)
        Log.i(TAG, "TUN VPN stopped")
    }

    override fun onDestroy() {
        stopVpn()
        super.onDestroy()
    }

    override fun onRevoke() {
        stopVpn()
        super.onRevoke()
    }

    // ═══ Helpers ═══

    private fun formatIp(data: ByteArray, offset: Int): String {
        return "${data[offset].toInt() and 0xFF}.${data[offset+1].toInt() and 0xFF}.${data[offset+2].toInt() and 0xFF}.${data[offset+3].toInt() and 0xFF}"
    }

    private fun parseIp(ip: String): ByteArray {
        val parts = ip.split(".")
        return byteArrayOf(
            parts[0].toInt().toByte(),
            parts[1].toInt().toByte(),
            parts[2].toInt().toByte(),
            parts[3].toInt().toByte()
        )
    }

    private fun ipChecksum(data: ByteArray, length: Int): Int {
        var sum = 0L
        var i = 0
        while (i < length) {
            sum += ((data[i].toInt() and 0xFF) shl 8) or (data[i+1].toInt() and 0xFF)
            i += 2
        }
        while (sum shr 16 != 0L) {
            sum = (sum and 0xFFFF) + (sum shr 16)
        }
        return (sum.inv().toInt() and 0xFFFF)
    }

    /**
     * TCP connection forwarder — connects to destination through exit proxy.
     */
    private inner class TcpConn(
        val srcPort: Int,
        val dstIp: String,
        val dstPort: Int
    ) {
        var socket: Socket? = null

        fun forwardThroughProxy() {
            try {
                if (!gate.isOpen()) {
                    throw ChannelDownException("kill switch: channel down")
                }
                socket = Socks5Client.connect(
                    proxyHost = exitProxyHost,
                    proxyPort = exitProxyPort,
                    destHost = dstIp,
                    destPort = dstPort,
                    timeoutMs = 10000
                )
                Log.i(TAG, "TCP $srcPort: SOCKS5 CONNECT $dstIp:$dstPort via $exitProxyHost:$exitProxyPort")
                bytesOut.addAndGet(1)

            } catch (e: Exception) {
                Log.e(TAG, "TCP $srcPort forward error", e)
            } finally {
                close()
            }
        }

        fun close() {
            try { socket?.close() } catch (_: Exception) {}
            socket = null
        }
    }
}
