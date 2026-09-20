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

@Composable
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

/** 📎 — pick any file via SAF, upload authed, send E2E descriptor. */
@Composable
fun AttachButton(chatVM: ChatViewModel, to: String) {
    val context = androidx.compose.ui.platform.LocalContext.current
    val picker = androidx.activity.compose.rememberLauncherForActivityResult(
        androidx.activity.result.contract.ActivityResultContracts.GetContent()
    ) { uri ->
        if (uri != null) {
            Thread {
                try {
                    val cr = context.contentResolver
                    val name = cr.query(uri, null, null, null, null)?.use { c ->
                        val idx = c.getColumnIndex(android.provider.OpenableColumns.DISPLAY_NAME)
                        if (c.moveToFirst() && idx >= 0) c.getString(idx) else "file"
                    } ?: "file"
                    val bytes = cr.openInputStream(uri)?.use { it.readBytes() } ?: return@Thread
                    val mime = cr.getType(uri) ?: "application/octet-stream"
                    chatVM.sendAttachment(to, name, bytes, mime)
                } catch (e: Exception) {
                    android.util.Log.e("Attach", "pick failed", e)
                }
            }.start()
        }
    }
    TextButton(
        onClick = { picker.launch("*/*") },
        modifier = Modifier.testTag("chat_attach")
    ) {
        Text("📎", fontSize = 20.sp)
    }
}

/** 🎤 — tap to start/stop recording, sends a VOICE message on stop. */
@Composable
fun VoiceButton(chatVM: ChatViewModel, to: String) {
    val context = androidx.compose.ui.platform.LocalContext.current
    var recording by remember { mutableStateOf(false) }
    var recorder by remember { mutableStateOf<android.media.MediaRecorder?>(null) }
    var outFile by remember { mutableStateOf<java.io.File?>(null) }
    var startedAt by remember { mutableStateOf(0L) }

    TextButton(
        onClick = {
            if (!recording) {
                if (android.os.Build.VERSION.SDK_INT >= 23 &&
                    context.checkSelfPermission(android.Manifest.permission.RECORD_AUDIO) !=
                        android.content.pm.PackageManager.PERMISSION_GRANTED
                ) {
                    (context as? android.app.Activity)?.requestPermissions(
                        arrayOf(android.Manifest.permission.RECORD_AUDIO), 43
                    )
                    return@TextButton
                }
                val f = java.io.File(
                    context.cacheDir, "voice_${System.currentTimeMillis()}.m4a"
                )
                val r = if (android.os.Build.VERSION.SDK_INT >= 31)
                    android.media.MediaRecorder(context) else @Suppress("DEPRECATION")
                    android.media.MediaRecorder()
                r.setAudioSource(android.media.MediaRecorder.AudioSource.MIC)
                r.setOutputFormat(android.media.MediaRecorder.OutputFormat.MPEG_4)
                r.setAudioEncoder(android.media.MediaRecorder.AudioEncoder.AAC)
                r.setOutputFile(f.absolutePath)
                r.prepare(); r.start()
                recorder = r; outFile = f
                startedAt = System.currentTimeMillis()
                recording = true
            } else {
                val dur = ((System.currentTimeMillis() - startedAt) / 1000).toInt()
                try { recorder?.stop() } catch (_: Exception) {}
                recorder?.release(); recorder = null
                recording = false
                val f = outFile
                if (f != null && f.exists() && dur > 0) {
                    chatVM.sendAttachment(
                        to, f.name, f.readBytes(),
                        "audio/mp4", voiceDurationSec = dur
                    )
                }
            }
        },
        modifier = Modifier.testTag("chat_voice")
    ) {
        Text(if (recording) "⏹" else "🎤", fontSize = 20.sp)
    }
}

