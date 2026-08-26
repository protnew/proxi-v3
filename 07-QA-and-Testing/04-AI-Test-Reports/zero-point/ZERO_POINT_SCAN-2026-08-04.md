# ZERO-POINT SCAN — after auto-test blanket

_Generated: 2026-08-04T06:06:26Z_ · physical native Windows

## 1. Точка отсчёта

| Метрика | Значение |
|---------|----------|
| Unit logic (Go statements) | **65.3%** |
| Frontend unit (Vitest statements) | **9.77%** |
| User scenarios E2E inventory | **11/20 = 55.0%** |
| Infra Bats | **15.0%** |
| LOC ≤500 | **99.4%** |
| **Overall test readiness** | **56.2%** |

## 2. Delta

| | Before | After | Δ |
|--|--------|-------|---|
| Go cover | 65.2% | 65.3% | 0.1 |
| **auth** | 54.4% | **91.2%** | **+36.8** |
| Vitest tests | 19 | **33** | +14 |
| Playwright | 16 | **20** | +4 |
| FE coverage | N/A | **9.77%** | instrumented |
| Readiness | 56.9% | **56.2%** | -0.7 |

## 3. Physical run

| Стек | Команда | Pass/Fail | Notes |
|------|---------|-----------|-------|
| Go | `go test` ×29 pkg | **29/0** | cover 65.3% |
| Vitest | `vitest run --coverage` | **33/0** | FE stmt 9.77% |
| Playwright | e2e/ + tests/e2e-messenger | **20/0** | orphan modernized |
| Bats | — | not run | no CLI |

## 4. What was added

- `auth/middleware_test.go` — AuthMiddleware, GetUserID, wrong secret → **auth 91.2%**
- `store/messages_edges_test.go` — empty/exact/oversize
- `validation_edges_test.go` + `cmd/webserver/message_validation_test.go`
- FE: chat-utils, api-payload, qr-parse, theme (+ setTheme return fix)
- `@vitest/coverage-v8` + playwright orphan in suite
- `theme.ts` setTheme returns Theme (bugfix from test)

## 5. Gaps remaining

| Gap | Now | Need |
|-----|-----|------|
| webserver cover | 34.2% | more handler tests |
| store cover | 58.1% | broader store paths |
| FE lib cover | 9.77% | vpn/identity/file-transfer still 0% |
| Scenarios | 55% | groups UI, files, multi-device, push |
| Bats | 15% | install bats or drop weight |
