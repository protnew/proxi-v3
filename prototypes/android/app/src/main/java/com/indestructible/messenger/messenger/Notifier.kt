package com.indestructible.messenger.messenger

import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.content.Context
import android.content.Intent
import android.os.Build
import androidx.core.app.NotificationCompat
import com.indestructible.messenger.R

/** Heads-up notifications for incoming messages while the app is backgrounded. */
object Notifier {
    private const val CHANNEL_ID = "proxi_messages"
    private var channelCreated = false

    /** Set by MainActivity onResume/onPause. */
    @Volatile var appForeground = false

    fun notifyIncoming(ctx: Context, fromName: String, msgId: String) {
        if (appForeground) return
        val nm = ctx.getSystemService(Context.NOTIFICATION_SERVICE) as NotificationManager
        if (!channelCreated) {
            nm.createNotificationChannel(
                NotificationChannel(CHANNEL_ID, "Сообщения", NotificationManager.IMPORTANCE_HIGH)
            )
            channelCreated = true
        }
        val intent = Intent(ctx, Class.forName("com.indestructible.messenger.MainActivity")).apply {
            flags = Intent.FLAG_ACTIVITY_NEW_TASK or Intent.FLAG_ACTIVITY_CLEAR_TOP
        }
        val pi = PendingIntent.getActivity(
            ctx, 0, intent,
            PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE
        )
        val n = NotificationCompat.Builder(ctx, CHANNEL_ID)
            // Privacy: sender name only, never message text on the lockscreen.
            .setSmallIcon(R.drawable.ic_notification)
            .setContentTitle(fromName)
            .setContentText("Новое сообщение")
            .setPriority(NotificationCompat.PRIORITY_HIGH)
            .setContentIntent(pi)
            .setAutoCancel(true)
            .build()
        nm.notify(msgId.hashCode(), n)
    }
}
