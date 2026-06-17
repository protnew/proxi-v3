// WebSocket connection service (moved from api.js)
import { get } from 'svelte/store';
import { myId, connState, connText, messages, showToast } from './stores.js';
import * as E2E from './e2e.js';
import * as VoiceMessages from './voice.js';
import * as OfflineStorage from './offline.js';
import * as WebRTCCall from './webrtc.js';
import * as PushNotifications from './push.js';

let ws = null;
let reconnectDelay = 1000;

export function getWS() {
  return ws;
}

export function connectWS() {
  const id = get(myId);
  const proto = location.protocol === 'https:' ? 'wss:' : 'ws:';
  const url = proto + '//' + location.host + '/ws?userId=' + encodeURIComponent(id);
  ws = new WebSocket(url);
  ws.binaryType = 'arraybuffer';

  ws.onopen = () => {
    connState.set('on');
    connText.set('Подключён');
    reconnectDelay = 1000;
    showToast('✅ Подключён к чату');
    WebRTCCall.setWS(ws);

    const pubKey = E2E.getPublicKeyBase64();
    if (pubKey) {
      ws.send(JSON.stringify({ type: 'key_exchange', publicKey: pubKey }));
    }

    OfflineStorage.syncFromServer().then((n) => {
      if (n > 0) showToast('📥 Синхронизировано ' + n + ' сообщений');
    });
    OfflineStorage.flushPending(ws).then((n) => {
      if (n > 0) showToast('📤 Отправлено ' + n + ' отложенных');
    });
  };

  ws.onmessage = async (e) => {
    try {
      if (e.data instanceof ArrayBuffer) {
        const parsed = VoiceMessages.parseBinaryVoiceFrame(e.data);
        if (parsed) {
          const id = get(myId);
          const isMe = parsed.meta.from === id;
          addVoiceMessage(parsed.meta.from, parsed.meta.duration, isMe, parsed.audioBlob);
          OfflineStorage.saveMessage({
            from: parsed.meta.from,
            text: '🎤 Голосовое (' + parsed.meta.duration + 'с)',
            ts: parsed.meta.ts || Math.floor(Date.now() / 1000),
            voiceDuration: parsed.meta.duration,
          });
          if (document.hidden && parsed.meta.from !== id) {
            PushNotifications.showLocal('Proxi Messenger', '🎤 Голосовое сообщение');
          }
        }
        return;
      }

      const msg = JSON.parse(e.data);
      const id = get(myId);

      if (msg.type === 'chat') {
        let text = msg.text;
        if (msg.from !== id && E2E.hasSharedKey(msg.from)) {
          try {
            text = await E2E.decrypt(msg.from, msg.text);
          } catch (e) {}
        }

        if (msg.voiceData) {
          const audioUrl = 'data:audio/webm;base64,' + msg.voiceData;
          addVoiceMessage(msg.from, msg.voiceDuration || 0, msg.from === id, null, audioUrl);
        } else {
          addChatMessage(msg.from, text, msg.from === id, {
            id: msg.id || null,
            replyTo: msg.replyTo || null,
            replyToFrom: msg.replyToFrom || null,
            replyToText: msg.replyToText || null,
            forwardedFrom: msg.forwardedFrom || null,
            ttl: msg.ttl || 0,
          });
        }

        OfflineStorage.saveMessage({
          id: msg.id,
          from: msg.from,
          to: msg.to,
          text: text,
          ts: msg.ts,
          replyTo: msg.replyTo,
          forwardedFrom: msg.forwardedFrom,
          ttl: msg.ttl,
        });

        if (document.hidden && msg.from !== id) {
          PushNotifications.showLocal('Proxi Messenger', text.substring(0, 100));
        }
      } else if (msg.type === 'key_exchange') {
        if (msg.from && msg.from !== id && msg.publicKey) {
          await E2E.deriveSharedKey(msg.from, msg.publicKey);
          console.log('E2E key exchanged with', msg.from.substring(0, 8));
        }
      } else if (msg.type === 'join') {
        addSystemMessage(msg.from + ' подключился');
        refreshOnline();
      } else if (msg.type === 'leave') {
        addSystemMessage(msg.from + ' отключился');
        refreshOnline();
      } else if (msg.type === 'users') {
        const baseUsers = msg.users || [];
        const testUsers = [
          { id: "npub1testsuperuser0000000000000000000000000000000000000000001", name: "📱 Смартфон 1", status: "online", active: true },
          { id: "npub1testsuperuser0000000000000000000000000000000000000000002", name: "📱 Смартфон 2", status: "online", active: true },
          { id: "npub1testsuperuser0000000000000000000000000000000000000000003", name: "📱 Смартфон 3", status: "online", active: true }
        ];
        testUsers.forEach(tu => {
          if (!baseUsers.find(u => u.id === tu.id)) {
            baseUsers.push(tu);
          }
        });
        onlineUsers.set(baseUsers);
      } else if (msg.type === 'typing') {
        // handled in component
      } else if (msg.type === 'message_edited') {
        showToast('✏️ Сообщение отредактировано');
      } else if (msg.type === 'message_deleted') {
        if (msg.text) {
          messages.update((msgs) => msgs.filter((m) => m.id !== msg.text));
        }
        showToast('🗑 Сообщение удалено');
      } else if (msg.type === 'webrtc-signal') {
        WebRTCCall.handleSignal({
          type: msg.signalType,
          from: msg.from,
          sdp: msg.sdp,
          candidate: msg.candidate,
        });
      }
    } catch (err) {}
  };

  ws.onclose = () => {
    connState.set('off');
    connText.set('Отключён. Переподключение...');
    setTimeout(() => {
      reconnectDelay = Math.min(reconnectDelay * 2, 30000);
      connectWS();
    }, reconnectDelay);
  };

  ws.onerror = () => {
    connState.set('off');
    connText.set('Ошибка подключения');
  };
}

