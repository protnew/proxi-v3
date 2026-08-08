# Dual-WiFi — что это

Это НЕ фича кода, а **физический тест isolation**.

## Сценарий
1. **Alice** в сети A (домашний Wi‑Fi) — `http://<IP-A>:8090/?role=alice` или PWA
2. **Bob** в сети B (раздача с телефона / другой Wi‑Fi / другой город) — `?role=bob`
3. Они **не** в одном LAN (не 192.168.x общая подсеть)

## Зачем
- WebRTC+STUN: пробивает ~85% NAT между разными сетями
- 15% symmetric NAT → Nostr data relay fallback
- Gift-wrap offline invite: Alice ушла offline, Bob позже забрал kind:1059

## Как прогнать руками (2 устройства)
1. ПК Alice: сервер :8090, дать VPN другу (или gift-wrap invite)
2. Телефон Bob на LTE: открыть URL (нужен reachable host — LAN IP не подойдёт через LTE без туннеля)
3. Для LTE→home нужен либо P2P hole punch после Nostr signaling, либо Nostr relay path

## Ограничение лаборатории
Один агент на одном Windows-хосте **не может** сам создать вторую физическую сеть.
Максимум автотеста: publish gift-wrap на public relay + userspace tunnel + UI.
