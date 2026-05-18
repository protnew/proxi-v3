#!/bin/bash
# Запуск Unkillable Messenger + Cloudflare Tunnel
# URL туннеля выводится в файл tunnel_url.txt

PORT=9999
BASEDIR="$(cd "$(dirname "$0")" && pwd)"
URL_FILE="$BASEDIR/tunnel_url.txt"

# Запускаем сервер
echo "Starting server on :$PORT..."
DIST_DIR="$BASEDIR/dist" PORT=$PORT "$BASEDIR/messenger-server" &
SERVER_PID=$!
sleep 1

# Запускаем cloudflared и ловим URL
echo "Starting Cloudflare Tunnel..."
$BASEDIR/cloudflared tunnel --url http://localhost:$PORT 2>&1 | while read line; do
    echo "$line"
    # Ищем URL в выводе
    if echo "$line" | grep -qoP 'https://[a-z0-9-]+\.trycloudflare\.com'; then
        URL=$(echo "$line" | grep -oP 'https://[a-z0-9-]+\.trycloudflare\.com')
        echo "$URL" > "$URL_FILE"
        echo ""
        echo "========================================="
        echo "  TUNNEL URL: $URL"
        echo "  Saved to: $URL_FILE"
        echo "========================================="
        echo ""
    fi
done
