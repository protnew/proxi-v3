# Единый бэклог Proxi (Indestructible Messenger)
**Обновлён:** 2026-07-27  
**Source of truth:** `00-Product-and-Agile/01-Backlog-Roadmap/backlog.db`  
**Ветка разработки:** `dev`  
**Прод-ветка:** `main` (не трогать без merge)  
**Миссия:** Messenger + VPN + P2P неубиваемый контент  
**Правило:** ОДИН бэклог. Итерационные копии → `10-Archive/backlogs/`  

---

## Правила приёмки (Supreme Auditor)

1. **DONE** только с `proof_type` ∈ {`autotest`, `e2e`, `api`} (+ optional manual screenshot).  
2. `keyword` / `build` / `exists` / «PWA build OK» = **не DONE**.  
3. Перед DONE — **10-POV** checklist в `auditor_pov` + `test_logs`.  
4. **TDD Loop** физически: `go test` + `vitest` + `playwright` (+ maestro/bats если применимо).  
5. **Security Gate** до тестов: secrets scan, input validation, no destructive SQL.  
6. **LOC < 500** на prod файлы.  
7. Успех → **commit + push** в `dev`.  
8. Структура ≈ **_TEMPLATE V2**.  
9. Врать о тестах запрещено.  

## Сводка

| Статус | Кол-во |
|---|---|
| DONE | 1 |
| IN_PROGRESS | 1 |
| TODO | 30 |
| **Всего** | **32** |

| Блок | Кол-во |
|---|---|
| QualityGate | 8 |
| Messenger | 6 |
| Testing | 4 |
| Security | 2 |
| DevOps | 2 |
| QA | 2 |
| Infra | 1 |
| Backlog | 1 |
| VPN | 1 |
| Mobile | 1 |
| ContentVault | 1 |
| P2P | 1 |
| Desktop | 1 |
| Economy | 1 |

## Очередь (priority_score DESC)

