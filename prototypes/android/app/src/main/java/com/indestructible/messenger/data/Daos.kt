package com.indestructible.messenger.data

import androidx.room.Dao
import androidx.room.Insert
import androidx.room.OnConflictStrategy
import androidx.room.Query

@Dao
interface ChatDao {
    @Query("SELECT * FROM chats ORDER BY lastActivity DESC")
    fun all(): List<ChatEntity>

    @Query("SELECT * FROM chats WHERE id = :id")
    fun byId(id: String): ChatEntity?

    @Insert(onConflict = OnConflictStrategy.REPLACE)
    fun upsert(chat: ChatEntity)

    @Query("UPDATE chats SET unread = unread + 1, lastActivity = :ts, lastMessageText = :preview WHERE id = :id")
    fun bumpIncoming(id: String, ts: Long, preview: String)

    @Query("UPDATE chats SET unread = 0 WHERE id = :id")
    fun markRead(id: String)

    @Query("DELETE FROM chats WHERE id = :id")
    fun delete(id: String)
}

@Dao
interface MessageDao {
    @Query("SELECT * FROM messages WHERE chatId = :chatId ORDER BY timestamp ASC")
    fun forChat(chatId: String): List<MessageEntity>

    @Insert(onConflict = OnConflictStrategy.REPLACE)
    fun upsert(msg: MessageEntity)

    @Query("UPDATE messages SET delivered = 1 WHERE id = :id")
    fun markDelivered(id: String)

    @Query("UPDATE messages SET read = 1 WHERE chatId = :chatId AND outgoing = 0")
    fun markIncomingRead(chatId: String)

    @Query("DELETE FROM messages WHERE id = :id")
    fun delete(id: String)

    @Query("SELECT * FROM messages WHERE text LIKE '%' || :q || '%' ORDER BY timestamp DESC LIMIT 50")
    fun search(q: String): List<MessageEntity>
}

@Dao
interface ContactDao {
    @Query("SELECT * FROM contacts ORDER BY name ASC")
    fun all(): List<ContactEntity>

    @Query("SELECT * FROM contacts WHERE pubkey = :pubkey")
    fun byPubkey(pubkey: String): ContactEntity?

    @Insert(onConflict = OnConflictStrategy.REPLACE)
    fun upsert(contact: ContactEntity)

    @Query("DELETE FROM contacts WHERE pubkey = :pubkey")
    fun delete(pubkey: String)
}
