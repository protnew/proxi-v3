package com.indestructible.messenger

import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.junit4.createComposeRule
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.test.ext.junit.runners.AndroidJUnit4
import com.indestructible.messenger.ui.navigation.MessengerNavHost
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith

@RunWith(AndroidJUnit4::class)
class AppFlowTest {

    @get:Rule
    val composeRule = createComposeRule()

    @Test
    fun onb_welcome_showsProxi() {
        composeRule.setContent { MessengerNavHost() }
        composeRule.waitForIdle()
        composeRule.onNodeWithText("Proxi").assertIsDisplayed()
    }

    @Test
    fun onb_createAccount_showsKeyBackup() {
        composeRule.setContent { MessengerNavHost() }
        composeRule.waitForIdle()
        // Step 1: click create
        composeRule.onNodeWithText("\u0421\u043E\u0437\u0434\u0430\u0442\u044C \u043D\u043E\u0432\u044B\u0439 \u0430\u043A\u043A\u0430\u0443\u043D\u0442").performClick()
        composeRule.waitForIdle()
        // Step 2: should show "Ваш ключ" text
        composeRule.onNodeWithText("\u0412\u0430\u0448 \u043A\u043B\u044E\u0447").assertIsDisplayed()
    }

    @Test
    fun onb_backupConfirm_showsNext() {
        composeRule.setContent { MessengerNavHost() }
        composeRule.waitForIdle()
        composeRule.onNodeWithText("\u0421\u043E\u0437\u0434\u0430\u0442\u044C \u043D\u043E\u0432\u044B\u0439 \u0430\u043A\u043A\u0430\u0443\u043D\u0442").performClick()
        composeRule.waitForIdle()
        composeRule.onNodeWithText("\u042F \u0441\u043E\u0445\u0440\u0430\u043D\u0438\u043B \u043A\u043B\u044E\u0447").performClick()
        composeRule.waitForIdle()
        composeRule.onNodeWithText("\u0414\u0430\u043B\u0435\u0435").assertIsDisplayed()
    }

    @Test
    fun onb_import_showsInput() {
        composeRule.setContent { MessengerNavHost() }
        composeRule.waitForIdle()
        composeRule.onNodeWithText("\u0418\u043C\u043F\u043E\u0440\u0442\u0438\u0440\u043E\u0432\u0430\u0442\u044C nsec / seed").performClick()
        composeRule.waitForIdle()
        composeRule.onNodeWithText("\u0418\u043C\u043F\u043E\u0440\u0442 \u043A\u043B\u044E\u0447\u0430").assertIsDisplayed()
    }
}
