# FileBox

Resumable file upload platform with per-user tracking, SHA-256 verification, and configurable upload targets.

A Go service that speaks the [TUS resumable upload protocol](https://tus.io/) in front of a small Vue frontend. Uploads are written to a temporary area, verified, and atomically moved into a named target directory. Migrations and the production frontend are embedded in the binary, so deployment is a single static file plus some environment variables.

## Features

- Resumable, chunked uploads via the TUS protocol (`tusd/v2`)
- Client-supplied SHA-256 verified after upload completes
- Per-user upload history tracked in SQLite (duration, bandwidth, offset, status)
- Multiple named upload targets, each bound to a filesystem directory
- Strict filename validation to prevent directory-traversal attacks
- Goose migrations embedded in the binary, applied automatically on startup
- Production frontend embedded in the binary; single-binary deploy

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
filebox.service    # reference systemd unit
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

## Quick start (development)

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

## Production build

```bash
make all           # generate + build frontend + build Go binary -> ./filebox
make build-linux   # cross-compile -> ./filebox-linux-amd64
make generate      # regenerate sqlc code after editing internal/db/queries/
make clean         # remove binaries and frontend dist
```

`make all` bundles `frontend/dist` into the Go binary, so the resulting `./filebox` is self-contained.

## Configuration

All configuration is via environment variables.

| Variable         | Default          | Description                                                                                                   |
| ---------------- | ---------------- | ------------------------------------------------------------------------------------------------------------- |
| `PORT`           | `8080`           | HTTP listen port.                                                                                             |
| `UPLOAD_DIR`     | `uploads`        | Working directory for in-flight TUS uploads. A `.tmp/` subdirectory is created inside it.                     |
| `DB_PATH`        | `filebox.db` | SQLite database file. Opened with WAL and a 5s busy timeout.                                                  |
| `BASE_URL`       | _(empty)_        | Absolute base URL used to build TUS upload URLs when behind a reverse proxy (e.g. `https://upload.example.com`). |
| `TARGET_N_NAME`  | —                | Name of upload target `N` (starting at 1). Referenced by the client via the TUS `target` metadata field.      |
| `TARGET_N_DIR`   | —                | Filesystem directory for target `N`. Must exist and be a directory. Completed uploads are moved here.         |

At least one `TARGET_N_NAME` / `TARGET_N_DIR` pair must be configured. Numbering is contiguous starting at `1`; the loader stops at the first fully empty pair.

Example:

```bash
TARGET_1_NAME=RawMaterial
TARGET_1_DIR=/srv/uploads/raw
TARGET_2_NAME=Processed
TARGET_2_DIR=/srv/uploads/processed
```

## HTTP API

### JSON API

- `GET /api/targets` — returns the configured target names.
- `GET /api/uploads?user_id=<ulid>` — returns upload history for a user, including status, filename, size, offset, SHA-256, duration, bandwidth, and timestamps.

### TUS endpoints

TUS is mounted at `/files/` and follows the standard [TUS 1.0.0 protocol](https://tus.io/protocols/resumable-upload). Use a TUS client library (the frontend uses `tus-js-client`).

The server consumes the following upload metadata fields:

| Field      | Required | Purpose                                                              |
| ---------- | -------- | -------------------------------------------------------------------- |
| `filename` | yes      | Final filename inside the target directory. Validated; see below.    |
| `filetype` | no       | Content-Type stored alongside the upload.                            |
| `userid`   | yes      | Client-generated ULID tying the upload to a user's history.          |
| `sha256`   | yes      | Hex-encoded expected SHA-256. Verified server-side before promotion. |
| `target`   | yes      | Name of a configured target (must match a `TARGET_N_NAME`).          |

When behind a proxy, set `BASE_URL` so the server advertises absolute upload URLs.

## User identity model

There is **no server-side authentication.** The frontend generates a [ULID](https://github.com/ulid/spec) on first visit, stores it in `localStorage`, and sends it as the TUS `userid` metadata on every upload. The backend uses this value as-is to group uploads in the history view.

This is only appropriate for trusted/internal environments. If you expose the service publicly, put it behind an authenticating reverse proxy and/or add real auth at the handler layer.

## Filename and path safety

Filenames are validated twice: in the TUS `PreUploadCreateCallback` (so bad names are rejected before any bytes are accepted) and again immediately before the final rename. The validator rejects — rather than silently strips — any of: empty names, `.` and `..`, NUL bytes, and any path separator (`/` or `\`). Before renaming into a target, the server also recomputes the relative path with `filepath.Rel` and refuses the operation if it escapes the target directory.

## Database and migrations

Migrations live in `internal/db/migrations/` and are embedded into the binary via `go:embed`. On startup the server runs `goose.Up` against the SQLite database, so there is no separate migration step.

Per `CLAUDE.md`:

- **Never modify a committed migration.** Always add a new migration file.
- Run `make generate` after editing anything under `internal/db/queries/` or the schema.

## Deployment

Two reference files ship in the repo:

- `filebox.service` — a hardened systemd unit (`NoNewPrivileges`, `ProtectSystem=strict`, `ProtectHome=true`, explicit `ReadWritePaths`). Adjust `Environment=` lines and `ReadWritePaths=` to match your install.
- `Caddyfile` — a minimal Caddy reverse-proxy config. When running behind a proxy, set `BASE_URL` so TUS advertises the public URL.

A typical deploy is:

1. `make build-linux` locally.
2. Copy `filebox-linux-amd64` to `/usr/local/bin/filebox` on the host.
3. Install the systemd unit, set the target env vars, and ensure the target directories exist and are writable.
4. Front the service with Caddy (or another reverse proxy) and set `BASE_URL` accordingly.
