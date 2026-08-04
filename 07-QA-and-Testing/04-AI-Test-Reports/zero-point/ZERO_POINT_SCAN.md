# ZERO-POINT SCAN — Indestructible Messenger (refresh)

_Generated: 2026-08-04T03:35:31Z_ · physical native Windows · anti-hallucination

## 1. Точка отсчёта

| Метрика | Значение |
|---------|----------|
| Unit logic coverage (Go statements) | **65.2%** |
| Frontend unit coverage (Vitest) | **N/A** — нет `@vitest/coverage-v8` |
| User scenarios (E2E inventory) | **11/20 = 55.0%** |
| Infra Bats | **15.0%** (5 files, CLI absent) |
| LOC ≤500 compliance | **99.4%** (1 archive file >500) |
| **Overall test readiness** | **56.9%** |
| Weights | unit 40% + scenarios 40% + infra 10% + loc 10% |

## 2. Delta vs previous Zero-Point (`bc52a0b`)

| Metric | Prev | Now | Δ pp |
|--------|------|-----|------|
| Go cover | 65.3% | 65.2% | -0.1 |
| Readiness | 57.5% | 56.9% | -0.6 |
| Vitest | 19/19 | 19/19 | 0 |
| Playwright | 16/16 | 16/16 | 0 |

_Noise ±0.1 pp on go cover = merge/order, not regression._

## 3. Physical run

| Стек | Команда | Всего | Pass/Fail | Notes |
|------|---------|-------|-----------|-------|
| Backend Go | `go test` ×27 pkg `-coverprofile` | **770** listed | **27/0 pkg** | cover **65.2%** |
| Frontend Vitest | `vitest run` | **19** | **19/0** | api / api-no-token / e2e-flag |
| E2E Playwright | `playwright test` (`e2e/`) | **16** | **16/0** | alice-bob, messenger, vpn, deep |
| Infra Bats | — | 5 files | **not run** | bats CLI missing |
| Maestro | — | stubs | **0** | no mobile pipeline |

## 4. Go coverage by package

| Package | Cover | Status |
|---------|------:|--------|
| `./i18n/` | 100.0% | PASS |
| `./integration/` | 100.0% | PASS |
| `./analytics/` | 94.4% | PASS |
| `./middleware/` | 94.2% | PASS |
| `./api/` | 94.2% | PASS |
| `./msgops/` | 92.9% | PASS |
| `./admin/` | 92.6% | PASS |
| `./bot/` | 92.0% | PASS |
| `./mesh/` | 91.6% | PASS |
| `./social/` | 91.2% | PASS |
| `./chat/` | 84.8% | PASS |
| `./stream/` | 83.5% | PASS |
| `./nat/` | 83.3% | PASS |
| `./content/` | 82.8% | PASS |
| `./media/` | 78.1% | PASS |
| `./federation/` | 75.1% | PASS |
| `./storage/` | 74.1% | PASS |
| `./crypto/` | 71.9% | PASS |
| `./nostr/` | 71.2% | PASS |
| `./push/` | 70.9% | PASS |
| `./tor/` | 64.0% | PASS |
| `.` | 62.4% | PASS |
| `./identity/` | 59.7% | PASS |
| `./store/` | 58.1% | PASS |
| `./ipfs/` | 57.0% | PASS |
| `./auth/` | 54.4% | PASS |
| `./cmd/webserver/` | 33.9% | PASS |

## 5. Scenarios 11/20 covered

Covered: Signup/JWT, Login, New chat, Alice→Bob, WS, empty/oversize msg, emoji, VPN SOCKS, settings screens (weak), group API client.

Missing: Group UI E2E, files, multi-device, crypto UI, calls, Android, Tor/mesh E2E, push E2E, Wintun.

## 6. LOC

| File | LOC | Verdict |
|------|-----|---------|
| archive/vpn-v1-*.go | 823 | archive only |
| api.ts | 500 | ON LIMIT |
| ChatView.svelte | 491 | near |
| migration.go | 474 | near |

## 7. Gaps to 100% readiness

| Gap | Need |
|-----|------|
| Go 65.2% → 80%+ | webserver 33.9%, auth 54.4%, store 58.1% |
| FE coverage N/A | install @vitest/coverage-v8 |
| Scenarios 55% → 80% | +5 E2E scenarios |
| Bats 15% | install bats-core or drop from weight |
| Orphan tests/*.spec.ts | not in playwright testDir |
