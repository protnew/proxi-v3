# ГДЕ КОД — Source of Truth (не обнулялся)

Дата сверки: 2026-07-28. Проверено по диску + git, не по памяти.

## 1. Одна рабочая точка

| Что | Значение |
|-----|----------|
| **Vault/проект** | `C:\Obsidian\New\Projects\04-Неубиваемый-контент V2` |
| **Код (канон)** | `…\04-Неубиваемый-контент V2\.04-Src` |
| **Git root** | **только** `.04-Src` (там лежит `.git`) |
| **Remote** | `https://github.com/protnew/proxi.git` |
| **Рабочая ветка** | **`dev`** |
| **Prod pin (не трогать)** | **`main`** @ `ac1871f` |

### Почему `.04-Src`, а не `04-Src`

| Папка | Роль |
|-------|------|
| **`.04-Src`** | Реальный код + git. Скрыта от индекса Obsidian |
| **`04-Src`** | Только pointer-README. **Кода нет** (0 .go / 0 .ts) |

## 2. Что запускать (native Windows, БЕЗ Docker)

| Слой | Путь | Порт | Сейчас |
|------|------|------|--------|
| **Backend Go** | `.04-Src\src\src-vpn` → `messenger-server-dev.exe` | **:8080** | жив |
| **Frontend PWA** | `.04-Src\prototypes\pwa-vpn` (Vite) | **:5173** | жив |
| UI | http://127.0.0.1:5173/ | | |
| API | http://127.0.0.1:8080/api/health | | |

```powershell
cd ".04-Src"
pwsh .\scripts\start-messenger-dev.ps1
```

## 3. Откуда Docker

| Факт | Пояснение |
|------|-----------|
| В корне vault есть Dockerfile/compose | Legacy docs/devops |
| В `.04-Src\src\…` тоже docker-файлы | Legacy, не dev-path |
| В бэклоге был «Docker smoke» | ICEBOX из шаблона |
| **Сейчас** | Go exe + Vite. Docker **не** runtime |

## 4. Бэклог

| SoT | `08-Backlog\backlog_proxi_v3.db` |
| git mirror | `.04-Src\docs\backlog\backlog_proxi_v3.db` |
| archive | `10-Archive\backlogs\` |

## 5. Не канон

- `main` @ ac1871f
- `04-Src\` pointer only
- старый `src/frontend` vs **pwa-vpn**
- Docker compose
- DONE без proof

## 6. 2 пользователя руками

1. Chrome → http://127.0.0.1:5173/
2. Chrome Incognito → тот же URL
3. New Chat → 📋 свой User ID
4. Alice вставляет ID Bob → пишет
5. Bob открывает чат с Alice → видит текст

Авто: `npx playwright test e2e/messenger.spec.ts e2e/alice-bob.spec.ts` (было 5/5 PASS)

## 7. Правило агента

Код только `.04-Src` / ветка `dev`. Docker без явной просьбы — нет.
