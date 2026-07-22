# Indestructible Messenger — Статус и Бэклог

**Дата:** 26 мая 2026
**Папка:** `C:\Сделать\Неубиваемый контент\prototype\pwa-vpn\`
**Тест:** `npm run dev` (порт 5173+)

---

## 📊 Статистика

| Метрика | Значение |
|---|---|
| Файлов src/ | 20 |
| Строк кода | ~4,500 |
| JS бандл (gzip) | 58 KB |
| CSS бандл (gzip) | 3.8 KB |
| Unit тесты | 6/6 ✅ |
| E2E тесты | 6 Playwright |
| Билд | ✅ 0 ошибок |

---

## ✅ Sprint 1 — ЗАВЕРШЁН (Минимальный мессенджер)

| # | Задача | Статус |
|---|---|---|
| 1.1 | Реактивный DM UI (callbacks до connect, enriched sortedChats) | ✅ |
| 1.2 | Reactive chat list с lastMessage | ✅ |
| 1.3 | Звук входящего (AudioContext + resume) | ✅ |
| 1.4 | Notification API push | ✅ |
| 1.5 | Emoji picker (100+ emoji, 4 группы) | ✅ |
| 1.6 | Reply to message (Nostr e-tag + quote) | ✅ |
| 1.7 | E2E тест Playwright (6 сценариев) | ✅ |
| — | **Критический фикс:** Svelte 5 runes ($store → subscribe+$state) | ✅ |

## ✅ Sprint 2 — ЗАВЕРШЁН (Файлы, голос, медиа)

| # | Задача | Статус |
|---|---|---|
| 2.1 | Чанковая передача файлов через WebRTC DC (64KB, SHA-256) | ✅ |
| 2.2 | Upload UI: прогресс бар, drag&drop, paste image | ✅ |
| 2.3 | Голосовые через P2P (>50KB) + waveform | ✅ |
| 2.4 | Входящие файлы через FileReceiver | ✅ |

## ✅ Sprint 3 — ЗАВЕРШЁН (Безопасность, группы, UX)

| # | Задача | Статус |
|---|---|---|
| 3.1 | **NIP-44 E2E шифрование DM** | ✅ |
| 3.2 | Image preview в чате | ✅ |
| 3.3 | NIP-28 группы — создание + sendGroupMessage | ✅ |
| 3.4 | Typing indicator (печатает...) | ✅ |
| 3.5 | Адаптив для мобильного (кнопка ←, media queries) | ✅ |
| 3.6 | Тёмная/светлая тема (CSS variables + toggle) | ✅ |
| 3.7 | Поиск по сообщениям (full-text, 3+ символов) | ✅ |

---

## ❌ Что НЕ готово (следующие спринты)

### 🔴 Критичное
- [ ] P2P WebRTC через NAT — нужен TURN сервер ($5/мес VPS)
- [ ] Tauri Desktop build — нужен Rust + cargo
- [ ] Android APK build — Kotlin scaffold есть, нужен SDK
- [ ] Service Worker VPN proxy (перехват fetch)

### 🟡 Фичи
- [ ] Групповые звонки (SFU)
- [ ] Каналы (публичные, подписка)
- [ ] Шифрование файлов (AES-256-GCM)
- [ ] IPFS хранение
- [ ] QR код для обмена pubkey
- [ ] Perfect forward secrecy
- [ ] Lightning Network микроплатежи

---

## 🏗️ Файловая структура

```
src/
├── App.svelte              ← Layout + Nostr callbacks + theme init
├── main.ts
├── components/
│   ├── Sidebar.svelte      ← Чаты + поиск по сообщениям
│   ├── ChatView.svelte     ← Сообщения, ввод, файлы, голос, drag&drop
│   ├── Settings.svelte     ← Профиль + тема + данные
│   ├── EmojiPicker.svelte  ← 100+ emoji
│   ├── NewChat.svelte      ← Новый DM
│   ├── GroupCreate.svelte  ← Новая группа (NIP-28)
│   ├── CallOverlay.svelte  ← UI звонков
│   └── VpnPanel.svelte     ← VPN toggle
├── stores/
│   └── messenger.ts        ← Svelte 5 stores + persistence
├── lib/
│   ├── nostr.ts            ← NIP-44 encrypted DM, relays, signaling
│   ├── peer-manager.ts     ← WebRTC P2P файлы + сигналинг
│   ├── file-transfer.ts    ← Чанки 64KB, SHA-256, flow control
│   ├── voice.ts            ← MediaRecorder + waveform
│   ├── calls.ts            ← WebRTC audio/video
│   ├── search.ts           ← Full-text search
│   ├── theme.ts            ← Light/dark theme
│   ├── sounds.ts           ← AudioContext beeps
│   ├── vpn.ts              ← VPN status
│   ├── webrtc-tunnel.ts    ← VPN tunnel
│   ├── exit-node.ts        ← HTTP proxy
│   ├── identity.ts         ← secp256k1 keys
│   ├── nostr-signaling.ts  ← VPN signaling
│   └── desktop.ts          ← Tauri bridge
├── test/test-connection.mjs
└── tests/e2e-messenger.spec.ts
```

## 📈 Roadmap

```
ФАЗА 1: VPN PWA          ← 85% (туннель + UI + P2P)
ФАЗА 2: Desktop (Tauri)   ← Scaffold, нужен build
ФАЗА 3: Mobile             ← Kotlin scaffold
ФАЗА 4: Мессенджер         ← 75% (DM✅ файлы✅ группы✅ шифрование✅)
ФАЗА 5: Крипто-экономика   ← Не начато
```
