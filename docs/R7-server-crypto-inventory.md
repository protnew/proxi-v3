# R7 server crypto inventory — X2 CONFIRMED 2026-09-23

## Status: FAIL-CLOSED ACTIVE (client-blind)

Server message handlers in `routing_chat.go` no longer call:

| Symbol | Former role | Status |
|--------|-------------|--------|
| `DecryptInbound` (DR) | GET auto-decrypt | **REMOVED** |
| `DecryptMessageFromSender` | GET legacy ECDH decrypt | **REMOVED** |
| `EncryptOutbound` (DR) | POST server encrypt | **REMOVED** |
| `EncryptMessageForRecipient` | POST legacy ECDH encrypt | **REMOVED** |

## Current behavior

### GET `/api/messages`
- Returns rows as stored.
- `Encrypted` flag preserved.
- Client decrypts locally (NIP-44 / DR on device).

### POST `/api/messages`
- Client ciphertext (`LooksLikeClientCiphertext`) → store as-is, `Encrypted=true`.
- `encrypted`/`is_e2e` flag without ciphertext shape → **422 `E2E_FLAG_MISMATCH`**.
- Non-broadcast DM plaintext → **422 `PLAINTEXT_DM_FORBIDDEN`**.
- Broadcast plaintext → OK (unencrypted storage).

## Tests
- `r7_fail_closed_test.go` — string-scan asserts zero forbidden call sites.
- `r7_public_only_bundle_test.go` — hygiene log (kept).

## Out of scope (locks)
- No machineId, no Amnezia, no slog, no group crypto in this wave.
- X4 deferred.
