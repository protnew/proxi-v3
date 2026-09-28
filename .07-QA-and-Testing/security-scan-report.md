# Security Scan Report — SEC-004
**Date:** 2026-08-09T17:54:20.558459
**Branch:** dev

## go vet
```exit: 0
No issues.
```

## gosec
gosec not installed — go vet is baseline.

### Manual security checklist
- [x] No hardcoded TURN credentials (table 58)
- [x] CORS whitelist enforced for LAN origins only (SEC-005)
- [x] private.key removed from git tracking
- [ ] private.key rotation (PRIVATE-KEY-ROT TODO)
- [x] JWT tokens stored in localStorage (dev only)