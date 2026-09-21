# RUNBOOK — Неубиваемый (Proxi messenger+VPN)

## Source of Truth (P2)

| Что | Канон |
|-----|--------|
| GitHub active | `protnew/unkillable` · ветка `dev` |
| Archive only | `protnew/proxi` — **не пушить** |
| Local path | `C:\Obsidian\New\Projects\04-Неубиваемый-контент V2\.04-Src` |
| API | **`:8090`** (`curl http://127.0.0.1:8090/api/health`) |
| UI Solo verify | Vite **`:5173`** + proxy на API `:8090` |
| Alt UI | dist с того же `:8090` |
| Backlog DB | `08-Backlog/backlog_proxi_v6.db` (не v3) |
| DEPRECATED | папка `… V2 DEV` — эксперимент, не канон |

## Clone / pull

```bash
git clone https://github.com/protnew/unkillable.git
cd unkillable
git checkout dev
```

Или работай в vault `.04-Src` (там же `.git`).

## Environment

- `PORT` — HTTP port (**default: 8090**)
- `DB_PATH` — SQLite path (default under data dir; not committed)
- Never commit `.env*` except `*.example`; never commit `*.identity.key` / `*.db`

## Health

```bash
curl http://127.0.0.1:8090/api/health
# expect {"status":"ok",...}
```

## Start (typical Solo)

1. Go API from `src/src-vpn`: webserver on **8090**
2. Vite from `prototypes/pwa-vpn`: `npm run dev -- --host 127.0.0.1 --port 5173`
3. Open `http://127.0.0.1:5173/`

## Troubleshooting

- API down → check process on **8090**, not 8080
- Playwright `ERR_CONNECTION_REFUSED` → raise Vite :5173 and API :8090
- Do not use `proxi` remote for push

Updated 2026-09-18 · P2/P12 port+remote sync
