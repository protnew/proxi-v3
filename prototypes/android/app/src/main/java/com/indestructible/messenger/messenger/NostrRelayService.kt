package com.indestructible.messenger.messenger

import android.app.*
import android.content.Intent
import android.os.Build
import android.os.IBinder
import androidx.core.app.NotificationCompat
import com.indestructible.messenger.MainActivity
import com.indestructible.messenger.R
import com.indestructible.messenger.nostr.NostrRelay
import kotlinx.coroutines.*

/**
 * Foreground service that maintains Nostr relay connections
 * Keeps the app receiving messages even when minimized
 */
class NostrRelayService : Service() {

    private val relays = mutableListOf<NostrRelay>()
    private val scope = CoroutineScope(Dispatchers.IO + SupervisorJob())
    private var reconnectJob: Job? = null

    companion object {
        const val CHANNEL_ID = "messenger_notification_channel"
        const val NOTIFICATION_ID = 1001
        var isRunning = false
            private set
        var onMessage: ((Message) -> Unit)? = null

        // P4: local authenticated /nostr relay only — public relays leak
        // the social graph. Set from ChatViewModel after JWT auth.
        var relayBase: String = "ws://10.0.2.2:8090/nostr"
        var jwtToken: String = ""
    }

    override fun onCreate() {
        super.onCreate()
        createNotificationChannel()
        startForeground(NOTIFICATION_ID, createNotification("Подключение..."))
        isRunning = true
        connectRelays()
    }

    override fun onBind(intent: Intent?): IBinder? = null

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        return START_STICKY
    }

    override fun onDestroy() {
        scope.cancel()
        relays.forEach { it.disconnect() }
        isRunning = false
        super.onDestroy()
    }

    private fun connectRelays() {
        // P4/P9: single local relay with JWT — WS has no Authorization header.
        val tok = jwtToken
        val relayUrls = listOf(
            relayBase + if (tok.isNotEmpty()) "?token=$tok" else ""
        )

        for (url in relayUrls) {
            val relay = NostrRelay(url)
            relay.onEvent { event ->
                // Convert NostrEvent to Message
                if (event.kind == 14) {
                    val pTag = event.tags.find { it.isNotEmpty() && it[0] == "p" }
                    if (pTag != null) {
                        val msg = Message(
                            id = event.id,
                            from = event.pubkey,
                            to = pTag.getOrElse(1) { "" },
                            text = event.content,
                            timestamp = event.created_at * 1000,
                        )
                        onMessage?.invoke(msg)
                        showIncomingNotification(msg)
                    }
                }
            }
            relay.connect()
            relays.add(relay)
        }

        updateNotification("Подключён к ${relays.size} relay")

        // Periodic reconnect
        reconnectJob = scope.launch {
            while (isActive) {
                delay(60000)
                // Check connection health, reconnect if needed
            }
        }
    }

    private fun showIncomingNotification(msg: Message) {
        val intent = Intent(this, MainActivity::class.java).apply {
            flags = Intent.FLAG_ACTIVITY_NEW_TASK or Intent.FLAG_ACTIVITY_CLEAR_TOP
        }
        val pendingIntent = PendingIntent.getActivity(
            this, 0, intent,
            PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE
        )

        val notification = NotificationCompat.Builder(this, CHANNEL_ID)
            .setSmallIcon(R.drawable.ic_notification)
            .setContentTitle("Новое сообщение")
            // P24: no plaintext/ciphertext on lockscreen — sender only.
            .setContentText(msg.from.take(12) + "…")
            .setPriority(NotificationCompat.PRIORITY_HIGH)
            .setContentIntent(pendingIntent)
            .setAutoCancel(true)
            .build()

        val notificationManager = getSystemService(NotificationManager::class.java)
        notificationManager.notify(msg.id.hashCode(), notification)
    }

    private fun createNotificationChannel() {
        val channel = NotificationChannel(
            CHANNEL_ID,
            getString(R.string.messenger_notification_channel),
            NotificationManager.IMPORTANCE_LOW
        )
        val manager = getSystemService(NotificationManager::class.java)
        manager.createNotificationChannel(channel)
    }

    private fun createNotification(text: String): Notification {
        val intent = Intent(this, MainActivity::class.java)
        val pendingIntent = PendingIntent.getActivity(
            this, 0, intent,
            PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE
        )
        return NotificationCompat.Builder(this, CHANNEL_ID)
            .setContentTitle("Proxi")
            .setContentText(text)
            .setSmallIcon(R.drawable.ic_notification)
            .setContentIntent(pendingIntent)
            .build()
    }

    private fun updateNotification(text: String) {
        val notification = createNotification(text)
        val manager = getSystemService(NotificationManager::class.java)
        manager.notify(NOTIFICATION_ID, notification)
    }
}
