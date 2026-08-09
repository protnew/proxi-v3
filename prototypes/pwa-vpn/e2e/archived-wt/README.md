# Archived WT/CF E2E Tests

These tests cover the **superseded WebTransport architecture** (table 01 decision:
WebRTC DataChannels won 178/220 over WebTransport). They are kept for reference
but are **not part of the functional test gate**.

## Why archived
- Table 01: WebRTC DataChannels selected as VPN primary transport (178/220)
- Table 58: TURN Fallback — Hybrid Nostr+TURN (153) winner; third-party TURN (77) rejected
- WT requires QUIC/HTTP3 server complexity not justified for P2P product

## Do NOT run these in CI
They will timeout or fail because the WT server path was never production-ready.
