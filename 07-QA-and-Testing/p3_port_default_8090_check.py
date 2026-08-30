"""P3: empty PORT listens 8090, Vite proxy /api/health 200. No secrets."""
import os, socket, sys, urllib.request, urllib.error
from pathlib import Path

VPN = Path(__file__).resolve().parents[1] / "src" / "src-vpn"
fail = 0

def port_open(p):
    s = socket.socket(); s.settimeout(0.4)
    try:
        s.connect(("127.0.0.1", p)); s.close(); return True
    except Exception:
        return False

def curl(url):
    try:
        with urllib.request.urlopen(url, timeout=5) as resp:
            body = resp.read().decode("utf-8", "replace")
            return resp.status, body
    except Exception as e:
        return None, str(e)

st = (VPN / "cmd" / "webserver" / "startup.go").read_text(encoding="utf-8")
if 'port = "8080"' in st:
    print("FAIL startup default 8080"); fail += 1
else:
    print("PASS startup no 8080 default")
if "resolvePort()" not in st:
    print("FAIL resolvePort not wired"); fail += 1
else:
    print("PASS resolvePort wired")
pg = (VPN / "cmd" / "webserver" / "port.go").read_text(encoding="utf-8")
if '"8090"' not in pg:
    print("FAIL port.go missing 8090"); fail += 1
else:
    print("PASS port.go 8090")

print("listen 8090", port_open(8090), "8080", port_open(8080), "5173", port_open(5173))
if not port_open(8090):
    print("FAIL 8090 down"); fail += 1
else:
    print("PASS 8090 up")
if port_open(8080):
    print("FAIL 8080 still listening"); fail += 1
else:
    print("PASS 8080 down")

s, b = curl("http://127.0.0.1:8090/api/health")
print("8090 health", s, b[:120].replace("\n", " "))
if s != 200 or '"status":"ok"' not in b:
    print("FAIL 8090 health"); fail += 1
else:
    print("PASS 8090 health 200")

s2, b2 = curl("http://127.0.0.1:5173/api/health")
print("5173 health", s2, (b2 or "")[:120].replace("\n", " "))
if s2 != 200 or '"status":"ok"' not in (b2 or ""):
    print("FAIL 5173 health"); fail += 1
else:
    print("PASS 5173 health 200")

print("RESULT", "PASS" if fail == 0 else "FAIL", "fail_count", fail)
sys.exit(0 if fail == 0 else 1)
