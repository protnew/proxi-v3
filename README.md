# Proxi — code root (ONLY git SoT)

| | |
|--|--|
| Git | this dir `.04-Src` |
| Work branch | `dev` |
| Prod pin | `main` (do not develop) |
| Backend | `src/src-vpn` → `:8090` |
| Frontend | `prototypes/pwa-vpn` → `:5173` |
| Android | `prototypes/android` (TUN+SOCKS5, second surface) |
| Start | `pwsh scripts/start-messenger-dev.ps1` |
| Tests | `pwsh scripts/tdd-loop.ps1` |
| Map | `docs/SOURCE_OF_TRUTH.md` |
| 2-user test | `docs/HOW_TO_TEST_2_USERS.md` |
| Backlog SoT | vault `08-Backlog/backlog_proxi_v6.db` |

**No Docker for daily dev.** Flutter / `src/frontend` / `:9999` / `:1420` are not Quick Start.

Daily: Vite UI `http://127.0.0.1:5173/` + Go API `http://127.0.0.1:8090/api/health`.

VPN by surface: PWA without `?dev=1` = WebRTC DataChannel in the browser (not a phone TUN). `?dev=1` = lab. Android = TUN + SOCKS5.

# Proxi / Unkillable Messenger — code root

**This folder is the ONLY git + code SoT.**

| | |
|--|--|
| Git | this dir (`.04-Src`) |
| Branch work | `dev` |
| Branch prod pin | `main` (do not develop here) |
| Backend | `src/src-vpn` → `:8090` |
| Frontend | `prototypes/pwa-vpn` → `:5173` |
| Android | `prototypes/android` |
| Start | `pwsh scripts/start-messenger-dev.ps1` |
| Tests | `pwsh scripts/tdd-loop.ps1` |
| Map | vault `00-ГДЕ-КОД-SOURCE-OF-TRUTH.md` |
| Manual 2-user | `docs/HOW_TO_TEST_2_USERS.md` |

**No Docker for daily dev.**
