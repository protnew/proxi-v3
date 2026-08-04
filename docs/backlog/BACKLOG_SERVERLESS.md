# F-SERVERLESS — Бэклог перехода на безсерверную архитектуру

> **Цель:** PWA работает без центрального Go-сервера. Статика на Cloudflare Pages ($0),
> сообщения через Nostr-реляи, файлы через IPFS, ключи в браузере.
>
> **Source of Truth:** `backlog_proxi_v3.db` (SQLite, WAL). Этот MD — зеркало.

---

## Сводка

| Блок | Задач | TODO | Время (ч) | user_value Σ |
|------|-------|------|-----------|--------------|
| IPFS-Storage | 5 | 5 | 0.9 | 39 |
| PWA-Production | 6 | 6 | 0.9 | 48 |
| NIP-E2E | 4 | 4 | 0.8 | 36 |
| F-Serverless | 1 | 1 | 0.2 | 10 |
| Browser-ID | 5 | 5 | 0.8 | 44 |
| Nostr-Transport | 5 | 5 | 1.0 | 44 |
| CF-Pages | 4 | 4 | 0.5 | 36 |
| **ИТОГО** | **30** | **30** | **5.2** | — |

## Архитектура: До → После

```
ДО (сейчас):  PWA → Go :8080 (API + WS + static)
               Go-сервер = единая точка отказа

ПОСЛЕ (цель):  PWA → Cloudflare Pages (static, $0, HTTPS)
               PWA → Nostr relays (wss://relay.damus.io…)
               PWA → IPFS (Helia / gateway)
               Ключи secp256k1 в localStorage браузера
               Go-сервер = опциональный self-hosted relay
```

---

## IPFS-Storage

| ID | Задача | Min | UV | Score | Deps | Status | DoD |
|---|---|---|---|---|---|---|---|
| `SL-042` | Download by CID → decrypt → display | 12 | 9 | 0.75 | — | **TODO** | File downloads and displays (image/pdf) |
| `SL-044` | File size limits + progress bar | 10 | 6 | 0.60 | — | **TODO** | Progress > 0; files >50MB rejected |
| `SL-043` | Image preview from IPFS in chat | 8 | 7 | 0.88 | — | **TODO** | Image preview visible in UI |
| `SL-041` | Upload file → CID → Nostr file event | 12 | 9 | 0.75 | — | **TODO** | File uploaded; CID received; event published |
| `SL-040` | Helia browser node init or gateway fallback | 15 | 8 | 0.53 | — | **TODO** | Helia node created or gateway works |

## PWA-Production

| ID | Задача | Min | UV | Score | Deps | Status | DoD |
|---|---|---|---|---|---|---|---|
| `SL-055` | Mobile PWA install test (Add to Home Screen) | 5 | 8 | 1.60 | — | **TODO** | Install prompt works; icon on home screen |
| `SL-054` | Final CF Pages deploy: full serverless stack | 8 | 10 | 1.25 | — | **TODO** | Public URL; all features work without Go server |
| `SL-053` | Offline queued messages via Nostr outbox | 15 | 8 | 0.53 | — | **TODO** | Offline message saved; sent when online |
| `SL-052` | App icons 192/512 PNG | 5 | 7 | 1.40 | — | **TODO** | manifest.icons with PNG; Lighthouse installable |
| `SL-051` | Push notifications via Web Push + Nostr | 15 | 7 | 0.47 | — | **TODO** | Push permission granted; notification shows |
| `SL-050` | Service Worker precache dist assets | 8 | 8 | 1.00 | — | **TODO** | SW registered; offline mode works (at least UI) |

## NIP-E2E

| ID | Задача | Min | UV | Score | Deps | Status | DoD |
|---|---|---|---|---|---|---|---|
| `SL-034` | E2E encryption unit tests | 8 | 8 | 1.00 | — | **TODO** | 5+ test cases PASS |
| `SL-033` | NIP-44 v2 upgrade (modern encryption) | 15 | 8 | 0.53 | — | **TODO** | NIP-44 encrypt/decrypt PASS; backward compat |
| `SL-031` | Send DM as encrypted Nostr event kind:4 | 12 | 10 | 0.83 | — | **TODO** | Relay receives kind:4; content encrypted |
| `SL-030` | NIP-04 E2E encryption (ECDH + AES) | 15 | 10 | 0.67 | — | **TODO** | Encrypt→decrypt round-trip in vitest; crypto.subtle |

## F-Serverless

