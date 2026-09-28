# 🗓️ План на 8 часов автономной работы

**Старт:** 25 мая 2026, 23:00
**Финиш:** 26 мая 2026, 07:00
**Токен бюджет:** 200M (хватит с запасом)
**Цель:** Полноценный мессенджер с VPN для Web + Desktop + Android

---

## 📋 Детальный план по часам

### ЧАС 1 (23:00–00:00) — Рабочий мессенджер: DM в реальном времени

| Задача | Время | Что делаю |
|---|---|---|
| 1.1 Починить DM доставку | 20 мин | Nostr subscription → store → reactive UI. Сейчас сообщения приходят но sidebar не обновляется |
| 1.2 Реактивный sidebar | 15 мин | $derived для sorted chats, auto-select первого чата |
| 1.3 Звук входящего | 10 мин | Web Audio API, короткий bip из base64 (без внешних файлов) |
| 1.4 Notification API | 10 мин | Push уведомление при новом сообщении, вкладка в фоне |
| 1.5 Emoji picker | 15 мин | Компонент с 80 emoji, вставка в textarea |
| **Результат:** | | Два человека открывают сайт → обмениваются pubkey → переписываются в реальном времени с звуком и уведомлениями |

### ЧАС 2 (00:00–01:00) — Передача файлов через WebRTC

| Задача | Время | Что делаю |
|---|---|---|
| 2.1 File transfer protocol | 20 мин | lib/file-transfer.ts: chunked отправка через DataChannel (64KB чанки), manifest, resume |
| 2.2 File send UI | 15 мин | Drag & drop, paste image, прогресс бар, отмена |
| 2.3 File receive | 15 мин | Автосохранение, preview для картинок, download button |
| 2.4 Изображения в чате | 10 мин | Thumbnail, lightbox preview, blur placeholder |
| **Результат:** | | Перетаскиваешь файл в чат → улетает через WebRTC → друг видит и скачивает |

### ЧАС 3 (01:00–02:00) — Голосовые сообщения + WebRTC звонки

| Задача | Время | Что делаю |
|---|---|---|
| 3.1 Голосовые через DataChannel | 20 мин | MediaRecorder → chunks → DataChannel → сборка на другой стороне |
| 3.2 Визуализация волны | 15 мин | Web Audio AnalyserNode → canvas bars при записи |
| 3.3 Воспроизведение голосовых | 10 мин | Audio player с прогрессом, скорость 1x/1.5x/2x |
| 3.4 Звонок 1-на-1 | 15 мин | WebRTC audio call, signaling через Nostr, UI ring/incoming |
| **Результат:** | | Удерживаешь 🎤 → записываешь → отправляешь. Кнопка 📞 → звонок |

### ЧАС 4 (02:00–03:00) — Группы, каналы, типинг

| Задача | Время | Что делаю |
|---|---|---|
| 4.1 Создание группы (NIP-28) | 20 мин | Kind:40 create channel, kind:41 message, join, leave |
| 4.2 Групповой чат UI | 20 мин | Список участников, аватарки в чате, упоминания |
| 4.3 Typing indicator | 10 мин | Debounced typing event → Nostr → UI "печатает..." |
| 4.4 Reply / Forward / Edit | 10 мин | Контекстное меню, reply-to tag, edit NIP support |
| **Результат:** | | Создаёшь группу, приглашаешь по pubkey, все видят сообщения, кто печатает |

### ЧАС 5 (03:00–04:00) — VPN интеграция + E2E шифрование

| Задача | Время | Что делаю |
|---|---|---|
| 5.1 VPN toggle в мессенджере | 15 мин | Кнопка в sidebar header, статус бар, список exit nodes |
| 5.2 Автоподключение VPN | 10 мин | При запуске → connect к последнему exit node |
| 5.3 NIP-44 E2E шифрование | 20 мин | Gift-wrapped DMs, расшифровка при получении |
| 5.4 Шифрование файлов | 15 мин | AES-256-GCM перед отправкой, decrypt при получении |
| **Результат:** | | VPN работает из мессенджера. Все сообщения зашифрованы E2E |

