package com.indestructible.messenger.messenger

import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import android.util.Log

/**
 * Debug-only helper for APK↔PWA dual E2E: Compose OutlinedTextField does not reliably
 * accept `adb shell input text` into mutableState, so dual scripts send via broadcast.
 *
 * adb shell am broadcast -a com.indestructible.messenger.DEBUG_SEND \
 *   -n com.indestructible.messenger/.messenger.DebugSendReceiver \
 *   --es to HEX64 --es text "ping-..."
 */
class DebugSendReceiver : BroadcastReceiver() {
    override fun onReceive(context: Context, intent: Intent) {
        val vm = ChatViewModelHolder.instance
        if (vm == null) {
            Log.e(TAG, "no ChatViewModel")
            return
        }
        val to = intent.getStringExtra("to")?.trim().orEmpty()
        val text = intent.getStringExtra("text")?.trim().orEmpty()
        val from = intent.getStringExtra("from")?.trim().orEmpty().ifBlank { vm.debugPubKey() }
        if (to.length != 64 || text.isEmpty()) {
            Log.e(TAG, "bad extras toLen=${to.length} textLen=${text.length}")
            return
        }
        Log.i(TAG, "DEBUG_SEND to=${to.take(8)}… text='${text.take(40)}' from=${from.take(8)}…")
        vm.sendMessage(from, to, text)
    }

    companion object {
        private const val TAG = "DebugSend"
    }
}
