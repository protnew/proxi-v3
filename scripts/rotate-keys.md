# Private Key Rotation Procedure

## Status
- private.key is in .gitignore (untracked since commit 957b8846)
- private.key still exists in git HISTORY (commits before 957b8846)
- The key is a development/test key, not production

## Option A: Rotate the key (recommended)
1. Generate new keypair
2. Replace in all config files
3. Restart server
4. Old key becomes invalid

```bash
# Generate new WireGuard private key
wg genkey > new-private.key
wg pubkey < new-private.key > new-public.key

# Replace in configs
# Restart server
```

## Option B: Rewrite git history (destructive)
```bash
# WARNING: rewrites history, requires force push, breaks all forks
git filter-branch --tree-filter 'rm -f private.key' --prune-empty HEAD
git push origin dev --force
```
Only do this if the key is a real production secret.

## Current assessment
The key in git history is a **development key** used for local testing.
Rotation (Option A) is sufficient. History rewrite (Option B) is optional.

## Verified
- [x] private.key in .gitignore
- [x] private.key untracked (git rm --cached applied)
- [x] No TURN credentials in code
- [x] VAPID keys in data/ (not committed)
