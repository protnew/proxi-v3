package com.indestructible.messenger.crypto

import java.security.MessageDigest
import java.security.SecureRandom
import javax.crypto.Cipher
import javax.crypto.spec.SecretKeySpec
import javax.crypto.spec.GCMParameterSpec

object Nip44 {

    private const val GCM_TAG_BITS = 128
    private const val GCM_IV_BYTES = 12
    private const val KEY_BYTES = 32

    fun encrypt(plaintext: String, key: ByteArray): String {
        require(key.size == KEY_BYTES) { "Key must be 32 bytes" }
        val iv = ByteArray(GCM_IV_BYTES).also { SecureRandom().nextBytes(it) }
        val cipher = Cipher.getInstance("AES/GCM/NoPadding")
        cipher.init(Cipher.ENCRYPT_MODE, SecretKeySpec(key, "AES"), GCMParameterSpec(GCM_TAG_BITS, iv))
        val ciphertext = cipher.doFinal(plaintext.toByteArray(Charsets.UTF_8))
        val packed = ByteArray(1 + GCM_IV_BYTES + ciphertext.size)
        packed[0] = 0x02
        System.arraycopy(iv, 0, packed, 1, GCM_IV_BYTES)
        System.arraycopy(ciphertext, 0, packed, 1 + GCM_IV_BYTES, ciphertext.size)
        return android.util.Base64.encodeToString(packed, android.util.Base64.NO_WRAP)
    }

    fun decrypt(ciphertextB64: String, key: ByteArray): String {
        require(key.size == KEY_BYTES) { "Key must be 32 bytes" }
        val packed = android.util.Base64.decode(ciphertextB64, android.util.Base64.NO_WRAP)
        require(packed.size > 1 + GCM_IV_BYTES) { "Ciphertext too short" }
        require(packed[0] == 0x02.toByte()) { "Unsupported version" }
        val iv = packed.copyOfRange(1, 1 + GCM_IV_BYTES)
        val ct = packed.copyOfRange(1 + GCM_IV_BYTES, packed.size)
        val cipher = Cipher.getInstance("AES/GCM/NoPadding")
        cipher.init(Cipher.DECRYPT_MODE, SecretKeySpec(key, "AES"), GCMParameterSpec(GCM_TAG_BITS, iv))
        return String(cipher.doFinal(ct), Charsets.UTF_8)
    }

    fun deriveConversationKey(sharedSecret: ByteArray): ByteArray {
        return MessageDigest.getInstance("SHA-256").digest(sharedSecret)
    }

    fun zeroBuffer(buf: ByteArray) {
        for (i in buf.indices) buf[i] = 0
    }
}
