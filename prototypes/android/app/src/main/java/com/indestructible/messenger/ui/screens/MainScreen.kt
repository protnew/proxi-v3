@file:OptIn(ExperimentalMaterial3Api::class, ExperimentalFoundationApi::class)

package com.indestructible.messenger.ui.screens
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.foundation.ExperimentalFoundationApi
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.combinedClickable
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalClipboardManager
import androidx.compose.ui.text.AnnotatedString
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.indestructible.messenger.messenger.Chat
import com.indestructible.messenger.messenger.Message
import com.indestructible.messenger.messenger.ChatViewModel
import com.indestructible.messenger.ui.theme.MessengerColors
import com.indestructible.messenger.ui.components.AttachButton
import com.indestructible.messenger.ui.components.CallOverlay
import com.indestructible.messenger.ui.components.ChatListItem
import com.indestructible.messenger.ui.components.DateSeparator
import com.indestructible.messenger.ui.components.EmptyState
import com.indestructible.messenger.ui.components.MessageBubble
import com.indestructible.messenger.ui.components.VoiceButton
import com.indestructible.messenger.ui.components.VpnStatusBar
import com.indestructible.messenger.ui.components.formatTime
import android.content.Intent
import androidx.activity.compose.BackHandler
import java.text.SimpleDateFormat
import java.util.Calendar
import java.util.Date
import java.util.Locale

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
    var replyTo by remember { mutableStateOf<Message?>(null) }
    var searchQuery by remember { mutableStateOf("") }

    // System back returns to the chat list instead of exiting the app.
    BackHandler(enabled = selectedChat != null) {
        selectedChat = null
        inputText = ""
        replyTo = null
        chatVM?.closeActiveChat()
    }

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
                        Text("☰", color = Color.White, fontSize = 20.sp)
                    }
                    OutlinedTextField(
                        value = searchQuery,
                        onValueChange = {
                            searchQuery = it
                            if (it.isBlank()) chatVM?.clearSearch() else chatVM?.searchMessages(it)
                        },
                        placeholder = { Text("Поиск", color = Color.Gray) },
                        modifier = Modifier.weight(1f).padding(horizontal = 8.dp)
                            .testTag("sidebar_search"),
                        singleLine = true,
                        colors = OutlinedTextFieldDefaults.colors(
                            unfocusedContainerColor = MessengerColors.InputBg,
                            focusedContainerColor = MessengerColors.InputBg,
                        )
                    )
                    TextButton(onClick = onNavigateToNewChat, modifier = Modifier.testTag("btn_new_chat_icon").semantics { contentDescription = "Новый чат" }) {
                        Text("✏️", fontSize = 18.sp)
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
                        "● " + status,
                        color = color,
                        fontSize = 10.sp,
                        modifier = Modifier.padding(horizontal = 16.dp, vertical = 2.dp)
                    )
                }

                LazyColumn(modifier = Modifier.weight(1f)) {
                    val q = searchQuery.trim()
                    val visibleChats = if (q.isEmpty()) chats
                        else chats.filter { it.name.contains(q, ignoreCase = true) }

                    items(visibleChats) { chat ->
                        ChatListItem(
                            chat = chat,
                            isSelected = selectedChat?.id == chat.id,
                            isOnline = chat.peerPubKey?.let { chatVM?.isOnline(it) } == true,
                            onClick = {
                                selectedChat = chat
                                replyTo = null
                                chatVM?.setActiveChat(chat.id)
                            }
                        )
                    }

                    // Message-search hits under the chat list while typing.
                    if (q.isNotEmpty() && chatVM != null && chatVM.searchResults.isNotEmpty()) {
                        item {
                            Text(
                                "Сообщения",
                                color = Color.Gray, fontSize = 11.sp,
                                modifier = Modifier.padding(horizontal = 16.dp, vertical = 6.dp)
                            )
                        }
                        items(chatVM.searchResults.take(10)) { msg ->
                            Column(
                                modifier = Modifier
                                    .fillMaxWidth()
                                    .clickable {
                                        val cid = "dm:" + if (msg.from == myPubKey) msg.to else msg.from
                                        chats.find { it.id == cid }?.let {
                                            selectedChat = it
                                            chatVM.setActiveChat(it.id)
                                        }
                                        searchQuery = ""
                                        chatVM.clearSearch()
                                    }
                                    .padding(horizontal = 16.dp, vertical = 8.dp)
                            ) {
                                Text(
                                    msg.text,
                                    color = Color.White, fontSize = 13.sp,
                                    maxLines = 1, overflow = TextOverflow.Ellipsis
                                )
                                Text(
                                    formatTime(msg.timestamp),
                                    color = Color.Gray, fontSize = 10.sp
                                )
                            }
                        }
                    }

                    if (chats.isEmpty() && q.isEmpty()) {
                        item { EmptyState(onNewChat = onNavigateToNewChat) }
                    }
                }

                VpnStatusBar()
            }
        }
    }

    @Composable
    fun ActiveChatPane(showBack: Boolean) {
        val chat = selectedChat ?: return
        val context = androidx.compose.ui.platform.LocalContext.current
        val vmMessages = chatVM?.messages ?: chat.messages
        ChatArea(
            chat = chat,
            messages = vmMessages,
            myPubKey = myPubKey,
            isOnline = chat.peerPubKey?.let { chatVM?.isOnline(it) } == true,
            isTyping = chat.peerPubKey?.let { chatVM?.typingPeers?.value?.contains(it) } == true,
            inputText = inputText,
            onInputTextChange = {
                inputText = it
                if (it.isNotBlank()) {
                    chat.peerPubKey?.let { p -> chatVM?.sendTyping(p) }
                }
            },
            replyTo = replyTo,
            onReplyTo = { replyTo = it },
            onDeleteMessage = { chatVM?.deleteMessage(it.id) },
            chatVM = chatVM,
            onSend = {
                if (inputText.isNotBlank()) {
                    val to = chat.id.removePrefix("dm:")
                    chatVM?.sendMessage(myPubKey, to, inputText, replyTo?.id)
                    inputText = ""
                    replyTo = null
                }
            },
            onBack = if (showBack) {{ selectedChat = null; inputText = ""; replyTo = null; chatVM?.closeActiveChat() }} else null,
            onAudioCall = {
                val peer = chat.peerPubKey ?: return@ChatArea
                if (android.os.Build.VERSION.SDK_INT >= 23 &&
                    context.checkSelfPermission(android.Manifest.permission.RECORD_AUDIO) !=
                        android.content.pm.PackageManager.PERMISSION_GRANTED
                ) {
                    (context as? android.app.Activity)?.requestPermissions(
                        arrayOf(android.Manifest.permission.RECORD_AUDIO), 44
                    )
                } else {
                    chatVM?.startAudioCall(peer)
                }
            },
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

    // Call overlay — drawn last so it floats above everything.
    CallOverlay()
}

/** Full-screen call UI driven by CallManager.state. */

fun ChatArea(
    chat: Chat,
    messages: List<Message>,
    myPubKey: String,
    isOnline: Boolean,
    isTyping: Boolean = false,
    inputText: String,
    onInputTextChange: (String) -> Unit,
    replyTo: Message?,
    onReplyTo: (Message?) -> Unit,
    onDeleteMessage: (Message) -> Unit,
    onSend: () -> Unit,
    onBack: (() -> Unit)? = null,
    onAudioCall: (() -> Unit)? = null,
    chatVM: ChatViewModel? = null,
) {
    Column(modifier = Modifier.fillMaxSize().background(MessengerColors.ChatBg)) {
        // Header
        Row(
            modifier = Modifier.fillMaxWidth().padding(12.dp),
            verticalAlignment = Alignment.CenterVertically
        ) {
            if (onBack != null) {
                TextButton(onClick = onBack, modifier = Modifier.testTag("chat_back").semantics { contentDescription = "Назад" }) {
                    Text("←", color = Color.White, fontSize = 20.sp)
                }
            }
            Box(
                modifier = Modifier.size(40.dp).clip(RoundedCornerShape(20.dp))
                    .background(MessengerColors.UserAvatarBg),
                contentAlignment = Alignment.Center
            ) {
                Text(chat.avatar, fontSize = 18.sp)
            }
            Column(modifier = Modifier.weight(1f).padding(start = 12.dp)) {
                Text(chat.name, color = Color.White, fontSize = 14.sp, fontWeight = FontWeight.SemiBold)
                Text(
                    if (isTyping) "печатает…"
                    else if (isOnline) "в сети" else "не в сети",
                    color = if (isTyping || isOnline) Color(0xFF4CAF50) else Color.Gray,
                    fontSize = 12.sp
                )
            }
            if (onAudioCall != null && chat.type == Chat.Type.DM) {
                TextButton(onClick = onAudioCall, modifier = Modifier.testTag("chat_call")) {
                    Text("📞", fontSize = 20.sp)
                }
            }
        }

        // Messages with day separators. Computed inline: `messages` is a
        // SnapshotStateList — its identity never changes on mutation, so a
        // remember(messages) key would freeze the grouping on first compose.
        val grouped = groupByDay(messages)
        LazyColumn(
            modifier = Modifier.weight(1f).padding(horizontal = 16.dp),
            reverseLayout = false,
        ) {
            grouped.forEach { (dayLabel, dayMessages) ->
                item { DateSeparator(dayLabel) }
                items(dayMessages) { msg ->
                    MessageBubble(
                        msg = msg,
                        isMine = msg.from == myPubKey,
                        isGroup = chat.type == Chat.Type.GROUP,
                        allMessages = messages,
                        chatVM = chatVM,
                        onReply = { onReplyTo(msg) },
                        onDelete = { onDeleteMessage(msg) },
                    )
                }
            }
        }

        // Reply chip
        if (replyTo != null) {
            Row(
                modifier = Modifier
                    .fillMaxWidth()
                    .background(MessengerColors.InputBg)
                    .padding(horizontal = 12.dp, vertical = 6.dp),
                verticalAlignment = Alignment.CenterVertically
            ) {
                Column(modifier = Modifier.weight(1f)) {
                    Text("Ответ на", color = Color(0xFF5EB5F7), fontSize = 11.sp)
                    Text(
                        replyTo.text,
                        color = Color.Gray, fontSize = 12.sp,
                        maxLines = 1, overflow = TextOverflow.Ellipsis
                    )
                }
                TextButton(onClick = { onReplyTo(null) }) {
                    Text("✕", color = Color.Gray, fontSize = 14.sp)
                }
            }
        }

        // Input: attach / text / voice
        Row(
            modifier = Modifier.fillMaxWidth().padding(8.dp),
            verticalAlignment = Alignment.Bottom
        ) {
            if (chatVM != null) {
                AttachButton(chatVM = chatVM, to = chat.id.removePrefix("dm:"))
            }
            OutlinedTextField(
                value = inputText,
                onValueChange = onInputTextChange,
                placeholder = { Text("Сообщение", color = Color.Gray) },
                modifier = Modifier.weight(1f).testTag("chat_composer").semantics { contentDescription = "Сообщение" },
                maxLines = 4,
                shape = RoundedCornerShape(20.dp),
                colors = OutlinedTextFieldDefaults.colors(
                    unfocusedContainerColor = MessengerColors.InputBg,
                    focusedContainerColor = MessengerColors.InputBg,
                )
            )
            if (inputText.isBlank() && chatVM != null) {
                VoiceButton(chatVM = chatVM, to = chat.id.removePrefix("dm:"))
            } else {
                TextButton(onClick = onSend, modifier = Modifier.testTag("chat_send").semantics { contentDescription = "Отправить" }) {
                    Text("➤", color = MessengerColors.Accent, fontSize = 20.sp)
                }
            }
        }
    }
}

private fun groupByDay(messages: List<Message>): List<Pair<String, List<Message>>> {
    val dayFmt = SimpleDateFormat("d MMMM", Locale("ru"))
    val today = Calendar.getInstance()
    return messages
        .groupBy { m ->
            val c = Calendar.getInstance().apply { timeInMillis = m.timestamp }
            c.get(Calendar.YEAR) * 1000 + c.get(Calendar.DAY_OF_YEAR)
        }
        .toSortedMap()
        .map { (dayKey, msgs) ->
            val c = Calendar.getInstance().apply {
                set(Calendar.YEAR, dayKey / 1000)
                set(Calendar.DAY_OF_YEAR, dayKey % 1000)
            }
            val label = when {
                dayKey == today.get(Calendar.YEAR) * 1000 + today.get(Calendar.DAY_OF_YEAR) -> "Сегодня"
                else -> dayFmt.format(c.time)
            }
            label to msgs
        }
}
