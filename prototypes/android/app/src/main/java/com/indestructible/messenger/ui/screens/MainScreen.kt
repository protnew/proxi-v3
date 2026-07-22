@file:OptIn(ExperimentalMaterial3Api::class)

package com.indestructible.messenger.ui.screens

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.indestructible.messenger.messenger.Chat
import com.indestructible.messenger.messenger.Message
import com.indestructible.messenger.ui.theme.MessengerColors

@Composable
fun MainScreen(
    onNavigateToSettings: () -> Unit,
    onNavigateToNewChat: () -> Unit,
) {
    var selectedChat by remember { mutableStateOf<Chat?>(null) }
    var inputText by remember { mutableStateOf("") }
    var chats by remember { mutableStateOf(sampleChats()) }

    Row(modifier = Modifier.fillMaxSize()) {
        // Sidebar
        Box(
            modifier = Modifier
                .width(320.dp)
                .fillMaxHeight()
                .background(MessengerColors.SidebarBg)
        ) {
            Column {
                // Header
                Row(
                    modifier = Modifier
                        .fillMaxWidth()
                        .padding(12.dp),
                    horizontalArrangement = Arrangement.SpaceBetween,
                    verticalAlignment = Alignment.CenterVertically
                ) {
                    TextButton(onClick = onNavigateToSettings) {
                        Text("☰", color = Color.White, fontSize = 20.sp)
                    }
                    OutlinedTextField(
                        value = "",
                        onValueChange = {},
                        placeholder = { Text("Поиск", color = Color.Gray) },
                        modifier = Modifier.weight(1f).padding(horizontal = 8.dp),
                        singleLine = true,
                        colors = OutlinedTextFieldDefaults.colors(
                            unfocusedContainerColor = MessengerColors.InputBg,
                            focusedContainerColor = MessengerColors.InputBg,
                        )
                    )
                    TextButton(onClick = onNavigateToNewChat) {
                        Text("✏️", fontSize = 18.sp)
                    }
                }

                // Chat list
                LazyColumn(modifier = Modifier.weight(1f)) {
                    items(chats) { chat ->
                        ChatListItem(
                            chat = chat,
                            isSelected = selectedChat?.id == chat.id,
                            onClick = { selectedChat = chat }
                        )
                    }
                }

                // VPN bar
                VpnStatusBar()
            }
        }

        // Chat area
        if (selectedChat != null) {
            ChatArea(
                chat = selectedChat!!,
                inputText = inputText,
                onInputTextChange = { inputText = it },
                onSend = {
                    if (inputText.isNotBlank()) {
                        // TODO: Send via Nostr
                        inputText = ""
                    }
                }
            )
        } else {
            EmptyState()
        }
    }
}

