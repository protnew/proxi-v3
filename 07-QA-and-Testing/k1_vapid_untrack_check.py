# -*- coding: utf-8 -*-
"""K1: cmd/webserver/data/vapid.json not in git index/HEAD. Do not print file contents."""
import subprocess, sys
from pathlib import Path

SRC = Path(__file__).resolve().parents[1]
REL = "src/src-vpn/cmd/webserver/data/vapid.json"
GI = SRC / ".gitignore"
fails = []

def sh(args):
    r = subprocess.run(args, cwd=str(SRC), capture_output=True, text=True)
    return r.returncode, (r.stdout or "").strip(), (r.stderr or "").strip()

def fail(m):
    fails.append(m)
    print("FAIL", m)

gi = GI.read_text(encoding="utf-8")
if REL not in gi:
    fail("gitignore missing " + REL)
else:
    print("PASS gitignore has path")

rc, out, _ = sh(["git", "ls-files", "--", REL])
if out:
    fail("index still tracks path")
else:
    print("PASS git ls-files empty")

rc, out, _ = sh(["git", "ls-tree", "HEAD", "--", REL])
if out:
    fail("HEAD tree still has path")
else:
    print("PASS git ls-tree HEAD empty")

rc, out, err = sh(["git", "check-ignore", "-v", "--", REL])
if rc != 0 or REL not in out:
    fail("check-ignore did not match")
else:
    print("PASS check-ignore")

# origin: informational — cannot empty without push
rc, out, _ = sh(["git", "ls-tree", "origin/dev", "--", REL])
if out:
    print("WARN origin/dev still tracks path (push blocked)")
else:
    print("PASS origin/dev empty")

# working tree file may remain; never print contents
p = SRC / REL
print("working_tree_exists", p.exists())

print("FAIL_COUNT", len(fails))
sys.exit(1 if fails else 0)
