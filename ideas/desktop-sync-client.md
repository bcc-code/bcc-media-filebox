# Desktop Sync Client (Windows Service)

Design note — not implemented. A Go companion client that runs as a Windows service, watches a local folder, and uploads new files to a configured FileBox target over tus, with mid-file resume and content-dedup skip. Changed files re-upload whole (server keeps both via `(N)` suffix rename); no block-level delta sync.

The server already speaks tus 1.0 (tusd v2) at `/files/`, so no new transfer protocol is needed. Two small server additions are required: API tokens (PAT) for machine auth, and a sha256 lookup endpoint for dedup/verification.

## A. Server additions

### A.1 API tokens (PAT)

Auth today is OIDC/guest session cookies only — nothing a headless client can use. Add personal access tokens.

**Migration `00016_add_api_tokens.sql`:**

```sql
CREATE TABLE api_tokens (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name TEXT NOT NULL,                 -- user-chosen label, e.g. "edit-bay-3"
    token_hash TEXT NOT NULL UNIQUE,    -- hex(sha256(secret))
    prefix TEXT NOT NULL,               -- first 12 chars, for display
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_used_at DATETIME,
    revoked_at DATETIME
);
CREATE INDEX idx_api_tokens_user_id ON api_tokens(user_id);

-- dedup lookup support (see A.2)
CREATE INDEX idx_uploads_sha256_target ON uploads(sha256, target_name);
```

**Token format & hashing.** `fbx_` + 32-byte base64url (reuse `randomToken` in `internal/auth/manager.go`). Shown to the user exactly once at creation. Only `hex(sha256(secret))` is stored — at 256 bits of entropy a single unsalted SHA-256 is sufficient (no bcrypt needed, and it allows O(1) indexed lookup per request). Lookup = hash the presented bearer value, join to `users`, reject if `revoked_at IS NOT NULL`.

**Issuance / revocation API** (new `internal/api/token_handlers.go` + `internal/db/queries/api_tokens.sql`):

- `POST /api/tokens` `{name}` → `{id, name, token, prefix, createdAt}` — `token` returned only here. Requires an authenticated *session* (refuse creation via PAT auth — no token-mints-token).
- `GET /api/tokens` → caller's tokens (id, name, prefix, createdAt, lastUsedAt).
- `DELETE /api/tokens/{id}` → sets `revoked_at` (own tokens only).

**Web UI:** minimal "API tokens" section (list + create with show-once dialog + revoke), reachable from the user menu.

**Bearer auth hook.** `Server.Handler()` (`internal/server/server.go:212`) wraps the mux in `SessionStore.Middleware`, which resolves the `filebox_session` cookie → `*Caller` (`internal/auth/caller.go`). Extend resolution, not wiring:

1. New `TokenStore.LookupBearer(r)` (`internal/auth/token.go`): parse `Authorization: Bearer fbx_...`, hash, query, build the **same `*Caller` struct** from the joined users row. Because the Caller is identical in shape, `EffectiveTargetIDs`, `/api/targets` filtering, `/api/me`, and admin RBAC all work unchanged.
2. In the middleware: cookie first, then bearer.
3. `resolveUploadUserID` (`internal/server/server.go:141`) re-reads auth from the tus hook's raw headers — add the symmetric bearer branch so PAT uploads get canonical `provider:subject` attribution exactly like session uploads.
4. Throttle `last_used_at` writes (once per N minutes per token, in-memory) — every chunk PATCH passes through the middleware.

**Existing behavior worth noting:** `/files/` is anonymous-allowed today (guest uploads are a feature) and `preUploadCreate` performs no target RBAC — anyone can upload to any target name. The PAT therefore provides *attribution* and authenticates the new lookup endpoint; it does not gate uploads. Whether to start enforcing target RBAC in `preUploadCreate` is an open question (it changes guest behavior).

### A.2 Dedup lookup endpoint

- Route: `GET /api/uploads/lookup?sha256=<hex>&target=<name>`
- Query (`internal/db/queries/uploads.sql`):

