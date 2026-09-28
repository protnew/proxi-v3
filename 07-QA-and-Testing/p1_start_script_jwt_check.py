# p1_start_script_jwt_check.py — P1 AC: script refuses without JWT_SECRET before "API already up"
from __future__ import annotations
import os, subprocess, sys, socket, urllib.request
from pathlib import Path

SRC = Path(__file__).resolve().parents[1]
PS1 = SRC / "scripts" / "start-messenger-dev.ps1"
VPN = SRC / "src" / "src-vpn"
POWERSHELL = r"C:\Windows\System32\WindowsPowerShell\v1.0\powershell.exe"

fail = 0

def fail_add(msg):
    global fail
    fail += 1
    print("FAIL", msg)

def ok(msg):
    print("OK", msg)

# 1 source shape
text = PS1.read_text(encoding="utf-8")
if "Import-DotEnv" not in text:
    fail_add("no Import-DotEnv")
else:
    ok("Import-DotEnv")
if text.find("JWT_SECRET is required") < 0:
    fail_add("no JWT required message")
else:
    ok("JWT required message")
if text.find("JWT_SECRET is required") > text.find("already up"):
    fail_add("JWT check after already-up")
else:
    ok("JWT check before already-up")
if (VPN / ".env").exists():
    # file may exist locally; must be gitignored
    gi = (SRC / ".gitignore").read_text(encoding="utf-8")
    if ".env" not in gi:
        fail_add(".env not gitignored")
    else:
        ok(".env gitignored")
else:
    ok("no .env on disk")

# git must not track .env
r = subprocess.run(["git","ls-files","src/src-vpn/.env",".env"], cwd=str(SRC), capture_output=True, text=True)
if r.stdout.strip():
    fail_add(".env tracked: " + r.stdout.strip())
else:
    ok(".env not tracked")

# 2 physical: no JWT in env, no leaked already-up
env = os.environ.copy()
env.pop("JWT_SECRET", None)
# do not inherit a secret
r = subprocess.run(
    [POWERSHELL, "-NoProfile", "-ExecutionPolicy", "Bypass", "-File", str(PS1)],
    cwd=str(SRC),
    capture_output=True,
    text=True,
    env=env,
    timeout=40,
)
out = (r.stdout or "") + (r.stderr or "")
print("NOJWT_RC", r.returncode)
print("NOJWT_OUT_LEN", len(out))
if r.returncode != 1:
    fail_add("empty JWT expected exit 1 got %s" % r.returncode)
else:
    ok("empty JWT exit 1")
if "API :8090 already up" in out:
    fail_add("printed already-up without JWT")
else:
    ok("no already-up without JWT")
if "JWT_SECRET is required" not in out:
    fail_add("missing required message in stdout")
else:
    ok("required message printed")
# never dump secret: checker itself must not print env value
if "Building Go server" in out:
    fail_add("started build without JWT")
else:
    ok("no build without JWT")

print("FAIL_COUNT", fail)
sys.exit(0 if fail == 0 else 1)
