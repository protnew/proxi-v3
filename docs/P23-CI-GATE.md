# P23 — CI gate
- Prefer a single active workflow under .04-Src/.github/workflows/ (duplicates → _archive_p23).
- Smoke: go test -run 'TestR7|TestR11' ./... in cmd/webserver + vitest relay/delivery guards.
- Android FIXED needs assembleDebug + unit tests separately when SDK present.