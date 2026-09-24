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
fun ChatListItem(chat: Chat, isSelected: Boolean, isOnline: Boolean, onClick: () -> Unit) {
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .clickable(onClick = onClick)
            .background(if (isSelected) MessengerColors.ActiveChat else Color.Transparent)
            .padding(12.dp),
        verticalAlignment = Alignment.CenterVertically
    ) {
        Box(contentAlignment = Alignment.BottomEnd) {
            Box(
                modifier = Modifier
                    .size(48.dp)
                    .clip(RoundedCornerShape(24.dp))
                    .background(
                        if (chat.type == Chat.Type.GROUP) MessengerColors.GroupAvatarBg
                        else MessengerColors.UserAvatarBg
                    ),
                contentAlignment = Alignment.Center
            ) {
                Text(chat.avatar, fontSize = 20.sp)
            }
            if (isOnline) {
                Box(
                    modifier = Modifier
                        .size(12.dp)
                        .clip(RoundedCornerShape(6.dp))
                        .background(Color(0xFF4CAF50))
                )
            }
        }

        Column(modifier = Modifier.weight(1f).padding(start = 12.dp)) {
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.SpaceBetween
            ) {
                Text(
                    chat.name,
                    color = Color.White,
                    fontSize = 14.sp,
                    fontWeight = FontWeight.SemiBold
                )
                Text(
                    formatTime(chat.lastActivity),
                    color = Color.Gray,
                    fontSize = 11.sp
                )
            }
            Text(
                chat.lastMessageText ?: chat.messages.lastOrNull()?.text ?: "Нет сообщений",
                color = Color.Gray,
                fontSize = 13.sp,
                maxLines = 1,
                overflow = TextOverflow.Ellipsis
            )
        }

        if (chat.unread > 0) {
            Badge { Text("${chat.unread}", fontSize = 10.sp) }
        }
    }
}
