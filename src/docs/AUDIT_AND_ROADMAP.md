# Продуктовый аудит MVP Unkillable Messenger
## Дата: 18 мая 2026 | Версия: 1.0

---

## 1. GAP ANALYSIS: Vision vs Реализация

### Оценка по шкале: 🟢 работает | 🟡 частично | 🔴 заглушка/отсутствует | ⚫ не начато

| Компонент | Vision (из Манифеста) | Текущее состояние | GAP |
|---|---|---|---|
| **UI Shell** | Tauri 2 + Svelte 5 десктоп + Web PWA | 🟢 Svelte 5 + Vite, 3 панели (Chat/VPN/Channels), Sidebar, Cloudflare tunnel | ✅ Базовый shell готов |
| **API Layer** | Tauri invoke + Web fallback | 🟢 api.js с auto-detect (Tauri internals / JSON-RPC HTTP) | ✅ Рабочий |
| **Go Web Server** | Раздача статики + API | 🟢 main.go: SPA fallback, /api/status, /api/vpn/rpc, CORS | ✅ Рабочий |
| **Docker** | Контейнеризация | 🟢 Dockerfile multi-stage (Node→Go→Debian+WG), docker-compose | ✅ Рабочий |
| **VPN Manager (Go)** | WireGuard mesh, exit node, peer mgmt | 🟡 Структура кода есть (vpn.go ~514 строк), НО web-сервер возвращает заглушки (demo-key) | ⚠️ Код есть, интеграция нет |
| **Identity / Ключи** | secp256k1 + npub/nsec + seed phrase | 🔴 Нет. WireGuard генерация ключей через `wg genkey` (упрощённо, fallback = hex(rand)) | ❌ Нет Nostr-identity |
| **Чат (DM)** | Nostr NIP-44 E2E, real-time через relay | 🔴 Полная заглушка. Input disabled. Нет Nostr-клиента, нет WebSocket, нет сообщений | ❌ 0% |
| **Групповой чат** | NIP-28 channels / NIP-44 shared key | ⚫ Не начат | ❌ 0% |
| **Каналы (контент)** | Nostr kind:30078/30079 + IPFS | 🔴 Placeholder «Phase 2» | ❌ 0% |
| **IPFS нода** | Embedded node, LRU cache, pin | ⚫ Не начат | ❌ 0% |
| **Шифрование файлов** | AES-256-GCM, erasure coding K=3/N=5 | ⚫ Не начат | ❌ 0% |
| **WireGuard VPN** | Mesh, NAT traversal, STUN | 🟡 Go-код для `ip link add`, `wg set`, iptables — работает только под root/Linux | ⚠️ 40% (только Linux) |
| **«Поделись интернет»** | Exit node toggle, friend management | 🟡 UI кнопка есть, Go-методы StartExitNode/StopExitNode написаны, но web-сервер возвращает stub | ⚠️ 30% |
| **Peer management** | Добавить/удалить друзей, online status | 🟡 Методы AddPeer/RemovePeer/heartbeatLoop есть, но UI не подключён | ⚠️ 20% |
| **Nostr relay** | Подключение к публичным relay | ⚫ Не начат | ❌ 0% |
| **Tor transport** | Анонимный режим | ⚫ Не начат | ❌ 0% |
| **WebRTC звонки** | Signaling через Nostr | ⚫ Не начат | ❌ 0% |
| **Telegram cross-posting** | Bot bridge TG→Nostr+IPFS | ⚫ Не начат | ❌ 0% |
| **Premium / монетизация** | Подписка, донаты | ⚫ Не начат | ❌ 0% |
| **PWA / Service Worker** | Офлайн-доступ | 🟢 manifest.json + sw.js (базовый) | ✅ Заготовка |

### ИТОГО: Проект на ~10-12% от vision

