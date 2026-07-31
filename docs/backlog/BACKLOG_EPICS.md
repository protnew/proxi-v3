# BACKLOG — INDESTRUCTIBLE MESSENGER

_Сгенерирован: 2026-07-31 22:46 UTC_

## SoT (Source of Truth)

| Что | Где |
|-----|-----|
| Бэклог-БД | `08-Backlog/backlog_proxi_v3.db` (SQLite) |
| Этот файл | `08-Backlog/BACKLOG_EPICS.md` |
| Код | `.04-Src` (git branch `dev`) |
| Дашборд | `08-Backlog/backlog_dashboard_v1.html` |

## Сводка

| Эпик | Всего | ✅DONE | 🟡TODO | 🧊ICEBOX |
|------|-------|--------|--------|----------|
| **MESSAGING CORE** | 25 | 18 | 0 | 7 |
| **VPN & PRIVACY** | 14 | 9 | 0 | 5 |
| **QUALITY & SECURITY GATES** | 26 | 13 | 7 | 6 |
| **USER EXPERIENCE** | 17 | 2 | 1 | 14 |
| **PLATFORM & SCALE** | 14 | 3 | 0 | 11 |
| **ИТОГО** | **96** | **45** | **8** | **43** |

---

## MESSAGING CORE

_Цель: один пользователь может зарегистрироваться и надёжно отправить сообщение другому. Auth, DM, WS hub, multi-device._

Задач: 25 | ✅18 · 🟡0 · 🧊7

### ✅ СДЕЛАНО (18)

| ID | Блок | Задача | Мин | Proof | Зависит |
|----|-------|--------|-----|-------|---------|
| `AUTH-002` | Auth.RouteMap | Reproduce signup 401 with curl/python | 10 | api,go-test | AUTH-001 |
| `AUTH-007` | Auth.Identity | /api/identity works with Bearer token | 10 | api | AUTH-006 |
| `AUTH-003` | Auth.Middleware | Identify middleware blocking unauthenticated signup | 12 | api,go-test | AUTH-002 |
| `AUTH-006` | Auth.Frontend | Persist JWT to localStorage after signup | 12 | playwright,e2e | AUTH-005 |
| `AUTH-001` | Auth.RouteMap | Map auth routes in Go routing files | 12 | manual | — |
| `AUTH-005` | Auth.Fix | Return JWT + user_id shape expected by frontend | 12 | api,go-test | AUTH-004 |
| `AUTH-004` | Auth.Fix | Exempt signup/login from JWT middleware | 15 | api,go-test | AUTH-003 |
| `AUTH-009` | Auth.WS | Reject or limit anonymous WS in authenticated mode | 15 | autotest,api | AUTH-006 |
| `AUTH-008` | Auth.Tests | Add Go table tests for signup edge cases | 15 | autotest | AUTH-004 |
| `MSG-001` | MessengerCore.SendPath | Trace frontend sendDM code path | 12 | manual | AUTH-006 |
| `MSG-002` | MessengerCore.SendPath | Trace backend hub.OnMessage → SaveMessage | 12 | manual | AUTH-004 |
| `MSG-003` | MessengerCore.SendPath | Fix empty message persistence / filter empties | 12 | autotest | MSG-002 |
| `MSG-005` | MessengerCore.History | SQL cleanup script for empty historical messages | 10 | autotest | MSG-003 |
| `MSG-004` | MessengerCore.SendPath | Fix UI send clears input and appends bubble with text | 15 | playwright,e2e | MSG-001,MSG-003,AUTH-006 |
| `MSG-008` | MessengerCore.Realtime | Cross-user DM delivery integration test | 15 | autotest,e2e | MSG-004,AUTH-007 |
| `MSG-006` | MessengerCore.History | UI does not render empty bubbles | 10 | manual | MSG-005 |
| `MSG-007` | MessengerCore.Realtime | WS delivers DM to second connection same user multi-device | 12 | autotest | AUTH-009 |
| `MSG-009` | MessengerCore.E2EFlag | isE2EEnabled respected on send path | 12 | autotest | MSG-004 |

### 🧊 ОТЛОЖЕНО (7)

