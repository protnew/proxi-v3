# P9 / R2 — public relay smoke guard

## Run
```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File scripts\guard-no-public-relays.ps1 -Root $PWD
```
Exit 1 if nos.lol|nostr.band|damus.io appear in serveable dist or active prototypes/pwa-vpn/src (excludes archive/node_modules/_quarantine_p9).

## CI snippet
Add a job step after vite build that runs the same script and fails on exit 1.