| Score | ID | Status | Title | TDD | POV |
|---|---|---|---|---|---|
| 100 | P0-001 | ⬜ TODO | Починить /api/auth/signup → 401 Unauthorized | go-test,playwright | Backend,SecOps,QA |
| 99 | P0-002 | ⬜ TODO | Починить отправку/получение DM (пустые msg-text, сообще | go-test,playwright | Backend,QA,UX,PO |
| 98 | P0-004 | ⬜ TODO | Доказать Alice→Bob encrypted DM на 2 browser contexts | playwright | QA,PO,Backend |
| 96 | GATE-008 | ✅ DONE | Единый бэклог: архивировать итерационные DB, запрет на  | none | PO,Architect |
| 95 | GATE-001 | ⬜ TODO | Ввести 10-POV Deep Test checklist как обязательный gate | none | Architect,SecOps,QA,UX,B |
| 94 | GATE-002 | ⬜ TODO | Continuous TDD Loop: 4 слоя тестов обязательны на каждо | go-test,vitest,playw | QA,DevOps |
| 93 | GATE-003 | ⬜ TODO | Physical Security Gate: ripgrep secrets + Zero Trust in | bats | SecOps,Compliance,DBA |
| 92 | GATE-005 | ⬜ TODO | Правило proof_type: DONE только с autotest|e2e|api+manu | none | QA,PO |
| 90 | P0-003 | ⬜ TODO | Очистить мусорные/пустые message bubbles из SQLite hist | go-test,manual | DBA,UX,QA |
| 88 | GATE-007 | 🔄 IN_PROGRESS | Выровнять структуру проекта под _TEMPLATE V2 | none | Architect,DevOps |
| 88 | P1-004 | ⬜ TODO | WS /ws только с валидным JWT (запрет anonymous token=NO | go-test,playwright | SecOps,Backend,QA |
| 87 | P1-002 | ⬜ TODO | Снять OOM/paging file blocker и прогнать go test ./...  | go-test | QA,DevOps,Performance |
| 86 | P1-001 | ⬜ TODO | Обновить e2e-messenger.spec.ts под текущий UI (input Us | playwright | QA,UX |
| 85 | GATE-004 | ⬜ TODO | Лимит 500 строк — hard gate на все .go/.ts/.svelte | bats | Architect |
| 80 | P1-005 | ⬜ TODO | Windows paging file / memory budget для Go toolchain (d | none | DevOps,Performance |
| 75 | P1-003 | ⬜ TODO | Расширить Vitest за пределы api.ts (stores, messenger,  | vitest | QA |
| 72 | P3-001 | ⬜ TODO | Переклассифицировать archived 100-task DONE по proof_ty | none | PO,QA |
| 70 | GATE-006 | ⬜ TODO | Каждая успешная итерация = commit + push в dev (если не | none | DevOps |
| 70 | P2-006 | ⬜ TODO | OWASP/gosec physical scan (не self-assess 10/10 md) | bats | SecOps,Compliance |
| 65 | P2-002 | ⬜ TODO | Рабочий GitHub Actions CI (не ci.yml.template) | bats | DevOps |
| 60 | P2-001 | ⬜ TODO | Проверить docker-compose up end-to-end (не keyword exis | bats | DevOps,QA |
| 55 | P2-003 | ⬜ TODO | Реальный SOCKS5/VPN tunnel вместо stub toggle | go-test,playwright | Backend,PO,QA |
| 55 | P3-008 | ⬜ TODO | Model Armor hooks: pre-hook injection scan docs + post- | none | SecOps,Compliance |
| 50 | P2-007 | ⬜ TODO | Починить e2e.spec.ts WebRTC tunnel (4173 refused) | playwright | QA |
| 45 | P2-004 | ⬜ TODO | Voice/Video calls — functional proof (не PWA build OK) | playwright | QA,UX,Backend |
| 40 | P2-005 | ⬜ TODO | Android APK build + backend connect proof | maestro | QA,DevOps |
| 40 | P3-007 | ⬜ TODO | Bats-core для deploy/scripts (TDD layer 4) | bats | DevOps,QA |
| 35 | P3-006 | ⬜ TODO | Добавить Maestro mobile/PWA flow (TDD layer 3) или явно | maestro | QA |
| 30 | P3-002 | ⬜ TODO | Erasure coding — functional test or mark LATER honestly | go-test | PO,Backend |
| 28 | P3-004 | ⬜ TODO | Tor transport — verify or defer | go-test | SecOps,Backend |
| 27 | P3-005 | ⬜ TODO | Desktop WireGuard — verify Tauri path or defer | none | Backend,DevOps |
| 25 | P3-003 | ⬜ TODO | Lightning Network — stub honesty (invoice package ≠ pay | go-test | PO,Backend |

---

## Карточки задач

### P0-001: Починить /api/auth/signup → 401 Unauthorized

- **Status:** TODO | **Score:** 100.0 | **Block:** Messenger/Auth  
- **Minutes:** 240 | **Value:** Блокер всего продукта  
- **Depends:** GATE-002  
- **proof_type:** `none` | **tdd_layer:** `go-test,playwright`  
- **auditor_pov:** Backend,SecOps,QA  
- **mission_link:** Без identity нет E2E DM — ядро миссии  

**Description:**  
Фронт вызывает signup, получает 401. JWT=NONE. WS коннектится без токена. Без auth DM/persistence мертвы.

**Acceptance:**  
curl/Playwright: POST /api/auth/signup → 200 + token; localStorage имеет jwt; /api/identity с Bearer → 200; go test auth PASS; e2e signup flow PASS

**test_logs:**  
PHYSICAL 2026-07-25: console '[api] initIdentityAsync: signup failed! status=401'; token=NONE

---

### P0-002: Починить отправку/получение DM (пустые msg-text, сообщение не уходит)

- **Status:** TODO | **Score:** 99.0 | **Block:** Messenger/DM  
- **Minutes:** 240 | **Value:** Главная функция мессенджера  
- **Depends:** P0-001  
- **proof_type:** `none` | **tdd_layer:** `go-test,playwright`  
- **auditor_pov:** Backend,QA,UX,PO  
- **mission_link:** Messenger = Trojan Horse daily habit → P2P nodes  

**Description:**  
Enter/send не очищает input; bubbles без текста; hub.OnMessage/SQLite persistence сломаны при anonymous WS.

**Acceptance:**  
Alice пишет текст → bubble с текстом у Alice; Bob в 2-м контексте видит то же; SQLite GetMessages содержит text; Playwright e2e Alice→Bob PASS

**test_logs:**  
PHYSICAL: input 'Привет от Алексея' не отправился; 48/50 msg-text empty

---

### P0-004: Доказать Alice→Bob encrypted DM на 2 browser contexts

- **Status:** TODO | **Score:** 98.0 | **Block:** Messenger/E2E-Proof  
- **Minutes:** 180 | **Value:** MVP proof  
- **Depends:** P0-001,P0-002  
- **proof_type:** `none` | **tdd_layer:** `playwright`  
- **auditor_pov:** QA,PO,Backend  
- **mission_link:** Доказательство что продукт существует  

**Description:**  
PM critic: главный блокер MVP. Нужен живой proof, не build OK.

**Acceptance:**  
Playwright: 2 contexts, real signup, send, receive, optional E2E encrypt flag ON; screenshots MEDIA; test PASS

**test_logs:**  
PHYSICAL: e2e-messenger DM test timeout/fail

---

### GATE-008: Единый бэклог: архивировать итерационные DB, запрет на новые параллельные

- **Status:** DONE | **Score:** 96.0 | **Block:** QualityGate/Backlog  
- **Minutes:** 30 | **Value:** Нет размножения сущностей  
- **Depends:** —  
- **proof_type:** `manual` | **tdd_layer:** `none`  
- **auditor_pov:** PO,Architect  
- **mission_link:** Один source of truth  

**Description:**  
5+ backlog*.db = нарушение. Один backlog.db. Итерации → 10-Archive/backlogs/ после закрытия.

**Acceptance:**  
В 08-Backlog и 00-Product только один active backlog.db; старые в 10-Archive; meta.rule записан

**test_logs:**  
PASS manual 2026-07-27: archived 5 DB + 1 MD → 10-Archive/backlogs/pre-unified-20260727/; single active backlog.db at 00-Product-and-Agile/01-Backlog-Roadmap/backlog.db; 08-Backlog/README.md pointer only; meta.rule set

---

### GATE-001: Ввести 10-POV Deep Test checklist как обязательный gate перед DONE

- **Status:** TODO | **Score:** 95.0 | **Block:** QualityGate/10-POV  
- **Minutes:** 60 | **Value:** Защита от fake DONE  
- **Depends:** —  
- **proof_type:** `none` | **tdd_layer:** `none`  
- **auditor_pov:** Architect,SecOps,QA,UX,Backend,DevOps,Compliance,Performance,DBA,PO  
- **mission_link:** Качество продукта = доверие к мессенджеру  

**Description:**  
Каждая задача перед DONE проходит 10 векторов: Architect, SecOps, QA, UX/UI, Backend, DevOps, Compliance, Performance, DBA, Product Owner. Без checklist в test_logs — DONE запрещён.

**Acceptance:**  
В backlog.db есть поле auditor_pov; README gate описывает 10 POV; хотя бы 1 задача закрыта с заполненным auditor_pov checklist

**test_logs:**  
AUDIT: Supreme Auditor prompt requires 10-POV before accept

---

### GATE-002: Continuous TDD Loop: 4 слоя тестов обязательны на каждой итерации

- **Status:** TODO | **Score:** 94.0 | **Block:** QualityGate/TDD-Loop  
- **Minutes:** 120 | **Value:** Честные метрики  
- **Depends:** GATE-001  
- **proof_type:** `none` | **tdd_layer:** `go-test,vitest,playwright,bats`  
- **auditor_pov:** QA,DevOps  
- **mission_link:** Без TDD нельзя доказать что DM/VPN работают  

**Description:**  
Физический запуск: (1) go test ./... (2) npx vitest run (3) npx playwright test (4) bats/maestro если есть. Если хоть 1 FAIL — блокировка DONE. Врать о тестах запрещено.

**Acceptance:**  
Скрипт или runbook scripts/tdd-loop.* запускает 4 слоя; метрики пишутся в 07-QA-and-Testing/03-Coverage-Reports/; таблица Pass/Fail в каждом PR/коммите

**test_logs:**  
AUDIT: Auditor Continuous TDD Loop — vitest/playwright/maestro/bats

---

### GATE-003: Physical Security Gate: ripgrep secrets + Zero Trust input + SQL linter

- **Status:** TODO | **Score:** 93.0 | **Block:** QualityGate/Security  
- **Minutes:** 90 | **Value:** Zero Trust  
- **Depends:** GATE-001  
- **proof_type:** `none` | **tdd_layer:** `bats`  
- **auditor_pov:** SecOps,Compliance,DBA  
- **mission_link:** Мессенджер с E2E не может иметь hardcoded secrets  

**Description:**  
Перед тестами: (1) scan sk-/AKIA/password=/Bearer /api keys вне .env (2) внешний input валидируется до бизнес-логики (3) нет DROP TABLE / безусловных DELETE. Найден секрет → REJECT.

**Acceptance:**  
Скрипт scripts/security-gate.py (или .sh) в проекте; 0 secrets в коде; отчёт в 07-QA-and-Testing/04-Security-Scans/

**test_logs:**  
AUDIT: Physical Security Gate required before test run

---

### GATE-005: Правило proof_type: DONE только с autotest|e2e|api+manual, не keyword/build

- **Status:** TODO | **Score:** 92.0 | **Block:** QualityGate/Proof  
- **Minutes:** 30 | **Value:** Честность бэклога  
- **Depends:** GATE-001  
- **proof_type:** `none` | **tdd_layer:** `none`  
- **auditor_pov:** QA,PO  
- **mission_link:** False DONE = false product readiness  

**Description:**  
keyword/build/exists = FAKE DONE. Минимальный proof для DONE: autotest (go test/vitest) или e2e (playwright) или api live + browser manual со скрином.

**Acceptance:**  
Все DONE задачи имеют proof_type in (autotest,e2e,api); SQL check 0 DONE with proof_type in (none,keyword,build)

**test_logs:**  
AUDIT: previous 100/100 DONE had ~35 build-only and ~6 keyword-only

---

### P0-003: Очистить мусорные/пустые message bubbles из SQLite history

- **Status:** TODO | **Score:** 90.0 | **Block:** Messenger/Data  
- **Minutes:** 60 | **Value:** UX + data integrity  
- **Depends:** P0-002  
- **proof_type:** `none` | **tdd_layer:** `go-test,manual`  
- **auditor_pov:** DBA,UX,QA  
- **mission_link:** Чистая история = доверие  

**Description:**  
50 bubbles, 48 empty — тестовый мусор Nocturnal/E2E. Ломает UX и путает proof.

**Acceptance:**  
SELECT COUNT(*) empty messages = 0; UI chat без пустых пузырей; screenshot в 07-QA

**test_logs:**  
PHYSICAL: DOM .msg-text mostly empty strings

---

### GATE-007: Выровнять структуру проекта под _TEMPLATE V2

- **Status:** IN_PROGRESS | **Score:** 88.0 | **Block:** QualityGate/Structure  
- **Minutes:** 60 | **Value:** TEMPLATE V2 compliance  
- **Depends:** —  
- **proof_type:** `manual` | **tdd_layer:** `none`  
- **auditor_pov:** Architect,DevOps  
- **mission_link:** Единая структура = единый процесс  

**Description:**  
MISSING: 07-QA-and-Testing (создать). EXTRA: 08-Backlog (указатель→единый backlog), scratch, scripts/infra — решить: в .04-Src или 06/10. Нет мусора вне project folder.

**Acceptance:**  
07-QA-and-Testing существует с подпапками; 08-Backlog только README-pointer; единый backlog в 00-Product-and-Agile/01-Backlog-Roadmap/backlog.db; нет orphan scripts вне проекта

**test_logs:**  
PARTIAL 2026-07-27: created 07-QA-and-Testing/{01-Unit,02-E2E,03-Coverage,04-Security,05-Audit}; 08-Backlog reduced to pointer; still open: scratch/scripts/infra placement vs TEMPLATE

---

### P1-004: WS /ws только с валидным JWT (запрет anonymous token=NONE mode для prod path)

- **Status:** TODO | **Score:** 88.0 | **Block:** Messenger/WS-Auth  
- **Minutes:** 120 | **Value:** SecOps + reliability  
- **Depends:** P0-001  
- **proof_type:** `none` | **tdd_layer:** `go-test,playwright`  
- **auditor_pov:** SecOps,Backend,QA  
- **mission_link:** E2E security boundary  

**Description:**  
Сейчас WS connect without token. Это security hole + причина broken persistence.

**Acceptance:**  
WS without JWT rejected or limited; with JWT full DM; go test + e2e

**test_logs:**  
PHYSICAL: connectRelays token=NONE but mode=go-backend

---

### P1-002: Снять OOM/paging file blocker и прогнать go test ./... -cover по всем 28 пакетам

- **Status:** TODO | **Score:** 87.0 | **Block:** Testing/Go  
- **Minutes:** 90 | **Value:** Честное backend coverage  
- **Depends:** GATE-002  
- **proof_type:** `none` | **tdd_layer:** `go-test`  
- **auditor_pov:** QA,DevOps,Performance  
- **mission_link:** Backend truth  

**Description:**  
Заявлено 26/26 PASS 78% — недоказуемо. Физически: chat 84 PASS, auth 10 PASS, остальные OOM.

**Acceptance:**  
go test ./... -cover exit 0; отчёт coverage в 07-QA-and-Testing/03-Coverage-Reports/; цифры в test_logs без галлюцинаций

**test_logs:**  
PHYSICAL: paging file too small / cannot allocate memory

---

### P1-001: Обновить e2e-messenger.spec.ts под текущий UI (input User ID, не textarea pubkey)

- **Status:** TODO | **Score:** 86.0 | **Block:** Testing/Playwright  
- **Minutes:** 120 | **Value:** Рабочие functional E2E  
- **Depends:** P0-001  
- **proof_type:** `none` | **tdd_layer:** `playwright`  
- **auditor_pov:** QA,UX  
- **mission_link:** E2E = proof for auditors  

**Description:**  
5/7 FAIL из-за stale selectors. UI: input 'Вставь ID друга', tabs Новый контакт/Контакты/Группа.

**Acceptance:**  
e2e-messenger.spec.ts 7/7 PASS на :5173; deep.spec.js 10/10 PASS; конфиг testDir согласован

**test_logs:**  
PHYSICAL: waiting for textarea[placeholder=Вставь pubkey друга] — element gone

---

### GATE-004: Лимит 500 строк — hard gate на все .go/.ts/.svelte

- **Status:** TODO | **Score:** 85.0 | **Block:** QualityGate/LOC  
- **Minutes:** 45 | **Value:** Антидеградация архитектуры  
- **Depends:** —  
- **proof_type:** `none` | **tdd_layer:** `bats`  
- **auditor_pov:** Architect  
- **mission_link:** Поддерживаемость = скорость фич  

**Description:**  
Файлы >=500 строк = REJECT. God Object split обязателен. Continuous check.

**Acceptance:**  
Скрипт проверки LOC; 0 файлов prod >=500; gate в CI или pre-commit

**test_logs:**  
PHYSICAL: 0 Go god objects сейчас OK; gate нужен чтобы не откатилось

---

### P1-005: Windows paging file / memory budget для Go toolchain (dev machine)

- **Status:** TODO | **Score:** 80.0 | **Block:** Infra/Env  
- **Minutes:** 45 | **Value:** Dev environment  
- **Depends:** P1-002  
- **proof_type:** `none` | **tdd_layer:** `none`  
- **auditor_pov:** DevOps,Performance  
- **mission_link:** Enables honest metrics  

**Description:**  
Без этого GATE TDD loop backend layer невыполним на этой машине.

**Acceptance:**  
Документирован runbook; go test ./... завершается; или dockerized test runner как fallback

**test_logs:**  
PHYSICAL OOM blocks verification

---

### P1-003: Расширить Vitest за пределы api.ts (stores, messenger, send path)

- **Status:** TODO | **Score:** 75.0 | **Block:** Testing/Vitest  
- **Minutes:** 120 | **Value:** Business logic tests  
- **Depends:** GATE-002,P0-001  
- **proof_type:** `none` | **tdd_layer:** `vitest`  
- **auditor_pov:** QA  
- **mission_link:** TDD layer 1  

**Description:**  
Сейчас 13/13 только api client mocks. Нет тестов на stores/messenger и sendDM failure paths.

**Acceptance:**  
Vitest >=30 tests; stores covered; edge cases signup fail / no token; report in QA folder

**test_logs:**  
PHYSICAL: only test/api.test.ts exists

---

### P3-001: Переклассифицировать archived 100-task DONE по proof_type (reference only)

- **Status:** TODO | **Score:** 72.0 | **Block:** Backlog/Honesty  
- **Minutes:** 90 | **Value:** Historical truth  
- **Depends:** GATE-005,GATE-008  
- **proof_type:** `none` | **tdd_layer:** `none`  
- **auditor_pov:** PO,QA  
- **mission_link:** Learn from false green  

**Description:**  
В архиве full_20260721: ~10 autotest, ~22 api, ~35 build, ~6 keyword, ~27 other. Не восстанавливать как active DONE.

**Acceptance:**  
Документ 07-QA-and-Testing/05-Audit-Reports/PROOF_RECLASS_20260727.md с таблицей; active backlog не содержит fake DONE

**test_logs:**  
PHYSICAL audit classification 2026-07-25

---

### GATE-006: Каждая успешная итерация = commit + push в dev (если не Not for GitHub)

- **Status:** TODO | **Score:** 70.0 | **Block:** QualityGate/Git  
- **Minutes:** 15 | **Value:** Прослеживаемость  
- **Depends:** —  
- **proof_type:** `none` | **tdd_layer:** `none`  
- **auditor_pov:** DevOps  
- **mission_link:** История = audit trail  

**Description:**  
Аудитор проверяет: working tree clean после DONE; origin/dev содержит коммит.

**Acceptance:**  
Документировано в AGENTS.md; dev branch tracking origin/dev

**test_logs:**  
AUDIT: Git Push rule from Absolute Rules

---

### P2-006: OWASP/gosec physical scan (не self-assess 10/10 md)

- **Status:** TODO | **Score:** 70.0 | **Block:** Security/OWASP  
- **Minutes:** 180 | **Value:** Real security  
- **Depends:** GATE-003  
- **proof_type:** `none` | **tdd_layer:** `bats`  
- **auditor_pov:** SecOps,Compliance  
- **mission_link:** Trust  

**Description:**  
OWASP_AUDIT.md without scanner = doc theater.

**Acceptance:**  
gosec/staticcheck report in 07-QA-and-Testing/04-Security-Scans/; critical=0 or ticketed

**test_logs:**  
OLD: SH-003 OWASP_AUDIT.md 10/10 checked

---

### P2-002: Рабочий GitHub Actions CI (не ci.yml.template)

- **Status:** TODO | **Score:** 65.0 | **Block:** DevOps/CI  
- **Minutes:** 120 | **Value:** Automation  
- **Depends:** GATE-002,GATE-006  
- **proof_type:** `none` | **tdd_layer:** `bats`  
- **auditor_pov:** DevOps  
- **mission_link:** Continuous quality  

**Description:**  
Template ≠ CI. Нужен .github/workflows/ci.yml: go test + vitest + (optional playwright). PAT workflow scope.

**Acceptance:**  
Push to dev triggers CI green; artifacts coverage

**test_logs:**  
OLD: ci.yml.template created = doc-only

---

### P2-001: Проверить docker-compose up end-to-end (не keyword exists)

- **Status:** TODO | **Score:** 60.0 | **Block:** DevOps/Docker  
- **Minutes:** 120 | **Value:** Deploy path  
- **Depends:** GATE-002  
- **proof_type:** `none` | **tdd_layer:** `bats`  
- **auditor_pov:** DevOps,QA  
- **mission_link:** Shipability  

**Description:**  
Ранее DONE по 'yml exists'. Нужен физический up + health.

**Acceptance:**  
docker-compose up -d; health endpoints 200; down clean; logs no panic

**test_logs:**  
OLD test_logs: docker-compose.yml exists = keyword

---

### P2-003: Реальный SOCKS5/VPN tunnel вместо stub toggle

- **Status:** TODO | **Score:** 55.0 | **Block:** VPN/SOCKS5  
- **Minutes:** 480 | **Value:** VPN product slice  
- **Depends:** P0-004  
- **proof_type:** `none` | **tdd_layer:** `go-test,playwright`  
- **auditor_pov:** Backend,PO,QA  
- **mission_link:** Messenger + VPN = product differentiation  

**Description:**  
UI VPN Отключён; endpoint stub. Миссия включает VPN.

**Acceptance:**  
Toggle ON → traffic via tunnel (measurable); kill switch; test not just UI click

**test_logs:**  
SKILL: VPV-001 stub not real SOCKS5

---

### P3-008: Model Armor hooks: pre-hook injection scan docs + post-hook PII/secrets in agent outputs

- **Status:** TODO | **Score:** 55.0 | **Block:** Security/ModelArmor  
- **Minutes:** 60 | **Value:** Agent security  
- **Depends:** GATE-003  
- **proof_type:** `none` | **tdd_layer:** `none`  
- **auditor_pov:** SecOps,Compliance  
- **mission_link:** Safe multi-agent development  

**Description:**  
Auditor section 11: protect swarm from prompt injection; mask PII before deploy.

**Acceptance:**  
Checklist in 08-Security; agent prompts reference; no secrets in committed agent logs

**test_logs:**  
Auditor Security Guard & Model Armor

---

### P2-007: Починить e2e.spec.ts WebRTC tunnel (4173 refused)

- **Status:** TODO | **Score:** 50.0 | **Block:** Testing/WebRTC  
- **Minutes:** 90 | **Value:** P2P transport proof  
- **Depends:** GATE-002  
- **proof_type:** `none` | **tdd_layer:** `playwright`  
- **auditor_pov:** QA  
- **mission_link:** P2P transport mission  

**Description:**  
Preview :4173 not running; test dead.

**Acceptance:**  
WebRTC tunnel test PASS on documented port; or retired with WONTFIX reason

**test_logs:**  
PHYSICAL: ERR_CONNECTION_REFUSED localhost:4173

---

### P2-004: Voice/Video calls — functional proof (не PWA build OK)

- **Status:** TODO | **Score:** 45.0 | **Block:** Messenger/Calls  
- **Minutes:** 360 | **Value:** Feature honesty  
- **Depends:** —  
- **proof_type:** `none` | **tdd_layer:** `playwright`  
- **auditor_pov:** QA,UX,Backend  
- **mission_link:** Messenger completeness  

**Description:**  
calls.ts + CallOverlay compile ≠ works.

**Acceptance:**  
2-context call connect or honest BLOCKED with root cause; no fake DONE

**test_logs:**  
OLD: C-001/C-002 PWA build OK only

---

### P2-005: Android APK build + backend connect proof

- **Status:** TODO | **Score:** 40.0 | **Block:** Mobile/Android  
- **Minutes:** 240 | **Value:** Mobile path  
- **Depends:** P0-001  
- **proof_type:** `none` | **tdd_layer:** `maestro`  
- **auditor_pov:** QA,DevOps  
- **mission_link:** Distribution  

**Description:**  
android/ exists + CONNECT.md ≠ APK works.

**Acceptance:**  
APK builds; install emulator; auth+DM smoke or BLOCKED with log

**test_logs:**  
OLD: MB-001 keyword exists

---

### P3-007: Bats-core для deploy/scripts (TDD layer 4)

- **Status:** TODO | **Score:** 40.0 | **Block:** QA/Bats  
- **Minutes:** 90 | **Value:** DevOps tests  
- **Depends:** GATE-002,P2-001  
- **proof_type:** `none` | **tdd_layer:** `bats`  
- **auditor_pov:** DevOps,QA  
- **mission_link:** Ops reliability  

**Description:**  
build.sh/deploy scripts without bats = untested DevOps.

**Acceptance:**  
bats tests for critical scripts PASS; or scripts marked manual-only

**test_logs:**  
Auditor requires bats-core layer

---

### P3-006: Добавить Maestro mobile/PWA flow (TDD layer 3) или явно N/A

- **Status:** TODO | **Score:** 35.0 | **Block:** QA/Maestro  
- **Minutes:** 90 | **Value:** TDD completeness  
- **Depends:** P2-005,GATE-002  
- **proof_type:** `none` | **tdd_layer:** `maestro`  
- **auditor_pov:** QA  
- **mission_link:** Mobile quality  

**Description:**  
Auditor requires maestro test layer. If no mobile yet — document N/A with date.

**Acceptance:**  
maestro test exists and PASS OR QA note N/A until Android APK

**test_logs:**  
Auditor TDD 4 layers includes Maestro

---

### P3-002: Erasure coding — functional test or mark LATER honestly

- **Status:** TODO | **Score:** 30.0 | **Block:** ContentVault/Erasure  
- **Minutes:** 120 | **Value:** Honesty LATER features  
- **Depends:** —  
- **proof_type:** `none` | **tdd_layer:** `go-test`  
- **auditor_pov:** PO,Backend  
- **mission_link:** Indestructible content mission — later phase  

**Description:**  
Package compiles ≠ K-of-N works.

**Acceptance:**  
Either go test erasure PASS with scenario OR status LATER/WONTFIX MVP with reason

**test_logs:**  
OLD: CT-005 erasure compiles

---

### P3-004: Tor transport — verify or defer

- **Status:** TODO | **Score:** 28.0 | **Block:** P2P/Tor  
- **Minutes:** 90 | **Value:** Anonymity path  
- **Depends:** —  
- **proof_type:** `none` | **tdd_layer:** `go-test`  
- **auditor_pov:** SecOps,Backend  
- **mission_link:** Anonymity mission component  

**Description:**  
tor/ package present; needs real proof or LATER.

**Acceptance:**  
go test tor PASS meaningful OR LATER with mission note

**test_logs:**  
Previously keyword/package level

---

### P3-005: Desktop WireGuard — verify Tauri path or defer

- **Status:** TODO | **Score:** 27.0 | **Block:** Desktop/WireGuard  
- **Minutes:** 180 | **Value:** Desktop VPN  
- **Depends:** P2-003  
- **proof_type:** `none` | **tdd_layer:** `none`  
- **auditor_pov:** Backend,DevOps  
- **mission_link:** Desktop distribution  

**Description:**  
src-tauri config ≠ system VPN.

**Acceptance:**  
Build desktop VPN path proof OR LATER

**test_logs:**  
OLD: DK-002 exists config

---

### P3-003: Lightning Network — stub honesty (invoice package ≠ payments)

- **Status:** TODO | **Score:** 25.0 | **Block:** Economy/Lightning  
- **Minutes:** 60 | **Value:** No fake economy  
- **Depends:** —  
- **proof_type:** `none` | **tdd_layer:** `go-test`  
- **auditor_pov:** PO,Backend  
- **mission_link:** Economy phase deferred  

**Description:**  
economy/lightning.go exists; not live LN.

**Acceptance:**  
Document stub boundaries; no DONE claim for live payments; integration test or LATER

**test_logs:**  
OLD: E-003 lightning.go invoice+payment code only

---

## Архив старых бэклогов

`10-Archive/backlogs/pre-unified-20260727/`:
- backlog.db (117, fake DONE era)
- backlog_proxi_full_20260721.db (100 DONE)
- backlog_proxi_rescue_20260720.db
- backlog_20260720_pre_rescue.db
- backlog_dev_audit_20260725.db (15 iteration)
- BACKLOG_DEV_20260725.md
