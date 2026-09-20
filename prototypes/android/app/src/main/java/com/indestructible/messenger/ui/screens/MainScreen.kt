@file:OptIn(ExperimentalMaterial3Api::class)

package com.indestructible.messenger.ui.screens
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
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

    @Composable
    fun SidebarPane(modifier: Modifier = Modifier) {
        Box(
            modifier = modifier
                .fillMaxHeight()
                .background(MessengerColors.SidebarBg)
        ) {
            Column {
                Row(
                    modifier = Modifier.fillMaxWidth().padding(12.dp),
                    horizontalArrangement = Arrangement.SpaceBetween,
                    verticalAlignment = Alignment.CenterVertically
                ) {
                    TextButton(onClick = onNavigateToSettings, modifier = Modifier.testTag("btn_settings").semantics { contentDescription = "Настройки" }) {
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
                    TextButton(onClick = onNavigateToNewChat, modifier = Modifier.testTag("btn_new_chat_icon").semantics { contentDescription = "Новый чат" }) {
                        Text("\u270F\uFE0F", fontSize = 18.sp)
                    }
                }

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
                    if (chats.isEmpty()) {
                        item {
                            EmptyState(onNewChat = onNavigateToNewChat)
                        }
                    }
                }

                VpnStatusBar()
            }
        }
    }

    @Composable
    fun ActiveChatPane(showBack: Boolean) {
        val chat = selectedChat ?: return
        val vmMessages = chatVM?.messages ?: chat.messages
        ChatArea(
            chat = chat,
            messages = vmMessages,
            myPubKey = myPubKey,
            inputText = inputText,
            onInputTextChange = { inputText = it },
            onSend = {
                if (inputText.isNotBlank()) {
                    val to = chat.id.removePrefix("dm:")
                    chatVM?.sendMessage(myPubKey, to, inputText)
                    inputText = ""
                }
            },
            onBack = if (showBack) {{ selectedChat = null; inputText = "" }} else null,
        )
    }

    // Phone (<600dp): list OR chat full-width. Tablet: side-by-side.
    BoxWithConstraints(modifier = Modifier.fillMaxSize()) {
        val phone = maxWidth < 600.dp
        if (phone) {
            if (selectedChat != null) {
                ActiveChatPane(showBack = true)
            } else {
                SidebarPane(modifier = Modifier.fillMaxSize())
            }
        } else {
            Row(modifier = Modifier.fillMaxSize()) {
                SidebarPane(modifier = Modifier.width(320.dp))
                if (selectedChat != null) {
                    ActiveChatPane(showBack = false)
                } else {
                    EmptyState(onNewChat = onNavigateToNewChat)
                }
            }
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
    onBack: (() -> Unit)? = null,
) {
    Column(modifier = Modifier.fillMaxSize().background(MessengerColors.ChatBg)) {
        // Header
        Row(
            modifier = Modifier.fillMaxWidth().padding(12.dp),
            verticalAlignment = Alignment.CenterVertically
        ) {
            if (onBack != null) {
                TextButton(onClick = onBack, modifier = Modifier.testTag("chat_back").semantics { contentDescription = "Назад" }) {
                    Text("\u2190", color = Color.White, fontSize = 20.sp)
                }
            }
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
                modifier = Modifier.weight(1f).testTag("chat_composer").semantics { contentDescription = "\u0421\u043E\u043E\u0431\u0449\u0435\u043D\u0438\u0435" },
                maxLines = 4,
                colors = OutlinedTextFieldDefaults.colors(
                    unfocusedContainerColor = MessengerColors.InputBg,
                    focusedContainerColor = MessengerColors.InputBg,
                )
            )
            TextButton(onClick = onSend, modifier = Modifier.testTag("chat_send").semantics { contentDescription = "\u041E\u0442\u043F\u0440\u0430\u0432\u0438\u0442\u044C" }) {
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

@Composable
fun VpnStatusBar() {
    val context = androidx.compose.ui.platform.LocalContext.current
    var vpnOn by remember { mutableStateOf(false) }
    var vpnStatus by remember { mutableStateOf("disconnected") }
    var exitHost by remember { mutableStateOf("") }
    var exitOn by remember { mutableStateOf(false) }

    fun startTunIntent(ctx: android.content.Context) {
        val parts = exitHost.trim().split(":")
        if (parts.size == 2) {
            com.indestructible.messenger.vpn.TunVpnService.exitProxyHost = parts[0].trim()
            com.indestructible.messenger.vpn.TunVpnService.exitProxyPort = parts[1].trim().toIntOrNull() ?: 10808
            com.indestructible.messenger.vpn.TunVpnService.t42 = com.indestructible.messenger.vpn.TunVpnService.t42.copy(socksHost = parts[0].trim(), socksPort = parts[1].trim().toIntOrNull() ?: 10808)
        }
        val intent = Intent(ctx, com.indestructible.messenger.vpn.TunVpnService::class.java)
        intent.action = "START_VPN"
        ctx.startService(intent)
    }

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
        Column(modifier = Modifier.fillMaxWidth().padding(horizontal = 8.dp)) {
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.SpaceBetween,
                verticalAlignment = Alignment.CenterVertically
            ) {
                Text("\uD83D\uDEE1\uFE0F VPN", color = Color.White, fontSize = 12.sp)
                val statusColor = if (vpnOn) Color(0xFF4CAF50) else Color(0xFFFF6B6B)
                Text(
                    if (vpnOn) "\u0412\u043A\u043B\u044E\u0447\u0435\u043D" else "\u041E\u0442\u043A\u043B\u044E\u0447\u0435\u043D",
                    color = statusColor,
                    fontSize = 12.sp
                )
                Switch(
                    checked = vpnOn,
                    onCheckedChange = { enabled ->
                        vpnOn = enabled
                        vpnStatus = if (enabled) "connecting" else "disconnected"
                        val ctx = context
                        if (enabled) {
                            val prepare = android.net.VpnService.prepare(ctx)
                            if (prepare != null) {
                                vpnOn = false
                                vpnStatus = "needs permission"
                                com.indestructible.messenger.VpnPermissionHolder.requestThen {
                                    startTunIntent(ctx)
                                    vpnOn = true
                                    vpnStatus = "connected (after permission)"
                                }
                            } else {
                                startTunIntent(ctx)
                                vpnStatus = if (exitHost.isBlank()) "connected (self-exit)" else "tun → ${exitHost.trim()}"
                            }
                        } else {
                            val intent = Intent(ctx, com.indestructible.messenger.vpn.TunVpnService::class.java)
                            intent.action = "STOP_VPN"
                            ctx.startService(intent)
                            vpnStatus = "disconnected"
                        }
                    },
                    colors = SwitchDefaults.colors(
                        checkedThumbColor = Color(0xFF4CAF50),
                        checkedTrackColor = Color(0xFF2E7D32),
                        uncheckedThumbColor = Color(0xFFEF5350),
                    )
                )
            }
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.SpaceBetween,
                verticalAlignment = Alignment.CenterVertically
            ) {
                Text("\uD83D\uDDA9 Exit: ", color = Color.Gray, fontSize = 11.sp)
                OutlinedTextField(
                    value = exitHost,
                    onValueChange = { exitHost = it },
                    placeholder = { Text("192.168.1.50:10808", color = Color.DarkGray, fontSize = 11.sp) },
                    singleLine = true,
                    textStyle = androidx.compose.ui.text.TextStyle(fontSize = 11.sp, color = Color.White),
                    modifier = Modifier.width(150.dp).height(52.dp),
                    colors = OutlinedTextFieldDefaults.colors(
                        unfocusedTextColor = Color.White,
                        focusedTextColor = Color.White,
                        unfocusedBorderColor = Color.DarkGray,
                        focusedBorderColor = Color(0xFF4CAF50)
                    )
                )
                TextButton(onClick = {
                    exitOn = !exitOn
                    if (exitOn) {
                        com.indestructible.messenger.vpn.Socks5ExitServer.bindHost = "0.0.0.0"
                        com.indestructible.messenger.vpn.Socks5ExitServer.bindPort = 10808
                        val ok = com.indestructible.messenger.vpn.Socks5ExitServer.start()
                        vpnStatus = if (ok) "exit :${com.indestructible.messenger.vpn.Socks5ExitServer.bindPort}" else "exit start failed"
                    } else {
                        com.indestructible.messenger.vpn.Socks5ExitServer.stop()
                        vpnStatus = "exit off"
                    }
                }) {
                    Text(
                        if (exitOn) "\u2623 \u042D\u0442\u043E\u0442 \u0442\u0435\u043B \u2014 \u0432\u044B\u0445\u043E\u0434: \u0412\u041A\u041B" else "\u2623 \u042D\u0442\u043E\u0442 \u0442\u0435\u043B \u2014 \u0432\u044B\u0445\u043E\u0434",
                        color = if (exitOn) Color(0xFF4CAF50) else Color.Gray,
                        fontSize = 10.sp
                    )
                }
            }
            Text(vpnStatus, color = Color.DarkGray, fontSize = 10.sp, modifier = Modifier.padding(top = 2.dp))
        }
    }
}

private fun formatTime(ts: Long): String {
    val sdf = java.text.SimpleDateFormat("HH:mm", java.util.Locale.getDefault())
    return sdf.format(java.util.Date(ts))
}


