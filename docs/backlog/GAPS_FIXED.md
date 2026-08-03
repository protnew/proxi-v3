# Gaps fixed 2026-08-03T07:17:17Z

| # | Gap | Status | Evidence |
|---|-----|--------|----------|
| 1 | Vite :5173 | UP | npm run dev --host 127.0.0.1 --port 5173 |
| 2 | Message 10k vs 16k | FIXED | routing_chat uses vpnroot.ValidateMessage; error uses MaxMessageLen=16384 |
| 3 | ValidateMessage dead | FIXED | wired in handleMessages POST + schedule |
| 4 | auth JWT edges | SKIPPED (later) | — |
| 5 | Untracked junk | FIXED | .gitignore + delete auth_cov/logs/dupe test |

Live:
- oversize 16385 → expect 400 with `message too long: 16385 > 16384`
- 10001 → accept (201) under 16KB SoT