@Composable
fun ChatListItem(chat: Chat, isSelected: Boolean, onClick: () -> Unit) {
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .clickable(onClick = onClick)
            .background(if (isSelected) MessengerColors.ActiveChat else Color.Transparent)
            .padding(12.dp),
        verticalAlignment = Alignment.CenterVertically
    ) {
        // Avatar
        Box(
            modifier = Modifier
                .size(48.dp)
                .background(
                    if (chat.type == Chat.Type.GROUP) MessengerColors.GroupAvatarBg
                    else MessengerColors.UserAvatarBg
                ),
            contentAlignment = Alignment.Center
        ) {
            Text(chat.avatar, fontSize = 20.sp)
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
                    fontWeight = androidx.compose.ui.text.font.FontWeight.SemiBold
                )
                Text(
                    formatTime(chat.lastActivity),
                    color = Color.Gray,
                    fontSize = 11.sp
                )
            }
            Text(
                chat.messages.lastOrNull()?.text ?: "Нет сообщений",
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

@Composable
fun ChatArea(
    chat: Chat,
    inputText: String,
    onInputTextChange: (String) -> Unit,
    onSend: () -> Unit,
) {
    Column(modifier = Modifier.fillMaxSize().background(MessengerColors.ChatBg)) {
        // Header
        Row(
            modifier = Modifier.fillMaxWidth().padding(12.dp),
            verticalAlignment = Alignment.CenterVertically
        ) {
            Box(modifier = Modifier.size(40.dp).background(MessengerColors.UserAvatarBg), contentAlignment = Alignment.Center) {
                Text(chat.avatar, fontSize = 18.sp)
            }
            Column(modifier = Modifier.weight(1f).padding(start = 12.dp)) {
                Text(chat.name, color = Color.White, fontSize = 14.sp, fontWeight = androidx.compose.ui.text.font.FontWeight.SemiBold)
                Text("был(а) недавно", color = Color.Gray, fontSize = 12.sp)
            }
        }

        // Messages
        LazyColumn(modifier = Modifier.weight(1f).padding(horizontal = 16.dp)) {
            items(chat.messages) { msg ->
                MessageBubble(msg, isMine = false) // TODO: compare with own pubkey
            }
        }

        // Input
        Row(
            modifier = Modifier.fillMaxWidth().padding(8.dp),
            verticalAlignment = Alignment.Bottom
        ) {
            OutlinedTextField(
                value = inputText,
                onValueChange = onInputTextChange,
                placeholder = { Text("Сообщение", color = Color.Gray) },
                modifier = Modifier.weight(1f),
                maxLines = 4,
                colors = OutlinedTextFieldDefaults.colors(
                    unfocusedContainerColor = MessengerColors.InputBg,
                    focusedContainerColor = MessengerColors.InputBg,
                )
            )
            TextButton(onClick = onSend) {
                Text("➤", color = MessengerColors.Accent, fontSize = 20.sp)
            }
        }
    }
}

@Composable
fun MessageBubble(msg: Message, isMine: Boolean) {
    Row(
        modifier = Modifier.fillMaxWidth().padding(vertical = 2.dp),
        horizontalArrangement = if (isMine) Arrangement.End else Arrangement.Start
    ) {
        Box(
            modifier = Modifier
                .widthIn(max = 280.dp)
                .background(if (isMine) MessengerColors.MineBubble else MessengerColors.TheirBubble)
                .padding(8.dp)
        ) {
            Column {
                Text(msg.text, color = Color.White, fontSize = 14.sp, lineHeight = 20.sp)
                Row(
                    modifier = Modifier.fillMaxWidth(),
                    horizontalArrangement = Arrangement.End
                ) {
                    Text(
                        formatTime(msg.timestamp),
                        color = Color.White.copy(alpha = 0.4f),
                        fontSize = 10.sp
                    )
                }
            }
        }
    }
}

@Composable
fun EmptyState() {
    Box(
        modifier = Modifier.fillMaxSize().background(MessengerColors.ChatBg),
        contentAlignment = Alignment.Center
    ) {
        Column(horizontalAlignment = Alignment.CenterHorizontally) {
            Text("🛡️", fontSize = 64.sp)
            Spacer(modifier = Modifier.height(16.dp))
            Text("Indestructible Messenger", color = Color.White, fontSize = 20.sp)
            Text("Выберите чат или начните новый", color = Color.Gray, fontSize = 14.sp)
            Text("P2P • E2E • Неубиваемо", color = Color.DarkGray, fontSize = 13.sp)
        }
    }
}

@Composable
fun VpnStatusBar() {
    Row(
        modifier = Modifier.fillMaxWidth().background(MessengerColors.DarkBg).padding(8.dp),
        horizontalArrangement = Arrangement.SpaceBetween
    ) {
        Text("🛡️ VPN", color = Color.White, fontSize = 12.sp)
        Text("Отключён", color = Color(0xFFFF6B6B), fontSize = 12.sp)
    }
}

private fun formatTime(ts: Long): String {
    val sdf = java.text.SimpleDateFormat("HH:mm", java.util.Locale.getDefault())
    return sdf.format(java.util.Date(ts))
}

private fun sampleChats(): List<Chat> = emptyList()
