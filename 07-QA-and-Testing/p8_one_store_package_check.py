"""P8: one UI store package = effector. nanostores gone from package.json and live src."""
import json, sys
from pathlib import Path

PWA = Path(__file__).resolve().parents[1] / "prototypes" / "pwa-vpn"
fail = 0
pkg = json.loads((PWA / "package.json").read_text(encoding="utf-8"))
deps = {**pkg.get("dependencies", {}), **pkg.get("devDependencies", {})}
print("effector", deps.get("effector"))
print("nanostores", deps.get("nanostores"))
print("@nanostores/persistent", deps.get("@nanostores/persistent"))
if "effector" not in deps:
    print("FAIL effector missing"); fail += 1
else:
    print("PASS effector kept")
if "nanostores" in deps or "@nanostores/persistent" in deps:
    print("FAIL nanostores still in package.json"); fail += 1
else:
    print("PASS nanostores not in package.json")

live_nano = []
for p in (PWA / "src").rglob("*"):
    if p.suffix not in {".ts", ".js", ".svelte"}:
        continue
    if "archive" in p.parts or "-v1-" in p.name:
        continue
    t = p.read_text(encoding="utf-8", errors="replace")
    if "nanostores" in t or "@nanostores" in t:
        live_nano.append(str(p.relative_to(PWA)))
print("live nanostores imports", live_nano)
if live_nano:
    print("FAIL live imports"); fail += 1
else:
    print("PASS no live nanostores imports")

# live unused nano files gone
for name in ["chats.ts", "profile.ts", "vpn.ts"]:
    p = PWA / "src" / "stores" / name
    if p.exists():
        print("FAIL live", name); fail += 1
    else:
        print("PASS archived", name)

eff = PWA / "src" / "lib" / "effector.ts"
if not eff.exists():
    print("FAIL effector.ts missing"); fail += 1
else:
    print("PASS effector.ts present")

nm_nano = PWA / "node_modules" / "nanostores"
if nm_nano.exists():
    print("FAIL node_modules/nanostores still there"); fail += 1
else:
    print("PASS node_modules/nanostores gone")

print("RESULT", "PASS" if fail == 0 else "FAIL", "fail_count", fail)
sys.exit(0 if fail == 0 else 1)