| ID | Блок | Задача | Мин | Proof | Зависит |
|----|-------|--------|-----|-------|---------|
| `AUTH-010` | Auth.Docs | Document auth flow sequence in 09-Docs | 10 | none | AUTH-007 |
| `MSG-011` | MessengerCore.Typing | Typing indicator WS roundtrip | 12 | none | MSG-008 |
| `MSG-010` | MessengerCore.Reply | Reply/forward/edit smoke if endpoints exist | 15 | none | MSG-008 |
| `MSG-012` | MessengerCore.Files | File upload path smoke | 12 | none | AUTH-007 |
| `GRP-001` | Groups.API | Inventory groups API endpoints vs UI | 12 | none | MSG-008 |
| `GRP-002` | Groups.Create | E2E create group after core DM green | 15 | none | GRP-001,E2E-004 |
| `GRP-003` | Groups.Members | Add member API smoke | 12 | none | GRP-001 |

---

## VPN & PRIVACY

_Цель: SOCKS5-туннель поднимается одной кнопкой, трафик проходит, IP проверяется. К 未来 — WireGuard mesh._

Задач: 14 | ✅9 · 🟡0 · 🧊5

### ✅ СДЕЛАНО (9)

| ID | Блок | Задача | Мин | Proof | Зависит |
|----|-------|--------|-----|-------|---------|
| `VPN-010` | VPN.real | Real SOCKS5 server + StartRealTunnel + check_egress_ip | 15 | api | — |
| `VPN-011` | VPN.real | UI real mode default + check IP button + SOCKS addr | 15 | e2e | — |
| `VPN-012` | VPN.real | Playwright real VPN e2e | 15 | e2e | — |
| `VPN-013` | VPN.real | Docs real VPN HOW_TO | 15 | manual | — |
| `VPN-001` | VPN.Truth | Wire FE VpnPanel to /api/vpn/rpc (drop fake WebRTC) | 10 | e2e | — |
| `VPN-002` | VPN.Design | Backend start_local_tunnel + Windows-safe connect (no route hijack) | 15 | api | VPN-001 |
| `VPN-003` | VPN.Impl | VPN panel UX: modes + Russian how-it-works | 15 | manual | VPN-002,E2E-004 |
| `VPN-004` | VPN.mvp | Playwright e2e vpn.spec.ts local tunnel | 20 | e2e | — |
| `VPN-005` | VPN.mvp | Docs HOW_TO_TEST_MESSENGER_VPN | 20 | manual | — |

### 🧊 ОТЛОЖЕНО (5)

| ID | Блок | Задача | Мин | Proof | Зависит |
|----|-------|--------|-----|-------|---------|
| `P2P-001` | P2PContent.Inventory | Inventory content/ipfs/erasure packages vs wired routes | 12 | none | — |
| `P2P-002` | P2PContent.Later | Mark erasure coding LATER with AC for future | 12 | none | E2E-004 |
| `P2P-003` | P2PContent.Later | Tor package go test smoke or ICEBOX | 12 | none | E2E-004 |
| `VPN-006` | VPN.mvp | Real exit-node peer E2E with two machines | 20 | manual | — |
| `VPN-014` | VPN.real | System-wide Wintun/WireGuard Windows | 15 | manual | — |

---

## QUALITY & SECURITY GATES

_Цель: автотесты зелёные, секретов в коде нет, лимиты соблюдены, DONE только с proof._

Задач: 26 | ✅13 · 🟡7 · 🧊6

### 🟡 СЛЕДУЕТ СДЕЛАТЬ (7)

| ID | Блок | Задача | Мин | Proof | Зависит |
|----|-------|--------|-----|-------|---------|
| `QG-002` | QualityGate.Proof | SQL view/query rejecting DONE without proof | 10 | none | QG-001 |
| `QG-001` | QualityGate.Rules | Commit AUDITOR_RULES + CPO DoR into 08-Backlog | 10 | none | — |
| `TEST-004` | TestingInfra.Vitest | Add vitest tests for sendDM failure without token | 12 | none | AUTH-006 |
| `TEST-003` | TestingInfra.Go | go test ./... -cover when memory allows | 15 | none | TEST-001 |
| `TEST-006` | TestingInfra.Report | Write Zero-Point metrics table template | 10 | none | QG-003 |
| `SEC-003` | Security.SQL | Scan for DROP TABLE / unconditional DELETE | 10 | none | QG-004 |
| `SEC-002` | Security.Input | Verify signup/message validation max size | 12 | none | AUTH-008 |

