# ClamAV integration

## Context

FileBox accepts uploads from authenticated users and guests and writes them to local target directories. There is currently no antivirus check — once an upload completes and its SHA-256 is verified, the file is moved to its target directory and exposed for download. ClamAV is already installed on the host (clamd running); the idea is to scan every completed upload before it becomes a `completed` upload visible in listings, and quarantine anything that comes back infected.

**Complexity verdict:** small. Roughly:
- One new migration (two columns + index).
- One new Go package wrapping the `clamd` Unix-socket protocol (~80 lines, no external dependency required — protocol is trivial).
- ~30 lines of new logic in `finalizeUpload`.
- Two new env vars (`CLAMD_SOCKET`, `QUARANTINE_DIR`).
- Two new sqlc queries.

Estimated effort: half a day, including tests.

## Approach

- **AV interface:** connect to `clamd` over a **Unix socket** using the `INSTREAM` command. Daemon stays warm, signature DB stays loaded — fast per-scan.
- **Infected files:** **move to a quarantine directory** configured by `QUARANTINE_DIR`. Mark `status='failed'`, `scan_status='infected'`, `scan_result=<threat name>`.
- **Timing:** scan **inline** inside the existing `finalizeUpload` goroutine, after SHA-256 verification, **before** the row is allowed to remain `status='completed'`.

## Changes

### 1. Migration: `internal/db/migrations/00009_add_scan_status.sql`

```sql
-- +goose Up
ALTER TABLE uploads ADD COLUMN scan_status TEXT;     -- NULL | 'scanning' | 'clean' | 'infected' | 'error'
ALTER TABLE uploads ADD COLUMN scan_result TEXT;     -- threat name when infected; error string when 'error'
CREATE INDEX idx_uploads_scan_status ON uploads(scan_status);

-- +goose Down
DROP INDEX IF EXISTS idx_uploads_scan_status;
ALTER TABLE uploads DROP COLUMN scan_result;
ALTER TABLE uploads DROP COLUMN scan_status;
```

No backfill — existing rows stay `scan_status = NULL`, which we treat as "uploaded before scanning was enabled".

### 2. sqlc queries: `internal/db/queries/uploads.sql`

Add:

```sql
-- name: SetScanStatus :exec
UPDATE uploads SET scan_status = ?, scan_result = ? WHERE id = ?;

-- name: FailUploadWithScan :exec
UPDATE uploads SET status = 'failed', scan_status = ?, scan_result = ? WHERE id = ?;
```

Run `make generate` to regenerate Go bindings.

`ListUploads` does **not** need to change — it already filters by `status = 'completed'`, so an infected upload (flipped to `failed`) is automatically hidden.

### 3. New package: `internal/clamav/clamav.go`

A tiny client that speaks clamd's `INSTREAM` protocol over Unix socket. Roughly:

```go
package clamav

type Client struct{ socketPath string }

type Result struct {
    Clean  bool
    Threat string // populated when !Clean
}

func New(socketPath string) *Client { ... }

// ScanFile streams the file at path to clamd via INSTREAM and returns the
// parsed result. Returns an error only on transport/protocol failure;
// a "FOUND" response is reported via Result.Threat with no error.
func (c *Client) ScanFile(ctx context.Context, path string) (Result, error) { ... }

// Ping verifies clamd is reachable at startup (sends "PING", expects "PONG").
func (c *Client) Ping(ctx context.Context) error { ... }
```

Protocol details to implement directly (no third-party library — keeps go.mod clean, the protocol is ~30 lines):
- Connect to Unix socket.
- Write `zINSTREAM\0`.
- Stream the file in chunks: 4-byte big-endian length, then chunk bytes; terminate with 4-byte zero.
- Read response line; parse `stream: OK` (clean) or `stream: <Threat> FOUND` (infected) or `stream: <err> ERROR`.

Defer hardening (timeout, max-scan-size) to config.

### 4. Wire it up: `cmd/server/main.go`

After `godotenv.Load()`, read:
- `CLAMD_SOCKET` (default `/var/run/clamav/clamd.ctl`)
- `QUARANTINE_DIR` (no default — required; fail fast if unset)

Construct `clamav.New(...)`, call `Ping` once at startup. On failure: **log a warning and continue** — we don't want clamd outages to take the upload service down — but flip a feature flag so finalize marks new uploads `scan_status='error'` and leaves them as `status='completed'`. (This keeps prior behavior available as a fallback. If you prefer hard-fail, swap the log for `log.Fatalf` — easy toggle.)

