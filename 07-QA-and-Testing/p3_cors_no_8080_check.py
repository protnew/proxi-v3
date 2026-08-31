# p3_cors_no_8080_check.py
from __future__ import annotations
import sys
from pathlib import Path

SRC = Path(__file__).resolve().parents[1]
CORS = SRC / "src" / "src-vpn" / "cmd" / "webserver" / "cors.go"
fail = 0

def bad(m):
    global fail
    fail += 1
    print("FAIL", m)

def ok(m):
    print("OK", m)

t = CORS.read_text(encoding="utf-8")
if "8080" in t:
    bad("8080 still in cors.go")
else:
    ok("8080 removed from cors.go")
if "http://localhost:8090" not in t or "http://127.0.0.1:8090" not in t:
    bad("8090 origins missing")
else:
    ok("8090 origins present")
if "http://localhost:5173" not in t:
    bad("5173 origin missing")
else:
    ok("5173 origin present")
print("FAIL_COUNT", fail)
sys.exit(0 if fail == 0 else 1)
