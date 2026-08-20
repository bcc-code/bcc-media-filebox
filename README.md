# FileBox

Resumable file upload platform with per-user tracking, SHA-256 verification, and configurable upload targets.

A Go service that speaks the [TUS resumable upload protocol](https://tus.io/) in front of a small Vue frontend. Uploads are written to a temporary area, verified, and atomically moved into a named target directory. Migrations and the production frontend are embedded in the binary, so deployment is a single static file plus some environment variables.

## Features

- Resumable, chunked uploads via the TUS protocol (`tusd/v2`)
- Client-supplied SHA-256 verified after upload completes
- Per-user upload history tracked in SQLite (duration, bandwidth, offset, status)
- Multiple named upload targets, each bound to a filesystem directory
- Background ZIP64 preparation for large Send packages, with live progress and restart recovery
- Strict filename validation to prevent directory-traversal attacks
- Optional OAuth (OpenID Connect) sign-in with BCC Login and/or Microsoft Entra ID; falls back to guest mode when unconfigured
- Goose migrations embedded in the binary, applied automatically on startup
- Production frontend embedded in the binary; single-binary deploy

## Development

Building from source, running locally, and editing the code are covered in [development.md](development.md).

## Configuration

All configuration is via environment variables.

| Variable         | Default          | Description                                                                                                   |
| ---------------- | ---------------- | ------------------------------------------------------------------------------------------------------------- |
| `PORT`           | `8080`           | HTTP listen port.                                                                                             |
| `UPLOAD_DIR`     | `uploads`        | Working directory for in-flight TUS uploads. A `.tmp/` subdirectory is created inside it.                     |
| `DB_PATH`        | `filebox.db`     | SQLite database file. Opened with WAL and a 5s busy timeout.                                                  |
| `BASE_URL`       | _(empty)_        | Absolute base URL used to build TUS upload URLs and OAuth callback URLs when behind a reverse proxy (e.g. `https://upload.example.com`). |
| `TARGET_N_NAME`  | —                | Name of upload target `N` (starting at 1). Referenced by the client via the TUS `target` metadata field.      |
| `TARGET_N_DIR`   | —                | Filesystem directory for target `N`. Must exist and be a directory. Completed uploads are moved here.         |
| `S3_BUCKET`      | _(empty)_        | When set, Send uploads are stored in this S3 bucket and recipients download via presigned URLs. Unset disables S3; Send then writes to a local target. See [S3 storage for Send](#s3-storage-for-send). |
| `S3_KEY_PREFIX`  | `send/`          | Key prefix for objects written to `S3_BUCKET`. A trailing `/` is added if missing.                             |
| `AWS_REGION`     | —                | Region of `S3_BUCKET`. Required when `S3_BUCKET` is set.                                                      |
| `AWS_ACCESS_KEY_ID` / `AWS_SECRET_ACCESS_KEY` | — | Credentials for the app's IAM user. Optional when running on AWS with an attached instance/task role. |
| `SESSION_KEY`    | —                | 32+ byte secret used for session storage. Required only when at least one OAuth provider is configured.       |
| `BOOTSTRAP_ADMIN_EMAIL` | —         | Optional. On startup, if the `users` table is empty, seeds an admin grant for this email (all targets, admin flag). Ignored once any user has signed in. See [Bootstrapping the first admin](#bootstrapping-the-first-admin). |
| `OIDC_BCC_*` / `OIDC_AZURE_*` | — | See [Authentication](#authentication). All `OIDC_*` variables are optional; OAuth is disabled when none are set. |

At least one `TARGET_N_NAME` / `TARGET_N_DIR` pair must be configured. Numbering is contiguous starting at `1`; the loader stops at the first fully empty pair.

Example:

```bash
TARGET_1_NAME=RawMaterial
TARGET_1_DIR=/srv/uploads/raw
TARGET_2_NAME=Processed
TARGET_2_DIR=/srv/uploads/processed
```

### S3 storage for Send

Files shared through **Send** can be stored in S3 rather than on the server's own disk, so that neither the upload's final resting place nor the recipients' download traffic touches local infrastructure. Set `S3_BUCKET` (plus `AWS_REGION` and credentials) to enable it; leave `S3_BUCKET` unset and Send behaves as before, writing into a local target directory.

How it works when enabled:

- Uploads still arrive over TUS and are assembled in `UPLOAD_DIR/.tmp`, so resumability is unchanged. Once complete, the SHA-256 is verified **before** the transfer, and the file is then streamed to S3 (multipart for large files) and removed from the temp directory.
- Send uploads are tagged with the reserved target name `s3` instead of a configured target. This name is never a row in the `targets` table, so it can't be created, renamed, or deleted from the admin UI.
- Object keys are `<S3_KEY_PREFIX><uploadID>/<filename>`. Namespacing by upload ID means same-named files never collide, and the key is derivable from the `uploads` row — so S3-backed shares need no extra columns.
- Generated ZIPs use `<S3_KEY_PREFIX>packages/<packageID>/artifacts/<artifactID>.zip`. They are streamed with 16 MiB multipart parts, so a 100 GiB non-seekable archive stays below S3's 10,000-part limit without requiring 100 GiB of local staging space.
- `GET /api/artifacts/{id}` performs all package checks (preparation, revocation, expiry, per-artifact download limit, and verification), records the access, and then responds `302` to a presigned S3 URL valid for 5 minutes. Old `/api/shares/{id}` links delegate to this policy path. The bucket itself stays entirely private.

### Send archive policy

All thresholds are binary GiB (`1 GiB = 2^30 bytes`), and the 100 GiB limit applies to the finished ZIP including its headers:

- 10 files or fewer stay as individual downloads.
- More than 10 files whose combined source size is at most 100 GiB are prepared as one ZIP. If ZIP envelope bytes would cross the strict limit, the planner safely splits or leaves an otherwise-unpackable source direct.
- Above 100 GiB total, each source smaller than 10 GiB is packed into ordered ZIP parts no larger than 100 GiB, while sources of 10 GiB or more stay as individual downloads.

Archives use ZIP64 with Store (no compression), which avoids spending CPU recompressing media and supports files over 4 GiB. Duplicate and legacy filenames are made safe and unique inside each archive. Original source objects are retained; package previews continue to list them, while only the planned ZIP/direct artifacts are downloadable. Preparation runs in the background, is restart-safe, and reports byte and percentage progress in both the sender and recipient views. Recipient email is delayed until the complete artifact set is ready. `maxDownloads` is enforced independently per downloadable artifact, so one ZIP download consumes one ZIP allowance.

Completed files selected in the Send form are remembered by server upload ID. Reloading the page restores that exact draft selection without uploading the bytes again. Removing a restored row only removes it from the draft; it does not delete the stored source.

Without S3, generated archives are published atomically under `UPLOAD_DIR/.archives/<packageID>/`.

See [Package archive validation](docs/package-archive-validation.md) for the recorded local end-to-end scenarios, including real 90 GiB and split 96/24 GiB archive downloads and integrity checks.


## HTTP API

### JSON API

- `GET /api/targets` — returns the configured target names.
- `GET /api/uploads` — returns upload history. For authenticated callers the session identifies the user and any `user_id` query parameter is ignored. Guests must pass `user_id=guest:<ulid>` (or a legacy raw ULID); requests asking for an authenticated user's history are rejected with `403`.
- `GET /api/me` — returns `{authenticated: false}` for guests or `{authenticated, userId, provider, email, name}` for signed-in users.
- `GET /auth/providers` — JSON list of configured OIDC providers (`[]` when OAuth is disabled).
- `GET /auth/login/{provider}`, `GET /auth/callback/{provider}`, `POST /auth/logout` — OAuth flow endpoints (browser-driven).

### TUS endpoints

TUS is mounted at `/files/` and follows the standard [TUS 1.0.0 protocol](https://tus.io/protocols/resumable-upload). Use a TUS client library (the frontend uses `tus-js-client`).

The server consumes the following upload metadata fields:

| Field      | Required | Purpose                                                              |
| ---------- | -------- | -------------------------------------------------------------------- |
| `filename` | yes      | Final filename inside the target directory. Validated; see below.    |
| `filetype` | no       | Content-Type stored alongside the upload.                            |
| `userid`   | no       | Hint only — the server overrides this field with the canonical `<provider>:<subject>` for authenticated callers, or `guest:<token>` for guests.          |
| `sha256`   | yes      | Hex-encoded expected SHA-256. Verified server-side before promotion. |
| `target`   | yes      | Name of a configured target (must match a `TARGET_N_NAME`).          |

When behind a proxy, set `BASE_URL` so the server advertises absolute upload URLs.

## Authentication

FileBox supports optional OAuth sign-in via any number of OpenID Connect providers. Sign-in is **never required**: if no providers are configured, the service runs in guest-only mode and behaves like the original anonymous-ULID build.

### Identity formats

Every row in the `uploads.user_id` column carries one of three formats:

| Source              | Value                          |
| ------------------- | ------------------------------ |
| Guest (no session)  | `guest:<random-token>`         |
| BCC Login           | `bcc:<oidc-subject>`           |
| Microsoft Entra ID  | `azure:<oidc-subject>`         |

The TUS `PreUploadCreateCallback` is the chokepoint that enforces this — clients cannot spoof another user's id even by forging the `userid` metadata field. Legacy raw-ULID rows from pre-OAuth deployments remain queryable as guest history.

### Configuring providers

Each provider has its own set of `OIDC_<ID>_*` environment variables. Recognised ids: `BCC`, `AZURE`. Variables for an unconfigured provider can be omitted entirely — providers are detected only when their `ISSUER`, `CLIENT_ID` and `CLIENT_SECRET` are all set.

| Variable                       | Default                | Description                                                  |
| ------------------------------ | ---------------------- | ------------------------------------------------------------ |
| `OIDC_<ID>_ISSUER`             | —                      | OIDC discovery URL (where `/.well-known/openid-configuration` lives). |
| `OIDC_<ID>_CLIENT_ID`          | —                      | OAuth client id registered with the provider.                |
| `OIDC_<ID>_CLIENT_SECRET`      | —                      | OAuth client secret.                                         |
| `OIDC_<ID>_DISPLAY_NAME`       | `BCC Login` / `Microsoft` | Label shown on the sign-in button.                        |
| `OIDC_<ID>_SCOPES`             | `openid profile email` | Space-separated scope list.                                  |

The callback URL each provider must whitelist is `<BASE_URL>/auth/callback/<id>` (e.g. `https://upload.example.com/auth/callback/bcc`). When `BASE_URL` is empty, the server falls back to the request's `Host` header — fine for local development, not for production.

Signed-in sessions are server-side: an httpOnly, SameSite=Lax cookie holds an opaque 32-byte token that maps to a row in the `sessions` table. Cookies are flagged `Secure` automatically when `BASE_URL` starts with `https://`. Sessions live for 30 days; logout (`POST /auth/logout`) clears the FileBox session only — it does not trigger SSO sign-out at the IdP.

### User registration and roles

The first successful sign-in for any `(provider, subject)` pair inserts a row into the `users` table. Subsequent logins refresh `email`, `name`, and `last_login_at`. Roles are derived from the `grants` table — direct user grants and matching group grants — and recomputed on sign-in and on every grant mutation through the admin UI. The current role is returned in `/api/me` and threaded through the `Caller` struct.

**Guests are never admins.** Grants attached to the built-in `All guests` group are rejected with `admin=true` at the admin API, and the role resolver (`computeRoleFor`) refuses to promote any caller whose provider is `guest` regardless of grant configuration. A user who signs up as a guest cannot escalate, and an email that happens to belong to both a guest and an OAuth identity only gains admin through the OAuth identity.

### Bootstrapping the first admin

A fresh deploy has no users and no admin grants, so the admin UI is unreachable. Set `BOOTSTRAP_ADMIN_EMAIL` before the first start to seed one:

```bash
BOOTSTRAP_ADMIN_EMAIL=alice@example.com ./filebox
```

On startup the server:

1. Counts the rows in `users`. **If non-empty, the variable is ignored** — operators should use the admin UI from here on.
2. Inserts a grant `(principal_kind='user', principal_value=<lowercased email>, admin=1, all_targets=1)` if none exists for that email.
3. Logs `BOOTSTRAP_ADMIN_EMAIL: seeded admin grant for <email>`.

The grant takes effect the first time that email signs in via OIDC — `RecomputeRoleForUser` promotes the new user row to `role='admin'`. The email must be one that can sign in via a configured OIDC provider; guest sign-up with the same email will not produce admin access (see above).

The bootstrap is idempotent across restarts: a subsequent restart with the same variable is a no-op once the grant exists, and trivially a no-op once any user has signed in.

### Guest mode

When OAuth is enabled, signing in is still optional: guests upload exactly as before, except their `user_id` is namespaced (`guest:<ulid>`). When OAuth is disabled, the sign-in button is hidden, `/auth/providers` returns `[]`, and the rest of the app is unchanged.

### Trust boundary

OAuth sign-in establishes identity but **does not yet gate access**. Any visitor — guest or authenticated — can list targets and upload. Per-target ACLs (e.g. restricting a target to a particular email domain or OIDC group) are plumbed through the `Caller` type for a follow-up change. Until those rules land, treat the public surface as you would the original anonymous build.

## Deployment

FileBox's SQLite database and background preparation queue assume one active server process per database. Do not overlap instances during a rolling deployment or point multiple replicas at the same DB/storage paths; stop the old process before starting the replacement.

Two reference files ship in the repo:

- `filebox.service` — a hardened systemd unit (`NoNewPrivileges`, `ProtectSystem=strict`, `ProtectHome=true`, explicit `ReadWritePaths`). Adjust `Environment=` lines and `ReadWritePaths=` to match your install.
- `Caddyfile` — a minimal Caddy reverse-proxy config. When running behind a proxy, set `BASE_URL` so TUS advertises the public URL.

A typical deploy is:

1. Build the binary with `make build-linux` (see [development.md](development.md) for build details).
2. Copy `filebox-linux-amd64` to `/usr/local/bin/filebox` on the host.
3. Install the systemd unit, set the target env vars, and ensure the target directories exist and are writable.
4. Front the service with Caddy (or another reverse proxy) and set `BASE_URL` accordingly.
