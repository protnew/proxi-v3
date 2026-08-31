# -*- coding: utf-8 -*-
"""P3 leftover: loopback CORS only 5173/5174/4173/8090, not 8080."""
import os, sys, re
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
CORS = ROOT / "src" / "src-vpn" / "cmd" / "webserver" / "cors.go"
TEST = ROOT / "src" / "src-vpn" / "cmd" / "webserver" / "cors_lan_test.go"
fails = []

def fail(m):
    fails.append(m)
    print("FAIL", m)

text = CORS.read_text(encoding="utf-8")
if "localhost:8080" in text or "127.0.0.1:8080" in text:
    fail("cors.go still lists 8080")
if 'host == "localhost"' not in text:
    fail("loopback host check missing")
if 'case "5173", "5174", "4173", "8090"' not in text and 'case "5173"' not in text:
    fail("loopback port switch missing")
# default return true for any localhost must be gone
m = re.search(r'host == "localhost".{0,400}', text, re.S)
if m and "return true" in m.group(0) and "switch" not in m.group(0):
    fail("localhost still return true without port switch")

tt = TEST.read_text(encoding="utf-8")
if '"http://localhost:8080"' not in tt:
    fail("test missing localhost:8080")
if "false" not in tt:
    fail("test has no false cases")

print("FAIL_COUNT", len(fails))
sys.exit(1 if fails else 0)
