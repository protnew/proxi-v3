# R7 — server auto-decrypt/encrypt inventory (2026-09-22)

**Do not expand** DecryptMessageFromSender(..., IdentityKey, ...).

## Call sites
```
C:\Obsidian\New\Projects\04-Неубиваемый-контент V2\.04-Src\src\src-vpn\cmd\webserver\routing_chat.go:76:pt, used, err := s.drSessions.DecryptInbound(currentUserNpub, m.From, m.Text)
C:\Obsidian\New\Projects\04-Неубиваемый-контент V2\.04-Src\src\src-vpn\cmd\webserver\routing_chat.go:89:plaintext, err := chat.DecryptMessageFromSender(m.Text, recipientBundle.IdentityKey, m.From)
C:\Obsidian\New\Projects\04-Неубиваемый-контент V2\.04-Src\src\src-vpn\cmd\webserver\routing_chat.go:178:ct, usedDR, derr := s.drSessions.EncryptOutbound(req.From, req.To, req.Text)
C:\Obsidian\New\Projects\04-Неубиваемый-контент V2\.04-Src\src\src-vpn\cmd\webserver\routing_chat.go:193:enc, err := chat.EncryptMessageForRecipient(req.Text, senderBundle.IdentityKey, req.To)
```

## Policy
- Until X2 decision: leave dead path; annotate; no new call sites.
- Full deletion / fail-closed only after Alexey yes on X2.
- Public-only prekey bundles: server must not require recipient privkey.
