# ZERO-POINT SCAN — Indestructible Messenger

_Generated: 2026-08-03T21:02:16Z_

**Анти-галлюцинация:** все цифры из физического прогона на native Windows.

## 1. Сводка (точка отсчёта)

| Метрика | Значение |
|---------|----------|
| Unit logic coverage (Go statements) | **65.3%** |
| Frontend unit coverage (Vitest) | **N/A** |
| User scenarios covered (E2E inventory) | **11/20 = 55.0%** |
| Infra (Bats) | **15.0%** (runner=no) |
| LOC ≤500 compliance | **99.4%** (1 files over) |
| **Overall test readiness** | **57.5%** |

## 2. Физический прогон

| Стек | Команда | Всего | Pass/Fail | Coverage / Notes |
|------|---------|-------|-----------|------------------|
| Backend Go | `go test <27 pkg> -coverprofile` | **770** listed | **None/0 pkg** (all PASS) | **65.3%** statements |
| Frontend Vitest | `vitest run` | **19** | **19/0** | provider coverage: None |
| E2E Playwright `e2e/` | `playwright test` | **16** | **16/0** | alice-bob, messenger, vpn, deep |
| E2E orphan `tests/` | playwright direct | ? | pass=0 fail=0 | вне testDir — см. лог |
| Infra Bats | bats 07-QA/.../bats | 5 files | not executed | bats CLI missing |

## 3. Go coverage by package

| Package | Cover % | Tests listed |
|---------|---------|--------------|
| `./i18n/` | 100.0% | 4 |
| `./integration/` | 100.0% | 13 |
| `./analytics/` | 94.4% | 8 |
| `./middleware/` | 94.2% | 18 |
| `./api/` | 94.2% | 13 |
| `./msgops/` | 92.9% | 8 |
| `./admin/` | 92.6% | 7 |
| `./bot/` | 92.0% | 16 |
| `./mesh/` | 91.6% | 42 |
| `./social/` | 91.2% | 3 |
| `./chat/` | 84.8% | 82 |
| `./stream/` | 83.5% | 34 |
| `./nat/` | 83.3% | 14 |
| `./content/` | 82.8% | 87 |
| `./media/` | 78.1% | 7 |
| `./federation/` | 75.1% | 19 |
| `./storage/` | 74.1% | 25 |
| `./crypto/` | 71.9% | 34 |
| `./nostr/` | 71.2% | 35 |
| `./push/` | 70.9% | 21 |
| `./tor/` | 64.0% | 42 |
| `.` | 62.4% | 61 |
| `./identity/` | 59.7% | 31 |
| `./store/` | 58.1% | 60 |
| `./ipfs/` | 57.0% | 16 |
| `./auth/` | 54.4% | 10 |
| `./cmd/webserver/` | 34.0% | 60 |

## 4. User scenarios inventory

| ID | Scenario | Layer | Covered | Note |
|----|----------|-------|---------|------|
| S01 | Signup / get JWT | unit+e2e | ✅ | auth + webserver + alice-bob |
| S02 | Login / token restore | unit | ✅ | auth tests |
| S03 | Create DM chat (User ID) | e2e | ✅ | messenger.spec + alice-bob |
| S04 | Send message Alice→Bob | e2e+unit | ✅ | alice-bob + chat hub + store |
| S05 | WS delivery realtime | e2e+unit | ✅ | alice-bob + chat |
| S06 | Empty message rejected | unit | ✅ | store SaveMessage |
| S07 | Oversized message rejected | unit+api | ✅ | ValidateMessage + store test + live |
| S08 | Emoji picker UI | e2e | ✅ | messenger.spec |
| S09 | VPN SOCKS start/IP/disconnect | e2e | ✅ | vpn.spec |
| S10 | Settings / profile / QR screens | e2e-screenshot | ✅ | deep.spec (weak asserts) |
| S11 | Group create/list API client | unit-fe | ✅ | vitest api.test |
| S12 | Group full E2E UI | e2e | ❌ | ICEBOX GRP |
| S13 | File upload E2E | e2e | ❌ | ICEBOX |
| S14 | Multi-device sync | e2e | ❌ | partial unit only |
| S15 | E2E encryption roundtrip UI | e2e | ❌ | crypto unit exists, UI weak |
| S16 | Voice/call | e2e | ❌ | ICEBOX |
| S17 | Android/mobile | maestro | ❌ | stubs only |
| S18 | Tor/mesh real path | e2e | ❌ | unit only |
| S19 | Push notifications | e2e | ❌ | unit push pkg |
| S20 | System-wide Wintun VPN | e2e | ❌ | ICEBOX |

## 5. LOC >500 (CRITICAL architecture)

| Stack | File | LOC |
|-------|------|-----|
| go | `archive\vpn-v1-2026-06-24.go` | **823** |

## 6. Gaps to 100%

| Gap | Impact | Action |
|-----|--------|--------|
| Go cover 65.3% → 80%+ | unit | JWT edges, webserver handlers (34%), auth 54% |
| FE coverage N/A / low surface | unit-fe | @vitest/coverage-v8 + more component tests |
| Scenarios 55% | e2e | groups/files/calls/multi-device/mobile |
| Bats not executed | infra | install bats-core or skip as ICEBOX |
| 1 files >500 LOC | arch | split God files |
| deep.spec screenshot-only | e2e quality | strengthen asserts |

## 7. Artifacts

- `C:\Obsidian\New\Projects\04-Неубиваемый-контент V2\.04-Src\07-QA-and-Testing\04-AI-Test-Reports\zero-point/ZERO_POINT_SCAN.json`
- `C:\Obsidian\New\Projects\04-Неубиваемый-контент V2\.04-Src\07-QA-and-Testing\04-AI-Test-Reports\zero-point/go_results.json`
- `C:\Obsidian\New\Projects\04-Неубиваемый-контент V2\.04-Src\07-QA-and-Testing\04-AI-Test-Reports\zero-point/go_cover_func.txt`
- `C:\Obsidian\New\Projects\04-Неубиваемый-контент V2\.04-Src\07-QA-and-Testing\04-AI-Test-Reports\zero-point/playwright_output.txt`
- `C:\Obsidian\New\Projects\04-Неубиваемый-контент V2\.04-Src\07-QA-and-Testing\04-AI-Test-Reports\zero-point/vitest_output.txt`