```sql
-- name: GetCompletedUploadBySha256Target :one
SELECT id, filename, size, completed_at FROM uploads
WHERE sha256 = ? AND target_name = ? AND status = 'completed' AND is_partial = 0
ORDER BY completed_at DESC LIMIT 1;
```

- Response: `200 {"exists": true, "filename": "...", "size": n, "completedAt": "..."}` or `200 {"exists": false}`.
- Auth: require a Caller (401) and require the target ∈ `EffectiveTargetIDs` (403) — otherwise this is a content-existence oracle across targets.
- Semantics caveat (accepted): answers "was this content ever completed and verified for this target" from the DB; it does not re-stat the file on disk (downstream workflows may move files out of target dirs). Good enough for skip-already-uploaded.
- It also doubles as the client's **completion verification**: server-side sha256 verification runs *asynchronously* in `finalizeUpload` (`internal/tus/hooks.go`) after tus reports complete, so the client polls this endpoint after upload until `exists:true` (verified) or the upload row flips to `failed`.

That's the entire server surface: one migration, one token store + 3 routes, one lookup route, ~10 lines in the middleware and `resolveUploadUserID`.

## B. Client architecture (Go, Windows service)

Lives in this repo as `cmd/sync-client` — shares the filename-sanitization rules with the server by extracting `SanitizeFilename` (currently `internal/tus/hooks.go`) into a small shared package (e.g. `internal/sanitize`).

### Service management

Use **github.com/kardianos/service** rather than raw `golang.org/x/sys/windows/svc`. It wraps SCM registration, start/stop control handlers, and Event Log logging — and, decisively for development, runs the same binary as a plain console process on macOS/Linux.

```
filebox-sync install | uninstall | start | stop | run   (run = console mode)
```

`install` registers "FileBoxSync" with automatic start + failure-restart actions.

### Watching + stability

- **fsnotify** (ReadDirectoryChangesW on Windows) for low-latency discovery, plus a **periodic full rescan** (default 5 min) as the source of truth — ReadDirectoryChangesW drops events on buffer overflow, and the rescan also catches files created while the service was stopped.
- **Quiescence gate** before hashing: (a) size + mtime unchanged for `stable_seconds` (default 10), and (b) an exclusive-open probe — `CreateFile` with `dwShareMode = 0`; a sharing violation means a writer still holds the file (catches slow network copies and apps holding handles). Both must pass.

### tus client

**Hand-rolled minimal tus client** (~200–300 lines): `POST` create (Upload-Length + Upload-Metadata), `HEAD` for offset, `PATCH` chunks, `DELETE` for termination. Rationale: eventials/go-tus is unmaintained, bdragon300/tusgo is low-adoption, and the needed protocol subset is tiny — hand-rolling gives clean `context.Context` support, Bearer header injection, and per-chunk resume without fighting a wrapper. Skip the concatenation extension — parallel connections matter for a browser tab, not a background service; sequential PATCHes saturate typical links.

- Chunk size: 64 MiB PATCHes streamed from the file (bounds retry cost; no full-file buffering).
- Retry: exponential backoff with jitter, 1 s → 5 min cap, unbounded while the source file still exists; every retry starts with `HEAD` to re-learn the server offset (mid-file resume).
- Metadata per upload: `filename` (pre-sanitized), `sha256`, `target`, `filetype`.

### Local state

SQLite (**modernc.org/sqlite** — pure Go, matches the server stack) at `C:\ProgramData\FileBoxSync\state.db`:

```sql
CREATE TABLE files (
    path TEXT PRIMARY KEY,        -- absolute local path
    size INTEGER, mtime INTEGER,  -- as captured at hash time
    sha256 TEXT,
    status TEXT,                  -- discovered|hashing|deduped|uploading|verifying|done|failed
    upload_url TEXT,              -- tus upload URL for resume
    remote_filename TEXT,         -- predicted/actual server-side name
    error TEXT, retry_count INTEGER, updated_at DATETIME
);
```

Recovery after restart/reboot: `uploading` rows → `HEAD upload_url`; if the upload still exists resume from the returned offset, else recreate. `done` rows are skipped unless size/mtime changed on disk → treated as a changed file and re-uploaded whole. `verifying` rows → re-poll the lookup endpoint.

