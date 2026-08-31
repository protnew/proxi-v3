# p2_effector_files_archived_check.py
from __future__ import annotations
import json, sys
from pathlib import Path

SRC = Path(__file__).resolve().parents[1]
PWA = SRC / "prototypes" / "pwa-vpn"
LIB = PWA / "src" / "lib"
fail = 0

def bad(m):
    global fail
    fail += 1
    print("FAIL", m)

def ok(m):
    print("OK", m)

if (LIB / "effector.ts").exists():
    bad("live effector.ts still present")
else:
    ok("live effector.ts gone")
if (LIB / "settings.ts").exists():
    bad("live settings.ts still present")
else:
    ok("live settings.ts gone")

arch = list((LIB / "archive").glob("effector-v1-*.ts")) + list((LIB / "archive").glob("settings-v1-*.ts"))
if len(arch) < 2:
    bad("archive missing copies: %s" % arch)
else:
    ok("archive copies %s" % [p.name for p in arch])

pkg = json.loads((PWA / "package.json").read_text(encoding="utf-8"))
deps = {**pkg.get("dependencies", {}), **pkg.get("devDependencies", {})}
if "effector" not in deps:
    bad("effector dropped from package.json (Winner #35 must stay)")
else:
    ok("effector in package.json %s" % deps["effector"])
if "nanostores" in deps:
    bad("nanostores back")
else:
    ok("nanostores absent")

hits = []
for p in (PWA / "src").rglob("*"):
    if p.suffix not in {".ts", ".js", ".svelte"}:
        continue
    if "archive" in p.parts or "-v1-" in p.name:
        continue
    t = p.read_text(encoding="utf-8", errors="replace")
    if "from 'effector'" in t or 'from "effector"' in t or "lib/effector" in t or "lib/settings" in t:
        hits.append(str(p.relative_to(PWA)))
if hits:
    bad("live import of archived modules: %s" % hits)
else:
    ok("no live import of archived effector/settings")

msg = PWA / "src" / "stores" / "messenger.ts"
if not msg.exists():
    bad("messenger.ts missing")
else:
    ok("live UI store messenger.ts present")

print("FAIL_COUNT", fail)
sys.exit(0 if fail == 0 else 1)
