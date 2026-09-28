#!/usr/bin/env bash
# FUNC-COV-001: Functional suite gate — Indestructible Messenger/VPN
# Exit 0 = all green; non-zero = something broken.
set -e
cd "$(dirname "$0")/.."

echo "============================================"
echo "  FUNCTIONAL TEST GATE — Indestructible"
echo "============================================"

echo ""
echo "[1/3] Go tests (crypto + vpn + webserver)..."
GOMAXPROCS=1 go test ./... -count=1 -timeout 120s 2>&1
echo "  → Go: PASS"

echo ""
echo "[2/3] Vitest (unit + integration)..."
npx vitest run --reporter=verbose 2>&1
echo "  → Vitest: PASS"

echo ""
echo "[3/3] Playwright (product E2E)..."
npx playwright test e2e/ --reporter=list --timeout 30000 2>&1
echo "  → Playwright: PASS"

echo ""
echo "============================================"
echo "  ALL GATES GREEN ✓"
echo "============================================"
