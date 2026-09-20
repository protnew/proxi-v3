package com.indestructible.messenger.crypto

import com.indestructible.messenger.nostr.NostrIdentity
import org.junit.Assert.assertArrayEquals
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Assert.fail
import org.junit.Test
import java.math.BigInteger
import java.security.MessageDigest

class CryptoTest {

    private fun hex(s: String): ByteArray =
        s.chunked(2).map { it.toInt(16).toByte() }.toByteArray()

    private fun ByteArray.hex(): String =
        joinToString("") { "%02x".format(it) }

    // ---- NostrIdentity ----

    @Test
    fun `derivePublicKey of secret key 1 is secp256k1 generator x`() {
        val sk = ByteArray(32).apply { this[31] = 1 }
        assertEquals(
            "79be667ef9dcbbac55a06295ce870b07029bfcdb2dce28d959f2815b16f81798",
            NostrIdentity.derivePublicKey(sk)
        )
    }

    @Test
    fun `fromSecretKey produces matching hex fields`() {
        val id = NostrIdentity.fromSecretKey(ByteArray(32).apply { this[31] = 1 })
        assertEquals(
            "79be667ef9dcbbac55a06295ce870b07029bfcdb2dce28d959f2815b16f81798",
            id.publicKey
        )
        assertEquals(32, id.privateKey.size)
    }

    // ---- Schnorr (BIP-340) ----

    @Test
    fun `schnorr sign then verify roundtrips`() {
        val sk = ByteArray(32).apply { this[31] = 3 }
        val pub = hex(NostrIdentity.derivePublicKey(sk))
        val msg = MessageDigest.getInstance("SHA-256").digest("proxi-auth".toByteArray())
        val sig = Schnorr.sign(msg, sk)
        assertEquals(64, sig.size)
        assertTrue(Schnorr.verify(msg, pub, sig))
    }

    @Test
    fun `schnorr rejects tampered message and wrong key`() {
        val sk = ByteArray(32).apply { this[31] = 3 }
        val pub = hex(NostrIdentity.derivePublicKey(sk))
        val msg = MessageDigest.getInstance("SHA-256").digest("proxi-auth".toByteArray())
        val sig = Schnorr.sign(msg, sk)

        val tampered = msg.copyOf().apply { this[0] = (this[0] + 1).toByte() }
        assertFalse(Schnorr.verify(tampered, pub, sig))

        val wrongPub = hex(NostrIdentity.derivePublicKey(ByteArray(32).apply { this[31] = 9 }))
        assertFalse(Schnorr.verify(msg, wrongPub, sig))
    }

    // ---- NIP-44 v2 ----

    @Test
    fun `nip44 ecdh is symmetric`() {
        val aliceSk = ByteArray(32).apply { this[31] = 5 }
        val bobSk = ByteArray(32).apply { this[31] = 7 }
        val alicePub = NostrIdentity.derivePublicKey(aliceSk)
        val bobPub = NostrIdentity.derivePublicKey(bobSk)
        val a = Nip44.ecdh(aliceSk.hex(), bobPub)
        val b = Nip44.ecdh(bobSk.hex(), alicePub)
        assertArrayEquals(a, b)
    }

    @Test
    fun `nip44 encrypt decrypt roundtrip`() {
        val aliceSk = ByteArray(32).apply { this[31] = 5 }
        val bobSk = ByteArray(32).apply { this[31] = 7 }
        val alicePub = NostrIdentity.derivePublicKey(aliceSk)
        val bobPub = NostrIdentity.derivePublicKey(bobSk)

        val plaintext = "привет, proxi — unicode ✓".repeat(3)
        val ct = Nip44.encryptFor(plaintext, aliceSk.hex(), bobPub)
        assertEquals(plaintext, Nip44.decryptFrom(ct, bobSk.hex(), alicePub))
    }

    @Test
    fun `nip44 decrypt with wrong key fails`() {
        val aliceSk = ByteArray(32).apply { this[31] = 5 }
        val bobPub = NostrIdentity.derivePublicKey(ByteArray(32).apply { this[31] = 7 })
        val ct = Nip44.encryptFor("secret", aliceSk.hex(), bobPub)

        val eveSk = ByteArray(32).apply { this[31] = 9 }
        val alicePub = NostrIdentity.derivePublicKey(aliceSk)
        try {
            Nip44.decryptFrom(ct, eveSk.hex(), alicePub)
            fail("decrypt with wrong key must throw")
        } catch (expected: Exception) { }
    }

    @Test
    fun `nip44 tampered ciphertext fails hmac`() {
        val convKey = ByteArray(32) { 0x42 }
        val ct = Nip44.encrypt("integrity-check", convKey)
        val raw = java.util.Base64.getDecoder().decode(ct)
        // flip a byte inside the encrypted region (after 1-byte version + 32-byte nonce)
        raw[40] = (raw[40] + 1).toByte()
        val tampered = java.util.Base64.getEncoder().encodeToString(raw)
        try {
            Nip44.decrypt(tampered, convKey)
            fail("tampered ciphertext must throw")
        } catch (expected: Exception) { }
    }

    @Test
    fun `nip44 encrypt is non-deterministic via random nonce`() {
        val convKey = ByteArray(32) { 0x11 }
        val a = Nip44.encrypt("same", convKey)
        val b = Nip44.encrypt("same", convKey)
        assertFalse(a == b)
        assertEquals("same", Nip44.decrypt(a, convKey))
        assertEquals("same", Nip44.decrypt(b, convKey))
    }

    // ---- Bech32 ----

    @Test
    fun `bech32 decodeNsec rejects garbage`() {
        try {
            Bech32.decodeNsec("nsec1notavalidkey")
            fail("must throw on bad checksum")
        } catch (expected: Exception) { }
        try {
            Bech32.decodeNsec("npub1whatever")
            fail("must throw on wrong hrp")
        } catch (expected: Exception) { }
    }

    @Test
    fun `bech32 decodeNsec accepts a well-formed nsec`() {
        // nsec for seckey = 0x01, generated by nostr-tools nip19.nsecEncode
        val decoded = Bech32.decodeNsec(
            "nsec1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqsmhltgl"
        )
        assertEquals(32, decoded.size)
        assertEquals(BigInteger.ONE, BigInteger(1, decoded))
    }
}