**Что реально работает:**
1. ✅ Фронтенд-шелл (Svelte 5, тёмная тема, 3 панели)
2. ✅ Go-сервер (статику + JSON-RPC API)
3. ✅ Docker-сборка + Cloudflare tunnel
4. ✅ API-мост (Tauri invoke ↔ HTTP fallback)

**Что НЕ работает, но код написан (потенциал):**
5. 🟡 VPN Manager на Go (нужна интеграция с webserver, а не stub-ответы)
6. 🟡 Peer management (UI + API не соединены)

**Критический блокер:** **Нет Nostr-протокола** = нет чата, нет каналов, нет identity. Это основа всей архитектуры.

---

## 2. ROADMAP на 4 недели

### Принципы приоритизации:
- **MVP = killer feature («Поделись интернет») + минимальный чат**
- Чат без E2E сначала (простой WebSocket) → потом Nostr
- VPN работает первым — это viral loop (поделись с другом → друг ставит приложение)
- Каналы — Phase 2 (после того как чат + VPN стабильно работают)

---

### НЕДЕЛЯ 1: «Поделись интернет» + Real VPN

**Цель:** Killer feature реально работает — можно поделиться интернетом с другом через WireGuard

| # | Задача | Часы | Стек | Зависимости | Приоритет |
|---|--------|------|------|-------------|-----------|
| 1.1 | **Подключить vpn.Manager к webserver** (убрать stub-ответы, реальный VPN status) | 4h | Go | — | **MUST** |
| 1.2 | **Peer management UI**: форма «Добавить друга» (имя + pubkey + endpoint), список peers с online/offline | 6h | Svelte 5, api.js | 1.1 | **MUST** |
| 1.3 | **VPN ключи**: реальная генерация WireGuard ключей без `wg` CLI (Go crypto/curve25519), сохранение в файл | 3h | Go (golang.org/x/crypto/curve25519) | — | **MUST** |
| 1.4 | **Exit node flow**: полная цепочка UI→API→WG interface→NAT→heartbeat | 8h | Go, Linux netlink | 1.1, 1.3 | **MUST** |
| 1.5 | **Сохранение peers**: персистентность в JSON файл (при рестарте не теряются) | 2h | Go | 1.2 | **MUST** |
| 1.6 | **Тест**: 2 машины → одна делит интернет, вторая подключается | 4h | Linux + WireGuard | 1.4 | **MUST** |
| 1.7 | **QR-код для обмена ключами**: генерация QR из pubkey+endpoint | 2h | JS (qrcode lib) | 1.2 | NICE |
| | **ИТОГО НЕДЕЛЯ 1** | **~29h** | | | |

---

### НЕДЕЛЯ 2: Рабочий чат (WebSocket + базовый протокол)

**Цель:** Можно написать сообщение и получить ответ в реальном времени

| # | Задача | Часы | Стек | Зависимости | Приоритет |
|---|--------|------|------|-------------|-----------|
| 2.1 | **WebSocket hub на Go**: chat server (broadcast + direct messages) | 6h | Go (gorilla/websocket или nhooyr.io/websocket) | — | **MUST** |
| 2.2 | **Identity v1**: генерация ключевой пары (secp256k1 или ed25519), npub/nsec формат, localStorage | 4h | JS (noble-secp256k1) | — | **MUST** |
| 2.3 | **Chat UI v1**: список контактов, выбор чата, ввод сообщения, отображение истории | 8h | Svelte 5 | 2.1, 2.2 | **MUST** |
| 2.4 | **Message storage**: SQLite (через Go) или IndexedDB (браузер) — история сообщений | 4h | Go (mattn/go-sqlite3) или JS | 2.1 | **MUST** |
| 2.5 | **Файловый обмен через VPN**: отправка файла → WireGuard tunnel → получение | 6h | Go, JS (File API) | 1.4 (VPN работает) | **MUST** |
| 2.6 | **Уведомления**: browser notifications о новых сообщениях | 2h | JS Notification API | 2.3 | NICE |
| 2.7 | **Emoji picker базовый** | 2h | Svelte component | 2.3 | NICE |
| | **ИТОГО НЕДЕЛЯ 2** | **~32h** | | | |

