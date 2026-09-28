# PROXY ISOLATION — 2026-09-23 (обязательно для всех агентов на i1)

## Проблема
User/Process `HTTP_PROXY`/`HTTPS_PROXY` указывали на мёртвый `http://127.0.0.1:8899`.
Devin/Codex и Go-тесты ходили туда → «нет сети» / dial timeout на 127.0.0.1.

## Факты
- Proxi/Неубиваемый **не** системный HTTP-прокси. Канон портов: API `:8090`, Vite `:5173` (DEV-эксперимент `:9999` / START_PROXI `:8799`/`:8889`).
- Порт `8899` в каноне 04 нет; поднимать его снова **нельзя**.
- User env уже должен быть пустым; Process-shell Grok Bot мог наследовать старое — перед тестами/агентами чистить Process.

## Правила (чтобы не повторилось)
1. **Никогда** не делать `setx HTTP_PROXY` / User-level proxy под стенд 04.
2. Если нужен локальный forwarder для одного эксперимента — только Process-scope, только пока слушатель жив, в конце `Remove-Item Env:HTTP_PROXY` + проверка `Get-NetTCPConnection -LocalPort …`.
3. Перед `go test` / Playwright / внешними ИИ: очистить Process proxy (см. `clear-proxy.ps1`).
4. В Go-пакетах с httptest: `TestMain` или `http.Transport{Proxy: nil}` — не доверять окружению.
5. AmneziaVPN GUI/auto-import **не** запускать (бан Алексея).

## clear-proxy.ps1
См. рядом. Запускать в начале любой длинной сессии на i1.