`os.MkdirAll(quarantineDir, 0700)` next to the existing `uploadDir` setup.

Pass the client (and quarantine dir) into the TUS event processor.

### 5. Hook in `internal/tus/hooks.go`

Extend `EventProcessor` to carry `clamClient *clamav.Client` and `quarantineDir string`. Inside `finalizeUpload`, after the SHA-256 block (around line 177), insert:

```go
if dstPath != "" && ep.clamClient != nil {
    ep.queries.SetScanStatus(ctx, db.SetScanStatusParams{
        ID: info.ID, ScanStatus: sql.NullString{"scanning", true},
    })
    res, err := ep.clamClient.ScanFile(ctx, dstPath)
    switch {
    case err != nil:
        log.Printf("clamav scan error for %s: %v", dstPath, err)
        ep.queries.SetScanStatus(ctx, ... "error", err.Error())
        // leave file in place, leave status='completed' — degraded mode
    case !res.Clean:
        log.Printf("INFECTED: %s — threat=%s", dstPath, res.Threat)
        if qpath, qerr := ep.quarantine(dstPath); qerr != nil {
            log.Printf("quarantine failed for %s: %v", dstPath, qerr)
        } else {
            log.Printf("quarantined %s -> %s", dstPath, qpath)
        }
        ep.queries.FailUploadWithScan(ctx, ... "infected", res.Threat)
    default:
        ep.queries.SetScanStatus(ctx, ... "clean", "")
    }
}
```

Add a `quarantine(srcPath string) (string, error)` helper on `EventProcessor`. Reuse the existing patterns from `renameUpload` / `crossDeviceMove`: same-filesystem `os.Rename` first, fall back to copy+remove on `EXDEV`, run the same containment check against `quarantineDir`. Quarantine filename can be `<uploadID>__<sanitized-original-name>` so there's no collision and the operator can trace it back.

### 6. Optional UI nudge

The frontend currently shows uploads from `ListUploads`. Since infected uploads are flipped to `status='failed'`, they vanish from the list automatically — no frontend work strictly required. **Skipping unless asked.**

## Critical files

- `internal/db/migrations/00009_add_scan_status.sql` (new)
- `internal/db/queries/uploads.sql` (add two queries)
- `internal/clamav/clamav.go` (new package)
- `internal/tus/hooks.go` (modify `finalizeUpload`, add `quarantine` helper, extend `EventProcessor` constructor)
- `cmd/server/main.go` (env vars, startup ping, pass client into TUS hooks)
- `internal/server/server.go` (only if `NewEventProcessor` signature changes propagate up — check call site)

## Existing utilities to reuse

- `SanitizeFilename` — already in `internal/tus/hooks.go:338`, reuse for quarantine filenames.
- `crossDeviceMove` — `internal/tus/hooks.go:278`, reuse for moving across filesystems if quarantine is on a different mount.
- Containment check pattern — `internal/tus/hooks.go:241-255`, replicate for the quarantine destination.
- `envOr` — `cmd/server/main.go:115`, reuse for `CLAMD_SOCKET` default.
- `FailUpload` query — already in `uploads.sql:23`, but the new `FailUploadWithScan` is a superset; both can coexist.

## Verification

1. **Unit-ish tests (no clamd required):**
   - `internal/clamav/clamav_test.go`: feed canned `stream: OK\0` / `stream: Eicar-Test-Signature FOUND\0` responses into a fake server bound to a temp Unix socket; assert parsing.
   - `SanitizeFilename` already covered; quarantine path-containment check gets its own test.

2. **Live integration (clamd running):**
   - Drop the EICAR test string (`X5O!P%@AP[4\PZX54(P^)7CC)7}$EICAR-STANDARD-ANTIVIRUS-TEST-FILE!$H+H*`) into a file and upload via the TUS client.
   - Expect: row in DB has `status='failed'`, `scan_status='infected'`, `scan_result='Eicar-Test-Signature'`. File present in `QUARANTINE_DIR`, absent from target dir. `ListUploads` does not return it.
   - Upload a normal file (e.g. a JPEG): row has `status='completed'`, `scan_status='clean'`, file in target dir.

3. **Degraded-mode check:**
   - Stop clamd, restart filebox. Confirm startup logs warning, uploads still complete with `scan_status='error'`.

4. **Crash / restart edge case:**
   - Manually set a row to `scan_status='scanning'` and restart. Document (don't fix in this PR) that there's no resumption — operator action: re-scan via a CLI tool or accept that row stays `scanning`. A small follow-up `filebox scan` admin command is the natural next step, out of scope here.