---

### НЕДЕЛЯ 3: Nostr-протокол + Шифрование

**Цель:** Чат работает через Nostr relay (децентрализация), сообщения зашифрованы

| # | Задача | Часы | Стек | Зависимости | Приоритет |
|---|--------|------|------|-------------|-----------|
| 3.1 | **Nostr client на Go**: WebSocket к relay, подписка на события, publish | 8h | Go (gorilla/websocket), NIP-01 | — | **MUST** |
| 3.2 | **NIP-44 шифрование**: E2E для DM (encrypt/decrypt) | 6h | Go (secp256k1 + AES-256-GCM) | 2.2 (identity) | **MUST** |
| 3.3 | **Nostr relay (свой)**: минимальный relay на Go для fallback | 8h | Go | 3.1 | **MUST** |
| 3.4 | **Bridge: WS chat ↔ Nostr**: миграция с простого WS на Nostr events | 6h | Go | 3.1, 2.1 | **MUST** |
| 3.5 | **Групповой чат v1**: простая группа (shared key, NIP-28) | 6h | Go, Svelte | 3.2, 2.3 | NICE |
| 3.6 | **Профиль**: аватар, никнейм, NIP-01 metadata event | 4h | Svelte, Nostr | 3.1 | NICE |
| | **ИТОГО НЕДЕЛЯ 3** | **~38h** | | | |

---

### НЕДЕЛЯ 4: Каналы v1 + Полировка + Сборка

**Цель:** Базовые каналы работают, приложение собирается для дистрибуции

| # | Задача | Часы | Стек | Зависимости | Приоритет |
|---|--------|------|------|-------------|-----------|
| 4.1 | **Каналы UI**: список каналов, создание канала, просмотр постов | 8h | Svelte 5 | 3.1 (Nostr) | **MUST** |
| 4.2 | **Nostr kind:30078/30079**: создание/чтение каналов и постов через relay | 6h | Go, NIP-28/30078 | 3.1 | **MUST** |
| 4.3 | **Текстовые посты**: публикация текста + изображений в канал | 4h | JS, Go | 4.2 | **MUST** |
| 4.4 | **Tauri сборка**: .exe / .dmg / .AppImage с embedded Go VPN binary | 6h | Tauri CLI, cross-compile | 1.4 | **MUST** |
| 4.5 | **Onboarding**: экран приветствия, генерация ключей, объяснение seed phrase | 4h | Svelte | 2.2 | **MUST** |
| 4.6 | **IPFS v1 (упрощённо)**: загрузка файлов через публичный IPFS gateway (pinata/web3.storage) | 4h | JS (ipfs-http-client) | — | NICE |
| 4.7 | **Тёмная/светлая тема toggle** | 2h | CSS variables | — | NICE |
| 4.8 | **Аналитика v1**: базовые метрики (кол-во сообщений, подключений, uptime) | 3h | Go, SQLite | — | NICE |
| | **ИТОГО НЕДЕЛЯ 4** | **~37h** | | | |

---

## 3. СВОДНАЯ ТАБЛИЦА ROADMAP

| Неделя | Тема | Ключевой результат | MUST HAVE часов | NICE TO HAVE часов | Итого |
|--------|------|--------------------|-----------------|--------------------|-------|
| **1** | VPN «Поделись интернет» | WireGuard exit node работает, peer management, реальные ключи | 27h | 2h | **29h** |
| **2** | Чат real-time | Отправка/получение сообщений, контакты, история, файлы через VPN | 28h | 4h | **32h** |
| **3** | Nostr + E2E | Децентрализованный чат через Nostr relay, NIP-44 шифрование | 28h | 10h | **38h** |
| **4** | Каналы + Сборка | Текстовые каналы, десктоп-сборка, onboarding | 28h | 9h | **37h** |
| | | **ИТОГО** | **111h** | **25h** | **~136h** |

