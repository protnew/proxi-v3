package com.indestructible.messenger.nostr

import android.content.Context
import android.security.keystore.KeyGenParameterSpec
import android.security.keystore.KeyProperties
import android.util.Base64
import java.security.KeyStore
import javax.crypto.Cipher
import javax.crypto.KeyGenerator
import javax.crypto.spec.GCMParameterSpec

/**
 * Identity persistence (T20 slice): the nsec never touches disk raw —
 * it is AES-256-GCM wrapped by a hardware-bound key living in AndroidKeyStore.
 * Ciphertext+IV sit in app-private SharedPreferences.
 */
object IdentityStore {
    private const val PREFS = "proxi_identity"
    private const val KEY_ALIAS = "proxi_nsec_wrap"
    private const val PREF_CT = "nsec_ct"
    private const val PREF_IV = "nsec_iv"

    private fun wrappingKey(): javax.crypto.SecretKey {
        val ks = KeyStore.getInstance("AndroidKeyStore").apply { load(null) }
        (ks.getEntry(KEY_ALIAS, null) as? KeyStore.SecretKeyEntry)?.let { return it.secretKey }
        val kg = KeyGenerator.getInstance(KeyProperties.KEY_ALGORITHM_AES, "AndroidKeyStore")
        kg.init(
            KeyGenParameterSpec.Builder(
                KEY_ALIAS,
                KeyProperties.PURPOSE_ENCRYPT or KeyProperties.PURPOSE_DECRYPT
            )
                .setBlockModes(KeyProperties.BLOCK_MODE_GCM)
                .setEncryptionPaddings(KeyProperties.ENCRYPTION_PADDING_NONE)
                .setKeySize(256)
                .build()
        )
        return kg.generateKey()
    }

    fun save(ctx: Context, id: NostrIdentity) {
        val cipher = Cipher.getInstance("AES/GCM/NoPadding")
        cipher.init(Cipher.ENCRYPT_MODE, wrappingKey())
        val ct = cipher.doFinal(id.privateKey)
        ctx.getSharedPreferences(PREFS, Context.MODE_PRIVATE).edit()
            .putString(PREF_CT, Base64.encodeToString(ct, Base64.NO_WRAP))
            .putString(PREF_IV, Base64.encodeToString(cipher.iv, Base64.NO_WRAP))
            .apply()
    }

    fun load(ctx: Context): NostrIdentity? {
        val prefs = ctx.getSharedPreferences(PREFS, Context.MODE_PRIVATE)
        val ctb64 = prefs.getString(PREF_CT, null) ?: return null
        val ivb64 = prefs.getString(PREF_IV, null) ?: return null
        return try {
            val cipher = Cipher.getInstance("AES/GCM/NoPadding")
            cipher.init(
                Cipher.DECRYPT_MODE, wrappingKey(),
                GCMParameterSpec(128, Base64.decode(ivb64, Base64.NO_WRAP))
            )
            val sk = cipher.doFinal(Base64.decode(ctb64, Base64.NO_WRAP))
            NostrIdentity.fromSecretKey(sk)
        } catch (e: Exception) {
            null // corrupted prefs or wiped keystore — treat as logged out
        }
    }

    fun clear(ctx: Context) {
        ctx.getSharedPreferences(PREFS, Context.MODE_PRIVATE).edit().clear().apply()
    }
}
