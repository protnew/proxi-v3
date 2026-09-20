package com.indestructible.messenger.ui.navigation

import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.mutableStateListOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.getValue
import androidx.compose.runtime.setValue
import androidx.navigation.compose.NavHost
import androidx.navigation.compose.composable
import androidx.navigation.compose.rememberNavController
import com.indestructible.messenger.ui.screens.*
import com.indestructible.messenger.messenger.Chat
import com.indestructible.messenger.messenger.ChatViewModel
import com.indestructible.messenger.messenger.ChatViewModelHolder
import com.indestructible.messenger.nostr.NostrIdentity

@Composable
fun MessengerNavHost() {
    val navController = rememberNavController()
    val context = androidx.compose.ui.platform.LocalContext.current
    val chats = remember { mutableStateListOf<Chat>() }
    var identity by remember { mutableStateOf<NostrIdentity?>(null) }
    var identityLoaded by remember { mutableStateOf(false) }
    val chatVM = remember {
        val vm = ChatViewModel()
        ChatViewModelHolder.instance = vm
        vm
    }

    // Keystore-wrapped nsec: returning users skip onboarding entirely.
    LaunchedEffect(Unit) {
        val saved = com.indestructible.messenger.nostr.IdentityStore.load(context)
        if (saved != null) {
            identity = saved
            chatVM.setPrivateKey(saved.privateKey.joinToString("") { "%02x".format(it) })
            chatVM.connect(saved.publicKey)
        }
        identityLoaded = true
    }
    if (!identityLoaded) return // brief splash — avoids auth flash for saved users

    val startDest = if (identity == null) "auth" else "main"

    NavHost(
        navController = navController,
        startDestination = startDest
    ) {
        composable("auth") {
            AuthScreen(onDone = { id ->
                identity = id
                com.indestructible.messenger.nostr.IdentityStore.save(context, id)
                chatVM.setPrivateKey(id.privateKey.joinToString("") { "%02x".format(it) })
                chatVM.connect(id.publicKey)
                navController.navigate("main") {
                    popUpTo("auth") { inclusive = true }
                }
            })
        }
        composable("main") {
            MainScreen(
                onNavigateToSettings = { navController.navigate("settings") },
                onNavigateToNewChat = { navController.navigate("new_chat") },
                chats = chats,
                chatVM = chatVM,
                myPubKey = identity?.publicKey ?: "me",
            )
        }
        composable("settings") {
            SettingsScreen(
                onBack = { navController.popBackStack() },
                serverUrl = chatVM.serverUrl,
                onServerUrlChange = { chatVM.serverUrl = it },
                onDisconnect = { chatVM.disconnect() },
            )
        }
        composable("new_chat") {
            NewChatScreen(
                onBack = { navController.popBackStack() },
                onCreateChat = { pubkey, name ->
                    chats.add(Chat(
                        id = "dm:$pubkey",
                        name = name,
                        avatar = name.take(1).ifBlank { "?" },
                        type = Chat.Type.DM,
                        messages = emptyList(),
                        unread = 0,
                        lastActivity = System.currentTimeMillis()
                    ))
                    navController.popBackStack()
                }
            )
        }
    }
}
