package io.github.untitled572.pichub.android

import android.content.Context
import android.net.Uri
import android.provider.DocumentsContract
import android.security.keystore.KeyGenParameterSpec
import android.security.keystore.KeyProperties
import android.util.Base64
import androidx.core.content.edit
import java.security.KeyStore
import javax.crypto.Cipher
import javax.crypto.KeyGenerator
import javax.crypto.SecretKey
import javax.crypto.spec.GCMParameterSpec

internal data class ViewerOptions(
    val saveTree: Uri? = null,
    val fillScreen: Boolean = false,
    val showDetails: Boolean = false
)

internal class ConnectionPreferences(context: Context) {
    private val preferences = context.getSharedPreferences("server", Context.MODE_PRIVATE)

    val address: String get() = preferences.getString("address", "").orEmpty()

    fun loadViewerOptions() = ViewerOptions(
        saveTree = preferences.getString("save_tree", null)?.let(Uri::parse)
            ?.takeIf { it.scheme == "content" && DocumentsContract.isTreeUri(it) },
        fillScreen = preferences.getBoolean("fill_screen", false),
        showDetails = preferences.getBoolean("show_details", false)
    )

    fun loadKey(): String {
        val value = preferences.getString("app_key", null) ?: return ""
        return runCatching {
            val (iv, content) = value.split(':', limit = 2).map { Base64.decode(it, Base64.NO_WRAP) }
            val cipher = Cipher.getInstance(TRANSFORMATION)
            cipher.init(Cipher.DECRYPT_MODE, getKey(), GCMParameterSpec(128, iv))
            cipher.doFinal(content).toString(Charsets.UTF_8)
        }.getOrDefault("")
    }

    fun save(address: String, appKey: String, options: ViewerOptions) {
        val encrypted = if (appKey.isNotEmpty()) {
            val cipher = Cipher.getInstance(TRANSFORMATION)
            cipher.init(Cipher.ENCRYPT_MODE, getKey())
            listOf(cipher.iv, cipher.doFinal(appKey.toByteArray()))
                .joinToString(":") { Base64.encodeToString(it, Base64.NO_WRAP) }
        } else {
            null
        }
        preferences.edit {
            putString("address", address)
            putString("app_key", encrypted)
            putString("save_tree", options.saveTree?.toString())
            putBoolean("fill_screen", options.fillScreen)
            putBoolean("show_details", options.showDetails)
        }
    }

    private fun getKey(): SecretKey {
        val store = KeyStore.getInstance("AndroidKeyStore").apply { load(null) }
        (store.getKey(ALIAS, null) as? SecretKey)?.let { return it }
        val generator = KeyGenerator.getInstance(KeyProperties.KEY_ALGORITHM_AES, "AndroidKeyStore")
        generator.init(
            KeyGenParameterSpec.Builder(ALIAS, KeyProperties.PURPOSE_ENCRYPT or KeyProperties.PURPOSE_DECRYPT)
                .setBlockModes(KeyProperties.BLOCK_MODE_GCM)
                .setEncryptionPaddings(KeyProperties.ENCRYPTION_PADDING_NONE)
                .build()
        )
        return generator.generateKey()
    }

    private companion object {
        const val ALIAS = "pichub_app_key"
        const val TRANSFORMATION = "AES/GCM/NoPadding"
    }
}
