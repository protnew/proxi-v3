package com.indestructible.messenger.nostr

import org.bouncycastle.asn1.sec.SECNamedCurves
import org.bouncycastle.asn1.x9.X9ECParameters
import java.math.BigInteger
import java.security.SecureRandom

/**
 * Nostr Identity — secp256k1 key generation and management.
 *
 * P24 (2026-09-20): derivePublicKey was a stub returning "abcdef..." — every
 * generated identity had a fake pubkey, breaking ECDH/E2E with real clients.
 * Now derives the real x-only (32-byte hex) pubkey per NIP-01.
 */
class NostrIdentity(
    val privateKey: ByteArray,
    val publicKey: String // 64-hex x-only pubkey (NIP-01)
) {
    companion object {
        private val secp256k1Params: X9ECParameters = SECNamedCurves.getByName("secp256k1")

        fun generate(): NostrIdentity {
            val sk = ByteArray(32).also { SecureRandom().nextBytes(it) }
            return NostrIdentity(sk, derivePublicKey(sk))
        }

        /** Rebuild identity from an existing 32-byte secret key. */
        fun fromSecretKey(secretKey: ByteArray): NostrIdentity {
            require(secretKey.size == 32) { "secret key must be 32 bytes" }
            return NostrIdentity(secretKey, derivePublicKey(secretKey))
        }

        /** Real secp256k1 x-only pubkey (BIP-340 style, 64 hex chars). */
        fun derivePublicKey(secretKey: ByteArray): String {
            val d = BigInteger(1, secretKey)
            require(d.signum() > 0 && d < secp256k1Params.n) { "invalid secret key" }
            val point = secp256k1Params.g.multiply(d).normalize()
            val x = point.affineXCoord.toBigInteger()
            // x-only: 32-byte big-endian
            val xb = x.toByteArray()
            val x32 = when {
                xb.size == 32 -> xb
                xb.size > 32 -> xb.copyOfRange(xb.size - 32, xb.size)
                else -> ByteArray(32 - xb.size) + xb
            }
            return x32.joinToString("") { "%02x".format(it) }
        }
    }
}
