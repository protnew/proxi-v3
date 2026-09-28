package com.indestructible.messenger.data

import android.content.Context
import androidx.room.Database
import androidx.room.Room
import androidx.room.RoomDatabase
import androidx.room.migration.Migration
import androidx.sqlite.db.SupportSQLiteDatabase

@Database(
    entities = [ChatEntity::class, MessageEntity::class, ContactEntity::class],
    version = 2,
    exportSchema = false
)
abstract class ProxiDatabase : RoomDatabase() {
    abstract fun chats(): ChatDao
    abstract fun messages(): MessageDao
    abstract fun contacts(): ContactDao

    companion object {
        @Volatile private var instance: ProxiDatabase? = null

        private val MIGRATION_1_2 = object : Migration(1, 2) {
            override fun migrate(db: SupportSQLiteDatabase) {
                db.execSQL("ALTER TABLE chats ADD COLUMN members TEXT")
            }
        }

        fun get(ctx: Context): ProxiDatabase =
            instance ?: synchronized(this) {
                instance ?: Room.databaseBuilder(
                    ctx.applicationContext,
                    ProxiDatabase::class.java,
                    "proxi.db"
                ).addMigrations(MIGRATION_1_2).build().also { instance = it }
            }
    }
}
