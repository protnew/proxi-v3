package com.indestructible.messenger.messenger

import android.util.Log
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.MultipartBody
import okhttp3.Request
import okhttp3.RequestBody.Companion.toRequestBody
import org.json.JSONObject
import com.indestructible.messenger.crypto.Nip44
import com.indestructible.messenger.data.ChatEntity
import com.indestructible.messenger.data.MessageEntity

fun ChatViewModel.createGroupChat(name: String, memberPubs: List<String>) {
    val gid = "grp-${System.currentTimeMillis()}"
    val chatId = "group:$gid"
    val all = (memberPubs + myPubKey).distinct()
    upsertChatEntity(ChatEntity(
        id = chatId, name = name, avatar = "👥", type = "GROUP",
        lastActivity = System.currentTimeMillis(),
        members = all.joinToString(","),
    ))
    val meta = "groupmeta:" + JSONObject().apply {
        put("id", gid); put("name", name)
        put("members", org.json.JSONArray(all))
    }.toString()
    Thread {
        for (m in memberPubs.filter { it != myPubKey }) {
            sendEncryptedDm(m, meta)
        }
        val fresh = db.chats().all().map { it.toModel() }
        android.os.Handler(android.os.Looper.getMainLooper()).post {
            chats.clear(); chats.addAll(fresh)
        }
    }.start()
}