function addChatMessage(from, text, isMe, msgData = {}) {
  const time = new Date().toLocaleTimeString('ru', { hour: '2-digit', minute: '2-digit' });
  const id =
    msgData.id || 'msg-dom-' + Date.now() + '-' + Math.random().toString(36).substr(2, 6);
  messages.update((msgs) => [
    ...msgs,
    {
      id,
      from,
      text,
      isMe,
      isSystem: false,
      isVoice: false,
      time,
      replyTo: msgData.replyTo || null,
      replyToFrom: msgData.replyToFrom || null,
      replyToText: msgData.replyToText || null,
      forwardedFrom: msgData.forwardedFrom || null,
      ttl: msgData.ttl || 0,
    },
  ]);
}

function addVoiceMessage(from, duration, isMe, audioBlob = null, audioUrl = '') {
  const time = new Date().toLocaleTimeString('ru', { hour: '2-digit', minute: '2-digit' });
  const id = 'voice-' + Date.now() + '-' + Math.random().toString(36).substr(2, 6);
  messages.update((msgs) => [
    ...msgs,
    {
      id,
      from,
      text: '',
      isMe,
      isSystem: false,
      isVoice: true,
      duration,
      audioBlob,
      audioUrl,
      time,
    },
  ]);
}

function addSystemMessage(text) {
  const time = new Date().toLocaleTimeString('ru', { hour: '2-digit', minute: '2-digit' });
  messages.update((msgs) => [
    ...msgs,
    {
      id: 'sys-' + Date.now(),
      from: '',
      text,
      isMe: false,
      isSystem: true,
      time,
    },
  ]);
}

export function sendMessage(text, replyToId = null, replyToFrom = '', replyToText = '', ttl = 0) {
  const id = get(myId);
  const msg = { type: 'chat', from: id, to: 'broadcast', text: text, ts: (Date.now() / 1000) | 0 };

  if (replyToId) {
    msg.replyTo = replyToId;
    msg.replyToFrom = replyToFrom;
    msg.replyToText = replyToText;
  }
  if (ttl > 0) {
    msg.ttl = ttl;
  }

  if (!ws || ws.readyState !== WebSocket.OPEN) {
    OfflineStorage.savePending(msg);
    OfflineStorage.saveMessage({ from: id, text: text, ts: msg.ts, replyTo: msg.replyTo, ttl: msg.ttl });
    addChatMessage(id, text, true, { ttl });
    showToast('💾 Сохранено (офлайн)');
    return;
  }

  if (msg.to !== 'broadcast' && E2E.hasSharedKey(msg.to)) {
    E2E.checkRotation(ws);
    E2E.encrypt(msg.to, text).then((ct) => {
      msg.text = ct;
      ws.send(JSON.stringify(msg));
    });
  } else {
    ws.send(JSON.stringify(msg));
  }

  const msgData = replyToId
    ? { replyTo: replyToId, replyToFrom: replyToFrom, replyToText: replyToText, ttl: ttl }
    : { ttl: ttl };
  addChatMessage(id, text, true, msgData);
  OfflineStorage.saveMessage({ from: id, text: text, ts: msg.ts, replyTo: msg.replyTo, ttl: msg.ttl });
}

export async function refreshOnline() {
  try {
    const r = await fetch('/api/online');
    const d = await r.json();
    const baseUsers = d.users || [];
    const testUsers = [
      { id: "npub1testsuperuser0000000000000000000000000000000000000000001", name: "📱 Смартфон 1", status: "online", active: true },
      { id: "npub1testsuperuser0000000000000000000000000000000000000000002", name: "📱 Смартфон 2", status: "online", active: true },
      { id: "npub1testsuperuser0000000000000000000000000000000000000000003", name: "📱 Смартфон 3", status: "online", active: true }
    ];
    testUsers.forEach(tu => {
      if (!baseUsers.find(u => u.id === tu.id)) {
        baseUsers.push(tu);
      }
    });
    onlineUsers.set(baseUsers);
  } catch (e) {}
}

// Init identity
export async function loadIdentity() {
  try {
    const r = await fetch('/api/identity');
    const d = await r.json();
    myId.set(d.npub);
    window.myId = d.npub;
    connectWS();
    return d;
  } catch (e) {
    console.error('Identity failed:', e);
    setTimeout(loadIdentity, 3000);
    return null;
  }
}