### ЧАС 6 (04:00–05:00) — Desktop приложение (Tauri)

| Задача | Время | Что делаю |
|---|---|---|
| 6.1 Tauri scaffold | 15 мин | cargo init tauri, tauri.conf.json, CommonJS bridge |
| 6.2 WireGuard daemon | 20 мин | Rust модуль: wg-quick up/down, netbird integration |
| 6.3 Системный tray | 10 мин | Иконка в трее, VPN on/off, статус |
| 6.4 Desktop build | 15 мин | Windows .exe, .msi installer, autoupdate |
| **Результат:** | | Устанавливаешь .msi → системный VPN + мессенджер в одном приложении |

### ЧАС 7 (05:00–06:00) — Android приложение

| Задача | Время | Что делаю |
|---|---|---|
| 7.1 Kotlin проект | 15 мин | Android scaffold, Jetpack Compose, gradle |
| 7.2 WireGuard VpnService | 20 мин | Android VpnService API, WireGuard tunnel library |
| 7.3 Nostr client (KMP) | 15 мин | Kotlin WebSocket client, event signing, relay pool |
| 7.4 UI (Compose) | 10 мин | Список чатов, экран сообщений, настройки |
| **Результат:** | | Android проект собирается, VPN работает на уровне системы |

### ЧАС 8 (06:00–07:00) — Тесты, полировка, деплой

| Задача | Время | Что делаю |
|---|---|---|
| 8.1 E2E тест мессенджера | 20 мин | Playwright: отправка DM, файл, голосовое, группа |
| 8.2 GitHub Pages деплой | 10 мин | gh-pages branch, vite build, auto deploy |
| 8.3 README + документация | 10 мин | Обновить README, ARCHITECTURE, CONTRIBUTING |
| 8.4 Финальная полировка | 20 мин | Fix warnings, убрать dead code, минификация |
| **Результат:** | | Всё работает на GitHub Pages, тесты зелёные, документация актуальна |

---

## 🎯 Что будет к концу 8 часов

### Web (PWA) — 100% готово
- ✅ Мессенджер: DM, группы, каналы, файлы, голосовые, звонки
- ✅ VPN: toggle, exit node, автоподключение
- ✅ E2E шифрование: NIP-44 + AES-256-GCM
- ✅ PWA: installable, offline, notifications
- ✅ Деплой: GitHub Pages

### Desktop (Tauri) — 80% готово
- ✅ Системный VPN (WireGuard)
- ✅ Мессенджер (тот же UI)
- ✅ Tray icon, автозапуск
- ⚠️ Нужна подпись кода для распространения

### Android — 60% готово
- ✅ Системный VPN (WireGuard + VpnService)
- ✅ Nostr клиент
- ✅ UI (Compose)
- ⚠️ Нужен Google Play аккаунт для публикации
- ⚠️ iOS — не успею (нужен Mac + Apple Developer)

---

## ⚠️ Честные ограничения

1. **iOS** — не успею за 8 часов (нужен Xcode + Mac + Apple Developer $99/год)
2. **Сборка Android APK** — нужен Android SDK + JDK, код будет готов но может не собраться на твоей машине без Android Studio
3. **Подпись кода Desktop** — .exe будет без цифровой подписи (Windows SmartScreen предупредит)
4. **TURN сервер** — нужен VPS ($5/мес) для NAT traversal в продакшене
5. **E2E шифрование** — NIP-44 реализация будет упрощённой (полная требует audit)
6. **IPFS хранение** — не успею вписать, файлы пойдут через WebRTC DataChannel (пока хватает)

---

## 🚀 Стартую?

Если да — я начинаю с Часа 1 прямо сейчас и иду по плану автономно.
Каждый час пишу краткий отчёт в чат.
Если нужно перенаправить — пиши в любой момент.