### ✅ СДЕЛАНО (13)

| ID | Блок | Задача | Мин | Proof | Зависит |
|----|-------|--------|-----|-------|---------|
| `QG-008` | QualityGate.Backlog | Only one active backlog_proxi_v*.db in 08-Backlog | 8 | manual | QG-001 |
| `QG-004` | QualityGate.Sec | Script scripts/security-gate secrets ripgrep patterns | 12 | autotest | QG-001 |
| `QG-005` | QualityGate.LOC | Script check no prod file >=500 lines | 10 | autotest | QG-001 |
| `QG-006` | QualityGate.10POV | Template 10-POV checklist snippet for test_logs | 10 | manual | QG-001 |
| `QG-003` | QualityGate.TDD | Script scripts/tdd-loop that runs go+vitest+playwright | 15 | manual | QG-001 |
| `QG-007` | QualityGate.Structure | Finish TEMPLATE V2: scratch/scripts/infra placement decision | 12 | manual | — |
| `E2E-002` | E2EProof.Config | Unify playwright testDir (e2e vs tests) | 10 | manual | — |
| `E2E-001` | E2EProof.Selectors | Update e2e-messenger selectors to input User ID UI | 12 | playwright,e2e | MSG-004 |
| `E2E-003` | E2EProof.AliceBob | Write Alice→Bob two-context DM test | 15 | playwright,e2e | E2E-001,MSG-008 |
| `E2E-004` | E2EProof.AliceBob | Run Alice→Bob e2e green on :5173+:8080 | 15 | playwright,e2e | E2E-003,AUTH-006 |
| `E2E-005` | E2EProof.Deep | Keep deep.spec.js 10 screenshot tests green | 10 | manual | E2E-002 |
| `TEST-002` | TestingInfra.Go | go test ./chat/ ./auth/ baseline capture | 12 | autotest | TEST-001 |
| `SEC-001` | Security.Scan | Run initial secrets scan on .04-Src | 12 | autotest | QG-004 |

### 🧊 ОТЛОЖЕНО (6)

| ID | Блок | Задача | Мин | Proof | Зависит |
|----|-------|--------|-----|-------|---------|
| `E2E-006` | E2EProof.WebRTC | Triage e2e.spec.ts :4173 refused | 12 | none | E2E-002 |
| `TEST-001` | TestingInfra.Go | Native Windows: paging file note (no Docker test runner) | 15 | manual | — |
| `TEST-005` | TestingInfra.Vitest | Add vitest tests for messenger store if present | 15 | none | — |
| `SEC-005` | Security.CORS | Verify CORS whitelist not reflection | 10 | none | — |
| `SEC-004` | Security.OWASP | Run gosec or staticcheck; store report | 15 | none | TEST-001 |
| `SEC-006` | Security.Armor | Model Armor checklist in 08-Security | 10 | none | QG-001 |

---

## USER EXPERIENCE

_Цель: понятный UI, который можно показать пользователю. Пустые экраны, подсказки, мелкая UX-полировка._

Задач: 17 | ✅2 · 🟡1 · 🧊14

### 🟡 СЛЕДУЕТ СДЕЛАТЬ (1)

| ID | Блок | Задача | Мин | Proof | Зависит |
|----|-------|--------|-----|-------|---------|
| `MVP-002` | MVP.user-test | Human smoke 2 browser profiles Alice→Bob on :5173 | 15 | manual | — |

### ✅ СДЕЛАНО (2)

| ID | Блок | Задача | Мин | Proof | Зависит |
|----|-------|--------|-----|-------|---------|
| `MVP-001` | MVP.user-test | SoT docs + start-messenger-dev.ps1 + HOW_TO_TEST_2_USERS | 15 | manual | — |
| `MVP-003` | MVP.user-test | My User ID + copy always visible on Sidebar | 15 | manual | — |

