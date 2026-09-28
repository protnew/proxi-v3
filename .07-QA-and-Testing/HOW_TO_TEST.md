# HOW TO TEST — Indestructible Messenger/VPN

**Branch:** `dev`
**Code root:** `.04-Src`
**Server:** `:8090` (DIST_DIR env var)

## Quick start

```bash
# 1. Build PWA
cd .04-Src/prototypes/pwa-vpn
npx vite build

# 2. Build Go server
cd ../../src/src-vpn
go build -o C:\Hermes\indestructible-run\webserver.exe ./cmd/webserver

# 3. Start server
set DIST_DIR=<path-to-pwa-vpn>\dist
set PORT=8090
C:\Hermes\indestructible-run\webserver.exe

# 4. Verify
curl http://127.0.0.1:8090/api/health
# → {"status":"ok",...}
```

## Functional test gate (one command)

```bash
cd .04-Src/prototypes/pwa-vpn
bash run-functional-tests.sh   # or run-functional-tests.bat on Windows
```

This runs Go tests + Vitest + Playwright in sequence.

## Individual suites

### Go (unit + integration)
```bash
cd .04-Src/src/src-vpn
set GOMAXPROCS=1
go test ./... -count=1 -timeout 120s
```

### Vitest (frontend unit)
```bash
cd .04-Src/prototypes/pwa-vpn
npx vitest run
```

### Playwright (E2E)
```bash
cd .04-Src/prototypes/pwa-vpn
npx playwright test e2e/ --reporter=list --timeout=30000
```

## Test inventory

| Suite | What | Last result |
|-------|------|-------------|
| Go crypto | X3DH + Double Ratchet | PASS |
| Go webserver | CORS + auth + routing | PASS |
| Go vpn | Tunnel + STUN config | PASS |
| Vitest (41 files) | Identity + NIP-44 + transport + UI | 355/355 PASS |
| Playwright product-same-wifi | LAN API + give VPN + DM | PASS |
| Playwright vpn-p2p-e2e | Give/accept/engine/phone | 4/4 PASS |
| Playwright push-amnezia | Push + tunnel UI | 2/2 PASS |
| Playwright messenger | DM send + emoji | PASS |
| Playwright alice-bob | Dual-window DM | PASS |
| Playwright webrtc-e2e | One-click WebRTC | 5/5 PASS |

## Archived (DO NOT RUN)
`e2e/archived-wt/` — WebTransport tests (superseded by WebRTC per table 01)

## Manual tests

### Same-WiFi laptop + phone
1. Laptop: open `http://127.0.0.1:8090/?role=alice`
2. Phone (same WiFi): open `http://<LAN-IP>:8090/?role=bob` (from `/api/network/lan`)
3. Alice: click "Дать VPN другу"
4. Bob: accept invite

### Dual-window (single machine)
1. Window 1: `http://127.0.0.1:8090/?role=alice`
2. Window 2 (incognito): `http://127.0.0.1:8090/?role=bob`
