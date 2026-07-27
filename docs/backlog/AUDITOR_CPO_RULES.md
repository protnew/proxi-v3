# CPO + Supreme Auditor — Rules (Proxi)

## Source of truth
- **Active DB (WAL):** `08-Backlog/backlog_proxi_v3.db`
- **Dashboard:** `08-Backlog/backlog_dashboard_v1.html`
- **Branch:** `dev`
- **Archive:** `10-Archive/backlogs/`

## Lead Worker queue SQL
```sql
SELECT * FROM tasks
WHERE status='TODO'
ORDER BY block_priority DESC, subblock_priority DESC, priority_score DESC
LIMIT 1;
```
`priority_score` = `user_value / agent_minutes` (ROI).

## Definition of Ready (leaf)
- agent_minutes **10–15** (hard 5–20)
- Hard acceptance_criteria
- depends_on_task_id DAG when needed
- tenant_id: backend|frontend|infra|mobile

## Definition of Done
- status DONE only with proof_type in (autotest, e2e, api) [+manual ok]
- test_logs = real stdout
- 10-POV notes in auditor_pov when closing critical path
- commit + push `dev`

## Status enum
ICEBOX | TODO | IN_PROGRESS | REVIEW | DONE | BLOCKED

## Forbidden
- Parallel active backlog DB
- Fake DONE (keyword/build/exists)
- Files ≥500 LOC
- Direct ALTER TABLE (use migrations/)
- File-copy backup of live WAL DB (use sqlite3.backup / .dump)

## Swarm roles
CPO, BA, Value Appraiser, Architect, DBA, UX Dashboard, Lead Worker, Parallel Worker, QA, Security Auditor
