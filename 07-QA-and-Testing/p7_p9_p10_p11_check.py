"""P7 HEX gone; P9 one tauri.conf; P10 meta v6; P11 catalog Winner T2."""
import re, sqlite3, sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
SRC = ROOT / ".04-Src"
fail = 0

ng = SRC / "prototypes/pwa-vpn/src/components/NetworkGraph.svelte"
t = ng.read_text(encoding="utf-8")
style = t.split("<style>")[-1] if "<style>" in t else t
if re.search(r"#[0-9a-fA-F]{3,8}\b", style):
    print("FAIL P7 hex remains", re.findall(r"#[0-9a-fA-F]{3,8}\b", style)); fail += 1
else:
    print("PASS P7 no css hex")
if "var(--text-muted)" not in t or "var(--success)" not in t:
    print("FAIL P7 vars missing"); fail += 1
else:
    print("PASS P7 vars")

lives = [p for p in SRC.rglob("tauri.conf.json") if "archive" not in p.parts and "node_modules" not in p.parts]
print("tauri live", len(lives), [str(p.relative_to(SRC)) for p in lives])
if len(lives) != 1:
    print("FAIL P9 live count"); fail += 1
else:
    print("PASS P9 one conf")
want = SRC / "prototypes/pwa-vpn/src-tauri/tauri.conf.json"
if not lives or lives[0].resolve() != want.resolve():
    print("FAIL P9 not pwa conf"); fail += 1
else:
    print("PASS P9 pwa conf")

db = ROOT / "08-Backlog/backlog_proxi_v6.db"
con = sqlite3.connect(str(db))
meta = dict(con.execute("SELECT key,value FROM meta"))
con.close()
print("meta version", meta.get("version"), "path_rule", meta.get("path_rule"))
if meta.get("version") != "v6":
    print("FAIL P10 version"); fail += 1
else:
    print("PASS P10 version v6")
if meta.get("path_rule") != "08-Backlog/backlog_proxi_v6.db":
    print("FAIL P10 path_rule"); fail += 1
else:
    print("PASS P10 path_rule")
zero = ROOT / "08-Backlog/backlog_proxi_full_20260721.db"
if zero.exists():
    print("FAIL P10 zero-byte still live"); fail += 1
else:
    print("PASS P10 zero archived")

cat = (ROOT / "09-Docs/ARCHITECTURE_CATALOG.md").read_text(encoding="utf-8")
row2 = [l for l in cat.splitlines() if l.startswith("| 2 |")]
print("row2", row2[0][:220] if row2 else "MISSING")
if not row2 or "AmneziaWG" not in row2[0] or "195" not in row2[0]:
    print("FAIL P11 winner"); fail += 1
else:
    print("PASS P11 AmneziaWG 195")
if "03-Неубиваемый" in cat:
    print("FAIL P4 regression 03-"); fail += 1
else:
    print("PASS P4 still 0")

print("RESULT", "PASS" if fail==0 else "FAIL", "fail_count", fail)
sys.exit(0 if fail==0 else 1)
