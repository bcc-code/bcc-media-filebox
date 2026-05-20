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
