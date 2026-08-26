package com.indestructible.messenger.vpn

import java.io.File

/**
 * T42-B tun2socks control plane.
 * Official hev-socks5-tunnel 2.17.1 artifacts live under jniLibs.
 * This file does NOT parse IP packets and does NOT start JNI
 * (release android bins are PIE tun2socks, not Java_* .so).
 */
object HevSocks5Engine {
    const val LIB_NAME = "hev-socks5-tunnel"
    const val VERSION = "2.17.1"
    const val FILE_NAME = "libhev-socks5-tunnel.so"

    fun expectedAbis(): List<String> = listOf(
        "arm64-v8a",
        "armeabi-v7a",
        "x86_64",
        "x86",
    )

    fun yamlConfig(cfg: T42BConfig): String {
        val host = cfg.socksHost
        val port = cfg.socksPort
        return """
            |tunnel:
            |  name: ${cfg.session}
            |  mtu: 1500
            |  ipv4: ${cfg.tunAddress}
            |socks5:
            |  port: $port
            |  address: $host
            |  udp: 'udp'
            |misc:
            |  log-level: warn
            |""".trimMargin()
    }

    fun artifactFile(abi: String): File {
        val rel = "src/main/jniLibs/$abi/$FILE_NAME"
        val candidates = listOf(
            File(rel),
            File("app/$rel"),
            File("../$rel"),
        )
        return candidates.firstOrNull { it.isFile } ?: File(rel)
    }
}
