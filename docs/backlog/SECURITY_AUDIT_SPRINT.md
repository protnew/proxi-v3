# Security & Quality Audit (post-sprint)

_Generated: 2026-08-03T06:23:30Z_

Subagents blocked on FS; parent native audit.

## Findings

| finding | severity | file:line | recommendation | status |
|---------|----------|-----------|----------------|--------|
| validation.go dead (never called) | CRITICAL | validation.go | Wire into handlers | **FIXED** — `vpnroot.ValidateSignupInput` in `startup_routes.go` signup |
| Message max size only empty-check | HIGH | store/messages.go SaveMessage | Enforce MaxMessageLen | **FIXED** — reject >16KB |
| TEST-004 mocked fetch without api.ts | HIGH | test/api-no-token.test.ts | Import production request() | **FIXED** — imports `request` from api.ts, 3/3 PASS |
| Dynamic SQL in store.go:308 | LOW | store/store.go:308-312 | Was false positive — `IN (?,?,?)` placeholders + args | no change |
| migration DROP TABLE | INFO | store/migration.go:314 | FTS5 probe table, safe | OK |
| auth coverage 54% | MED | auth/jwt.go error paths | Add JWT expiry/invalid tests later | open |
| agent_minutes CHECK 5-20 | OK | tasks DDL | Enforced | OK |
| views reject empty proof/logs | OK | v_dod_audit | Consistent with DoD | OK |

## Evidence commands

```
go test . -run Validate -count=1
go test ./store/ -run SaveMessageRejectsOversize -v
go build ./cmd/webserver/
npx vitest run test/api-no-token.test.ts
```
