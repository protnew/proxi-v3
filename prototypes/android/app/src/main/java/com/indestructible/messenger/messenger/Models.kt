package com.indestructible.messenger.messenger

/**
 * Message data model
 */
data class Message(
    val id: String,
    val from: String,     // pubkey
    val to: String,       // pubkey or group:id
    val text: String,
    val timestamp: Long,
    val type: Type = Type.TEXT,
    val read: Boolean = false,
    val fileName: String? = null,
    val fileSize: Long? = null,
    val fileUrl: String? = null,
    val voiceDuration: Int? = null,
    val replyTo: String? = null,
    val edited: Boolean = false,
) {
    enum class Type { TEXT, VOICE, FILE, SYSTEM }
}

/**
 * Chat (conversation) model
 */
data class Chat(
    val id: String,        // dm:<pubkey> or group:<channelId>
    val name: String,
    val avatar: String,
    val type: Type = Type.DM,
    val messages: List<Message> = emptyList(),
    val unread: Int = 0,
    val lastActivity: Long = System.currentTimeMillis(),
    val typing: List<String> = emptyList(),
    val members: List<String>? = null,
) {
    enum class Type { DM, GROUP }
}

/**
 * Contact model
 */
data class Contact(
    val pubkey: String,
    val name: String,
    val avatar: String = "👤",
    val isOnline: Boolean = false,
    val lastSeen: Long = 0,
)

/**
 * Profile model
 */
data class Profile(
    val pubkey: String,
    val name: String = "User",
    val about: String = "",
    val avatar: String = "👤",
)
