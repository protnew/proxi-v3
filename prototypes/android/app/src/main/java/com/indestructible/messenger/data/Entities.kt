package com.indestructible.messenger.data

import androidx.room.Entity
import androidx.room.Index
import androidx.room.PrimaryKey

@Entity(tableName = "chats")
data class ChatEntity(
    @PrimaryKey val id: String,          // dm:<pubkey> | group:<id>
    val name: String,
    val avatar: String,
    val type: String,                    // "DM" | "GROUP"
    val lastActivity: Long,
    val unread: Int = 0,
    val lastMessageText: String? = null,
    val peerPubKey: String? = null,      // DM peer pubkey for display/search
)

@Entity(
    tableName = "messages",
    indices = [Index("chatId"), Index("timestamp")]
)
data class MessageEntity(
    @PrimaryKey val id: String,
    val chatId: String,
    val fromPub: String,
    val toPub: String,
    val text: String,
    val timestamp: Long,
    val type: String = "TEXT",           // TEXT | VOICE | FILE | SYSTEM
    val read: Boolean = false,
    val outgoing: Boolean = false,
    val delivered: Boolean = false,
    val replyTo: String? = null,
    val edited: Boolean = false,
    val fileUrl: String? = null,
    val fileName: String? = null,
    val fileSize: Long? = null,
    val voiceDuration: Int? = null,
)

@Entity(tableName = "contacts")
data class ContactEntity(
    @PrimaryKey val pubkey: String,
    val name: String,
    val avatar: String = "👤",
    val lastSeen: Long = 0,
    val online: Boolean = false,
)
