# ZERO-POINT SCAN — 2026-08-15 (physical)

Снято сегодня. Старый отчёт 2026-08-04 (Vitest 33, Playwright 20/0, readiness 56.2%) **не действует**.

## 1. Точка отсчёта

| Метрика | Сейчас | 2026-08-04 | Δ |
|---------|--------|------------|---|
| Unit logic (Go statements) | **62.5%** | 65.3% | -2.8 |
| Frontend unit (Vitest statements) | **53.4%** | 9.77% | +43.6 |
| User scenarios E2E | **9/22 = 40.9%** | 11/20 = 55% | -14.1 |
| Infra Bats (прогон) | **0.0%** | 15% | -15 |
| LOC ≤500 (active source) | **100.0%** | 99.4% | +0.6 |
| **Overall test readiness** | **51.4%** | 56.2% | -4.8 |

Формула: `Go unit × 0.40 + scenarios × 0.40 + infra × 0.10 + LOC × 0.10`.

## 2. Физический прогон

| Стек | Команда | Всего | Pass/Fail | Модули |
|------|---------|------:|-----------|--------|
| Go | `go test <pkg> -count=1 -coverprofile` ×29, GOMAXPROCS=1 | 29 pkg / 844 Test* | **29/0** | cover **62.5%** (317 с) |
| Vitest | `npx vitest run --coverage` | 397 | **397/0** | 49 files, stmt 53.4% |
| Playwright | `npx playwright test --workers=1` | 57 | **35/22** | Vite :5173 + Go :8090 |
| Android instrumented | не запускался | 14 in source | — | adb devices пустой |
| Maestro | нет CLI | 1 yaml | — | N/A |
| Bats | `bats --tap` | 11 @test | **не прогнан** | WSL bash отсутствует |

## 3. Product proof (не unit)

- VPN 2 вкладки: host exit + joiner **IP 2.54.134.210**, DataChannel open.
- MSG-101: Bob видит `MSG101-WS-…` без reload.
- Auth UI live: «Создать новый аккаунт» на http://127.0.0.1:5173/

## 4. Дыры

1. Playwright 22 FAIL — в т.ч. archived-wt (WebTransport) всё ещё в `testMatch e2e/**`.
2. `tests/vpn-e2e.spec.ts` не в testMatch (orphan).
3. Go `cmd/webserver` 33.7%, `cmd/bridge` 0%, `economy` 0%.
4. FE 0%: amnezia-tunnel, nostr-data-relay, nostr-vpn, update-check, vpn-utils.svelte.ts, web-push (+ archive).
5. Android system VPN (T42-B) — кода нет, тестов нет.
6. Bats/Maestro/Android сегодня = 0 прогонов.
7. 8 Go test-файлов >500 LOC. Active source ≤500 (кроме archive 823).
8. Старт Go: ошибка SQL `idx_content_manifests_file_name` / no such column file_name.
9. Даты в чате E2E: 21 января 1970.

## 5. Liveness

- `GET :8090/api/health` → 200 ok
- `GET :5173/` → 200 AuthScreen
