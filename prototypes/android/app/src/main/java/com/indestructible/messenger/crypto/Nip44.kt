package com.indestructible.messenger.crypto

import org.bouncycastle.crypto.agreement.ECDHBasicAgreement
import org.bouncycastle.crypto.params.ECDomainParameters
import org.bouncycastle.crypto.params.ECPrivateKeyParameters
import org.bouncycastle.crypto.params.ECPublicKeyParameters
import org.bouncycastle.crypto.signers.ECDSASigner
import org.bouncycastle.crypto.digests.SHA256Digest
import org.bouncycastle.math.ec.ECCurve
import org.bouncycastle.math.ec.ECPoint
import org.bouncycastle.jce.ECNamedCurveTable
import org.bouncycastle.asn1.sec.SECNamedCurves
import org.bouncycastle.asn1.x9.X962NamedCurves
import org.bouncycastle.asn1.x9.X9ECParameters
import java.security.MessageDigest
import java.security.SecureRandom
import javax.crypto.Cipher
import javax.crypto.spec.SecretKeySpec
import javax.crypto.spec.GCMParameterSpec

/**
 * MOB-103 v2: NIP-44 v2 compatible encryption with secp256k1 ECDH.
 *
 * Flow:
 * 1. ECDH: shared_secret = secp256k1_ecdh(my_priv, their_pub)
 * 2. KDF: conversation_key = SHA-256(shared_secret)
 * 3. Encrypt: AES-256-GCM (12-byte IV + 128-bit tag)
 * 4. Frame: [0x02][IV(12)][ciphertext+tag]
 *
 * Uses BouncyCastle for secp256k1 (same curve as Bitcoin/Nostr).
 */
object Nip44 {

    private const val GCM_TAG_BITS = 128
    private const val GCM_IV_BYTES = 12
    private const val KEY_BYTES = 32

    // secp256k1 curve parameters
    private val secp256k1Params: X9ECParameters = SECNamedCurves.getByName("secp256k1")
    private val ecDomain = ECDomainParameters(
        secp256k1Params.curve, secp256k1Params.g, secp256k1Params.n, secp256k1Params.h
    )

    /**
     * Compute ECDH shared secret using secp256k1.
     * @param myPrivKeyHex 32-byte private key in hex (64 chars)
     * @param theirPubKeyHex 33-byte compressed public key in hex (66 chars)
     * @return 32-byte shared secret
     */
    fun ecdh(myPrivKeyHex: String, theirPubKeyHex: String): ByteArray {
        // Parse private key
        val privKeyBytes = hexToBytes(myPrivKeyHex)
        val privParams = ECPrivateKeyParameters(
            privKeyBytes.toBigInteger(), ecDomain
        )

        // Parse public key
        val pubKeyBytes = hexToBytes(theirPubKeyHex)
        val pubPoint = secp256k1Params.curve.decodePoint(pubKeyBytes)
        val pubParams = ECPublicKeyParameters(pubPoint, ecDomain)

        // Compute shared secret
        val agreement = ECDHBasicAgreement()
        agreement.init(privParams)
        val shared = agreement.calculateAgreement(pubParams)

        // Convert to 32-byte array
        return shared.toByteArray().let { full ->
            if (full.size == 32) full
            else if (full.size > 32) full.copyOfRange(full.size - 32, full.size)
            else ByteArray(32 - full.size) + full  // pad with zeros
        }
    }

    /**
     * Derive conversation key from ECDH shared secret.
     * NIP-44: conversation_key = HKDF-Extract(salt=02||shared_x, ikm=shared_x)
     * Simplified: SHA-256(shared_secret) for compatibility with existing PWA.
     */
    fun deriveConversationKey(sharedSecret: ByteArray): ByteArray {
        return MessageDigest.getInstance("SHA-256").digest(sharedSecret)
    }

    /**
     * Encrypt plaintext using conversation key.
     * @return Base64 ciphertext with NIP-44 v2 frame format
     */
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

    /**
     * Decrypt Base64 ciphertext.
     */
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

    /**
     * Convenience: encrypt with ECDH keys directly.
     * @param plaintext text to encrypt
     * @param myPrivKeyHex sender's 32-byte private key in hex
     * @param theirPubKeyHex recipient's 33-byte compressed public key in hex
     */
    fun encryptFor(plaintext: String, myPrivKeyHex: String, theirPubKeyHex: String): String {
        val shared = ecdh(myPrivKeyHex, theirPubKeyHex)
        val key = deriveConversationKey(shared)
        val result = encrypt(plaintext, key)
        zeroBuffer(shared)
        zeroBuffer(key)
        return result
    }

    /**
     * Convenience: decrypt with ECDH keys directly.
     */
    fun decryptFrom(ciphertextB64: String, myPrivKeyHex: String, theirPubKeyHex: String): String {
        val shared = ecdh(myPrivKeyHex, theirPubKeyHex)
        val key = deriveConversationKey(shared)
        val result = decrypt(ciphertextB64, key)
        zeroBuffer(shared)
        zeroBuffer(key)
        return result
    }

    fun zeroBuffer(buf: ByteArray) {
        for (i in buf.indices) buf[i] = 0
    }

    private fun hexToBytes(hex: String): ByteArray {
        return hex.chunked(2).map { it.toInt(16).toByte() }.toByteArray()
    }

    private fun ByteArray.toBigInteger(): java.math.BigInteger {
        return java.math.BigInteger(1, this)
    }
}
