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
fun EmptyState(onNewChat: () -> Unit = {}) {
    Box(
        modifier = Modifier.fillMaxSize().background(MessengerColors.ChatBg),
        contentAlignment = Alignment.Center
    ) {
        Column(horizontalAlignment = Alignment.CenterHorizontally) {
            Text("🛡️", fontSize = 64.sp)
            Spacer(modifier = Modifier.height(16.dp))
            Text("Proxi", color = Color.White, fontSize = 20.sp)
            Text("Выберите чат или начните новый", color = Color.Gray, fontSize = 14.sp)
            Text("P2P • E2E • Неубиваемо", color = Color.DarkGray, fontSize = 13.sp)
            Spacer(modifier = Modifier.height(20.dp))
            Button(
                onClick = onNewChat,
                modifier = Modifier
                    .testTag("btn_new_chat")
                    .semantics { contentDescription = "Новый чат" },
                colors = ButtonDefaults.buttonColors(containerColor = Color(0xFF3A7BD5))
            ) { Text("Новый чат") }
        }
    }
}
