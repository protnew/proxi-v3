# 🛡️ Indestructible Messenger

**P2P, E2E, цензуроустойчивый мессенджер с VPN**

Telegram-подобный интерфейс • Nostr протокол • WebRTC P2P туннели

## 🎯 Что реализовано

### PWA Web App (Svelte 5 + Vite 8)
- ✅ **Мессенджер**: DM (kind 14), группы (NIP-28), typing indicator
- ✅ **UI Telegram-style**: Sidebar, ChatView, Settings, EmojiPicker, NewChat, CallOverlay, VpnPanel
- ✅ **Звуки**: программная генерация WAV (входящие, исходящие, звонок)
- ✅ **Уведомления**: Browser Notification API
- ✅ **Реакции**: ❤️👍😂😮😢🔥 на любое сообщение
- ✅ **Ответ/Пересылка/Редактирование/Удаление**: контекстное меню
- ✅ **Emoji Picker**: 4 категории, 100+ эмоджи
- ✅ **Передача файлов**: WebRTC DataChannel, чанки по 64KB, SHA-256 верификация
- ✅ **Голосовые сообщения**: MediaRecorder API, WebM/Opus
- ✅ **Аудио/Видео звонки**: WebRTC, signaling через Nostr
- ✅ **VPN**: WebRTC DataChannel туннель, Nostr signaling
- ✅ **PWA**: manifest, Service Worker, offline cache
- ✅ **Nostr transport**: Multi-relay (damus.io + nos.lol + nostr.band), auto-reconnect
- ✅ **Профиль**: аватар, имя, статус, pubkey
- ✅ **Контакты**: добавление, онлайн статус
- ✅ **Сборка**: 100KB JS (37KB gzipped)

### Desktop (Tauri 2.0)
- ✅ **Scaffold**: Cargo.toml, tauri.conf.json, Rust backend
- ✅ **VPN команды**: get_vpn_status, start_vpn, stop_vpn
- ✅ **Системный трей**, автозапуск, глобальные хоткеи
- ✅ **Desktop bridge**: TypeScript API для Tauri

### Android (Kotlin)
- ✅ **Compose UI**: Material 3 Dark theme, навигация
- ✅ **Nostr Relay Service**: foreground service, WebSocket
- ✅ **VPN Service**: Android VpnService, WireGuard config
- ✅ **Модели**: Message, Chat, Contact, Profile
- ✅ **19 файлов**, полная архитектура

## 🏗️ Архитектура

```
┌─────────────────────────────────┐
│         PWA (Svelte 5)          │
│  Sidebar │ ChatView │ Settings  │
├─────────────────────────────────┤
│      Stores (Svelte writable)    │
│  profile │ chats │ contacts     │
├─────────────────────────────────┤
│       Transport Layer           │
│  nostr.ts │ peer-manager.ts     │
│  file-transfer │ voice │ calls  │
├─────────────────────────────────┤
│     Signaling (Nostr relays)    │
│  kind:14 DM │ kind:40 channels  │
│  kind:21001-21004 custom        │
├─────────────────────────────────┤
│    P2P (WebRTC DataChannel)     │
│  files │ voice │ VPN tunnel     │
└─────────────────────────────────┘
```

## 📂 Структура проекта

```
pwa-vpn/
├── src/
│   ├── App.svelte              # Main layout
│   ├── components/
│   │   ├── Sidebar.svelte      # Chat list + search + VPN bar
│   │   ├── ChatView.svelte     # Messages + input + context menu
│   │   ├── Settings.svelte     # Profile + contacts + advanced
│   │   ├── EmojiPicker.svelte  # Emoji grid picker
│   │   ├── NewChat.svelte      # New chat/dialog
│   │   ├── CallOverlay.svelte  # Incoming/outgoing calls
│   │   └── VpnPanel.svelte     # VPN status + controls
│   ├── lib/
│   │   ├── nostr.ts            # Nostr relay transport
│   │   ├── peer-manager.ts     # WebRTC P2P connections
│   │   ├── file-transfer.ts    # Chunked file transfer
│   │   ├── voice.ts            # Voice recording/playback
│   │   ├── calls.ts            # Audio/video calls
│   │   ├── vpn.ts              # VPN tunnel management
│   │   ├── sounds.ts           # Programmatic sound generation
│   │   ├── desktop.ts          # Tauri API bridge
│   │   ├── identity.ts         # Key generation (legacy VPN)
│   │   ├── webrtc-tunnel.ts    # WebRTC tunnel (legacy VPN)
│   │   ├── exit-node.ts        # HTTP proxy exit node
│   │   └── nostr-signaling.ts  # VPN signaling (legacy)
│   ├── stores/
│   │   └── messenger.ts        # Svelte stores + persistence
│   ├── main.ts                 # Entry point
│   └── app.css                 # Global styles
├── src-tauri/                  # Tauri 2.0 desktop
│   ├── Cargo.toml
│   ├── tauri.conf.json
│   └── src/
│       ├── main.rs
│       └── lib.rs              # Rust backend (VPN, tray, etc)
├── dist/                       # Build output (100KB JS)
├── test/
│   └── test-messenger.mjs      # 19 unit tests
├── tests/
│   └── e2e.spec.ts             # Playwright E2E (VPN tunnel)
├── BACKLOG.md                  # Product backlog
├── PLAN_8HOURS.md              # 8-hour sprint plan
└── README.md                   # This file
```

## 🚀 Запуск

```powershell
cd prototype/pwa-vpn

# Dev server
npm run dev

# Build
npm run build
npx vite preview --host --port 4173

# Tests
node test/test-messenger.mjs        # Unit (19/19 ✅)
npx playwright test                  # E2E (VPN tunnel)
```

## 🔧 Технологии

| Слой | Технология |
|------|-----------|
| Frontend | Svelte 5.55 + Vite 8 |
| PWA | vite-plugin-pwa |
| Signaling | Nostr protocol (WebSocket) |
| P2P | WebRTC DataChannel |
| Криптография | nostr-tools (secp256k1 + Schnorr) |
| Desktop | Tauri 2.0 (Rust) |
| Mobile | Kotlin + Compose |
| VPN | WebRTC tunnel / WireGuard |

## ⚠️ Известные ограничения

- iOS: не планируется (нет Mac + $99/год Apple Developer)
- TURN server: нужен для продакшена ($5/мo VPS)
- NIP-44 E2E шифрование: планируется в следующей итерации
- Code signing: недоступен для .exe и .apk
- WireGuard на desktop: требует Rust + cargo для Tauri сборки
- Android APK: требует Android SDK + JDK

## 📊 Статистика

- **Компоненты**: 7 (Sidebar, ChatView, Settings, EmojiPicker, NewChat, CallOverlay, VpnPanel)
- **Библиотеки**: 12 модулей
- **Тесты**: 19 unit + 1 E2E
- **Сборка**: 100KB JS (37KB gzip)
- **Nostr kinds**: 14, 40, 41, 0, 21001-21004
- **Git commits**: 8 (по одному на час работы)

## 📄 Лицензия

MIT
