"""P1: identity.json must not be in origin/dev tree. No secret contents."""
import subprocess, sys
from pathlib import Path
SRC = Path(__file__).resolve().parents[1]
def sh(cmd):
    r = subprocess.run(cmd, shell=True, cwd=str(SRC), capture_output=True, text=True)
    return r.returncode, (r.stdout or '').strip(), (r.stderr or '').strip()
fail = 0
rc, _, err = sh('git fetch origin dev')
print('fetch_rc', rc)
if rc != 0:
    print('FAIL fetch', err[:200]); fail += 1
rc, out, _ = sh('git ls-tree origin/dev src/src-vpn/data/identity.json')
print('ls-tree empty', out == '')
if out:
    print('FAIL tree still has path'); fail += 1
else:
    print('PASS ls-tree empty')
rc, gi, _ = sh('git show origin/dev:.gitignore')
if 'src/src-vpn/data/identity.json' not in gi:
    print('FAIL gitignore missing path'); fail += 1
else:
    print('PASS gitignore has path')
rc, idx, _ = sh('git ls-files src/src-vpn/data/identity.json')
if idx:
    print('FAIL local index still tracks'); fail += 1
else:
    print('PASS local index empty')
print('RESULT', 'PASS' if fail == 0 else 'FAIL', 'fail_count', fail)
sys.exit(0 if fail == 0 else 1)
