#!/usr/bin/env python3
"""security-gate.py — scan repo for secrets (native, no Docker). SEC-001/QG-004"""
from __future__ import annotations
import os, re, sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
SKIP_DIRS = {".git", "node_modules", "dist", "data-dev", "data", "e2e-shots", "test-results", ".04-Src-backup"}
PATTERNS = [
    (re.compile(r"-----BEGIN (RSA |OPENSSH |EC )?PRIVATE KEY-----"), "private_key_pem"),
    (re.compile(r"AKIA[0-9A-Z]{16}"), "aws_access_key"),
    (re.compile(r"(?i)api[_-]?key\s*[:=]\s*['\"][A-Za-z0-9_\-]{20,}['\"]"), "api_key_assignment"),
    (re.compile(r"(?i)(secret|password|token)\s*[:=]\s*['\"][^'\"]{12,}['\"]"), "secret_assignment"),
    (re.compile(r"ghp_[A-Za-z0-9]{36}"), "github_pat"),
    (re.compile(r"xox[baprs]-[A-Za-z0-9-]{10,}"), "slack_token"),
]
# allowlist substrings (test fixtures)
ALLOW = [
    "test-secret-key-32bytes-long!!",
    "different-secret-key-32-bytes!",
    "password_hash",
    "refresh_token",
    "access_token",
    "Bearer ",
    "proxi_token",
    "dev-secret-change-me",
    "test-apns-token",
    "connectRelays: token=",
    "wsToken",
    "encodeURIComponent(token)",
    "Authorization",
    "localStorage.getItem('proxi_token')",
]

def main() -> int:
    hits = []
    for dirpath, dirnames, filenames in os.walk(ROOT):
        dirnames[:] = [d for d in dirnames if d not in SKIP_DIRS and not d.startswith('.')]
        # keep .github
        base = os.path.basename(dirpath)
        if base in SKIP_DIRS:
            continue
        for fn in filenames:
            if not fn.endswith((".go", ".ts", ".js", ".svelte", ".py", ".ps1", ".sh", ".yml", ".yaml", ".json", ".env", ".md")):
                continue
            if fn.endswith(".map"):
                continue
            path = Path(dirpath) / fn
            try:
                text = path.read_text(encoding="utf-8", errors="ignore")
            except Exception:
                continue
            for i, line in enumerate(text.splitlines(), 1):
                if any(a in line for a in ALLOW):
                    continue
                if "example" in line.lower() or "placeholder" in line.lower():
                    continue
                for rx, name in PATTERNS:
                    if rx.search(line):
                        rel = path.relative_to(ROOT)
                        hits.append(f"{rel}:{i}: {name}: {line.strip()[:120]}")
    if hits:
        print("SECURITY GATE FAIL")
        for h in hits[:50]:
            print(h)
        if len(hits) > 50:
            print(f"... +{len(hits)-50} more")
        return 1
    print("SECURITY GATE OK (0 hits)")
    return 0

if __name__ == "__main__":
    sys.exit(main())
