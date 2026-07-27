# Proxi Backlog CPO v3 (2026-07-27)

**DB:** `08-Backlog/backlog_proxi_v3.db`  
**Tasks:** 85  

## Status

| Status | N |
|---|---|
| DONE | 2 |
| ICEBOX | 45 |
| TODO | 38 |

## Lead queue (TODO only)

| bp | sp | ROI | ID | Title | min | UV | tenant |
|---|---|---|---|---|---|---|---|
| 100 | 100 | 7.50 | AUTH-001 | Map auth routes in Go routing files | 12 | 90 | backend |
| 100 | 95 | 9.50 | AUTH-002 | Reproduce signup 401 with curl/python | 10 | 95 | backend |
| 100 | 90 | 7.92 | AUTH-003 | Identify middleware blocking unauthenticated sig | 12 | 95 | backend |
| 100 | 85 | 6.67 | AUTH-004 | Exempt signup/login from JWT middleware | 15 | 100 | backend |
| 100 | 80 | 7.50 | AUTH-005 | Return JWT + user_id shape expected by frontend | 12 | 90 | backend |
| 100 | 75 | 7.92 | AUTH-006 | Persist JWT to localStorage after signup | 12 | 95 | frontend |
| 100 | 70 | 8.50 | AUTH-007 | /api/identity works with Bearer token | 10 | 85 | backend |
| 100 | 65 | 4.67 | AUTH-008 | Add Go table tests for signup edge cases | 15 | 70 | backend |
| 100 | 60 | 5.87 | AUTH-009 | Reject or limit anonymous WS in authenticated mo | 15 | 88 | backend |
| 98 | 100 | 7.50 | MSG-001 | Trace frontend sendDM code path | 12 | 90 | frontend |
| 98 | 95 | 7.50 | MSG-002 | Trace backend hub.OnMessage → SaveMessage | 12 | 90 | backend |
| 98 | 90 | 7.08 | MSG-003 | Fix empty message persistence / filter empties | 12 | 85 | backend |
| 98 | 88 | 6.67 | MSG-004 | Fix UI send clears input and appends bubble with | 15 | 100 | frontend |
| 98 | 85 | 7.00 | MSG-005 | SQL cleanup script for empty historical messages | 10 | 70 | backend |
| 98 | 80 | 6.50 | MSG-006 | UI does not render empty bubbles | 10 | 65 | frontend |
| 98 | 75 | 6.25 | MSG-007 | WS delivers DM to second connection same user mu | 12 | 75 | backend |
| 98 | 70 | 6.67 | MSG-008 | Cross-user DM delivery integration test | 15 | 100 | backend |
| 98 | 60 | 5.83 | MSG-009 | isE2EEnabled respected on send path | 12 | 70 | frontend |
| 95 | 100 | 6.00 | QG-001 | Commit AUDITOR_RULES + CPO DoR into 08-Backlog | 10 | 60 | infra |
| 95 | 90 | 7.00 | QG-002 | SQL view/query rejecting DONE without proof | 10 | 70 | infra |
| 95 | 85 | 5.00 | QG-003 | Script scripts/tdd-loop that runs go+vitest+play | 15 | 75 | infra |
| 95 | 80 | 6.67 | QG-004 | Script scripts/security-gate secrets ripgrep pat | 12 | 80 | infra |
| 95 | 75 | 6.50 | QG-005 | Script check no prod file >=500 lines | 10 | 65 | infra |
| 95 | 70 | 5.50 | QG-006 | Template 10-POV checklist snippet for test_logs | 10 | 55 | infra |
| 95 | 60 | 4.17 | QG-007 | Finish TEMPLATE V2: scratch/scripts/infra placem | 12 | 50 | infra |
| 92 | 100 | 6.67 | E2E-001 | Update e2e-messenger selectors to input User ID  | 12 | 80 | frontend |
| 92 | 95 | 7.00 | E2E-002 | Unify playwright testDir (e2e vs tests) | 10 | 70 | frontend |
| 92 | 90 | 6.67 | E2E-003 | Write Alice→Bob two-context DM test | 15 | 100 | frontend |
| 92 | 85 | 6.67 | E2E-004 | Run Alice→Bob e2e green on :5173+:8080 | 15 | 100 | frontend |
| 92 | 50 | 5.00 | E2E-005 | Keep deep.spec.js 10 screenshot tests green | 10 | 50 | frontend |
| 88 | 100 | 5.67 | TEST-001 | Document/run paging file or dockerized go test r | 15 | 85 | infra |
| 88 | 90 | 5.83 | TEST-002 | go test ./chat/ ./auth/ baseline capture | 12 | 70 | backend |
| 88 | 85 | 5.33 | TEST-003 | go test ./... -cover when memory allows | 15 | 80 | backend |
| 88 | 70 | 5.42 | TEST-004 | Add vitest tests for sendDM failure without toke | 12 | 65 | frontend |
| 88 | 50 | 4.00 | TEST-006 | Write Zero-Point metrics table template | 10 | 40 | infra |
| 85 | 100 | 6.67 | SEC-001 | Run initial secrets scan on .04-Src | 12 | 80 | infra |
| 85 | 80 | 5.83 | SEC-002 | Verify signup/message validation max size | 12 | 70 | backend |
| 85 | 75 | 6.00 | SEC-003 | Scan for DROP TABLE / unconditional DELETE | 10 | 60 | backend |

