# Proxi Messenger — Flutter Mobile App

Decentralized, E2E encrypted messenger built with Flutter.

## Features
- 🔐 End-to-end encryption (X25519 + AES-256-GCM)
- 💬 Real-time messaging via WebSocket
- 📢 Channels (subscribe/publish)
- 🌙 Telegram-style dark theme (Material Design 3)
- 🔑 Identity-based authentication
- 💾 Offline-first with local caching

## Getting Started

```bash
flutter pub get
flutter run
```

## Project Structure

```
mobile/
├── lib/
│   ├── main.dart                  # Entry point
│   ├── models/                    # Data models
│   │   ├── chat.dart
│   │   ├── message.dart
│   │   ├── user.dart
│   │   ├── channel.dart
│   │   └── identity.dart
│   ├── screens/                   # UI screens
│   │   ├── login_screen.dart      # Identity creation/restore
│   │   ├── chat_list_screen.dart  # Chat list + bottom nav
│   │   ├── chat_screen.dart       # Messages + input bar
│   │   ├── profile_screen.dart    # Profile & key management
│   │   ├── channels_screen.dart   # Channel list
│   │   └── settings_screen.dart   # App settings
│   ├── widgets/                   # Reusable widgets
│   │   ├── message_bubble.dart    # Chat bubble
│   │   ├── voice_recorder.dart    # Voice recording
│   │   └── contact_avatar.dart    # Avatar with online status
│   └── services/                  # Business logic
│       ├── api_service.dart       # REST API client
│       ├── ws_service.dart        # WebSocket with reconnect
│       ├── e2e_service.dart       # Encryption
│       ├── storage_service.dart   # Local persistence
│       ├── notification_service.dart
│       └── theme.dart             # Telegram-style dark theme
├── pubspec.yaml
└── assets/
```

## Backend

Connects to Go server at `localhost:9999` (configurable in settings).
