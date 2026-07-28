# CI (native, no Docker)

Файл `github-actions-ci.yml` — готовый workflow.

Чтобы включить на GitHub:
1. PAT с scope **workflow** (или push через SSH/account с правами)
2. Скопировать:
   `docs/ci/github-actions-ci.yml` → `.github/workflows/ci.yml`
3. git add + push

Локально без GitHub:
```powershell
pwsh scripts/tdd-loop.ps1
# или
bash scripts/tdd-loop.sh
```
