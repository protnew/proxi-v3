package com.indestructible.messenger.ui.screens

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.*
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalClipboardManager
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.AnnotatedString
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

@Composable
fun SettingsScreen(
    onBack: () -> Unit,
    serverUrl: String = "ws://10.0.2.2:8090/ws",
    myPubKey: String = "",
    onServerUrlChange: (String) -> Unit = {},
    onDisconnect: () -> Unit = {},
    onLogout: () -> Unit = {},
) {
    val context = LocalContext.current
    val clipboard = LocalClipboardManager.current
    var editingUrl by remember { mutableStateOf(serverUrl) }
    val profileName = remember {
        context.getSharedPreferences("proxi_profile", android.content.Context.MODE_PRIVATE)
            .getString("name", "") ?: ""
    }

    Column(
        modifier = Modifier.fillMaxSize().background(Color(0xFF17212B)).padding(16.dp)
    ) {
        TextButton(onClick = onBack) { Text("← Назад", color = Color.White) }
        Spacer(modifier = Modifier.height(16.dp))
        Text("⚙️ Настройки", color = Color.White, fontSize = 20.sp)
        Spacer(modifier = Modifier.height(24.dp))
        Text("Профиль", color = Color.Gray, fontSize = 12.sp)
        Text(
            profileName.ifBlank { "Без имени" },
            color = Color.White, fontSize = 16.sp
        )
        if (myPubKey.isNotEmpty()) {
            Spacer(modifier = Modifier.height(8.dp))
            Text("Ваш ключ (npub/hex)", color = Color.Gray, fontSize = 12.sp)
            Text(
                myPubKey,
                color = Color(0xFF5EB5F7), fontSize = 11.sp,
                fontFamily = FontFamily.Monospace
            )
            TextButton(onClick = { clipboard.setText(AnnotatedString(myPubKey)) }) {
                Text("Скопировать ключ", color = Color(0xFF5EB5F7), fontSize = 12.sp)
            }
        }
        Spacer(modifier = Modifier.height(16.dp))
        Text("Сервер сообщений", color = Color.Gray, fontSize = 12.sp)
        OutlinedTextField(
            value = editingUrl,
            onValueChange = { editingUrl = it },
            modifier = Modifier.fillMaxWidth(),
            singleLine = true,
            colors = OutlinedTextFieldDefaults.colors(
                unfocusedContainerColor = Color(0xFF242F3D),
                focusedContainerColor = Color(0xFF242F3D),
            )
        )
        Spacer(modifier = Modifier.height(8.dp))
        Button(
            onClick = { onServerUrlChange(editingUrl.trim()) },
            modifier = Modifier.fillMaxWidth(),
            colors = ButtonDefaults.buttonColors(containerColor = Color(0xFF3A7BD5))
        ) { Text("Сохранить", color = Color.White) }
        Spacer(modifier = Modifier.height(24.dp))
        OutlinedButton(
            onClick = onDisconnect,
            modifier = Modifier.fillMaxWidth(),
            colors = ButtonDefaults.outlinedButtonColors(contentColor = Color(0xFFEF5350))
        ) { Text("Отключиться от сервера") }
        Spacer(modifier = Modifier.height(8.dp))
        OutlinedButton(
            onClick = onLogout,
            modifier = Modifier.fillMaxWidth(),
            colors = ButtonDefaults.outlinedButtonColors(contentColor = Color(0xFFEF5350))
        ) { Text("Выйти из аккаунта (стереть ключ)") }
    }
}

@Composable
fun NewChatScreen(
    onBack: () -> Unit,
    onCreateChat: (pubkey: String, name: String) -> Unit
) {
    var pubkey by remember { mutableStateOf("") }
    var name by remember { mutableStateOf("") }

    Column(
        modifier = Modifier.fillMaxSize().background(Color(0xFF17212B)).padding(16.dp)
    ) {
        TextButton(onClick = onBack) { Text("← Назад", color = Color.White) }
        Spacer(modifier = Modifier.height(16.dp))
        Text("Новый чат", color = Color.White, fontSize = 20.sp)
        Spacer(modifier = Modifier.height(24.dp))
        OutlinedTextField(
            value = pubkey,
            onValueChange = { pubkey = it },
            label = { Text("Public Key (hex или npub)") },
            modifier = Modifier.fillMaxWidth(),
            colors = OutlinedTextFieldDefaults.colors(
                unfocusedContainerColor = Color(0xFF242F3D),
                focusedContainerColor = Color(0xFF242F3D),
            )
        )
        Spacer(modifier = Modifier.height(8.dp))
        OutlinedTextField(
            value = name,
            onValueChange = { name = it },
            label = { Text("Имя (необязательно)") },
            modifier = Modifier.fillMaxWidth(),
            colors = OutlinedTextFieldDefaults.colors(
                unfocusedContainerColor = Color(0xFF242F3D),
                focusedContainerColor = Color(0xFF242F3D),
            )
        )
        Spacer(modifier = Modifier.height(16.dp))
        Button(
            onClick = {
                val raw = pubkey.trim()
                if (raw.isNotEmpty()) {
                    // Accept npub too: decode to hex before creating the chat.
                    val hex = try {
                        if (raw.startsWith("npub1")) {
                            com.indestructible.messenger.crypto.Bech32.decodeNpub(raw)
                                .joinToString("") { "%02x".format(it) }
                        } else raw
                    } catch (e: Exception) { return@Button }
                    onCreateChat(hex, name.trim().ifBlank { "${hex.take(8)}…" })
                }
            },
            modifier = Modifier.fillMaxWidth(),
            enabled = pubkey.isNotBlank(),
            colors = ButtonDefaults.buttonColors(
                containerColor = Color(0xFF3A7BD5),
                disabledContainerColor = Color(0xFF2A3A4D)
            )
        ) { Text("Начать чат") }
    }
}