### Config

`C:\ProgramData\FileBoxSync\config.yaml`:

```yaml
server_url: https://filebox.example.org
token: fbx_...            # or token_dpapi after first run, see below
watch_dir: 'D:\Footage\Outbox'
target: raw-material       # must match a FileBox target name
include: ['*.mov', '*.mxf', '*.wav']
exclude: ['*.tmp', '~*']
recursive: true
concurrency: 2             # parallel uploads (hashing stays at 1)
chunk_size_mb: 64
stable_seconds: 10
rescan_interval: 5m
log_level: info
```

### Pipeline

```
discover (fsnotify + rescan) → stability gate → hash (sha256, streaming)
  → dedup check (GET /api/uploads/lookup) ── exists → mark deduped/done
  → create tus upload → PATCH chunks (resume via HEAD on any error)
  → verify: poll lookup until exists (server-side sha256 verified) → done
```

- Hashing is a full read of the file — run **one hasher at a time** (a 500 GB file is a long sequential read either way), uploads at `concurrency` (default 2).
- At startup and on 401s: `GET /api/me` to validate the token, `GET /api/targets` to confirm the configured target is visible/writable.

### Edge cases

- **File deleted/renamed mid-upload:** PATCH source read fails → send tus `DELETE` (best effort, cleans server `.tmp`), mark `failed:removed`. Renames appear as delete+create; the new path re-enters the pipeline and dedup-skips by hash.
- **File changed mid-upload:** re-stat (size, mtime) before each chunk against values captured at hash time; on change, terminate and requeue — the server would fail it anyway (sha256 mismatch in `finalizeUpload`).
- **Name sanitization:** client mirrors the server rules (`[A-Za-z0-9_-]` + one extension dot) to predict the server-side name for logging. Mismatch is cosmetic — dedup is by sha256, and collisions get server-side `(N)` suffixes, never overwrites.
- **Token revoked:** API calls return 401 → pause pipeline, log to Event Log, retry `GET /api/me` on long backoff, resume when a fresh token appears (re-read config on mtime change). Note: the tus endpoint itself will **not** 401 today — without the periodic `/api/me` check a revoked-token client would silently keep uploading with guest attribution.
- **Server unreachable:** everything already sits behind persistent state + backoff; the watch dir is effectively the offline queue.
- **Duplicate filename, different content:** uploads normally; server suffixes. Duplicate content, any name: dedup-skipped.
- **Subdirectories:** recurse and **flatten**, joining the relative path with `_` (`day1/clip.mov` → `day1_clip.mov`) before sanitization — target dirs are flat and sanitized names can't contain separators. True path mirroring would need server-side changes; out of scope.
- **Logging:** kardianos/service Event Log logger for lifecycle/errors + rolling text log in `C:\ProgramData\FileBoxSync\logs\` for per-file detail.

### Token storage security

Pragmatic v1: plaintext token in `config.yaml` with a tight NTFS ACL on `C:\ProgramData\FileBoxSync\` (SYSTEM + Administrators only, set by `install`). Recommended follow-up (not a blocker): on first run, encrypt with DPAPI machine scope, write back as `token_dpapi`, blank the plaintext field. Note this binds the config to the machine.

## C. Open questions

1. Should `preUploadCreate` start enforcing target RBAC (Caller must have the target in `EffectiveTargetIDs`) now that authenticated machine clients exist? Today `/files/` accepts any target from anyone, including guests — tightening changes guest-upload behavior.
2. Token scope: v1 tokens inherit the full grants of the owning user (including admin API access for admins). Do we need target-scoped or role-downgraded tokens before rollout?
3. Token lifetime: revocation-only, or mandatory expiry (e.g. 90 days) with client-visible warnings?
4. Post-upload local file handling: leave files in place (state DB and rescan cost grow with the folder) vs move to an `uploaded/` subfolder after verification (mutates the user's folder)?
5. Form-bound targets (`form_key` set) require valid `formdata` at create; a headless client can't prompt. Restrict the client to non-form targets, or allow a static `formdata` map in config?
