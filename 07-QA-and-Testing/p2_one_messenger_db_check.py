"""P2: one live messenger.db under module data/. No secrets."""
import os, sqlite3, subprocess, sys
from pathlib import Path
VPN = Path(__file__).resolve().parents[1] / 'src' / 'src-vpn'
SRC = Path(__file__).resolve().parents[1]
fail = 0
lives = [p for p in VPN.rglob('messenger.db') if 'archive' not in p.parts]
print('live_count', len(lives))
if len(lives) != 1:
    print('FAIL live_count'); fail += 1
else:
    print('PASS live_count 1')
canon = VPN / 'data' / 'messenger.db'
if not canon.exists():
    print('FAIL canon missing'); fail += 1
else:
    print('PASS canon exists')
if lives and lives[0].resolve() != canon.resolve():
    print('FAIL live is not data/messenger.db', lives[0]); fail += 1
else:
    print('PASS live is data/messenger.db')
cwd1 = VPN / 'messenger.db'
cwd2 = VPN / 'cmd' / 'webserver' / 'messenger.db'
if cwd1.exists() or cwd2.exists():
    print('FAIL cwd copies still present'); fail += 1
else:
    print('PASS cwd copies gone')
con = sqlite3.connect(str(canon))
n = con.execute('select count(*) from messages').fetchone()[0]
con.close()
print('messages', n)
if n != 3:
    print('FAIL messages want 3'); fail += 1
else:
    print('PASS messages 3')
st = (VPN/'cmd'/'webserver'/'startup.go').read_text(encoding='utf-8')
if 'dataDir = "."' in st:
    print('FAIL cwd default remains'); fail += 1
else:
    print('PASS no cwd default')
if 'resolveDBPath()' not in st:
    print('FAIL no resolveDBPath'); fail += 1
else:
    print('PASS resolveDBPath wired')
env = (VPN/'.env.example').read_text(encoding='utf-8')
if 'DB_PATH=proxi.db' in env:
    print('FAIL example still proxi.db'); fail += 1
else:
    print('PASS example not proxi.db')
ps = (SRC/'scripts'/'start-messenger-dev.ps1').read_text(encoding='utf-8')
if 'data-dev' in ps:
    print('FAIL ps1 still data-dev'); fail += 1
else:
    print('PASS ps1 not data-dev')
print('RESULT', 'PASS' if fail==0 else 'FAIL', 'fail_count', fail)
sys.exit(0 if fail==0 else 1)
