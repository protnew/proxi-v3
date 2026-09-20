package com.indestructible.messenger.crypto

import org.bouncycastle.crypto.agreement.ECDHBasicAgreement
import org.bouncycastle.crypto.engines.ChaCha7539Engine
import org.bouncycastle.crypto.params.ECDomainParameters
import org.bouncycastle.crypto.params.ECPrivateKeyParameters
import org.bouncycastle.crypto.params.ECPublicKeyParameters
import org.bouncycastle.crypto.params.KeyParameter
import org.bouncycastle.crypto.params.ParametersWithIV
import org.bouncycastle.crypto.digests.SHA256Digest
import org.bouncycastle.crypto.macs.HMac
import org.bouncycastle.asn1.sec.SECNamedCurves
import org.bouncycastle.asn1.x9.X9ECParameters
import java.security.SecureRandom

/**
 * MOB-103 v3 (2026-09-20): real NIP-44 v2 — wire-compatible with nostr-tools (PWA)
 * and Go src-vpn/nostr/nip44.go. Official vectors: paulmillr/nip44 nip44.vectors.json.
 *
 * Previous version (v2) used SHA-256(shared)+AES-256-GCM with the same 0x02 byte —
 * silently incompatible: every cross-platform DM died on MAC/unpad.
 *
 * Spec:
 *   conversation_key = HKDF-Extract(salt="nip44-v2", ikm=ecdh_shared_x)
 *   message_keys     = HKDF-Expand(conversation_key, info=nonce(32), L=76)
 *   padded           = [u16-BE len][utf8 plaintext][zero fill to calcPaddedLen]
 *   ciphertext       = ChaCha20(chacha_key, chacha_nonce, padded)
 *   mac              = HMAC-SHA256(hmac_key, nonce || ciphertext)
 *   wire             = base64(0x02 || nonce || ciphertext || mac)
 */
object Nip44 {

    private const val KEY_BYTES = 32
    private const val NONCE_BYTES = 32
    private const val MAC_BYTES = 32
    private const val MIN_PLAINTEXT = 1
    private const val MAX_PLAINTEXT = 65535
    private const val MIN_PADDED = 32

    private val secp256k1Params: X9ECParameters = SECNamedCurves.getByName("secp256k1")
    private val ecDomain = ECDomainParameters(
        secp256k1Params.curve, secp256k1Params.g, secp256k1Params.n, secp256k1Params.h
    )

    // ==================== ECDH / keys ====================

    /** secp256k1 ECDH → 32-byte shared x-coordinate. */
    fun ecdh(myPrivKeyHex: String, theirPubKeyHex: String): ByteArray {
        val privKeyBytes = hexToBytes(myPrivKeyHex)
        val privParams = ECPrivateKeyParameters(privKeyBytes.toBigInteger(), ecDomain)

        // Accepts 64-hex x-only pubkey (NIP-01/nostr convention → lift_x with even Y)
        // or 66-hex compressed SEC1.
        var pubKeyBytes = hexToBytes(theirPubKeyHex)
        if (pubKeyBytes.size == 32) pubKeyBytes = byteArrayOf(0x02) + pubKeyBytes
        val pubPoint = secp256k1Params.curve.decodePoint(pubKeyBytes)
        val pubParams = ECPublicKeyParameters(pubPoint, ecDomain)

        val agreement = ECDHBasicAgreement()
        agreement.init(privParams)
        val shared = agreement.calculateAgreement(pubParams)

        return shared.toByteArray().let { full ->
            if (full.size == 32) full
            else if (full.size > 32) full.copyOfRange(full.size - 32, full.size)
            else ByteArray(32 - full.size) + full
        }
    }

    /** conversation_key = HKDF-Extract(salt="nip44-v2", ikm=shared_x) */
    fun deriveConversationKey(sharedSecret: ByteArray): ByteArray {
        return hmacSha256("nip44-v2".toByteArray(Charsets.UTF_8), sharedSecret)
    }

    /** HKDF-Expand(prk, info=nonce, L=76) → chacha_key|chacha_nonce|hmac_key */
    private fun messageKeys(conversationKey: ByteArray, nonce: ByteArray): Triple<ByteArray, ByteArray, ByteArray> {
        require(conversationKey.size == KEY_BYTES && nonce.size == NONCE_BYTES)
        // HKDF-Expand: T(1) = HMAC(prk, T(0)="" || info || 0x01), T(2), T(3) — L=76 needs 3 blocks.
        val out = ByteArray(76)
        var prev = ByteArray(0)
        var offset = 0
        var counter = 1
        while (offset < 76) {
            prev = hmacSha256(conversationKey, prev + nonce + byteArrayOf(counter.toByte()))
            val take = minOf(32, 76 - offset)
            System.arraycopy(prev, 0, out, offset, take)
            offset += take
            counter++
        }
        return Triple(
            out.copyOfRange(0, 32),
            out.copyOfRange(32, 44),
            out.copyOfRange(44, 76)
        )
    }

    // ==================== Padding ====================

    private fun calcPaddedLen(unpadded: Int): Int {
        if (unpadded <= MIN_PADDED) return MIN_PADDED
        val nextPower = 1 shl (32 - Integer.numberOfLeadingZeros(unpadded - 1))
        var chunk = nextPower / 8
        if (chunk < 32) chunk = 32
        return chunk * ((unpadded - 1) / chunk + 1)
    }

