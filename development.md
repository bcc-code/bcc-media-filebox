# FileBox — Development

Developer guide for working on FileBox. For deployment and operation, see [README.md](README.md).

## Tech stack

- **Backend:** Go 1.26, [`tusd/v2`](https://github.com/tus/tusd), [`pressly/goose`](https://github.com/pressly/goose), [sqlc](https://sqlc.dev/), [`modernc.org/sqlite`](https://pkg.go.dev/modernc.org/sqlite) (pure-Go SQLite driver)
- **Frontend:** Vue 3 + TypeScript, Vite, Tailwind CSS, [`tus-js-client`](https://github.com/tus/tus-js-client)
- **Tooling:** pnpm (frontend), `make`, `sqlc` (only needed when changing SQL)

## Repository layout

```
cmd/server/            # main.go, embedded frontend hook
internal/
  api/                 # JSON API handlers (/api/targets, /api/uploads)
  config/              # TARGET_N_* env var loader
  db/
    gen/               # sqlc-generated query code
    migrations/        # goose SQL migrations (embedded)
    queries/           # hand-written SQL consumed by sqlc
  server/              # HTTP mux, TUS handler wiring
  tus/                 # TUS hooks: finalization, SHA-256 check, filename safety
frontend/src/          # Vue app (composables, components)
Caddyfile              # reference reverse-proxy config
filebox.service        # reference systemd unit
Makefile
```

Key entry points:

- `cmd/server/main.go` — process entry, env vars, DB open, migrations, server start
- `internal/server/server.go` — TUS + API + frontend wiring
- `internal/api/handlers.go` — JSON API
- `internal/tus/hooks.go` — upload lifecycle, filename sanitization, SHA-256 verification, atomic rename into target
- `internal/config/targets.go` — target env var parsing

## Prerequisites

- Go 1.26 or newer
- Node.js with [pnpm](https://pnpm.io/)
- [`sqlc`](https://sqlc.dev/) — only required when modifying SQL in `internal/db/queries/`

Goose is not a separate dependency: migrations ship embedded in the binary and run on startup.

## Local development

The dev workflow uses two processes: the Go server in API-only mode and the Vite dev server for the frontend.

```bash
# 1. Configure at least one upload target (startup fails without one)
export TARGET_1_NAME=RawMaterial
export TARGET_1_DIR="$PWD/uploads-raw"
mkdir -p "$TARGET_1_DIR"

# 2. Backend (API only, no embedded frontend)
make dev

# 3. Frontend (Vite dev server, in another terminal)
make frontend-dev
```

The Vite dev server proxies API and TUS requests to the Go backend. Open the URL it prints.

## Make targets

```bash
make all           # generate + build frontend + build Go binary -> ./filebox
make build-linux   # cross-compile -> ./filebox-linux-amd64
make generate      # regenerate sqlc code after editing internal/db/queries/
make clean         # remove binaries and frontend dist
```

`make all` bundles `frontend/dist` into the Go binary, so the resulting `./filebox` is self-contained.

## Database and migrations

Migrations live in `internal/db/migrations/` and are embedded into the binary via `go:embed`. On startup the server runs `goose.Up` against the SQLite database, so there is no separate migration step.

Per `CLAUDE.md`:

- **Never modify a committed migration.** Always add a new migration file.
- Run `make generate` after editing anything under `internal/db/queries/` or the schema.

## Filename and path safety

Filenames are validated twice: in the TUS `PreUploadCreateCallback` (so bad names are rejected before any bytes are accepted) and again immediately before the final rename. The validator rejects — rather than silently strips — any of: empty names, `.` and `..`, NUL bytes, and any path separator (`/` or `\`). Before renaming into a target, the server also recomputes the relative path with `filepath.Rel` and refuses the operation if it escapes the target directory.

## Email

Transactional mail (share notifications now, expiry-extension requests later) lives in `internal/mail`. Delivery is off unless `MAIL_SMTP_HOST` is set — `NewFromEnv` returns a `NoopSender` that logs the message instead of sending it, so dev and CI never put mail on the wire.

Three headers do three different jobs, and the split is deliberate:

- **Envelope sender** (`MAIL_FROM_ADDRESS`) — fixed service address. SPF checks this against the relay, and bounces return here, so it is never a user's own address.
- **`From:`** — the same service address, with the sharing user's name in the display part: `"John Doe (via FileBox)" <filebox@bcc.no>`. Keeps DMARC alignment while the recipient still sees who shared.
- **`Reply-To:`** — the sharing user. A recipient hitting reply reaches a human, not an unattended mailbox.

### Local testing with Mailpit

```bash
make mailpit   # SMTP on :1025, web UI on http://localhost:8025
```

Point `.env` at it:

```bash
MAIL_SMTP_HOST=localhost
MAIL_SMTP_PORT=1025
MAIL_SMTP_TLS=none
MAIL_FROM_ADDRESS=filebox@localhost
MAIL_LINK_BASE_URL=http://localhost:8091   # where a recipient opens the link
```

`MAIL_LINK_BASE_URL` exists because the two origins differ in dev: `BASE_URL` is the Go server (and the OAuth redirect URI registered with the provider), while a recipient opens the Vite dev server on `:8091`. In production one `BASE_URL` covers both and this can stay unset.

`make dev` builds with the `dev` tag and falls back to `http://localhost:8091` when neither is set, so local testing needs no extra config. Production builds have **no** fallback and refuse to start if mail is configured without an origin — guessing would mail real recipients a link to the wrong host, and a sent mail cannot be recalled.

Creating a package in the UI now mails every recipient. Sending happens in a background goroutine, so package creation never blocks on the relay; the outcome lands on `package_recipients`:

```bash
sqlite3 filebox.db "SELECT email, sent_at, send_error FROM package_recipients ORDER BY id DESC LIMIT 5;"
```

Or send a rendered sample without running the app:

```bash
MAILPIT_SMTP=localhost:1025 go test ./internal/mail/ -run Mailpit -v
```

Mailpit captures everything and delivers nothing onward, so it is safe to point at real-looking addresses. Check both the HTML and plain-text tabs — clients that refuse HTML fall back to the text part.

### The logo

`frontend/public/logo-email.png` is a raster of `AppLogo.vue` (SVG does not render in any mail client), baked to `--ink` `#e7ecf5` and served unauthenticated from the app origin — in production by the embedded frontend, in dev by the Vite server, which is one more reason `MAIL_LINK_BASE_URL` points there. It is referenced as an absolute URL built by `mail.LogoURL`.

Two deliberate properties: `alt=""` (the wordmark beside it already says FileBox, so a blocked image degrades to the wordmark rather than showing the name twice), and `width`/`height` attributes, since Outlook ignores CSS sizing. No other images are used — every one is a blockable request, and the file rows read fine without icons.

Regenerate it after changing `AppLogo.vue`:

```bash
# 45x52 = 2x the 23x26 display size, transparent background
"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome" --headless \
  --default-background-color=00000000 --window-size=45,52 \
  --screenshot=frontend/public/logo-email.png file://<wrapper.html with the SVG at 45x52>
```

### Templates

`internal/mail/templates/*.tmpl`, embedded via `go:embed`, one `.html.tmpl` + `.txt.tmpl` pair per mail. The HTML pair renders through `html/template`, so sender-supplied text is escaped; the text pair uses `text/template`.

The share notification mirrors the recipient page it links to — the `.public-card` block in `PackageDownloadScreen.vue` — down to the wording ("<name> sent you a package"), the file list with `fmtBytes` sizes, and the primary button.

Two constraints shape the markup:

- **Tables and inline styles.** Outlook ignores `<style>` blocks and Gmail strips external CSS, so every colour and spacing value is inline. Padding goes on a `<td>`, never on a `<table>` — Outlook's Word engine drops the latter.
- **Design tokens are hard-coded hex.** No mail client understands CSS custom properties or `oklch()`, so `assets/send.css` tokens are resolved to hex and listed in a comment at the top of the HTML template. **Changing a token in `send.css` does not update the mail** — update both, and `TestNotificationMatchesRecipientPageDesign` will tell you if the hex drifts out of the template.

Preview the rendered design in a browser without a mail server:

```bash
MAIL_PREVIEW_DIR=/tmp/fb go test ./internal/mail/ -run Preview && open /tmp/fb/share.html
```
