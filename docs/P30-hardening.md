# P30 hardening
- TURN static-auth-secret must not live in repo (env/secret store).
- VACUUM INTO: validate path under DATA_DIR + retention.
- refresh_token: sha256 only (verify P17; no full rewrite here).
