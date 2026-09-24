package com.indestructible.messenger.ui.components
import java.text.SimpleDateFormat
import java.util.Date
import java.util.Locale

fun formatTime(ts: Long): String {
    val sdf = SimpleDateFormat("HH:mm", Locale.getDefault())
    return sdf.format(Date(ts))
}
