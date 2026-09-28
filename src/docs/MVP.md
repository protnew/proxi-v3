# MVP — Phase 0: VPN «Поделись интернет»

## Что делаем

Десктопное приложение (Windows/Mac/Linux):
- WireGuard mesh VPN
- Кнопка «Поделись интернет» (exit node)
- Friend management (добавить/удалить)
- Статус подключения

## НЕ делаем (пока)
- Чат / мессенджер
- Каналы / контент
- Мобильная версия
- Анонимность (Tor)

## Технические решения

| Что | Решение |
|-----|---------|
| UI | Tauri 2 + Svelte 5 |
| VPN ядро | Netbird (Go, BSD-3) |
| Туннель | WireGuard (через Netbird) |
| NAT traversal | STUN (встроен в Netbird) |
| Signaling | Свой relay или Netbird Signal |
| Key exchange | X25519 (WireGuard ключи) |
| Сборка | Tauri CLI → .exe / .dmg / .AppImage |

## Срок: 4 недели

| Неделя | Что |
|--------|-----|
| 1 | Tauri shell + Svelte UI + Netbird integration |
| 2 | Exit node toggle + peer management |
| 3 | NAT traversal + STUN + fallback relay |
| 4 | Тестирование +打包 + багфикс |
