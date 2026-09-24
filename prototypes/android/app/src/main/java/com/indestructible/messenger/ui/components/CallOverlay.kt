@file:OptIn(ExperimentalMaterial3Api::class, ExperimentalFoundationApi::class)

package com.indestructible.messenger.ui.components
import androidx.compose.foundation.ExperimentalFoundationApi
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.combinedClickable
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalClipboardManager
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.AnnotatedString
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import android.content.Intent
import com.indestructible.messenger.messenger.Chat
import com.indestructible.messenger.messenger.ChatViewModel
import com.indestructible.messenger.messenger.Message
import com.indestructible.messenger.ui.theme.MessengerColors

@Composable
fun CallOverlay() {
    val st = com.indestructible.messenger.messenger.CallManager.state.value
    if (st == com.indestructible.messenger.messenger.CallManager.State.IDLE) return
    val peer = com.indestructible.messenger.messenger.CallManager.peerPub.value
    val context = androidx.compose.ui.platform.LocalContext.current

    Box(
        modifier = Modifier
            .fillMaxSize()
            .background(Color(0xCC0E1621)),
        contentAlignment = Alignment.Center
    ) {
        Column(horizontalAlignment = Alignment.CenterHorizontally) {
            Box(
                modifier = Modifier.size(96.dp).clip(RoundedCornerShape(48.dp))
                    .background(MessengerColors.UserAvatarBg),
                contentAlignment = Alignment.Center
            ) { Text("📞", fontSize = 40.sp) }
            Spacer(modifier = Modifier.height(16.dp))
            Text(
                peer.take(12) + "…",
                color = Color.White, fontSize = 18.sp, fontWeight = FontWeight.SemiBold
            )
            Spacer(modifier = Modifier.height(8.dp))
            Text(
                when (st) {
                    com.indestructible.messenger.messenger.CallManager.State.INCOMING -> "Входящий звонок"
                    com.indestructible.messenger.messenger.CallManager.State.OUTGOING -> "Вызов…"
                    com.indestructible.messenger.messenger.CallManager.State.CONNECTING -> "Соединение…"
                    com.indestructible.messenger.messenger.CallManager.State.CONNECTED -> "В разговоре"
                    else -> "Завершён"
                },
                color = Color.Gray, fontSize = 14.sp
            )
            Spacer(modifier = Modifier.height(32.dp))
            Row {
                when (st) {
                    com.indestructible.messenger.messenger.CallManager.State.INCOMING -> {
                        Button(
                            onClick = { com.indestructible.messenger.messenger.CallManager.acceptCall(context) },
                            colors = ButtonDefaults.buttonColors(containerColor = Color(0xFF4CAF50))
                        ) { Text("Принять", color = Color.White) }
                        Spacer(modifier = Modifier.width(16.dp))
                        Button(
                            onClick = { com.indestructible.messenger.messenger.CallManager.rejectCall() },
                            colors = ButtonDefaults.buttonColors(containerColor = Color(0xFFEF5350))
                        ) { Text("Отклонить", color = Color.White) }
                    }
                    com.indestructible.messenger.messenger.CallManager.State.CONNECTED -> {
                        val muted = com.indestructible.messenger.messenger.CallManager.muted.value
                        OutlinedButton(
                            onClick = { com.indestructible.messenger.messenger.CallManager.toggleMute() },
                            colors = ButtonDefaults.outlinedButtonColors(contentColor = Color.White)
                        ) { Text(if (muted) "🔇 Вкл. микрофон" else "🎙 Мьют") }
                        Spacer(modifier = Modifier.width(16.dp))
                        Button(
                            onClick = { com.indestructible.messenger.messenger.CallManager.endCall() },
                            colors = ButtonDefaults.buttonColors(containerColor = Color(0xFFEF5350))
                        ) { Text("Завершить", color = Color.White) }
                    }
                    else -> {
                        Button(
                            onClick = { com.indestructible.messenger.messenger.CallManager.endCall() },
                            colors = ButtonDefaults.buttonColors(containerColor = Color(0xFFEF5350))
                        ) { Text("Отмена", color = Color.White) }
                    }
                }
            }
        }
    }
}