## All tasks

- `AUTH-001` **TODO** bp=100 sp=100 roi=7.50 — Map auth routes in Go routing files _Auth/RouteMap_
- `AUTH-002` **TODO** bp=100 sp=95 roi=9.50 — Reproduce signup 401 with curl/python _Auth/RouteMap_
- `AUTH-003` **TODO** bp=100 sp=90 roi=7.92 — Identify middleware blocking unauthenticated signup _Auth/Middleware_
- `AUTH-004` **TODO** bp=100 sp=85 roi=6.67 — Exempt signup/login from JWT middleware _Auth/Fix_
- `AUTH-005` **TODO** bp=100 sp=80 roi=7.50 — Return JWT + user_id shape expected by frontend _Auth/Fix_
- `AUTH-006` **TODO** bp=100 sp=75 roi=7.92 — Persist JWT to localStorage after signup _Auth/Frontend_
- `AUTH-007` **TODO** bp=100 sp=70 roi=8.50 — /api/identity works with Bearer token _Auth/Identity_
- `AUTH-008` **TODO** bp=100 sp=65 roi=4.67 — Add Go table tests for signup edge cases _Auth/Tests_
- `AUTH-009` **TODO** bp=100 sp=60 roi=5.87 — Reject or limit anonymous WS in authenticated mode _Auth/WS_
- `AUTH-010` **ICEBOX** bp=100 sp=40 roi=4.00 — Document auth flow sequence in 09-Docs _Auth/Docs_
- `MSG-001` **TODO** bp=98 sp=100 roi=7.50 — Trace frontend sendDM code path _MessengerCore/SendPath_
- `MSG-002` **TODO** bp=98 sp=95 roi=7.50 — Trace backend hub.OnMessage → SaveMessage _MessengerCore/SendPath_
- `MSG-003` **TODO** bp=98 sp=90 roi=7.08 — Fix empty message persistence / filter empties _MessengerCore/SendPath_
- `MSG-004` **TODO** bp=98 sp=88 roi=6.67 — Fix UI send clears input and appends bubble with text _MessengerCore/SendPath_
- `MSG-005` **TODO** bp=98 sp=85 roi=7.00 — SQL cleanup script for empty historical messages _MessengerCore/History_
- `MSG-006` **TODO** bp=98 sp=80 roi=6.50 — UI does not render empty bubbles _MessengerCore/History_
- `MSG-007` **TODO** bp=98 sp=75 roi=6.25 — WS delivers DM to second connection same user multi-device _MessengerCore/Realtime_
- `MSG-008` **TODO** bp=98 sp=70 roi=6.67 — Cross-user DM delivery integration test _MessengerCore/Realtime_
- `MSG-009` **TODO** bp=98 sp=60 roi=5.83 — isE2EEnabled respected on send path _MessengerCore/E2EFlag_
- `MSG-010` **ICEBOX** bp=98 sp=40 roi=3.00 — Reply/forward/edit smoke if endpoints exist _MessengerCore/Reply_
- `MSG-011` **ICEBOX** bp=98 sp=35 roi=3.33 — Typing indicator WS roundtrip _MessengerCore/Typing_
- `MSG-012` **ICEBOX** bp=98 sp=30 roi=2.92 — File upload path smoke _MessengerCore/Files_
- `QG-008` **DONE** bp=95 sp=100 roi=8.75 — Only one active backlog_proxi_v*.db in 08-Backlog _QualityGate/Backlog_
- `QG-001` **TODO** bp=95 sp=100 roi=6.00 — Commit AUDITOR_RULES + CPO DoR into 08-Backlog _QualityGate/Rules_
- `QG-002` **TODO** bp=95 sp=90 roi=7.00 — SQL view/query rejecting DONE without proof _QualityGate/Proof_
- `QG-003` **TODO** bp=95 sp=85 roi=5.00 — Script scripts/tdd-loop that runs go+vitest+playwright _QualityGate/TDD_
- `QG-004` **TODO** bp=95 sp=80 roi=6.67 — Script scripts/security-gate secrets ripgrep patterns _QualityGate/Sec_
- `QG-005` **TODO** bp=95 sp=75 roi=6.50 — Script check no prod file >=500 lines _QualityGate/LOC_
- `QG-006` **TODO** bp=95 sp=70 roi=5.50 — Template 10-POV checklist snippet for test_logs _QualityGate/10POV_
- `QG-007` **TODO** bp=95 sp=60 roi=4.17 — Finish TEMPLATE V2: scratch/scripts/infra placement decision _QualityGate/Structure_
- `E2E-001` **TODO** bp=92 sp=100 roi=6.67 — Update e2e-messenger selectors to input User ID UI _E2EProof/Selectors_
- `E2E-002` **TODO** bp=92 sp=95 roi=7.00 — Unify playwright testDir (e2e vs tests) _E2EProof/Config_
- `E2E-003` **TODO** bp=92 sp=90 roi=6.67 — Write Alice→Bob two-context DM test _E2EProof/AliceBob_
- `E2E-004` **TODO** bp=92 sp=85 roi=6.67 — Run Alice→Bob e2e green on :5173+:8080 _E2EProof/AliceBob_
- `E2E-005` **TODO** bp=92 sp=50 roi=5.00 — Keep deep.spec.js 10 screenshot tests green _E2EProof/Deep_
- `E2E-006` **ICEBOX** bp=92 sp=40 roi=3.33 — Triage e2e.spec.ts :4173 refused _E2EProof/WebRTC_
- `TEST-001` **TODO** bp=88 sp=100 roi=5.67 — Document/run paging file or dockerized go test runner _TestingInfra/Go_
- `TEST-002` **TODO** bp=88 sp=90 roi=5.83 — go test ./chat/ ./auth/ baseline capture _TestingInfra/Go_
- `TEST-003` **TODO** bp=88 sp=85 roi=5.33 — go test ./... -cover when memory allows _TestingInfra/Go_
- `TEST-004` **TODO** bp=88 sp=70 roi=5.42 — Add vitest tests for sendDM failure without token _TestingInfra/Vitest_
- `TEST-005` **ICEBOX** bp=88 sp=60 roi=3.33 — Add vitest tests for messenger store if present _TestingInfra/Vitest_
- `TEST-006` **TODO** bp=88 sp=50 roi=4.00 — Write Zero-Point metrics table template _TestingInfra/Report_
- `SEC-001` **TODO** bp=85 sp=100 roi=6.67 — Run initial secrets scan on .04-Src _Security/Scan_
- `SEC-002` **TODO** bp=85 sp=80 roi=5.83 — Verify signup/message validation max size _Security/Input_
- `SEC-003` **TODO** bp=85 sp=75 roi=6.00 — Scan for DROP TABLE / unconditional DELETE _Security/SQL_
- `SEC-004` **ICEBOX** bp=85 sp=60 roi=4.33 — Run gosec or staticcheck; store report _Security/OWASP_
- `SEC-005` **ICEBOX** bp=85 sp=55 roi=5.50 — Verify CORS whitelist not reflection _Security/CORS_
- `SEC-006` **ICEBOX** bp=85 sp=40 roi=4.00 — Model Armor checklist in 08-Security _Security/Armor_
- `GRP-001` **ICEBOX** bp=70 sp=80 roi=4.17 — Inventory groups API endpoints vs UI _Groups/API_
- `GRP-002` **ICEBOX** bp=70 sp=70 roi=3.67 — E2E create group after core DM green _Groups/Create_
- `GRP-003` **ICEBOX** bp=70 sp=60 roi=3.33 — Add member API smoke _Groups/Members_
- `UI-001` **ICEBOX** bp=65 sp=70 roi=4.50 — Verify premium empty state still intact post-fixes _UI/Chat_
- `UI-002` **ICEBOX** bp=65 sp=60 roi=3.33 — Context menu reply visible on bubble _UI/Chat_
- `UI-003` **ICEBOX** bp=65 sp=50 roi=3.50 — Emoji picker inserts into input _UI/Emoji_
- `UI-004` **ICEBOX** bp=65 sp=40 roi=3.00 — VPN panel toggle UI only smoke _UI/VPN_
- `PARITY-001` **ICEBOX** bp=65 sp=39 roi=1.67 — Parity spike: Pinned chats _UI/Parity_
- `PARITY-002` **ICEBOX** bp=65 sp=38 roi=1.67 — Parity spike: Message search UI _UI/Parity_
- `PARITY-003` **ICEBOX** bp=65 sp=37 roi=1.67 — Parity spike: Unread badges _UI/Parity_
- `PARITY-004` **ICEBOX** bp=65 sp=36 roi=1.67 — Parity spike: Push notification opt-in _UI/Parity_
- `PARITY-005` **ICEBOX** bp=65 sp=35 roi=1.67 — Parity spike: Link previews _UI/Parity_
- `PARITY-006` **ICEBOX** bp=65 sp=34 roi=1.67 — Parity spike: Voice message playback UI _UI/Parity_
- `PARITY-007` **ICEBOX** bp=65 sp=33 roi=1.67 — Parity spike: Sticker send UI _UI/Parity_
- `PARITY-008` **ICEBOX** bp=65 sp=32 roi=1.67 — Parity spike: Chat folders _UI/Parity_
- `PARITY-009` **ICEBOX** bp=65 sp=31 roi=1.67 — Parity spike: Disappearing messages settings _UI/Parity_
- `PARITY-010` **ICEBOX** bp=65 sp=30 roi=1.67 — Parity spike: Multi-account switcher _UI/Parity_
- `VPN-001` **ICEBOX** bp=60 sp=90 roi=5.00 — Document stub vs real SOCKS5 boundary _VPN/Truth_
- `VPN-002` **ICEBOX** bp=60 sp=70 roi=3.67 — Architecture spike real browser VPN path _VPN/Design_
- `VPN-003` **ICEBOX** bp=60 sp=50 roi=4.67 — Implement minimal measurable tunnel proof _VPN/Impl_
- `P2P-001` **ICEBOX** bp=55 sp=70 roi=2.92 — Inventory content/ipfs/erasure packages vs wired routes _P2PContent/Inventory_
- `P2P-002` **ICEBOX** bp=55 sp=60 roi=2.92 — Mark erasure coding LATER with AC for future _P2PContent/Later_
- `P2P-003` **ICEBOX** bp=55 sp=50 roi=2.92 — Tor package go test smoke or ICEBOX _P2PContent/Later_
- `MOB-001` **ICEBOX** bp=50 sp=80 roi=2.67 — Verify Android project opens / gradle tasks list _MobileDesktop/Android_
- `MOB-002` **ICEBOX** bp=50 sp=70 roi=3.33 — APK debug build attempt _MobileDesktop/Android_
- `MOB-003` **ICEBOX** bp=50 sp=60 roi=3.00 — Tauri config inventory _MobileDesktop/Desktop_
- `MOB-004` **ICEBOX** bp=50 sp=40 roi=3.12 — Decide Maestro N/A until APK _MobileDesktop/Maestro_
- `DO-001` **ICEBOX** bp=45 sp=90 roi=4.50 — docker-compose config validate _DevOps/Docker_
- `DO-002` **ICEBOX** bp=45 sp=80 roi=4.00 — docker compose up smoke health _DevOps/Docker_
- `DO-003` **ICEBOX** bp=45 sp=70 roi=3.67 — Create real .github/workflows/ci.yml from template _DevOps/CI_
- `DO-004` **ICEBOX** bp=45 sp=60 roi=3.75 — Add bats tests for tdd-loop and security-gate scripts _DevOps/Bats_
- `DO-005` **ICEBOX** bp=45 sp=50 roi=4.38 — Confirm dev branch tracking and push policy in AGENTS.md _DevOps/Git_
- `ECO-001` **ICEBOX** bp=30 sp=80 roi=3.00 — Document lightning/storage/bandwidth as stubs _Economy/Honesty_
- `ECO-002` **ICEBOX** bp=30 sp=60 roi=2.08 — go test ./economy/ if package builds _Economy/Tests_
- `DOC-001` **DONE** bp=25 sp=100 roi=5.83 — Competitive benchmark MD in 00-Product _DocsCompliance/CPO_
- `DOC-002` **ICEBOX** bp=25 sp=50 roi=2.92 — Verify openapi.yaml still matches auth after fix _DocsCompliance/OpenAPI_
- `DOC-003` **ICEBOX** bp=25 sp=40 roi=3.00 — Link TDD+security scripts in RUNBOOK _DocsCompliance/Runbook_