| ID | Задача | Min | UV | Score | Deps | Status | DoD |
|---|---|---|---|---|---|---|---|
| `SL-032` | Receive + decrypt DM kind:4 events | 12 | 10 | 0.83 | — | **TODO** | Received message decrypted and shown in UI |

## Browser-ID

| ID | Задача | Min | UV | Score | Deps | Status | DoD |
|---|---|---|---|---|---|---|---|
| `SL-024` | Identity persistence unit test | 8 | 7 | 0.88 | — | **TODO** | Vitest PASS: identity round-trip |
| `SL-023` | BIP39 seed phrase generation + recovery | 15 | 8 | 0.53 | — | **TODO** | Seed generated; recovery → same pubkey |
| `SL-022` | Remove Go signup dependency from initIdentityAsync | 8 | 10 | 1.25 | — | **TODO** | Network tab: no /api/auth/signup; identity works offline |
| `SL-021` | Derive npub/nsec bech32 in browser | 8 | 9 | 1.12 | — | **TODO** | npub generated; matches Go format |
| `SL-020` | @noble/curves secp256k1 keypair in browser | 10 | 10 | 1.00 | — | **TODO** | localStorage has privkey/pubkey; no Go API call |

## Nostr-Transport

| ID | Задача | Min | UV | Score | Deps | Status | DoD |
|---|---|---|---|---|---|---|---|
| `SL-014` | Fallback to Go WS if Nostr relays unreachable | 15 | 7 | 0.47 | — | **TODO** | Graceful switch to Go WS on relay failure |
| `SL-013` | Presence: Nostr status events | 10 | 7 | 0.70 | — | **TODO** | Online indicator works without Go /api/online |
| `SL-012` | sendDM via Nostr publish instead of Go POST | 12 | 10 | 0.83 | — | **TODO** | Message goes via wss://relay; not via Go API |
| `SL-011` | onMessage: Nostr event → chat store | 10 | 10 | 1.00 | — | **TODO** | Received message appears in UI without Go server |
| `SL-010` | Import nostr-signaling.ts into App.svelte | 12 | 10 | 0.83 | — | **TODO** | App.svelte imports NostrClient; console: connected to N rela |

## CF-Pages

| ID | Задача | Min | UV | Score | Deps | Status | DoD |
|---|---|---|---|---|---|---|---|
| `SL-004` | Verify HTTPS + PWA manifest on Pages URL | 8 | 8 | 1.00 | — | **TODO** | Lighthouse PWA; SW registered; manifest valid |
| `SL-003` | Deploy dist/ to Cloudflare Pages | 10 | 10 | 1.00 | — | **TODO** | URL *.pages.dev responds 200; HTTPS works |
| `SL-002` | Build script: dist ready for Pages deploy | 5 | 9 | 1.80 | — | **TODO** | dist/index.html no localhost; VITE_API_URL empty |
| `SL-001` | Create wrangler.toml for Cloudflare Pages | 8 | 9 | 1.12 | — | **TODO** | wrangler.toml exists; npx wrangler pages deploy --dry-run ex |

## Dependency Graph

```
[root] → SL-042
[root] → SL-044
[root] → SL-043
[root] → SL-041
[root] → SL-040
[root] → SL-055
[root] → SL-054
[root] → SL-053
[root] → SL-052
[root] → SL-051
[root] → SL-050
[root] → SL-034
[root] → SL-033
[root] → SL-031
[root] → SL-030
[root] → SL-032
[root] → SL-024
[root] → SL-023
[root] → SL-022
[root] → SL-021
[root] → SL-020
[root] → SL-014
[root] → SL-013
[root] → SL-012
[root] → SL-011
[root] → SL-010
[root] → SL-004
[root] → SL-003
[root] → SL-002
[root] → SL-001
```

---

## Правила (CPO v7)

- **priority_score** = user_value / agent_minutes (uv clamped 1–10)
- **DONE** только autotest / e2e / api proof (не manual)
- **LOC** ≤ 500 на файл
- **0** God Objects
- DoD: acceptance_criteria выполнено + test_logs заполнен

## Зависимости по этапам

1. **Epic 1 (CF-Pages):** SL-001→002→003→004 — деплой статики
2. **Epic 2 (Nostr):** SL-010→011→012→013→014 — транспорт
3. **Epic 3 (Identity):** SL-020→021→022→023→024 — браузерные ключи
4. **Epic 4 (E2E):** SL-030→031→032→033→034 — шифрование
5. **Epic 5 (IPFS):** SL-040→041→042→043→044 — файлы
6. **Epic 6 (PWA):** SL-050→051→052→053→054→055 — финал
