package com.indestructible.messenger.ui.navigation

import androidx.compose.runtime.Composable
import androidx.navigation.compose.NavHost
import androidx.navigation.compose.composable
import androidx.navigation.compose.rememberNavController
import com.indestructible.messenger.ui.screens.*

@Composable
fun MessengerNavHost() {
    val navController = rememberNavController()

    NavHost(
        navController = navController,
        startDestination = "main"
    ) {
        composable("main") {
            MainScreen(
                onNavigateToSettings = { navController.navigate("settings") },
                onNavigateToNewChat = { navController.navigate("new_chat") },
            )
        }
        composable("settings") {
            SettingsScreen(
                onBack = { navController.popBackStack() }
            )
        }
        composable("new_chat") {
            NewChatScreen(
                onBack = { navController.popBackStack() }
            )
        }
    }
}