    private fun pad(plaintext: ByteArray): ByteArray {
        require(plaintext.size in MIN_PLAINTEXT..MAX_PLAINTEXT) { "nip44: bad plaintext length ${plaintext.size}" }
        val paddedLen = calcPaddedLen(plaintext.size)
        val out = ByteArray(2 + paddedLen)
        out[0] = (plaintext.size ushr 8).toByte()
        out[1] = plaintext.size.toByte()
        System.arraycopy(plaintext, 0, out, 2, plaintext.size)
        return out
    }

    private fun unpad(data: ByteArray): ByteArray {
        require(data.size >= 2 + MIN_PADDED) { "nip44: padded data too short" }
        val ulen = ((data[0].toInt() and 0xFF) shl 8) or (data[1].toInt() and 0xFF)
        require(ulen in MIN_PLAINTEXT..MAX_PLAINTEXT) { "nip44: invalid unpadded length $ulen" }
        require(data.size == 2 + calcPaddedLen(ulen)) { "nip44: padded length mismatch" }
        for (i in 2 + ulen until data.size) {
            require(data[i] == 0.toByte()) { "nip44: non-zero padding" }
        }
        return data.copyOfRange(2, 2 + ulen)
    }

    // ==================== Encrypt / Decrypt ====================

    /**
     * Encrypt plaintext with the 32-byte conversation key.
     * @return base64(0x02 || nonce32 || ciphertext || mac32) — nostr-tools compatible.
     */
    fun encrypt(plaintext: String, key: ByteArray): String {
        require(key.size == KEY_BYTES) { "Key must be 32 bytes" }
        val nonce = ByteArray(NONCE_BYTES).also { SecureRandom().nextBytes(it) }
        return encryptWithNonce(plaintext, key, nonce)
    }

    /** Deterministic core (vectors); nonce must be fresh CSPRNG in production. */
    fun encryptWithNonce(plaintext: String, key: ByteArray, nonce: ByteArray): String {
        val (chachaKey, chachaNonce, hmacKey) = messageKeys(key, nonce)
        val padded = pad(plaintext.toByteArray(Charsets.UTF_8))

        val ciphertext = ByteArray(padded.size)
        val chacha = ChaCha7539Engine()
        chacha.init(true, ParametersWithIV(KeyParameter(chachaKey), chachaNonce))
        chacha.processBytes(padded, 0, padded.size, ciphertext, 0)

        val mac = hmacSha256(hmacKey, nonce + ciphertext)

        val packed = ByteArray(1 + NONCE_BYTES + ciphertext.size + MAC_BYTES)
        packed[0] = 0x02
        System.arraycopy(nonce, 0, packed, 1, NONCE_BYTES)
        System.arraycopy(ciphertext, 0, packed, 1 + NONCE_BYTES, ciphertext.size)
        System.arraycopy(mac, 0, packed, 1 + NONCE_BYTES + ciphertext.size, MAC_BYTES)
        zeroBuffer(chachaKey); zeroBuffer(chachaNonce); zeroBuffer(hmacKey); zeroBuffer(padded)

        return android.util.Base64.encodeToString(packed, android.util.Base64.NO_WRAP)
    }

    /** Decrypt base64 NIP-44 v2 payload with the conversation key. */
    fun decrypt(ciphertextB64: String, key: ByteArray): String {
        require(key.size == KEY_BYTES) { "Key must be 32 bytes" }
        val packed = android.util.Base64.decode(ciphertextB64, android.util.Base64.NO_WRAP)
        require(packed.size >= 1 + NONCE_BYTES + 2 + MIN_PADDED + MAC_BYTES) { "nip44: payload too short" }
        require(packed[0] == 0x02.toByte()) { "nip44: unsupported version ${packed[0]}" }

        val nonce = packed.copyOfRange(1, 1 + NONCE_BYTES)
        val ct = packed.copyOfRange(1 + NONCE_BYTES, packed.size - MAC_BYTES)
        val macGiven = packed.copyOfRange(packed.size - MAC_BYTES, packed.size)

        val (chachaKey, chachaNonce, hmacKey) = messageKeys(key, nonce)
        val macCalc = hmacSha256(hmacKey, nonce + ct)
        require(macGiven.contentEquals(macCalc)) { "nip44: invalid MAC" }

        val padded = ByteArray(ct.size)
        val chacha = ChaCha7539Engine()
        chacha.init(false, ParametersWithIV(KeyParameter(chachaKey), chachaNonce))
        chacha.processBytes(ct, 0, ct.size, padded, 0)

        val plaintext = unpad(padded)
        zeroBuffer(chachaKey); zeroBuffer(chachaNonce); zeroBuffer(hmacKey); zeroBuffer(padded)
        return String(plaintext, Charsets.UTF_8)
    }

    // ==================== Convenience ====================

    fun encryptFor(plaintext: String, myPrivKeyHex: String, theirPubKeyHex: String): String {
        val shared = ecdh(myPrivKeyHex, theirPubKeyHex)
        val key = deriveConversationKey(shared)
        val result = encrypt(plaintext, key)
        zeroBuffer(shared)
        zeroBuffer(key)
        return result
    }

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

    private fun hmacSha256(key: ByteArray, data: ByteArray): ByteArray {
        val mac = HMac(SHA256Digest())
        mac.init(KeyParameter(key))
        mac.update(data, 0, data.size)
        val out = ByteArray(32)
        mac.doFinal(out, 0)
        return out
    }

    private fun hexToBytes(hex: String): ByteArray {
        return hex.chunked(2).map { it.toInt(16).toByte() }.toByteArray()
    }

    private fun ByteArray.toBigInteger(): java.math.BigInteger {
        return java.math.BigInteger(1, this)
    }
}
