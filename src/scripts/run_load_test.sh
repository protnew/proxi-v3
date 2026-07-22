#!/bin/bash
# Build, test, and load-test Proxi Messenger
set -e

cd "$(dirname "$0")/.."

echo "=== BUILD ==="
cd src-vpn && CGO_ENABLED=1 go build -o ../messenger-server ./cmd/webserver/
echo "Build OK"

echo ""
echo "=== GO TESTS ==="
CGO_ENABLED=1 go test ./... -count=1 -timeout 5m
echo "Tests OK"

echo ""
echo "=== LOAD TEST ==="
echo "Starting server..."
PORT=9999 DIST_DIR=./dist DATA_DIR=/tmp/um-bench ../messenger-server &
SERVER_PID=$!
sleep 2

echo "Running REST load test..."
k6 run ../scripts/load_test_rest.js || true

echo ""
echo "Running WebSocket load test..."
k6 run ../scripts/load_test_ws.js || true

echo ""
echo "Stopping server..."
kill $SERVER_PID 2>/dev/null
echo "Done."
