# Zero-Point Scan — 2026-08-16

Физический прогон. Не публиковать scenario % и readiness.

## Прогоны

| Стек | Команда | Всего | Pass/Fail | Модули |
|---|---|---|---|---|
| Go | `go test ./... -count=1 -timeout 240s -coverprofile` | 28 ok + 1 fail + 1 no-test | archive FAIL | cover-func **62.7%** statements |
| Vitest | `npx vitest run --coverage` | 399 | **399/0** | stmts 52.46 / lines 54.65 |
| Playwright list | `npx playwright test --list` | 57 in 24 files | — | skip 2 в исходниках |
| Playwright suite | `npx playwright test --reporter=line` | 57 | **13 passed / 34 failed / 2 skipped / 8 did not run** | 18.9 мин, :5173+:8090 живы |
| Android JVM | `gradlew :app:testDebugUnitTest --rerun-tasks` | 18 | **18/0** | XML 06:19:39 |
| Bats | — | 5 файлов | **не запускались** | CLI нет |
| Maestro | — | 1 yaml | **не запускался** | CLI нет, adb нет |

## Запрещено в этом файле

- scenario %
- readiness
- PASS = list − fail

## Git

`dev` @ `cc8aea91` — HEAD как 15.08. JNI / ProxiBit locks на диске, в этом хеше их нет.
