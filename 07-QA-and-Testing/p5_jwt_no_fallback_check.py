"""P5: no JWT fallback literal; empty env refuses start. Do not print secrets."""
import os, subprocess, sys
from pathlib import Path

VPN = Path(__file__).resolve().parents[1] / "src" / "src-vpn"
WS = VPN / "cmd" / "webserver"
fail = 0
LIT = "dev-secret-change-me"
for p in VPN.rglob("*.go"):
    if "archive" in p.parts:
        continue
    if LIT in p.read_text(encoding="utf-8", errors="replace"):
        print("FAIL literal in", p.name); fail += 1
if fail == 0:
    print("PASS literal absent in live go")
st = (WS / "startup.go").read_text(encoding="utf-8")
if "requireJWTSecret()" not in st:
    print("FAIL not wired"); fail += 1
else:
    print("PASS requireJWTSecret wired")
print("RESULT", "PASS" if fail == 0 else "FAIL", "fail_count", fail)
sys.exit(0 if fail == 0 else 1)
