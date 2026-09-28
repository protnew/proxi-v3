#!/bin/bash
set -e

echo "🔥 Unkillable Messenger starting..."

# Start web server (serves frontend + VPN API)
exec /app/ messenger-server
