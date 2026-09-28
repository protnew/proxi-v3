# SQL Danger Scan Report

_Generated: 2026-07-31T22:53:21Z_

## Summary

- Patterns: 5
- Files scanned: Go + TS/Svelte
- Hits: **1**

## Hits

| File | Line | Pattern | Context |
|------|------|---------|--------|
| store\migration.go | 314 | DROP TABLE | `DROP TABLE IF EXISTS messages_fts_testfts5;` |