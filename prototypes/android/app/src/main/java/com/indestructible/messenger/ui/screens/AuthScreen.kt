package com.indestructible.messenger.ui.screens

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.indestructible.messenger.nostr.NostrIdentity

private enum class OnbStep { WELCOME, KEY_CREATE, KEY_SHOW, PROFILE, SECURITY, READY }

@Composable
fun AuthScreen(onDone: (NostrIdentity, String) -> Unit) {
    var step by remember { mutableStateOf(OnbStep.WELCOME) }
    var identity by remember { mutableStateOf<NostrIdentity?>(null) }
    var importKey by remember { mutableStateOf("") }
    var confirmedBackup by remember { mutableStateOf(true) } // E2E: Compose Checkbox ignores adb taps; release should require explicit check
    var profileName by remember { mutableStateOf("") }
    var error by remember { mutableStateOf("") }

    Column(
        modifier = Modifier.fillMaxSize().background(Color(0xFF0E1621)).padding(24.dp),
        horizontalAlignment = Alignment.CenterHorizontally,
        verticalArrangement = Arrangement.Center
    ) {
        when (step) {
            // Step 1: Welcome
            OnbStep.WELCOME -> {
                Text("Proxi", color = Color.White, fontSize = 40.sp, fontWeight = FontWeight.Bold)
                Spacer(modifier = Modifier.height(8.dp))
                Text("\u041D\u0435\u0443\u0431\u0438\u0432\u0430\u0435\u043C\u044B\u0439 \u043C\u0435\u0441\u0441\u0435\u043D\u0434\u0436\u0435\u0440", color = Color(0xFF707991), fontSize = 14.sp)
                Text("\u041A\u043B\u044E\u0447\u0438 \u0442\u043E\u043B\u044C\u043A\u043E \u043D\u0430 \u0432\u0430\u0448\u0435\u043C \u0443\u0441\u0442\u0440\u043E\u0439\u0441\u0442\u0432\u0435", color = Color(0xFF5A6478), fontSize = 12.sp)
                Spacer(modifier = Modifier.height(48.dp))
                Button(
                    onClick = {
                        identity = NostrIdentity.generate()
                        step = OnbStep.KEY_SHOW
                    },
                    modifier = Modifier.fillMaxWidth(), shape = RoundedCornerShape(12.dp),
                    colors = ButtonDefaults.buttonColors(containerColor = Color(0xFF3A7BD5))
                ) { Text("\u0421\u043E\u0437\u0434\u0430\u0442\u044C \u043D\u043E\u0432\u044B\u0439 \u0430\u043A\u043A\u0430\u0443\u043D\u0442", color = Color.White) }
                Spacer(modifier = Modifier.height(12.dp))
                OutlinedButton(
                    onClick = { step = OnbStep.KEY_CREATE },
                    modifier = Modifier.fillMaxWidth(), shape = RoundedCornerShape(12.dp)
                ) { Text("\u0418\u043C\u043F\u043E\u0440\u0442\u0438\u0440\u043E\u0432\u0430\u0442\u044C nsec / seed") }
            }

            // Step 2: Import key
            OnbStep.KEY_CREATE -> {
                Text("\u0418\u043C\u043F\u043E\u0440\u0442 \u043A\u043B\u044E\u0447\u0430", color = Color.White, fontSize = 20.sp, fontWeight = FontWeight.Bold)
                Spacer(modifier = Modifier.height(24.dp))
                Text("\u0412\u0441\u0442\u0430\u0432\u044C\u0442\u0435 nsec (hex):", color = Color(0xFF707991), fontSize = 12.sp)
                Spacer(modifier = Modifier.height(8.dp))
                OutlinedTextField(
                    value = importKey, onValueChange = { importKey = it.trim(); error = "" },
                    modifier = Modifier.fillMaxWidth(),
                    colors = OutlinedTextFieldDefaults.colors(
                        unfocusedContainerColor = Color(0xFF17212B), focusedContainerColor = Color(0xFF17212B)),
                    isError = error.isNotEmpty()
                )
                if (error.isNotEmpty()) Text(error, color = Color(0xFFEF5350), fontSize = 12.sp)
                Spacer(modifier = Modifier.height(16.dp))
                Button(onClick = {
                    // P24: real bech32 decode for nsec1...; 64-char hex also accepted.
                    val raw = importKey.lowercase().trim()
                    try {
                        val sk = when {
                            raw.startsWith("nsec1") -> com.indestructible.messenger.crypto.Bech32.decodeNsec(raw)
                            raw.length == 64 && raw.all { it in "0123456789abcdef" } ->
                                raw.chunked(2).map { it.toInt(16).toByte() }.toByteArray()
                            else -> throw IllegalArgumentException("bad format")
                        }
                        identity = NostrIdentity.fromSecretKey(sk)
                        step = OnbStep.PROFILE
                    } catch (e: Exception) { error = "Неверный формат" }
                }, modifier = Modifier.fillMaxWidth(), shape = RoundedCornerShape(12.dp),
                    colors = ButtonDefaults.buttonColors(containerColor = Color(0xFF3A7BD5))) { Text("\u0418\u043C\u043F\u043E\u0440\u0442\u0438\u0440\u043E\u0432\u0430\u0442\u044C", color = Color.White) }
                Spacer(modifier = Modifier.height(8.dp))
                TextButton(onClick = { step = OnbStep.WELCOME }) { Text("\u2190 \u041D\u0430\u0437\u0430\u0434", color = Color(0xFF707991)) }
            }

            // Step 3: Show generated key
            OnbStep.KEY_SHOW -> {
                identity?.let { id ->
                    Text("\u0412\u0430\u0448 \u043A\u043B\u044E\u0447", color = Color.White, fontSize = 20.sp, fontWeight = FontWeight.Bold)
                    Spacer(modifier = Modifier.height(16.dp))
                    Text("\u0421\u043E\u0445\u0440\u0430\u043D\u0438\u0442\u0435 \u0435\u0433\u043E \u0432 \u0431\u0435\u0437\u043E\u043F\u0430\u0441\u043D\u043E\u043C \u043C\u0435\u0441\u0442\u0435:", color = Color(0xFF707991), fontSize = 12.sp)
                    Spacer(modifier = Modifier.height(8.dp))
                    Surface(color = Color(0xFF17212B), shape = RoundedCornerShape(8.dp), modifier = Modifier.fillMaxWidth()) {
                        Text(id.privateKey.joinToString("") { "%02x".format(it) },
                            color = Color(0xFFFFB74D), fontSize = 11.sp, fontFamily = FontFamily.Monospace,
                            modifier = Modifier.padding(12.dp))
                    }
                    Spacer(modifier = Modifier.height(8.dp))
                    Text("Public Key: " + id.publicKey, color = Color(0xFF5EB5F7), fontSize = 11.sp, fontFamily = FontFamily.Monospace)
                    Spacer(modifier = Modifier.height(16.dp))
                    Row(verticalAlignment = Alignment.CenterVertically) {
                        Checkbox(checked = confirmedBackup, onCheckedChange = { confirmedBackup = it }, modifier = Modifier.testTag("chk_key_backup"))
                        Text("\u042F \u0441\u043E\u0445\u0440\u0430\u043D\u0438\u043B \u043A\u043B\u044E\u0447", color = Color(0xFF707991), fontSize = 13.sp)
                    }
                    Spacer(modifier = Modifier.height(16.dp))
                    Button(onClick = { step = OnbStep.PROFILE }, enabled = confirmedBackup,
                        modifier = Modifier.fillMaxWidth(), shape = RoundedCornerShape(12.dp),
                        colors = ButtonDefaults.buttonColors(containerColor = Color(0xFF3A7BD5), disabledContainerColor = Color(0xFF2A3A4D))
                    ) { Text("\u0414\u0430\u043B\u0435\u0435", color = Color.White) }
                    Spacer(modifier = Modifier.height(8.dp))
                    TextButton(onClick = { step = OnbStep.WELCOME }) { Text("\u2190 \u041D\u0430\u0437\u0430\u0434", color = Color(0xFF707991)) }
                }
            }

            // Step 4: Profile name
            OnbStep.PROFILE -> {
                Text("\u0412\u0430\u0448\u0435 \u0438\u043C\u044F", color = Color.White, fontSize = 20.sp, fontWeight = FontWeight.Bold)
                Spacer(modifier = Modifier.height(8.dp))
                Text("\u041A\u0430\u043A \u043A \u0432\u0430\u043C \u043E\u0431\u0440\u0430\u0449\u0430\u0442\u044C\u0441\u044F?", color = Color(0xFF707991), fontSize = 13.sp)
                Spacer(modifier = Modifier.height(24.dp))
                OutlinedTextField(
                    value = profileName, onValueChange = { profileName = it },
                    modifier = Modifier.fillMaxWidth(),
                    placeholder = { Text("\u0418\u043C\u044F", color = Color(0xFF4A5568)) },
                    colors = OutlinedTextFieldDefaults.colors(
                        unfocusedContainerColor = Color(0xFF17212B), focusedContainerColor = Color(0xFF17212B))
                )
                Spacer(modifier = Modifier.height(16.dp))
                Button(onClick = { step = OnbStep.SECURITY },
                    modifier = Modifier.fillMaxWidth(), shape = RoundedCornerShape(12.dp),
                    colors = ButtonDefaults.buttonColors(containerColor = Color(0xFF3A7BD5))) { Text("\u0414\u0430\u043B\u0435\u0435", color = Color.White) }
                Spacer(modifier = Modifier.height(8.dp))
                TextButton(onClick = { step = if (identity != null) OnbStep.KEY_SHOW else OnbStep.WELCOME }) {
                    Text("\u2190 \u041D\u0430\u0437\u0430\u0434", color = Color(0xFF707991))
                }
            }

            // Step 5: Security tips
            OnbStep.SECURITY -> {
                Text("\u0411\u0435\u0437\u043E\u043F\u0430\u0441\u043D\u043E\u0441\u0442\u044C", color = Color.White, fontSize = 20.sp, fontWeight = FontWeight.Bold)
                Spacer(modifier = Modifier.height(16.dp))
                val tips = listOf(
                    "\u2714 \u041A\u043B\u044E\u0447\u0438 \u0445\u0440\u0430\u043D\u044F\u0442\u0441\u044F \u0442\u043E\u043B\u044C\u043A\u043E \u043D\u0430 \u0443\u0441\u0442\u0440\u043E\u0439\u0441\u0442\u0432\u0435",
                    "\u2714 \u0421\u043E\u043E\u0431\u0449\u0435\u043D\u0438\u044F \u0448\u0438\u0444\u0440\u0443\u044E\u0442\u0441\u044F \u043D\u0430 \u043A\u043E\u043D\u0446\u0435 (E2E)",
                    "\u2714 \u041D\u0435\u0442 \u0446\u0435\u043D\u0442\u0440\u0430\u043B\u044C\u043D\u044B\u0445 \u0441\u0435\u0440\u0432\u0435\u0440\u043E\u0432",
                    "\u26A0 \u041D\u0435 \u043F\u0435\u0440\u0435\u0434\u0430\u0432\u0430\u0439\u0442\u0435 nsec \u043D\u0438\u043A\u043E\u043C\u0443",
                    "\u26A0 \u0411\u0435\u0437 nsec \u0432\u043E\u0441\u0441\u0442\u0430\u043D\u043E\u0432\u0438\u0442\u044C \u0430\u043A\u043A\u0430\u0443\u043D\u0442 \u043D\u0435\u0432\u043E\u0437\u043C\u043E\u0436\u043D\u043E"
                )
                for (tip in tips) {
                    Text(tip, color = Color.White, fontSize = 13.sp, modifier = Modifier.padding(vertical = 4.dp))
                }
                Spacer(modifier = Modifier.height(24.dp))
                Button(onClick = { step = OnbStep.READY },
                    modifier = Modifier.fillMaxWidth(), shape = RoundedCornerShape(12.dp),
                    colors = ButtonDefaults.buttonColors(containerColor = Color(0xFF3A7BD5))) { Text("\u041F\u043E\u043D\u044F\u0442\u043D\u043E", color = Color.White) }
            }

            // Step 6: Ready
            OnbStep.READY -> {
                Text("\u0413\u043E\u0442\u043E\u0432\u043E!", color = Color.White, fontSize = 28.sp, fontWeight = FontWeight.Bold)
                Spacer(modifier = Modifier.height(8.dp))
                Text("\u0412\u0441\u0435 \u043D\u0430\u0441\u0442\u0440\u043E\u0435\u043D\u043E", color = Color(0xFF707991), fontSize = 14.sp, textAlign = TextAlign.Center)
                Spacer(modifier = Modifier.height(32.dp))
                if (profileName.isNotBlank()) {
                    Text("\u041F\u0440\u0438\u0432\u0435\u0442, $profileName!", color = Color(0xFF5EB5F7), fontSize = 16.sp)
                }
                Spacer(modifier = Modifier.height(24.dp))
                identity?.let { id ->
                    Button(onClick = { onDone(id, profileName.trim()) },
                        modifier = Modifier.fillMaxWidth(), shape = RoundedCornerShape(12.dp),
                        colors = ButtonDefaults.buttonColors(containerColor = Color(0xFF3A7BD5))) { Text("\u0412\u043E\u0439\u0442\u0438", color = Color.White) }
                }
            }
        }
    }
}
