package com.indestructible.messenger.ui.navigation

import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.getValue
import androidx.compose.runtime.setValue
import androidx.navigation.compose.NavHost
import androidx.navigation.compose.composable
import androidx.navigation.compose.rememberNavController
import com.indestructible.messenger.ui.screens.*
import com.indestructible.messenger.messenger.ChatViewModel
import com.indestructible.messenger.messenger.ChatViewModelHolder
import com.indestructible.messenger.nostr.NostrIdentity

@Composable
fun MessengerNavHost() {
    val navController = rememberNavController()
    val context = androidx.compose.ui.platform.LocalContext.current
    var identity by remember { mutableStateOf<NostrIdentity?>(null) }
    var identityLoaded by remember { mutableStateOf(false) }
    val chatVM = remember {
        val vm = ChatViewModel(context.applicationContext)
        ChatViewModelHolder.instance = vm
        vm
    }

    // Keystore-wrapped nsec: returning users skip onboarding entirely.
    LaunchedEffect(Unit) {
        val saved = com.indestructible.messenger.nostr.IdentityStore.load(context)
        if (saved != null) {
            identity = saved
            chatVM.setPrivateKey(saved.privateKey.joinToString("") { "%02x".format(it) })
            chatVM.loadChats()
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
            AuthScreen(onDone = { id, profileName ->
                identity = id
                com.indestructible.messenger.nostr.IdentityStore.save(context, id)
                context.getSharedPreferences("proxi_profile", android.content.Context.MODE_PRIVATE)
                    .edit().putString("name", profileName).apply()
                chatVM.setPrivateKey(id.privateKey.joinToString("") { "%02x".format(it) })
                chatVM.loadChats()
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
                chats = chatVM.chats,
                chatVM = chatVM,
                myPubKey = identity?.publicKey ?: "me",
            )
        }
        composable("settings") {
            SettingsScreen(
                onBack = { navController.popBackStack() },
                serverUrl = chatVM.serverUrl,
                myPubKey = identity?.publicKey ?: "",
                onServerUrlChange = { chatVM.serverUrl = it },
                onDisconnect = { chatVM.disconnect() },
                onLogout = {
                    chatVM.wipeAll()
                    com.indestructible.messenger.nostr.IdentityStore.clear(context)
                    context.getSharedPreferences("proxi_profile", android.content.Context.MODE_PRIVATE)
                        .edit().clear().apply()
                    identity = null
                    navController.navigate("auth") { popUpTo(0) { inclusive = true } }
                },
            )
        }
        composable("new_chat") {
            NewChatScreen(
                onBack = { navController.popBackStack() },
                onCreateChat = { pubkey, name ->
                    chatVM.createDmChat(pubkey, name)
                    navController.popBackStack()
                },
                onCreateGroup = { name, members ->
                    chatVM.createGroupChat(name, members)
                    navController.popBackStack()
                },
            )
        }
    }
}
