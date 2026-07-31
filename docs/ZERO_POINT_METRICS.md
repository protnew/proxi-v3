# Zero-Point Metrics Template

## Что это

Шаблон для метрик качества при каждом e2e / smoke прогоне.
Цель: зафиксировать «нулевой» baseline и отслеживать регрессии.

## Метрики

| ID | Метрика | Цель | Как измерить |
|----|---------|------|--------------|
| ZP-01 | Go unit tests PASS rate | 100% | `go test ./... 2>&1 \| grep -c "^ok"` |
| ZP-02 | Vitest PASS rate | 100% | `npx vitest run --reporter=json \| jq .numPassedTestsWithFilters` |
| ZP-03 | Playwright e2e PASS | 100% | `npx playwright test --reporter=line` |
| ZP-04 | Cold start time (API) | < 3s | `time curl http://127.0.0.1:8080/api/health` (from server start) |
| ZP-05 | Cold start time (UI) | < 5s | `time curl http://127.0.0.1:5173/` (from vite start) |
| ZP-06 | DM latency Alice→Bob | < 500ms | Playwright: time from send to visible in Bob context |
| ZP-07 | VPN SOCKS connect time | < 2s | RPC `start_real_tunnel` → `get_status connected` |
| ZP-08 | Files > 500 LOC | 0 | `python scripts/loc-check.py` |
| ZP-09 | Secrets in code | 0 | `python scripts/security-gate.py` |
| ZP-10 | Backlog DoD violations | 0 | `SELECT count(*) FROM v_dod_audit WHERE dod_status LIKE 'REJECT%'` |
| ZP-11 | Signup HTTP status | 200 | `curl -X POST /api/auth/signup` → check status |
| ZP-12 | Auth middleware on public routes | Absent | Manual: verify signup/login not behind JWT |

## Baseline (snapshot)

| Metric | Value | Date |
|--------|-------|------|
| Go tests | 94/94 PASS (auth+chat+store) | 2026-07-31 |
| Vitest | 16/16 PASS | 2026-07-31 |
| Playwright | 6/6 PASS | 2026-07-31 |
| Files >500 | 0 (after extraction) | 2026-07-31 |
| Secrets | 0 | 2026-07-31 |
| DoD violations | 0 | 2026-07-31 |
| DM latency | ~300ms (Playwright) | 2026-07-31 |
| VPN connect | <1s (RPC) | 2026-07-31 |

## Использование

```powershell
# Снять snapshot метрик
cd ".04-Src"
python scripts/collect-metrics.py > docs/metrics-snapshot-YYYY-MM-DD.md
```

_(Скрипт collect-metrics.py — будущая задача)_
