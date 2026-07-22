package com.indestructible.messenger.ui.screens

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.*
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

@Composable
fun SettingsScreen(onBack: () -> Unit) {
    Column(
        modifier = Modifier.fillMaxSize().background(Color(0xFF17212B)).padding(16.dp)
    ) {
        TextButton(onClick = onBack) {
            Text("← Назад", color = Color.White)
        }
        Spacer(modifier = Modifier.height(16.dp))
        Text("⚙️ Настройки", color = Color.White, fontSize = 20.sp)
        Spacer(modifier = Modifier.height(24.dp))
        Text("Профиль", color = Color.Gray, fontSize = 12.sp)
        Text("User", color = Color.White, fontSize = 16.sp)
        Spacer(modifier = Modifier.height(16.dp))
        Text("Public Key", color = Color.Gray, fontSize = 12.sp)
        Text("—", color = Color.White, fontSize = 13.sp, fontFamily = androidx.compose.ui.text.font.FontFamily.Monospace)
    }
}

@Composable
fun NewChatScreen(onBack: () -> Unit) {
    var pubkey by remember { mutableStateOf("") }
    var name by remember { mutableStateOf("") }

    Column(
        modifier = Modifier.fillMaxSize().background(Color(0xFF17212B)).padding(16.dp)
    ) {
        TextButton(onClick = onBack) {
            Text("← Назад", color = Color.White)
        }
        Spacer(modifier = Modifier.height(16.dp))
        Text("Новый чат", color = Color.White, fontSize = 20.sp)
        Spacer(modifier = Modifier.height(24.dp))

        OutlinedTextField(
            value = pubkey,
            onValueChange = { pubkey = it },
            label = { Text("Public Key (hex)") },
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
            onClick = { /* TODO: Create chat */ },
            modifier = Modifier.fillMaxWidth(),
            colors = ButtonDefaults.buttonColors(containerColor = Color(0xFF3A7BD5))
        ) {
            Text("💬 Начать чат")
        }
    }
}
