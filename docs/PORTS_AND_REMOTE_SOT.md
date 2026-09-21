# PORTS_AND_REMOTE_SOT — 2026-09-18 (P2 + P12)

## Remote
- **Active:** `https://github.com/protnew/unkillable` · branch `dev`
- **Archive:** `protnew/proxi` — never push
- **Local canon:** `…\04-Неубиваемый-контент V2\.04-Src`

## Ports
| Role | Port | Notes |
|------|------|--------|
| Go API / health / WS | **8090** | `/api/health`, `/ws` |
| Vite Solo UI | **5173** | proxies API to 8090 |
| Dist served by Go | 8090 | alt to Vite |
| Emulator → host API | `10.0.2.2:8090` | Android AVD |
| Choser local | 3002 | decision tables SoT |

## Do not
- Cite `:8080` as live API in Solo HOW_TO / RUNBOOK / tdd (except `archive/`)
- Treat `proxi` as active remote
- Use DEPRECATED `V2 DEV` folder as code root
