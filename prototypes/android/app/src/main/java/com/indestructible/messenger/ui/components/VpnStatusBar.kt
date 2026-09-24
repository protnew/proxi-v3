@file:OptIn(ExperimentalMaterial3Api::class, ExperimentalFoundationApi::class)

package com.indestructible.messenger.ui.components
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
