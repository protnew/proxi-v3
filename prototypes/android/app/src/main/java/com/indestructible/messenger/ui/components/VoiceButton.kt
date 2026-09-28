@file:OptIn(ExperimentalMaterial3Api::class, ExperimentalFoundationApi::class)

package com.indestructible.messenger.ui.components
import com.indestructible.messenger.messenger.sendAttachment
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
