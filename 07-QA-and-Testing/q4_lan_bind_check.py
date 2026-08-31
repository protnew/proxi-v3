"""Q4: /api/network/lan bind field matches httpBindAddr (loopback), not hardcoded 0.0.0.0."""
from pathlib import Path
import sys

SRC = Path(__file__).resolve().parents[1]
WS = SRC / "src" / "src-vpn" / "cmd" / "webserver"
fail = 0

def bad(m):
    global fail
    fail += 1
    print("FAIL", m)

def ok(m):
    print("OK", m)

lan = (WS / "lan_info.go").read_text(encoding="utf-8")
st = (WS / "startup.go").read_text(encoding="utf-8")

if 'func httpBindAddr(' not in lan:
    bad("httpBindAddr missing in lan_info.go")
else:
    ok("httpBindAddr defined")

if '"0.0.0.0:" + port' in lan:
    bad("lan_info.go still concatenates 0.0.0.0 bind")
else:
    ok("no 0.0.0.0 bind concat in lan_info.go")

if "httpBindAddr(port)" not in st:
    bad("startup.go Addr not httpBindAddr(port)")
else:
    ok("startup.go uses httpBindAddr")

if '"127.0.0.1:" + port' in st:
    bad("startup.go still inlines 127.0.0.1 bind")
else:
    ok("startup.go no inline bind concat")

if '"reachable_from_lan"' not in lan:
    bad("reachable_from_lan missing")
else:
    ok("reachable_from_lan present")

print("FAIL_COUNT", fail)
sys.exit(0 if fail == 0 else 1)