> При 8h/день ≈ **17 рабочих дней** (3.5 недели). Реалистичный бюджет с багфиксом и тестированием: **4 недели**.

---

## 4. MUST HAVE vs NICE TO HAVE

### 🔴 MUST HAVE (111h) — без этого продукт не MVP

| Компонент | Почему MUST |
|-----------|-------------|
| VPN exit node (реальный) | Killer feature, viral loop, уникальность |
| Peer management + exchange | Без этого VPN бесполезен |
| WireGuard ключи (Go crypto) | Без зависимости от `wg` CLI |
| Чат real-time (WebSocket) | Мессенджер без чата = не мессенджер |
| Identity (ключевая пара) | Основа для Nostr и шифрования |
| История сообщений | Базовый UX |
| Файловый обмен через VPN | Дифференциатор от Telegram |
| Nostr client + relay | Децентрализация = суть продукта |
| NIP-44 E2E шифрование | Приватность = promise продукта |
| Каналы (текстовые посты) | Контент-платформа = второй pillar |
| Tauri сборка десктоп | Дистрибуция юзерам |
| Onboarding + seed phrase | Без этого юзер потеряет аккаунт |

### 🟢 NICE TO HAVE (25h) — улучшает UX, не блокирует MVP

| Компонент | Почему NICE |
|-----------|-------------|
| QR-код обмена ключами | Удобно, но можно копипастить |
| Browser notifications | UX плюс |
| Emoji picker | Красиво |
| Групповой чат | Важен, но можно в Phase 2 |
| Профиль (аватар, ник) | NIP-05, не критично для старта |
| IPFS через gateway | Можно хранить файлы на relay пока |
| Тёмная/светлая тема | Тёмная уже есть |
| Аналитика | Для внутренней оптимизации |

---

## 5. КРИТИЧЕСКИЕ РИСКИ

| Риск | Вероятность | Влияние | Митигация |
|------|-------------|---------|-----------|
| WireGuard не работает в Docker на macOS/Windows | Высокое | Высокое | Документировать: VPN только Linux desktop. macOS/Windows — через Tauri + WG userspace (wireguard-go) |
| Nostr relay нестабилен | Среднее | Среднее | Свои relay (2-3 VPS по $5/мес) + fallback к публичным |
| NAT traversal не работает для некоторых ISP | Высокое | Среднее | STUN + TURN relay (Coturn) для сложных NAT |
| Криптография ключей некорректна | Низкое | Критическое | Использовать проверенные библиотеки (noble-secp256k1, golang.org/x/crypto) |
| Нет virality = нет юзеров | Высокое | Критическое | «Поделись интернет» — единственный надёжный viral loop. Кнопка → QR → друг ставит приложение |

---

## 6. РЕКОМЕНДАЦИИ

1. **Не реализовывать IPFS в MVP.** Для начала файлы хранить на relay/server. IPFS — Phase 2, когда есть юзеры и контент.
2. **Начать с простого WS чата (Неделя 2), потом мигрировать на Nostr (Неделя 3).** Так можно быстрее показать working chat.
3. **VPN — только Linux desktop для MVP.** macOS/Windows WireGuard = отдельная неделя работы (wireguard-go + Tauri sidecar).
4. **Не делать Tor в MVP.** Гибридный транспорт = Phase 3, после стабилизации базового функционала.
5. **Сфокусироваться на viral loop:** «Поделись интернет» → друг ставит приложение → может общаться → делится с другими.

---

*Документ подготовлен на основе анализа дизайн-документов (00_МАНИФЕСТ, 01_АРХИТЕКТУРА, 02_ЭКОНОМИКА) и исходного кода репозитория unkillable-messenger.*
