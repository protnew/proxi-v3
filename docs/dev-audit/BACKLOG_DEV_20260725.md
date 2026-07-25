# Бэклог dev-ветки — аудит 25.07.2026

**Создан:** 25.07.2026 на основе физической проверки проекта  
**Ветка:** `dev` (от main `ac1871f`)  
**Всего задач:** 15 (3 P0 + 3 P1 + 5 P2 + 4 P3)  
**БД:** `backlog_dev_audit_20260725.db`

---

## Метод проверки

| Что делал | Результат |
|---|---|
| `npm run build` | ✅ exit 0, 132KB JS |
| `npx vitest run` | ✅ 13/13 PASS |
| `go test ./chat/` (GOMAXPROCS=1) | ✅ 84/84 PASS |
| `go test ./auth/` | ✅ 10/10 PASS |
| `go test ./crypto/ ./store/ ./mesh/ ./nostr/` | ❌ OOM (paging file too small) |
| `npx playwright test` (deep.spec.js) | ✅ 10/10 PASS |
| `npx playwright test` (e2e-messenger.spec.ts) | ❌ 5/7 FAIL (устаревшие селекторы) |
| Browser: UI, создание чата, отправка | 🔴 сообщения не отправляются |
| Console: проверка логов | 🔴 signup 401, JWT=NONE, WS без токена |
| DOM: проверка .msg-text | 🔴 50 пузырей, 48 пустые |

---

## P0 — Критично (мессенджер мёртв без этого)

### DEV-001: Починить /api/auth/signup — возвращает 401
**Блокер:** Без JWT невозможна отправка сообщений, identity не привязан.  
**Симптом:** Console: `[api] initIdentityAsync: signup failed! status=401 error=[object Object]`  
**Фронтенд:** Вызывает `/api/auth/signup` правильно, получает 401  
**WS:** Подключается БЕЗ токена -> `ws://localhost:8080/ws?userId=...`  
**localStorage:** `jwt_token` = NONE  
**Действие:** Проверить `routing_auth.go`, middleware, порядок роутов. 401 на signup = auth middleware блокирует  
**Acceptance:** signup возвращает 200 + JWT; токен сохраняется; `/api/identity` работает  

### DEV-002: Починить отправку/получение DM — сообщения пустые
**Симптом:** 50 message bubbles в чате, `.msg-text` пустой у большинства  
**"Привет от Алексея"** введён в input, Enter нажат — остался в input, не отправился  
**WS:** Подключён (mode: go-backend), но hub.OnMessage callback не сохраняет в SQLite  
**Acceptance:** Отправленное сообщение появляется с текстом; сохраняется в SQLite; второй контекст видит его  

### DEV-003: Удалить пустые message bubbles из истории
**Симптом:** 48 из 50 пузырей пустые — только timestamp, без текста  
**Проверка:** `document.querySelectorAll('.msg-text')` -> `['', '', '', '', '']`  
**Причина:** Тестовый мусор от Nocturnal Pipeline / старых E2E прогонов  
**Acceptance:** В чате нет пустых bubble'ов; история содержит только реальные сообщения  

---

## P1 — Демонстрируемое MVP

### DEV-004: Обновить e2e-messenger.spec.ts — устаревшие селекторы
**Проблема:** Тесты ищут `textarea[placeholder="Вставь pubkey друга"]`, но UI использует `input`  
**Результат:** 5/7 FAIL не из-за багов кода, а из-за устаревших тестов  
**Acceptance:** Все 7 тестов PASS; селекторы обновлены под текущий UI  

### DEV-005: Добавить реальный DM E2E: Alice->Bob на 2 контекстах
**Доказательство MVP** по PM critic verdict  
**Текущий статус:** Тест timeout, browserContext.close падает  
**Acceptance:** Alice отправляет — Bob видит в реальном времени через WS  

### DEV-006: Увеличить paging file Windows для Go тестов
**Проблема:** `fork/exec compile.exe: The paging file is too small`  
**Заявлено:** "26/26 PASS, 78.1% покрытие" — проверить невозможно  
**Проверены:** Только chat/ (84 PASS) и auth/ (10 PASS) из 28 пакетов  
**Acceptance:** `go test ./...` выполняется без OOM для всех 28 пакетов  

---

## P2 — Production Readiness

### DEV-007: Проверить docker-compose.yml
test_logs = "docker-compose.yml exists" — keyword-only, не тестировался  
**Acceptance:** `docker-compose up` поднимает Go+PWA, health checks проходят  

### DEV-008: Создать рабочий GitHub Actions CI
`ci.yml.template` — шаблон, не рабочий конфиг  
**Acceptance:** CI запускается на push; go test + vitest  

### DEV-009: Реализовать настоящий SOCKS5 proxy
VPV-001 — toggle вызывает stub endpoint, не реальный прокси  
**Acceptance:** VPN toggle ON — HTTP запросы идут через SOCKS5 туннель  

### DEV-010: Voice/Video calls — протестировать реально
Код компилируется, но 0 функциональных тестов  
**Acceptance:** Voice call между 2 контекстами работает  

### DEV-011: Android APK — проверить сборку
prototypes/android/ существует, но CONNECT.md не равно рабочий APK  
**Acceptance:** APK собирается; устанавливается; подключается к backend  

---

## P3 — Качество и Tech Debt

### DEV-012: Прогнать ВСЕ Go пакеты (28) с -cover
После фикса paging file — прогнать все пакеты  
**Acceptance:** Реальные цифры покрытия зафиксированы  

### DEV-013: Починить e2e.spec.ts (WebRTC туннель)
Тест указывает на localhost:4173 (preview), но сервер не запущен  
**Acceptance:** WebRTC туннель между 2 контекстами работает  

### DEV-014: OWASP audit — реальные проверки
OWASP_AUDIT.md = self-assessment "10/10 checked" без доказательств  
**Acceptance:** gosec сканер; 0 критических; CORS/rate-limit проверены  

### DEV-015: Очистить старый backlog от fake DONE
100/100 DONE, но: ~10 реальных тестов, ~35 "build OK", ~6 "exists"  
**Acceptance:** Все test_logs содержат реальные доказательства  

---

## Сверка с предыдущим отчётом

| Заявлено | Реальность |
|---|---|
| "Go 26/26 PASS" | Проверены 2/28 (OOM блокирует) |
| "Playwright 22/22 PASS" | deep 10/10 OK, но functional 5/7 FAIL |
| "~553 автотеста" | Реально прогнанных: ~117 |
| "78.1% покрытие" | Недоказуемо без всех пакетов |
| "100/100 DONE" | ~10 с реальными тестами |
| "MVP 6/10" | Реально 3/10 — DM не работает, auth сломана |
