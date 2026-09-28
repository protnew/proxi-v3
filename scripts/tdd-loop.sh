#!/usr/bin/env bash
# tdd-loop.sh — native host (Linux CI or Git Bash). NO Docker required.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
VPN="$ROOT/src/src-vpn"
PWA="$ROOT/prototypes/pwa-vpn"
export GOMAXPROCS="${GOMAXPROCS:-1}"

echo "=== 1) go test ==="
cd "$VPN"
for pkg in ./auth/ ./store/ ./chat/; do
  go test "$pkg" -count=1 -timeout 90s
done

echo "=== 2) vitest ==="
cd "$PWA"
if [[ ! -d node_modules ]]; then npm ci --no-audit --no-fund; fi
npx vitest run

if [[ "${SKIP_E2E:-}" == "1" ]]; then
  echo "SKIP_E2E=1"
  exit 0
fi

echo "=== 3) playwright ==="
if curl -fsS "http://127.0.0.1:8090/api/health" >/dev/null 2>&1; then
  npx playwright test --config=playwright.config.ts
else
  echo "SKIP playwright: :8090 down"
fi
echo "TDD LOOP OK"
