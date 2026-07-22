# DEPRECATED — Legacy Code

**Дата архивации:** 02 июля 2026

Этот каталог содержит **legacy код** (.04-Src/src/src-vpn/) — 200 Go файлов, 43K строк.

## Статус: ЗАБРОШЕН

Единственный активный код: `src/core/` (26 Go файлов, 2.5K строк).

## Что здесь:
- Legacy monolith (src-vpn/) — main.go 1921 строк
- Prototypes (pwa-vpn/, proxi-server/)
- 4 Dockerfile + 7 docker-compose.yml
- cloudflared.exe (37MB) — УДАЛЁН
- cert.key/cert.pem — УДАЛЕНЫ (security fix)

## План:
Переместить в 10-Archive/legacy-monolith/ в Phase 1.
