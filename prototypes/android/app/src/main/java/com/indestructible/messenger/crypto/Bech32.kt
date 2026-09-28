package com.indestructible.messenger.crypto

/**
 * P24 (2026-09-20): Bech32 decoder for nsec/npub import.
 * Reference: BIP-0173 (bech32, no bech32m needed — Nostr uses bech32).
 */
object Bech32 {

    private const val CHARSET = "qpzry9x8gf2tvdw0s3jn54khce6mua7l"
    private val REV = IntArray(128) { -1 }.also { t ->
        CHARSET.forEachIndexed { i, c -> t[c.code] = i }
    }

    /**
     * Decode a bech32 string (e.g. "nsec1...") to (hrp, data5bit-without-checksum).
     * Throws IllegalArgumentException on any malformed input.
     */
    fun decode(bech: String): Pair<String, ByteArray> {
        require(bech.length in 8..90) { "bech32: bad length" }
        require(bech == bech.lowercase() || bech == bech.uppercase()) { "bech32: mixed case" }
        val s = bech.lowercase()
        val sep = s.lastIndexOf('1')
        require(sep >= 1 && sep + 7 <= s.length) { "bech32: missing separator/checksum" }
        val hrp = s.substring(0, sep)
        val data = s.substring(sep + 1).map { c ->
            val v = if (c.code < 128) REV[c.code] else -1
            require(v >= 0) { "bech32: invalid char '$c'" }
            v
        }
        // checksum
        val values = hrpExpand(hrp) + data
        require(polymod(values) == 1) { "bech32: bad checksum" }
        return hrp to data.dropLast(6).map { it.toByte() }.toByteArray()
    }

    /** Decode "nsec1…" → raw 32-byte secret key. */
    fun decodeNsec(nsec: String): ByteArray {
        val (hrp, data5) = decode(nsec)
        require(hrp == "nsec") { "expected nsec1 prefix, got ${hrp}1" }
        val bytes = convertBits(data5, 5, 8, false)
        require(bytes.size == 32) { "nsec payload must be 32 bytes, got ${bytes.size}" }
        return bytes
    }

    /** Decode "npub1…" → raw 32-byte public key (x-only). */
    fun decodeNpub(npub: String): ByteArray {
        val (hrp, data5) = decode(npub)
        require(hrp == "npub") { "expected npub1 prefix, got ${hrp}1" }
        val bytes = convertBits(data5, 5, 8, false)
        require(bytes.size == 32) { "npub payload must be 32 bytes, got ${bytes.size}" }
        return bytes
    }

    private fun hrpExpand(hrp: String): IntArray {
        val out = IntArray(hrp.length * 2 + 1)
        hrp.forEachIndexed { i, c -> out[i] = c.code ushr 5 }
        out[hrp.length] = 0
        hrp.forEachIndexed { i, c -> out[hrp.length + 1 + i] = c.code and 31 }
        return out
    }

    private fun polymod(values: IntArray): Int {
        val gen = intArrayOf(0x3b6a57b2, 0x26508e6d, 0x1ea119fa, 0x3d4233dd, 0x2a1462b3)
        var chk = 1
        for (v in values) {
            val top = chk ushr 25
            chk = (chk and 0x1ffffff) shl 5 xor v
            for (i in 0..4) if ((top ushr i) and 1 == 1) chk = chk xor gen[i]
        }
        return chk
    }

    /** BIP-0173 convertBits (5→8 for decode). */
    private fun convertBits(data: ByteArray, from: Int, to: Int, pad: Boolean): ByteArray {
        var acc = 0
        var bits = 0
        val out = ArrayList<Byte>()
        val maxv = (1 shl to) - 1
        for (b in data) {
            val v = b.toInt() and 0xFF
            require(v ushr from == 0) { "bech32: value out of range" }
            acc = (acc shl from) or v
            bits += from
            while (bits >= to) {
                bits -= to
                out.add(((acc ushr bits) and maxv).toByte())
            }
        }
        if (pad) {
            if (bits > 0) out.add(((acc shl (to - bits)) and maxv).toByte())
        } else {
            require(bits < from) { "bech32: incomplete group" }
            require(((acc shl (to - bits)) and maxv) == 0) { "bech32: non-zero padding" }
        }
        return out.toByteArray()
    }
}
