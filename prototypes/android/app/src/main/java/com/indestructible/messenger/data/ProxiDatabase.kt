package com.indestructible.messenger.data

import android.content.Context
import androidx.room.Database
import androidx.room.Room
import androidx.room.RoomDatabase

@Database(
    entities = [ChatEntity::class, MessageEntity::class, ContactEntity::class],
    version = 1,
    exportSchema = false
)
abstract class ProxiDatabase : RoomDatabase() {
    abstract fun chats(): ChatDao
    abstract fun messages(): MessageDao
    abstract fun contacts(): ContactDao

    companion object {
        @Volatile private var instance: ProxiDatabase? = null

        fun get(ctx: Context): ProxiDatabase =
            instance ?: synchronized(this) {
                instance ?: Room.databaseBuilder(
                    ctx.applicationContext,
                    ProxiDatabase::class.java,
                    "proxi.db"
                ).build().also { instance = it }
            }
    }
}
