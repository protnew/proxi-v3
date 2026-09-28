@file:OptIn(ExperimentalMaterial3Api::class, ExperimentalFoundationApi::class)

package com.indestructible.messenger.ui.components
import com.indestructible.messenger.messenger.downloadAttachment
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
