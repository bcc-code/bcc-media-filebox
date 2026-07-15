---
name: verify
description: Build, run, and drive FileBox locally with an authenticated admin session for end-to-end verification.
---

# Verifying FileBox changes

## Build & run (scratch instance)

```bash
# Full binary with embedded frontend (frontend build needs nvm-managed node):
source ~/.nvm/nvm.sh && nvm use node
cd frontend && pnpm build && cd ..
rm -rf cmd/server/frontend_dist && cp -r frontend/dist cmd/server/frontend_dist
go build -o /tmp/filebox-verify/filebox ./cmd/server
```

Gotchas:
- The shell session exports `TARGET_1_*`/`TARGET_2_*`/`DB_PATH` (and the repo
  has a `.env`); launch with `env -u TARGET_1_NAME -u TARGET_1_DIR -u TARGET_2_NAME -u TARGET_2_DIR`
  and explicit `DB_PATH`/`UPLOAD_DIR` pointing at a scratch dir, from a cwd
  without a `.env`, or the server picks up the dev config.
- zsh does not word-split unquoted vars — don't stuff curl flags into a `$C`
  variable.

## Admin auth without real OIDC

`requireAdmin` needs an OIDC-backed session, but sessions are opaque tokens in
the `sessions` table, and `oidc.NewProvider` only needs a discovery document
at startup:

1. Serve a static discovery doc: write `oidc/.well-known/openid-configuration`
   with `issuer: http://127.0.0.1:9899` (+ authorization/token/jwks endpoints,
   any URLs) and run `python3 -m http.server 9899` from `oidc/`.
2. Launch with `OIDC_BCC_ISSUER=http://127.0.0.1:9899 OIDC_BCC_CLIENT_ID=x
   OIDC_BCC_CLIENT_SECRET=y SESSION_KEY=<32+ chars> PORT=8091
   BASE_URL=http://localhost:8091` (http BASE_URL keeps cookies non-Secure).
3. Seed an admin session after migrations run:
   ```sql
   INSERT INTO users (provider, subject, email, name, role)
     VALUES ('bcc','verify-sub','verify-admin@test.local','Verify Admin','admin');
   INSERT INTO sessions (id, user_id, expires_at)
     VALUES ('verify-session-token-123', last_insert_rowid(), datetime('now','+1 day'));
   ```
4. `curl -b "filebox_session=verify-session-token-123" http://localhost:8091/api/admin/...`

## Driving the UI

Playwright's Chromium is cached at
`~/Library/Caches/ms-playwright/chromium-*/chrome-mac-arm64/Google Chrome for Testing.app/Contents/MacOS/Google Chrome for Testing`.
`npm i playwright-core` in a scratch dir and `chromium.launch({ executablePath })`
— no browser download. Set the session cookie via `ctx.addCookies` before
`page.goto('http://localhost:8091/admin')`.

Watch out: elements inside a `.fb-fade` wrapper have a CSS transform animation
(`fill: both`), which makes them the containing block for `position: fixed`
descendants — modals rendered inside a tab component must be template siblings
of the `.fb-fade` div, not children.
