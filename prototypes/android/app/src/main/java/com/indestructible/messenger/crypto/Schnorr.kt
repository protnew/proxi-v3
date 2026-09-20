package com.indestructible.messenger.crypto

import org.bouncycastle.asn1.sec.SECNamedCurves
import org.bouncycastle.asn1.x9.X9ECParameters
import java.math.BigInteger
import java.security.MessageDigest
import java.security.SecureRandom

/**
 * BIP-340 Schnorr signatures over secp256k1 — needed for NIP-01/NIP-42
 * event signing (P1 challenge-response auth on Android).
 *
 * No external schnorr lib in the project; implemented directly per
 * https://github.com/bitcoin/bips/blob/master/bip-0340.mediawiki
 */
object Schnorr {
    private val params: X9ECParameters = SECNamedCurves.getByName("secp256k1")
    private val n: BigInteger = params.n
    private val g = params.g

    private fun sha256(data: ByteArray): ByteArray =
        MessageDigest.getInstance("SHA-256").digest(data)

    private fun taggedHash(tag: String, vararg parts: ByteArray): ByteArray {
        val th = sha256(tag.toByteArray(Charsets.UTF_8))
        var msg = th + th
        for (p in parts) msg += p
        return sha256(msg)
    }

    private fun bytesOf(x: BigInteger): ByteArray {
        val b = x.toByteArray()
        return when {
            b.size == 32 -> b
            b.size > 32 -> b.copyOfRange(b.size - 32, b.size)
            else -> ByteArray(32 - b.size) + b
        }
    }

    private fun hasEvenY(d: BigInteger): Boolean =
        !g.multiply(d).normalize().affineYCoord.toBigInteger().testBit(0)

    /**
     * Sign 32-byte message with 32-byte secret key → 64-byte signature.
     * auxRand: optional 32 bytes (random if null).
     */
    fun sign(msg32: ByteArray, secKey32: ByteArray, auxRand: ByteArray? = null): ByteArray {
        require(msg32.size == 32 && secKey32.size == 32)
        val d0 = BigInteger(1, secKey32)
        require(d0.signum() > 0 && d0 < n) { "invalid secret key" }
        val d = if (hasEvenY(d0)) d0 else n.subtract(d0)
        val p = bytesOf(g.multiply(d).normalize().affineXCoord.toBigInteger())

        val aux = auxRand ?: ByteArray(32).also { SecureRandom().nextBytes(it) }
        require(aux.size == 32)
        val t = bytesOf(d).mapIndexed { i, b -> (b.toInt() xor taggedHash("BIP0340/aux", aux)[i].toInt()).toByte() }.toByteArray()
        val k0 = BigInteger(1, taggedHash("BIP0340/nonce", t, p, msg32)).mod(n)
        require(k0.signum() != 0) { "nonce is zero — retry" }
        val rPoint = g.multiply(k0).normalize()
        val r = bytesOf(rPoint.affineXCoord.toBigInteger())
        val k = if (rPoint.affineYCoord.toBigInteger().testBit(0)) n.subtract(k0) else k0
        val e = BigInteger(1, taggedHash("BIP0340/challenge", r, p, msg32)).mod(n)
        val s = k.add(e.multiply(d)).mod(n)
        return r + bytesOf(s)
    }

    /** Verify a 64-byte BIP-340 signature against 32-byte x-only pubkey. */
    fun verify(msg32: ByteArray, pubKey32: ByteArray, sig64: ByteArray): Boolean {
        if (msg32.size != 32 || pubKey32.size != 32 || sig64.size != 64) return false
        val px = BigInteger(1, pubKey32)
        if (px >= params.curve.field.characteristic) return false
        val r = BigInteger(1, sig64.copyOfRange(0, 32))
        val s = BigInteger(1, sig64.copyOfRange(32, 64))
        if (r >= params.curve.field.characteristic || s >= n) return false
        // Lift x to even-y point.
        val curve = params.curve
        val xEl = curve.fromBigInteger(px)
        val alpha = xEl.multiply(xEl.square()).add(curve.b) // x^3 + 7 (a=0)
        val beta = alpha.sqrt() ?: return false
        val p = if (beta.toBigInteger().testBit(0)) curve.createPoint(px, curve.field.characteristic.subtract(beta.toBigInteger())) else curve.createPoint(px, beta.toBigInteger())
        val e = BigInteger(1, taggedHash("BIP0340/challenge", sig64.copyOfRange(0, 32), pubKey32, msg32)).mod(n)
        val rPoint = g.multiply(s).subtract(p.multiply(e)).normalize()
        if (rPoint.isInfinity) return false
        if (rPoint.affineYCoord.toBigInteger().testBit(0)) return false
        return rPoint.affineXCoord.toBigInteger() == r
    }
}
