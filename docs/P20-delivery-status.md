# P20 — delivery status machine

Canonical: pending (hourglass) -> sent (check) -> delivered/read (double-check) -> failed+retry (warn)

PWA outbox MUST stay pending until ACK — never instant double-check.