@Composable
fun MessageBubble(
    msg: Message,
    isMine: Boolean,
    isGroup: Boolean = false,
    allMessages: List<Message>,
    chatVM: ChatViewModel?,
    onReply: () -> Unit,
    onDelete: () -> Unit,
) {
    val clipboard = LocalClipboardManager.current
    val context = androidx.compose.ui.platform.LocalContext.current
    var menuOpen by remember { mutableStateOf(false) }
    var downloading by remember { mutableStateOf(false) }
    val quoted = msg.replyTo?.let { rid -> allMessages.find { it.id == rid } }

    Row(
        modifier = Modifier.fillMaxWidth().padding(vertical = 2.dp),
        horizontalArrangement = if (isMine) Arrangement.End else Arrangement.Start
    ) {
        Box {
            Box(
                modifier = Modifier
                    .widthIn(max = 280.dp)
                    .clip(
                        RoundedCornerShape(
                            topStart = 16.dp,
                            topEnd = 16.dp,
                            bottomStart = if (isMine) 16.dp else 4.dp,
                            bottomEnd = if (isMine) 4.dp else 16.dp,
                        )
                    )
                    .background(if (isMine) MessengerColors.MineBubble else MessengerColors.TheirBubble)
                    .combinedClickable(
                        onClick = {
                            if ((msg.type == Message.Type.FILE || msg.type == Message.Type.VOICE) &&
                                msg.fileUrl != null && chatVM != null && !downloading
                            ) {
                                downloading = true
                                chatVM.downloadAttachment(msg.fileUrl, msg.fileName ?: "file") { f ->
                                    downloading = false
                                    if (f == null) {
                                        android.widget.Toast.makeText(
                                            context, "Не удалось скачать", android.widget.Toast.LENGTH_SHORT
                                        ).show()
                                    } else if (msg.type == Message.Type.VOICE) {
                                        try {
                                            android.media.MediaPlayer().apply {
                                                setDataSource(f.absolutePath)
                                                prepare(); start()
                                            }
                                        } catch (_: Exception) {}
                                    } else {
                                        android.widget.Toast.makeText(
                                            context, "Сохранено: ${f.absolutePath}",
                                            android.widget.Toast.LENGTH_LONG
                                        ).show()
                                    }
                                }
                            }
                        },
                        onLongClick = { menuOpen = true },
                    )
                    .padding(horizontal = 12.dp, vertical = 8.dp)
            ) {
                Column {
                    if (isGroup && !isMine) {
                        Text(
                            msg.from.take(8),
                            color = Color(0xFF5EB5F7),
                            fontSize = 11.sp,
                            fontWeight = FontWeight.SemiBold
                        )
                        Spacer(modifier = Modifier.height(2.dp))
                    }
                    if (quoted != null) {
                        Column(
                            modifier = Modifier
                                .fillMaxWidth()
                                .clip(RoundedCornerShape(6.dp))
                                .background(Color.White.copy(alpha = 0.08f))
                                .padding(6.dp)
                        ) {
                            Text(
                                quoted.text,
                                color = Color.White.copy(alpha = 0.7f),
                                fontSize = 11.sp,
                                maxLines = 2,
                                overflow = TextOverflow.Ellipsis
                            )
                        }
                        Spacer(modifier = Modifier.height(4.dp))
                    }
                    when (msg.type) {
                        Message.Type.VOICE -> Row(verticalAlignment = Alignment.CenterVertically) {
                            Text("🎤", fontSize = 18.sp)
                            Spacer(modifier = Modifier.width(8.dp))
                            Text(
                                "Голосовое ${(msg.voiceDuration ?: 0)}с",
                                color = Color.White, fontSize = 14.sp
                            )
                            if (downloading) Text(" …", color = Color.Gray, fontSize = 12.sp)
                        }
                        Message.Type.FILE -> Row(verticalAlignment = Alignment.CenterVertically) {
                            Text("📎", fontSize = 18.sp)
                            Spacer(modifier = Modifier.width(8.dp))
                            Column {
                                Text(msg.fileName ?: msg.text, color = Color.White, fontSize = 14.sp)
                                Text(
                                    "${((msg.fileSize ?: 0) / 1024)} КБ",
                                    color = Color.White.copy(alpha = 0.5f), fontSize = 11.sp
                                )
                            }
                            if (downloading) Text(" …", color = Color.Gray, fontSize = 12.sp)
                        }
                        else -> Text(msg.text, color = Color.White, fontSize = 14.sp, lineHeight = 20.sp)
                    }
                    Row(
                        modifier = Modifier.fillMaxWidth(),
                        horizontalArrangement = Arrangement.End,
                        verticalAlignment = Alignment.CenterVertically,
                    ) {
                        Text(
                            formatTime(msg.timestamp),
                            color = Color.White.copy(alpha = 0.4f),
                            fontSize = 10.sp
                        )
                        if (isMine) {
                            Text(
                                if (msg.delivered) " ✓" else " …",
                                color = if (msg.delivered) Color(0xFF5EB5F7)
                                        else Color.White.copy(alpha = 0.4f),
                                fontSize = 10.sp
                            )
                        }
                    }
                }
            }

            DropdownMenu(expanded = menuOpen, onDismissRequest = { menuOpen = false }) {
                DropdownMenuItem(
                    text = { Text("Копировать") },
                    onClick = {
                        clipboard.setText(AnnotatedString(msg.text))
                        menuOpen = false
                    }
                )
                DropdownMenuItem(
                    text = { Text("Ответить") },
                    onClick = { onReply(); menuOpen = false }
                )
                DropdownMenuItem(
                    text = { Text("Удалить", color = Color(0xFFEF5350)) },
                    onClick = { onDelete(); menuOpen = false }
                )
            }
        }
    }
}

@Composable
fun DateSeparator(label: String) {
    Row(
        modifier = Modifier.fillMaxWidth().padding(vertical = 8.dp),
        horizontalArrangement = Arrangement.Center
    ) {
        Text(
            label,
            color = Color.White.copy(alpha = 0.7f),
            fontSize = 11.sp,
            modifier = Modifier
                .clip(RoundedCornerShape(10.dp))
                .background(Color(0xFF242F3D))
                .padding(horizontal = 12.dp, vertical = 4.dp)
        )
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

    Column(
        modifier = Modifier.fillMaxWidth().background(MessengerColors.DarkBg).padding(8.dp),
    ) {
        Row(
            modifier = Modifier.fillMaxWidth(),
            horizontalArrangement = Arrangement.SpaceBetween,
            verticalAlignment = Alignment.CenterVertically
        ) {
            Text("🛡️ VPN", color = Color.White, fontSize = 12.sp)
            val statusColor = if (vpnOn) Color(0xFF4CAF50) else Color(0xFFFF6B6B)
            Text(
                if (vpnOn) "Включён" else "Отключён",
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
            Text("🖥 Exit: ", color = Color.Gray, fontSize = 11.sp)
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
                    if (exitOn) "☣ Этот тел — выход: ВКЛ" else "☣ Этот тел — выход",
                    color = if (exitOn) Color(0xFF4CAF50) else Color.Gray,
                    fontSize = 10.sp
                )
            }
        }
        Text(vpnStatus, color = Color.DarkGray, fontSize = 10.sp, modifier = Modifier.padding(top = 2.dp))
    }
}

private fun formatTime(ts: Long): String {
    val sdf = SimpleDateFormat("HH:mm", Locale.getDefault())
    return sdf.format(Date(ts))
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
