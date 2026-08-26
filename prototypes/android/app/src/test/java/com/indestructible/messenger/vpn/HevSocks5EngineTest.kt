package com.indestructible.messenger.vpn

import org.junit.Assert.*
import org.junit.Test

class HevSocks5EngineTest {
    @Test
    fun lib_name_is_hev_socks5_tunnel() {
        assertEquals("hev-socks5-tunnel", HevSocks5Engine.LIB_NAME)
    }

    @Test
    fun yaml_points_at_local_socks_from_t42_config() {
        val yaml = HevSocks5Engine.yamlConfig(T42BConfig())
        assertTrue(yaml.contains("127.0.0.1"))
        assertTrue(yaml.contains("10808"))
        assertFalse(yaml.contains("0.0.0.0:10808"))
    }

    @Test
    fun expected_abis_cover_phone_and_emulator() {
        val abis = HevSocks5Engine.expectedAbis()
        assertTrue(abis.contains("arm64-v8a"))
        assertTrue(abis.contains("x86_64"))
    }

    @Test
    fun native_artifact_present_for_arm64() {
        val f = HevSocks5Engine.artifactFile("arm64-v8a")
        assertTrue("missing " + f.absolutePath, f.isFile && f.length() > 10_000)
    }

    @Test
    fun official_jni_names_match_upstream() {
        val names = HevSocks5Engine.officialJniNames()
        assertTrue(names.contains("TProxyStartService"))
        assertTrue(names.contains("TProxyStopService"))
        assertTrue(names.contains("TProxyIsRunning"))
    }

    @Test
    fun arm64_so_is_jni_not_pie_cli() {
        val f = HevSocks5Engine.artifactFile("arm64-v8a")
        assertTrue("missing " + f.absolutePath, f.isFile && f.length() > 10_000)
        assertTrue(
            "T42B-011: need JNI .so with JNI_OnLoad/TProxy, not GitHub PIE bin: " + f.absolutePath,
            HevSocks5Engine.isJniLibrary(f),
        )
        assertTrue(HevSocks5Engine.canStartNative("arm64-v8a"))
    }

    @Test
    fun jni_so_registers_our_package() {
        val f = HevSocks5Engine.artifactFile("arm64-v8a")
        val bytes = f.readBytes()
        val hay = String(bytes, Charsets.ISO_8859_1)
        assertTrue(
            "JNI_OnLoad must FindClass our engine, not hev/htproxy",
            hay.contains("com/indestructible/messenger/vpn/HevSocks5Engine"),
        )
        assertTrue(hay.contains("JNI_OnLoad"))
    }

    @Test
    fun start_refuses_bad_fd() {
        val yaml = HevSocks5Engine.writeYaml(java.io.File("build/tmp-hev"))
        assertTrue(yaml.isFile)
        assertFalse(HevSocks5Engine.start(yaml.absolutePath, -1))
    }

    @Test
    fun start_on_jvm_does_not_crash_without_android_loader() {
        val yaml = HevSocks5Engine.writeYaml(java.io.File("build/tmp-hev"))
        // host JVM cannot load android .so — must return false, not throw
        assertFalse(HevSocks5Engine.start(yaml.absolutePath, 3))
    }

    @Test
    fun all_four_abis_are_jni() {
        for (abi in HevSocks5Engine.expectedAbis()) {
            val f = HevSocks5Engine.artifactFile(abi)
            assertTrue(abi + " missing", f.isFile && f.length() > 10_000)
            assertTrue(abi + " not JNI: " + f.absolutePath, HevSocks5Engine.isJniLibrary(f))
        }
    }
}
