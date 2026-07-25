# Production Runbook — Indestructible Messenger

## Deployment

### VPS Setup (Hetzner $5/mo)
```bash
# 1. Install Docker
curl -fsSL https://get.docker.com | sh

# 2. Clone repo
git clone https://github.com/protnew/proxi.git
cd proxi

# 3. Build & run
docker-compose up -d --build
```

### Environment Variables
- `PORT` — HTTP port (default: 8080)
- `DB_PATH` — SQLite file path (default: proxi.db)
- `JWT_SECRET` — JWT signing secret (REQUIRED in production)

### Health Check
```bash
curl http://localhost:8080/api/health
# Expected: {"status":"ok"}
```

## Monitoring

### Logs
```bash
docker-compose logs -f
```

### SQLite WAL Checkpoint
Runs automatically every 24h. Manual:
```bash
sqlite3 proxi.db "PRAGMA wal_checkpoint(TRUNCATE);"
```

## Troubleshooting

### WebSocket not connecting
- Check CORS headers in browser DevTools
- Verify port 8080 is open in firewall
- Check `/api/status` for hub stats

### Messages not persisting
- Check SQLite WAL mode: `PRAGMA journal_mode;`
- Verify disk space
- Check `proxi.db` permissions

### Rate limit (429)
- Default: 100 req/s per user
- Adjust in `routing.go`: `middleware.NewRateLimiter(R, B)`
