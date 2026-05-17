# 🔥 Unkillable Messenger

Децентрализованный мессенджер + VPN + контент-платформа.

## Killer Feature: «Поделись интернет»

Нажал кнопку → друг получил VPN-точку через твоё устройство. Без серверов. Без провайдеров. P2P.

## Архитектура

```
┌─────────────────────────────────────────────────┐
│                  UI (Tauri + Svelte)             │
├─────────────┬──────────────┬────────────────────┤
│  Nostr Chat │  VPN Module  │   IPFS Storage     │
│  (текст/мей)│  (Netbird/WG)│   (файлы/видео)    │
├─────────────┴──────────────┴────────────────────┤
│              P2P Mesh Network                    │
│         WireGuard + STUN/TURN + libp2p           │
└─────────────────────────────────────────────────┘
```

## Стек

| Компонент | Технология |
|---|---|
| UI Framework | Tauri 2.x + Svelte 5 |
| VPN ядро | Netbird (Go, BSD-3) |
| Мессенджер | Nostr protocol |
| Хранение | IPFS / Helia |
| Шифрование | WireGuard (ChaCha20-Poly1305) |
| Анонимность | Tor (опционально) |

## Быстрый старт

```bash
# Установка зависимостей
npm install

# Разработка
npm run tauri dev

# Сборка
npm run tauri build
```

## Roadmap

### Phase 0: VPN-ядро (MVP) — 4 недели
- Netbird fork → desktop клиент
- Кнопка «Поделись интернет» (exit node)
- WireGuard mesh между 2+ устройствами

### Phase 1: Мессенджер — 6 недель
- Nostr-клиент (чат 1-1)
- Peer discovery через Nostr relays
- Шифрованные сообщения (NIP-04/NIP-44)

### Phase 2: Каналы + контент — 8 недель
- Nostr channels (публичные каналы)
- IPFS для файлов/фото
- Push-to-talk голос

### Phase 3: Платформа — 12 недель
- Видео (HLS сегменты через IPFS)
- Подписки на каналы
- Premium ($3/мес)

## Лицензия

AGPL-3.0 — open source, все изменения должны быть открыты

## Контакты

Alexey Shekhovtsov — TG: @Alex1452
