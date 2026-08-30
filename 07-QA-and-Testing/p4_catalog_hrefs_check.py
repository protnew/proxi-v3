"""P4: catalog hrefs point at live 04 tables, not missing 03- folder."""
import re, sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
# script is .04-Src/07-QA-and-Testing → parents[1]=.04-Src, parents[2]=project
DOCS = ROOT / "09-Docs"
cat = DOCS / "ARCHITECTURE_CATALOG.md"
TABLES = DOCS / "01-Decision-Tables"
fail = 0

if not cat.exists():
    print("FAIL catalog missing"); sys.exit(1)
text = cat.read_text(encoding="utf-8")
n03 = text.count("03-Неубиваемый")
print("03-count", n03)
if n03 != 0:
    print("FAIL 03- still present"); fail += 1
else:
    print("PASS 03-count 0")

hrefs = re.findall(r"\]\(([^)]+)\)", text)
print("hrefs", len(hrefs))
if len(hrefs) < 80:
    print("FAIL too few hrefs"); fail += 1
missing = []
for h in hrefs:
    p = (cat.parent / h.replace("%20", " ")).resolve()
    if not p.exists():
        missing.append(h)
print("missing", len(missing))
if missing:
    print("FAIL missing", missing[:8]); fail += 1
else:
    print("PASS all hrefs exist")

# every href filename is in 01-Decision-Tables
bad_dir = [h for h in hrefs if "01-Decision-Tables/" not in h]
print("not under 01-Decision-Tables", len(bad_dir), bad_dir[:3])
if bad_dir:
    print("FAIL href not under tables dir"); fail += 1
else:
    print("PASS hrefs under 01-Decision-Tables")

print("RESULT", "PASS" if fail == 0 else "FAIL", "fail_count", fail)
sys.exit(0 if fail == 0 else 1)
