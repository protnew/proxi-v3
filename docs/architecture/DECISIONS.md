# Architecture Decisions

## D-ARCH-001 — Транспорт PWA VPN: WebTransport (QUIC), НЕ WireGuard

**Дата:** 2026-08-06
**Статус:** ПРИНЯТО
**Supersedes:** Т1 (01_VPN_Primary_Transport.md — WebRTC DataChannels)

### Решение
PWA VPN (Фаза 1) использует **WebTransport (QUIC)** как основной транспорт.

### Обоснование
Таблица Т2 (ARCHITECTURE_TABLES.md) провела глубокий анализ и выявила критическую ошибку Т1:
- `RTCPeerConnection` (WebRTC) **физически недоступен внутри Service Workers** — убивает CPU/батарею
- WireGuard получил **0/10** за поддержку в браузере — невозможно использовать в PWA
- WebTransport (QUIC) нативно работает в Service Workers, мультиплексирование, минимум библиотек

### Баллы
| Кандидат | Т1 Score | Т2 Score | Итог |
|----------|----------|----------|------|
| WebTransport (QUIC) | 174 (2-е) | **166** 🥇 | **Победитель** |
| WebRTC DataChannels | 178 (1-е Т1) | 126 | Т2 выявил ошибку |
| WireGuard | — | 126 | Браузер: 0/10 |
| Shadowsocks | — | 119 | — |

### WireGuard — где уместен
- **Фаза 2 (Desktop Tauri):** WireGuard через Netbird daemon ✓
- **Фаза 3 (Mobile):** WireGuard через VpnService (Android) / NetworkExtension (iOS) ✓
- **Фаза 1 (PWA):** ❌ НЕВОЗМОЖЕН — заменить на WebTransport

---

## D-ARCH-002 — DPI Fallback: AmneziaWG (таблица 02)

**Дата:** 2026-08-06
**Статус:** ПРИНЯТО

Когда WebTransport блокируется DPI → fallback на AmneziaWG (195 баллов, +19 от XTLS-Reality).
AmneziaWG = WireGuard с обфускацией заголовков.

---

## D-ARCH-003 — Signaling: Nostr NIP-01/NIP-44 (таблицы 26, 53)

**Дата:** 2026-08-06
**Статус:** ПРИНЯТО

Nostr используется как:
1. **Signaling для VPN** — обмен ICE/QUIC candidates (kind:30090)
2. **Messaging** — NIP-44 E2E сообщения
3. **Identity** — secp256k1 ключи (npub/nsec)
4. **Offline** — NIP-59 Gift Wrap для store-and-forward

---

## D-ARCH-004 — E2E Шифрование: Double Ratchet (Signal) (таблицы 22, 56)

**Дата:** 2026-08-06
**Статус:** ПРИНЯТО

P2P мессенджер без центральных серверов ключей → Double Ratchet + X3DH.
НЕ чистый NIP-44 (он слабее), а кастомная связка Double Ratchet + CRDT.

---

## D-ARCH-005 — Канонический стек Phase 1 (PWA VPN)

| Слой | Технология | Таблица |
|------|-----------|---------|
| UI | Svelte 5 | Т34 |
| State | Nano Stores | Т55 |
| Routing | Tinro | Т52 |
| VPN транспорт | **WebTransport (QUIC)** | Т2 |
| DPI fallback | AmneziaWG | 02 |
| Signaling | **Nostr NIP-01** | 26, 53 |
| Messaging E2E | **Double Ratchet** | 22, 56 |
| Offline | **NIP-59 Gift Wrap** | 30 |
| Voice | WebRTC over DERP (Pion) | 32 (Фаза 4) |
| Group chat | Loro CRDT | 31 (Фаза 4) |
| Desktop | Tauri 2 + JSON-RPC stdin/stdout | 40, 41 (Фаза 2) |
| DB | SQLite + sqlc | 14 |
| E2E tests | Playwright | 39 |

---

## D-AUDIT-2026-08-05 — api.ts library LOC waive

**Decision:** `prototypes/pwa-vpn/src/lib/api.ts` may exceed 500 LOC as a **single client library surface**.
**Rationale:** Splitting would break import surface for Svelte components without architectural gain.
**Constraint:** New feature modules go to sibling files, not unbounded growth.
**Owner:** accepted after Supreme Audit 2026-08-05.
