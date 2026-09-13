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
import com.indestructible.messenger.messenger.ChatViewModel
import com.indestructible.messenger.ui.theme.MessengerColors
import android.content.Intent
import androidx.compose.ui.platform.LocalContext

@Composable
fun MainScreen(
    onNavigateToSettings: () -> Unit,
    onNavigateToNewChat: () -> Unit,
    chats: List<Chat> = emptyList(),
    chatVM: ChatViewModel? = null,
    myPubKey: String = "me",
) {
    var selectedChat by remember { mutableStateOf<Chat?>(null) }
    var inputText by remember { mutableStateOf("") }

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
                    modifier = Modifier.fillMaxWidth().padding(12.dp),
                    horizontalArrangement = Arrangement.SpaceBetween,
                    verticalAlignment = Alignment.CenterVertically
                ) {
                    TextButton(onClick = onNavigateToSettings) {
                        Text("\u2630", color = Color.White, fontSize = 20.sp)
                    }
                    OutlinedTextField(
                        value = "",
                        onValueChange = {},
                        placeholder = { Text("\u041F\u043E\u0438\u0441\u043A", color = Color.Gray) },
                        modifier = Modifier.weight(1f).padding(horizontal = 8.dp),
                        singleLine = true,
                        colors = OutlinedTextFieldDefaults.colors(
                            unfocusedContainerColor = MessengerColors.InputBg,
                            focusedContainerColor = MessengerColors.InputBg,
                        )
                    )
                    TextButton(onClick = onNavigateToNewChat) {
                        Text("\u270F\uFE0F", fontSize = 18.sp)
                    }
                }

                // Connection status
                chatVM?.let { vm ->
                    val status = vm.connectionStatus.value
                    val color = when {
                        status.startsWith("connected") -> Color(0xFF4CAF50)
                        status.startsWith("connecting") -> Color(0xFFFFB74D)
                        else -> Color(0xFFEF5350)
                    }
                    Text(
                        "\u25CF " + status,
                        color = color,
                        fontSize = 10.sp,
                        modifier = Modifier.padding(horizontal = 16.dp, vertical = 2.dp)
                    )
                }

                // Chat list
                LazyColumn(modifier = Modifier.weight(1f)) {
                    items(chats) { chat ->
                        ChatListItem(
                            chat = chat,
                            isSelected = selectedChat?.id == chat.id,
                            onClick = {
                                selectedChat = chat
                                chatVM?.setActiveChat(chat.id)
                            }
                        )
                    }
                }

                // VPN bar
                VpnStatusBar()
            }
        }

        // Chat area
        if (selectedChat != null) {
            val vmMessages = chatVM?.messages ?: selectedChat!!.messages
            ChatArea(
                chat = selectedChat!!,
                messages = vmMessages,
                myPubKey = myPubKey,
                inputText = inputText,
                onInputTextChange = { inputText = it },
                onSend = {
                    if (inputText.isNotBlank()) {
                        val to = selectedChat!!.id.removePrefix("dm:")
                        chatVM?.sendMessage(myPubKey, to, inputText)
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
                chat.messages.lastOrNull()?.text ?: "\u041D\u0435\u0442 \u0441\u043E\u043E\u0431\u0449\u0435\u043D\u0438\u0439",
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
    messages: List<Message>,
    myPubKey: String,
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
                Text("\u0431\u044B\u043B(\u0430) \u043D\u0435\u0434\u0430\u0432\u043D\u043E", color = Color.Gray, fontSize = 12.sp)
            }
        }

        // Messages from ChatViewModel (live WS messages)
        LazyColumn(modifier = Modifier.weight(1f).padding(horizontal = 16.dp)) {
            items(messages) { msg ->
                MessageBubble(msg, isMine = msg.from == myPubKey)
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
                placeholder = { Text("\u0421\u043E\u043E\u0431\u0449\u0435\u043D\u0438\u0435", color = Color.Gray) },
                modifier = Modifier.weight(1f),
                maxLines = 4,
                colors = OutlinedTextFieldDefaults.colors(
                    unfocusedContainerColor = MessengerColors.InputBg,
                    focusedContainerColor = MessengerColors.InputBg,
                )
            )
            TextButton(onClick = onSend) {
                Text("\u27A4", color = MessengerColors.Accent, fontSize = 20.sp)
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
            Text("\uD83D\uDEE1\uFE0F", fontSize = 64.sp)
            Spacer(modifier = Modifier.height(16.dp))
            Text("Indestructible Messenger", color = Color.White, fontSize = 20.sp)
            Text("\u0412\u044B\u0431\u0435\u0440\u0438\u0442\u0435 \u0447\u0430\u0442 \u0438\u043B\u0438 \u043D\u0430\u0447\u043D\u0438\u0442\u0435 \u043D\u043E\u0432\u044B\u0439", color = Color.Gray, fontSize = 14.sp)
            Text("P2P \u2022 E2E \u2022 \u041D\u0435\u0443\u0431\u0438\u0432\u0430\u0435\u043C\u043E", color = Color.DarkGray, fontSize = 13.sp)
        }
    }
}

@Composable
fun VpnStatusBar() {
    val context = androidx.compose.ui.platform.LocalContext.current
    var vpnOn by remember { mutableStateOf(false) }
    var vpnStatus by remember { mutableStateOf("disconnected") }

    Row(
        modifier = Modifier.fillMaxWidth().background(MessengerColors.DarkBg).padding(8.dp),
        horizontalArrangement = Arrangement.SpaceBetween,
        verticalAlignment = Alignment.CenterVertically
    ) {
        Text("\uD83D\uDEE1\uFE0F VPN", color = Color.White, fontSize = 12.sp)
        val statusColor = if (vpnOn) Color(0xFF4CAF50) else Color(0xFFFF6B6B)
        Text(
            if (vpnOn) "\u0412\u043A\u043B\u044E\u0447\u0435\u043D" else "\u041E\u0442\u043A\u043B\u044E\u0447\u0451\u043D",
            color = statusColor,
            fontSize = 12.sp
        )
        Switch(
            checked = vpnOn,
            onCheckedChange = { enabled ->
                vpnOn = enabled
                vpnStatus = if (enabled) "connecting" else "disconnected"
                // MOB-101c: Start/stop VPN via Intent
                val ctx = context
                if (enabled) {
                    val intent = Intent(ctx, com.indestructible.messenger.vpn.VpnService::class.java)
                    intent.action = "START_VPN"
                    ctx.startService(intent)
                    vpnStatus = "connected"
                } else {
                    val intent = Intent(ctx, com.indestructible.messenger.vpn.VpnService::class.java)
                    intent.action = "STOP_VPN"
                    ctx.startService(intent)
                    vpnStatus = "disconnected"
                }
                // For now: toggle state + visual feedback
            },
            colors = SwitchDefaults.colors(
                checkedThumbColor = Color(0xFF4CAF50),
                checkedTrackColor = Color(0xFF2E7D32),
                uncheckedThumbColor = Color(0xFFEF5350),
            )
        )
    }
}

private fun formatTime(ts: Long): String {
    val sdf = java.text.SimpleDateFormat("HH:mm", java.util.Locale.getDefault())
    return sdf.format(java.util.Date(ts))
}
