package com.indestructible.messenger

import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
import androidx.compose.foundation.layout.*
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import com.indestructible.messenger.ui.theme.MessengerTheme
import com.indestructible.messenger.ui.navigation.MessengerNavHost

class MainActivity : ComponentActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        enableEdgeToEdge()
        VpnPermissionHolder.activity = this
        setContent {
            MessengerTheme {
                Surface(
                    modifier = Modifier.fillMaxSize(),
                    color = Color(0xFF0E1621)
                ) {
                    MessengerNavHost()
                }
            }
        }
    }

    override fun onActivityResult(requestCode: Int, resultCode: Int, data: android.content.Intent?) {
        super.onActivityResult(requestCode, resultCode, data)
        if (requestCode == VpnPermissionHolder.REQUEST_CODE) {
            VpnPermissionHolder.onResult(resultCode == android.app.Activity.RESULT_OK)
        }
    }
}

object VpnPermissionHolder {
    const val REQUEST_CODE = 100
    var activity: android.app.Activity? = null
    var onGranted: (() -> Unit)? = null

    fun requestThen(onGranted: () -> Unit) {
        val act = activity ?: return
        val intent = android.net.VpnService.prepare(act) ?: run { onGranted(); return }
        this.onGranted = onGranted
        try { act.startActivityForResult(intent, REQUEST_CODE) } catch (_: Exception) {}
    }

    fun onResult(granted: Boolean) {
        val cb = onGranted
        onGranted = null
        if (granted && cb != null) cb()
    }
}
