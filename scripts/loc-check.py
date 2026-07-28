#!/usr/bin/env python3
"""loc-check.py — fail if any prod source file has > 500 lines. QG-005"""
from __future__ import annotations
import os, sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
LIMIT = 500
SKIP = {"node_modules", "dist", ".git", "data-dev", "data", "e2e-shots", "test-results"}
EXTS = {".go", ".ts", ".js", ".svelte"}
# tests can be longer
def is_test(p: Path) -> bool:
    n = p.name
    return "_test.go" in n or n.endswith(".test.ts") or n.endswith(".spec.ts") or n.endswith(".spec.js") or "/test/" in str(p).replace("\\\\", "/")

def main() -> int:
    bad = []
    for dirpath, dirnames, filenames in os.walk(ROOT):
        dirnames[:] = [d for d in dirnames if d not in SKIP and d != "vendor"]
        for fn in filenames:
            p = Path(dirpath) / fn
            if p.suffix not in EXTS:
                continue
            if is_test(p):
                continue
            parts = {x.lower() for x in p.parts}
            if parts & {s.lower() for s in SKIP} or "archive" in parts:
                continue
            try:
                n = sum(1 for _ in open(p, encoding="utf-8", errors="ignore"))
            except Exception:
                continue
            if n > LIMIT:
                bad.append((n, str(p.relative_to(ROOT))))
    bad.sort(reverse=True)
    if bad:
        print(f"LOC GATE FAIL (> {LIMIT} lines):")
        for n, rel in bad:
            print(f"  {n:4d}  {rel}")
        return 1
    print(f"LOC GATE OK (all prod files <= {LIMIT})")
    return 0

if __name__ == "__main__":
    sys.exit(main())
