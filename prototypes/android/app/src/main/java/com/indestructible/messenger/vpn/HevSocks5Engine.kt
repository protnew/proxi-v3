package com.indestructible.messenger.vpn

import java.io.File

/**
 * T42-B tun2socks. Official hev JNI:
 *   TProxyStartService(config_path, fd) / Stop / IsRunning / GetStats
 * JNI_OnLoad RegisterNatives → this class (PKGNAME/CLSNAME at ndk-build).
 * Do not System.loadLibrary a PIE android-bin. Do not write Kotlin TCP/IP.
 */
object HevSocks5Engine {
    const val LIB_NAME = "hev-socks5-tunnel"
    const val VERSION = "2.17.1"
    const val FILE_NAME = "libhev-socks5-tunnel.so"

    @Volatile private var loaded: Boolean = false
    @Volatile var lastLoadError: String? = null
        private set

    fun expectedAbis(): List<String> = listOf("arm64-v8a", "armeabi-v7a", "x86_64", "x86")

    fun officialJniNames(): List<String> = listOf(
        "TProxyStartService",
        "TProxyStopService",
        "TProxyIsRunning",
        "TProxyGetStats",
    )

    fun yamlConfig(cfg: T42BConfig): String {
        return """
            |tunnel:
            |  name: ${cfg.session}
            |  mtu: 1500
            |  ipv4: ${cfg.tunAddress}
            |socks5:
            |  port: ${cfg.socksPort}
            |  address: ${cfg.socksHost}
            |  udp: 'tcp'
            |misc:
            |  log-level: warn
            |""".trimMargin()
    }

    fun artifactFile(abi: String): File {
        val rel = "src/main/jniLibs/$abi/$FILE_NAME"
        val candidates = listOf(File(rel), File("app/$rel"), File("../$rel"))
        return candidates.firstOrNull { it.isFile } ?: File(rel)
    }

    fun isJniLibrary(file: File): Boolean {
        if (!file.isFile || file.length() < 16) return false
        val bytes = file.readBytes()
        if (bytes.size < 4 || bytes[0] != 0x7F.toByte() || bytes[1] != 'E'.code.toByte()) return false
        return indexOf(bytes, "JNI_OnLoad".toByteArray()) >= 0 ||
            indexOf(bytes, "TProxyStartService".toByteArray()) >= 0
    }

    fun canStartNative(abi: String): Boolean = isJniLibrary(artifactFile(abi))

    fun writeYaml(dir: File, cfg: T42BConfig = T42BConfig()): File {
        dir.mkdirs()
        val f = File(dir, "t42b-hev.yaml")
        f.writeText(yamlConfig(cfg))
        return f
    }

    fun tryLoadNative(): Boolean {
        if (loaded) return true
        return try {
            System.loadLibrary(LIB_NAME)
            loaded = true
            lastLoadError = null
            true
        } catch (e: UnsatisfiedLinkError) {
            lastLoadError = e.message
            false
        }
    }

    fun start(configPath: String, fd: Int): Boolean {
        if (fd < 0) return false
        if (!File(configPath).isFile) return false
        if (!tryLoadNative()) return false
        return TProxyStartService(configPath, fd)
    }

    fun stop(): Boolean {
        if (!loaded) return true
        return TProxyStopService()
    }

    fun isRunning(): Boolean = loaded && TProxyIsRunning()

    @JvmStatic
    external fun TProxyStartService(configPath: String, fd: Int): Boolean

    @JvmStatic
    external fun TProxyStopService(): Boolean

    @JvmStatic
    external fun TProxyIsRunning(): Boolean

    @JvmStatic
    external fun TProxyGetStats(): LongArray

    private fun indexOf(hay: ByteArray, needle: ByteArray): Int {
        outer@ for (i in 0..hay.size - needle.size) {
            for (j in needle.indices) {
                if (hay[i + j] != needle[j]) continue@outer
            }
            return i
        }
        return -1
    }
}
