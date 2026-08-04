# Как прогнать полные функциональные тесты (native Windows)

**Дата прогона агента:** 2026-08-04T07:55:06Z  
**Код:** ветка `dev` · `0d028dd chore(qa): drop coverage *.out from git; keep Zero-Point reports only`  
**Стек:** Go API `:8080` + Vite PWA `:5173` · **без Docker**

---

## 0. Требования

- Go 1.22+ (`go version`)
- Node.js 20+ + npm
- Playwright browsers уже ставились (`npx playwright install` один раз)
- Проект:  
  `C:\Obsidian\New\Projects\04-Неубиваемый-контент V2\.04-Src`

Открой **cmd** или **PowerShell** (не WSL).

---

## 1. Поднять серверы (два окна)

### Окно A — API

```bat
cd /d "C:\Obsidian\New\Projects\04-Неубиваемый-контент V2\.04-Src\src\src-vpn"
set GOMAXPROCS=1
go build -o messenger-server-dev.exe ./cmd/webserver/
messenger-server-dev.exe
```

Проверка:

```bat
curl http://127.0.0.1:8080/api/health
```

Ожидание: HTTP 200.

### Окно B — UI

```bat
cd /d "C:\Obsidian\New\Projects\04-Неубиваемый-контент V2\.04-Src\prototypes\pwa-vpn"
npm install
npm run dev -- --host 127.0.0.1 --port 5173
```

Проверка: в браузере `http://127.0.0.1:5173/` — «Indestructible Messenger».

---

## 2. Backend unit/integration (Go)

```bat
cd /d "C:\Obsidian\New\Projects\04-Неубиваемый-контент V2\.04-Src\src\src-vpn"
set GOMAXPROCS=1
go test ./... -count=1 -timeout 120s
```

С coverage по пакету:

```bat
go test ./auth/ -cover -count=1
go test ./chat/ -cover -count=1
go test ./store/ -cover -count=1
go test ./cmd/webserver/ -cover -count=1
go test ./nostr/ -cover -count=1
```

**Ок:** все пакеты `ok`, exit code 0.  
**Если OOM / paging file:** не гонять параллельно; оставить `GOMAXPROCS=1`, пакеты по одному.

---

## 3. Frontend unit (Vitest)

```bat
cd /d "C:\Obsidian\New\Projects\04-Неубиваемый-контент V2\.04-Src\prototypes\pwa-vpn"
npx vitest run
```

С coverage:

```bat
npx vitest run --coverage
```

**Ок:** `33 passed` (или больше), exit 0.

---

## 4. E2E / функциональные UI (Playwright)

Серверы из §1 **обязаны** быть живы.

```bat
cd /d "C:\Obsidian\New\Projects\04-Неубиваемый-контент V2\.04-Src\prototypes\pwa-vpn"
npx playwright test --config=playwright.config.ts --reporter=list
```

**Ок:** `20 passed` (alice-bob, messenger, vpn, deep×10, orphan×4).

Только messenger:

```bat
npx playwright test e2e/messenger.spec.ts e2e/alice-bob.spec.ts --reporter=list
```

Только VPN:

```bat
npx playwright test e2e/vpn.spec.ts --reporter=list
```

---

## 5. Ручной smoke (MVP-002) — 2 браузера

1. Открой два окна (Chrome + Chrome Incognito или Edge).  
2. `http://127.0.0.1:5173/` в обоих.  
3. В каждом: New chat → User ID собеседника → имя → «Начать чат».  
4. Alice пишет сообщение → у Bob появляется.  
5. Обратно Bob → Alice.

---

## 6. Что НЕ входит в этот suite

| Исключено | Почему |
|-----------|--------|
| `tests/e2e.spec.ts` (WebRTC :4173) | Нужен preview-сервер, не native dev |
| Bats (`07-QA-and-Testing/bats/`) | CLI `bats` не установлен |
| Maestro (Android) | Нет mobile pipeline |
| Docker compose smoke | ICEBOX, не dev-runtime |

---

## 7. Критерии «зелёно»

| Слой | Минимум |
|------|---------|
| Go `./...` | 0 FAIL |
| Vitest | 0 FAIL |
| Playwright | 20/20 (или актуальный total) |
| API health | 200 |
| UI | открывается title |

---

## 8. Результат прогона агента (2026-08-04T07:55:06Z)

| Слой | Результат |
|------|-----------|
| Go packages | **29/0 fail** (повтор clean) |
| Vitest | **33/0** · FE stmt cover ~9.8% |
| Playwright | **20/0** · 97s |
| API/Vite | UP |
| Git | `dev` @ `0d028dd chore(qa): drop coverage *.out from git; keep Zero-Point reports only` |

Первый проход Go с coverprofile: 28 pass + 1 flaky `nostr` под I/O; **повтор clean `go test` без shared cov: см. ниже**.

### Go clean re-run
- pass packages: **29**
- fail packages: **0**
- (none)

### Key covers
{
  ".": {
    "ok": true,
    "cov": 62.4
  },
  "./auth/": {
    "ok": true,
    "cov": 91.2
  },
  "./chat/": {
    "ok": true,
    "cov": 84.7
  },
  "./store/": {
    "ok": true,
    "cov": 58.1
  },
  "./cmd/webserver/": {
    "ok": true,
    "cov": 34.2
  },
  "./nostr/": {
    "ok": true,
    "cov": 71.2
  }
}

---

## 9. Если упало

| Симптом | Действие |
|---------|----------|
| Playwright `ERR_CONNECTION_REFUSED` | Поднять Vite :5173 и API :8080 |
| signup 401 | Нужна ветка `dev` с fix public routes |
| Go OOM | `GOMAXPROCS=1`, пакеты по одному |
| stale selectors textarea | Не использовать старый `tests/e2e.spec.ts`; брать `e2e/messenger.spec.ts` |
| nostr flaky | `go test ./nostr/ -count=3` — должен PASS |

---

## 10. Одной командой (после поднятых серверов)

PowerShell:

```powershell
cd "C:\Obsidian\New\Projects\04-Неубиваемый-контент V2\.04-Src\src\src-vpn"
$env:GOMAXPROCS=1
go test ./... -count=1 -timeout 120s
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
cd "..\..\prototypes\pwa-vpn"
npx vitest run
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
npx playwright test --config=playwright.config.ts --reporter=list
```

---

_Файл сгенерирован агентом. Source of truth кода: `.04-Src` branch `dev`._
