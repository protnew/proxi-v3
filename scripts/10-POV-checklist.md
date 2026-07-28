# 10-POV checklist (paste into task test_logs)

- [ ] Product: user-visible behavior correct
- [ ] Architecture: no new god-file (>=500 LOC)
- [ ] Security: no secrets; JWT on protected routes
- [ ] Performance: no UI hang on send
- [ ] Reliability: empty inputs rejected
- [ ] Test: autotest or e2e attached
- [ ] Ops: native Windows/Linux path (no Docker required)
- [ ] DX: scripts/tdd-loop runs
- [ ] Privacy: no plaintext secrets in logs
- [ ] Rollback: change isolated to dev branch