### 🧊 ОТЛОЖЕНО (14)

| ID | Блок | Задача | Мин | Proof | Зависит |
|----|-------|--------|-----|-------|---------|
| `UI-001` | UI.Chat | Verify premium empty state still intact post-fixes | 10 | none | MSG-004 |
| `UI-003` | UI.Emoji | Emoji picker inserts into input | 10 | none | MSG-004 |
| `UI-002` | UI.Chat | Context menu reply visible on bubble | 12 | none | MSG-004,E2E-001 |
| `UI-004` | UI.VPN | VPN panel toggle UI only smoke | 10 | none | — |
| `PARITY-001` | UI.Parity | Parity spike: Pinned chats | 12 | none | E2E-004 |
| `PARITY-002` | UI.Parity | Parity spike: Message search UI | 12 | none | E2E-004 |
| `PARITY-003` | UI.Parity | Parity spike: Unread badges | 12 | none | E2E-004 |
| `PARITY-004` | UI.Parity | Parity spike: Push notification opt-in | 12 | none | E2E-004 |
| `PARITY-005` | UI.Parity | Parity spike: Link previews | 12 | none | E2E-004 |
| `PARITY-006` | UI.Parity | Parity spike: Voice message playback UI | 12 | none | E2E-004 |
| `PARITY-007` | UI.Parity | Parity spike: Sticker send UI | 12 | none | E2E-004 |
| `PARITY-008` | UI.Parity | Parity spike: Chat folders | 12 | none | E2E-004 |
| `PARITY-009` | UI.Parity | Parity spike: Disappearing messages settings | 12 | none | E2E-004 |
| `PARITY-010` | UI.Parity | Parity spike: Multi-account switcher | 12 | none | E2E-004 |

---

## PLATFORM & SCALE

_Цель: Android, Docker, CI на GitHub, OpenAPI, экономика, мобильные клиенты._

Задач: 14 | ✅3 · 🟡0 · 🧊11

### ✅ СДЕЛАНО (3)

| ID | Блок | Задача | Мин | Proof | Зависит |
|----|-------|--------|-----|-------|---------|
| `MOB-004` | MobileDesktop.Maestro | Decide Maestro N/A until APK | 8 | manual | MOB-002 |
| `DO-003` | DevOps.CI | Create real .github/workflows/ci.yml from template | 15 | manual | QG-003 |
| `DOC-001` | DocsCompliance.CPO | Competitive benchmark MD in 00-Product | 12 | manual | — |

### 🧊 ОТЛОЖЕНО (11)

| ID | Блок | Задача | Мин | Proof | Зависит |
|----|-------|--------|-----|-------|---------|
| `MOB-002` | MobileDesktop.Android | APK debug build attempt | 15 | none | MOB-001,AUTH-006 |
| `MOB-003` | MobileDesktop.Desktop | Tauri config inventory | 10 | none | — |
| `MOB-001` | MobileDesktop.Android | Verify Android project opens / gradle tasks list | 15 | none | — |
| `DO-001` | DevOps.Docker | docker-compose config validate | 10 | manual | — |
| `DO-005` | DevOps.Git | Confirm dev branch tracking and push policy in AGENTS.md | 8 | none | QG-001 |
| `DO-002` | DevOps.Docker | docker compose up smoke health | 15 | manual | DO-001 |
| `DO-004` | DevOps.Bats | Add bats tests for tdd-loop and security-gate scripts | 12 | none | QG-003,QG-004 |
| `ECO-001` | Economy.Honesty | Document lightning/storage/bandwidth as stubs | 10 | none | — |
| `ECO-002` | Economy.Tests | go test ./economy/ if package builds | 12 | none | TEST-001 |
| `DOC-003` | DocsCompliance.Runbook | Link TDD+security scripts in RUNBOOK | 10 | none | QG-003 |
| `DOC-002` | DocsCompliance.OpenAPI | Verify openapi.yaml still matches auth after fix | 12 | none | AUTH-005 |
