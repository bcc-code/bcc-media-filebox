# On-the-fly assembly of parallel uploads

Status: design note, not implemented. Branch `explore/on-the-fly-assembly`.
Claims below were re-verified against `tusd/v2 v2.9.2` and `tus-js-client 4.3.1`
after the first draft got three of them wrong; the corrected ones are called out
in place.

## Problem

tus-js-client's `parallelUploads` splits a file into N parts, uploads each as
its own tus upload (`Upload-Concat: partial`), then sends one final POST
(`Upload-Concat: final;<urls>`). Since 58cc134 the final POST returns
immediately and the server assembles the file in the background by appending
every partial into the final file. For a 264 GB upload that is a full extra
read + write pass over the data (minutes to an hour depending on the disk),
during which the file is nowhere visible, plus a second full read if a SHA-256
was supplied — though the browser client sends none today, so that pass does not
actually run. Peak temp disk is file size plus one partial.

Goal: have the bytes land in their final position as they arrive, so that
when the last part finishes there is nothing left to assemble.

## What we know at each point in time

- Client, before starting: total size, part count, exact part boundaries
  (tus-js-client's `splitSizeIntoParts` is deterministic, and
  `parallelUploadBoundaries` lets us dictate them).
- Server, at partial creation: only `Upload-Length` of that partial and
  whatever the client put in `metadataForPartialUploads`. That metadata option
  is a single static object shared by *all* parts, so out of the box a partial
  does not say which part it is or where it belongs.
- Server, at the final POST: the ordered list of partial URLs. Too late for
  positioned writes.

Arrival order of the partial creation POSTs cannot be used to infer the index:
tus-js-client fires them in index order, but with HTTP/2 multiplexing and
concurrent handlers the server sees them in any order.

So the missing piece is purely *which offset does this partial start at*, and
the client is the only party that knows it. Everything else is server plumbing.

## Verified building blocks

Checked against `github.com/tus/tusd/v2 v2.9.2`, the pinned version.

- tusd `PreUploadCreateCallback` returns `FileInfoChanges`, which may set the
  upload `ID`, `MetaData` and `Storage` map (`unrouted_handler.go:375-390` for
  the plain create path, `:537-552` for the concat-final one). `ID` is validated
  as URL-safe by `validateUploadId` (`:1713-1731`), which a ULID passes.
  filestore honours `Storage["Path"]` in `NewUpload` as the binary path,
  absolute as-is or relative to the store root (`filestore.go:89-101`).
- **filestore's `NewUpload` truncates the binary file.** It calls
  `createFile(binPath, ...)` (`filestore.go:110`), which opens
  `O_CREATE|O_WRONLY|O_TRUNC` (`:324`, retried at `:335`) — its own doc comment
  says "If the file already exists, its content is removed." So pointing
  `Storage["Path"]` or a hook-assigned `ID` at a preallocated group file
  **zeroes it**. Any positioned design must keep filestore's `NewUpload` away
  from the group file entirely.
- **filestore's `NewUpload` discards custom `Storage` keys.** It replaces the
  map wholesale with only `Type`/`Path`/`InfoPath` (`filestore.go:103-107`), so
  hook-supplied keys never reach `WriteChunk` or the finalizer. tusd's own docs
  concede `Storage` adjustments are "currently not supported by any data store
  in the tusd package" (`handler/datastore.go:80-81`); `Path` works only because
  filestore happens to read it. Per-upload state must therefore travel in
  `MetaData`, which *is* persisted to the `.info` sidecar and returned by
  `GetInfo`.
- filestore's `WriteChunk` is `O_APPEND` to the upload's own binary path
  (`filestore.go:221`), so a positioned write needs a custom
  `handler.DataStore` (or a wrapper around filestore) for partials.
- **`GetInfo`/`.info` handling cannot stay as is.** filestore's `GetUpload`
  ignores the sidecar's offset and sets `info.Offset = stat.Size()` of the
  binary file, turning a missing binary into `ErrNotFound`
  (`filestore.go:157-166`). A positioned partial has no binary of its own, so
  delegating `GetUpload` gives either a 404 for every partial or — if the path
  points at the group file — an offset equal to the whole group size. Both break
  HEAD-based resume *and* `sizeOfUploads` at the final POST, which is what
  verifies every partial is complete (`unrouted_handler.go:1393-1416`). The
  wrapper must own `GetUpload` and persist each partial's offset itself.
- We already own the concater (`DeferredConcater`, already a no-op) and the
  finalizer (`finalizeUpload`), so skipping assembly when partials were
  positioned is a local change. Order at the final POST is `NewUpload` ->
  `ConcatUploads` -> `emitFinishEvents` (`unrouted_handler.go:420-432`).
- Space reservation: `fallocate(2)` on Linux (`unix.Fallocate`), `F_PREALLOCATE`
  on macOS (`unix.FcntlFstore`). Both are in the pinned
  `golang.org/x/sys v0.47.0`, and `golang.org/x/sys/unix` is already imported
  directly by `internal/tus` (see `rename_noreplace_{linux,darwin}.go`), so
  there is no new dependency. Reservation makes ENOSPC fail at the first PATCH
  instead of after hours of upload, and avoids fragmentation from N interleaved
  writers.
- `filelocker` locks per upload **id**, so N partials writing into one group
  file are not mutually excluded. Correctness rests entirely on their offset
  ranges being disjoint, which makes boundary validation a safety property
  rather than an input check.

## Design

### Client

Keep tus-js-client, keep its built-in parallel mode and resume handling, but
give the server what it needs:

1. Generate a `group` id (ULID) per file and compute part boundaries ourselves
   (`parallelUploadBoundaries`), aligned to 4 KiB (see reflink option below).
2. `metadataForPartialUploads = { group, total, boundaries: JSON, userid }` —
   static, which the option supports. `userid` has to be in there: partials go
   through the same pre-create hook, and for a guest upload
   `resolveUploadUserID` mints a *fresh random* id when the metadata carries
   none, so the group could never be tied to one owner.
3. The final upload keeps today's metadata (filename, target, formdata, ...)
   plus `group`.

The server then knows the boundary list at partial creation. It still has to
map *this* partial to *one* boundary, and `Upload-Length` is the only per-part
signal — tus-js-client passes `metadataForPartialUploads` and `headers` as one
shared object to every part (`lib.esm/upload.js:284-288`), so nothing else
distinguishes them. Make it unambiguous: choose boundaries so every part has a
distinct length (equal split, then shift each boundary by its index in 4 KiB
units). Sizes stay within a few KiB of equal, and the server matches
`Upload-Length` against the boundary list exactly. If the match fails the
partial is rejected at creation (400), so a mismatch can never corrupt data.
Note the default `splitSizeIntoParts` is `floor(total/N)` with the remainder on
the last part (`upload.js:1068-1080`), i.e. N-1 *identical* lengths — the trick
only works with an explicit `parallelUploadBoundaries`.

**Resume constrains both values.** On resume tus-js-client takes the part count
from the stored partial URLs and recomputes the split from the
`parallelUploadBoundaries` option (`upload.js:249-259`). So the boundary
function must be **pure in `(size, partCount)`** — otherwise a resumed part
writes at the wrong offset — and the `group` id must be **stable across page
reloads**, because the final POST carries it and the server keys the group file
by it. A fresh ULID per session would orphan the previous group file and start
a second one. Derive it from the file fingerprint or persist it next to the
part URLs.

Alternative without the size trick: replace tus-js-client's built-in parallel
mode with our own orchestration (N `tus.Upload` instances, one per part,
`metadata: { group, index, offset }`). Cleaner protocol, but we would re-own
resume (part URLs in localStorage), pause/resume fan-out, and error
aggregation, which tus-js-client does for us today. Start with the size trick;
move to explicit orchestration only if it proves fragile.

### Server

Custom per-upload state travels in `MetaData`, not `Storage` — filestore drops
unknown `Storage` keys (see building blocks).

1. **Pre-create hook, partial with `group`**: validate `total`, boundaries
   (contiguous, sum == total, all distinct lengths, each <= total), and that
   `group` belongs to the calling user (first-come binds `group -> userid` in
   an in-memory map with TTL; a second user on the same group is rejected).
   A single-process deployment makes the map sufficient — after a restart a
   re-created partial simply re-binds to the same user — so this needs no
   migration. Match `Upload-Length` to a boundary -> `partOffset`. Add
   `group` and `partOffset` to the `MetaData` the hook already returns.
2. **Group file**: on first partial for a group, create `tempDir/<group>` and
   preallocate `total` bytes. Idempotent; concurrent partials race on
   `O_CREATE|O_EXCL` and the loser just proceeds.
3. **Positioned store**: a `DataStore` wrapper registered as the composer's
   `Core`. For uploads whose `MetaData` has `group`:
   - `NewUpload` must **not** delegate to filestore, which would `O_TRUNC` the
     group file. It ensures/preallocates `tempDir/<group>`, writes the `.info`
     sidecar itself, and returns its own upload bound to the group path.
   - `WriteChunk(offset, src)` writes at `partOffset + offset` (`pwrite` /
     `Seek` then `io.Copy`), bounds-checked against `total` on every call, then
     persists the partial's new offset to its `.info`. No per-partial binary
     file.
   - `GetUpload` must also be owned, reading the offset back from the sidecar:
     filestore derives `Offset` from the binary's size and 404s when it is
     missing, which a positioned partial always is.
   - `Terminate` of a partial does nothing to the group file.
   Everything without `group` delegates to filestore unchanged.
4. **Pre-create hook, final with `group`**: set `FileInfoChanges.ID = group`
   so the final upload's binary path *is* the group file (`tempDir/<group>`),
   which is what `finalizeUpload` and `RecoverPending` already expect at
   `tempDir/<id>`. This is only safe because the wrapper's `NewUpload` handles
   the group case; filestore's would truncate the finished file. tusd's
   `sizeOfUploads` still verifies every partial is complete before emitting the
   final upload.
5. **Finalizer**: when the final's `MetaData` marks it positioned, skip
   `assembleConcat`; the file is already whole. Run the existing path:
   optional SHA-256 verification, rename into the target, sidecar, webhook,
   `FinalizeUploadStorage`, cleanup of partial `.info` files and rows.
6. **Legacy clients / no group metadata**: fall through to today's
   `assembleConcat` path unchanged, so old tabs mid-upload keep working.

### Failure and recovery

- Client aborts mid-way: the group file sits in `tempDir` alongside partial
  `.info` files. **There is no temp cleanup to inherit** — nothing in
  `internal/tus`, `internal/server` or `cmd/server` ever scans `tempDir`, and
  `RecoverPending` works from `uploads` rows filtered `is_partial = 0 AND
  status = 'completed'`, so it never sees an abandoned upload. Today that leaks
  only the bytes actually uploaded; with preallocation the group file is full
  length from the first PATCH, so an abandoned 264 GB upload pins 264 GB
  immediately. **A reaper is a prerequisite, not a follow-up.** It must key off
  the partials' `.info` offsets, because a preallocated group file looks
  complete by size.
- Server restart: partials resume via HEAD on their own URLs (offsets live in
  `.info`, which the wrapper maintains). Nothing to recover for the group file
  itself. `recoveryFileInfo` prefers the sidecar and only overwrites the routing
  metadata keys, so `group` survives; if the sidecar is gone it yields no
  `PartialUploads` either, so the finalizer still treats `tempDir/<id>` as the
  whole file — the right outcome, but by accident, so pin it with a test.
- A corrupt boundary list from a hostile client can at worst write inside its
  *own* group file (offset + length are checked against `total` on every
  `WriteChunk`). It cannot touch another user's group.
- ENOSPC surfaces at preallocation (first PATCH) instead of at assembly.

### What it buys

| | today | positioned |
|---|---|---|
| assembly pass | full read + write | none |
| peak temp disk | size + one partial | size |
| time to "file in target" after last byte | minutes to an hour | immediate (the browser client sends no SHA-256, so no verification pass runs) |
| ENOSPC detection | after full upload | at start |

## Cheaper partial alternatives

1. **Reflink instead of copy** (server only, Linux): on XFS with reflink or
   Btrfs, `ioctl(FICLONERANGE)` splices each partial into the final file at
   its offset without moving data. Needs every partial except the last to be
   a multiple of the FS block size (4 KiB), which the client can guarantee
   with `parallelUploadBoundaries`. Fall back to today's copy when the ioctl
   fails (ext4, macOS). Zero protocol change, but only helps on reflink
   filesystems; check what the production box uses before investing.
2. **Rename the first partial as the base**: assembly starts by renaming
   partial 0 to the final path and appends the rest, saving 1/N of the copy.
   Trivial, but only a marginal win with N = 3..6.
3. **Skip the SHA-256 re-read**: hash incrementally per part during
   `WriteChunk` (positioned store can keep a running hash per part; the final
   hash of a concatenation is *not* derivable from part hashes for SHA-256,
   so this only helps if the client also sends per-part hashes). Probably not
   worth it.

## Recommendation

Implement the positioned store with the distinct-length boundary trick, and the
temp reaper with it. Rough size: ~450 lines Go (store wrapper — note it must own
`NewUpload` *and* `GetUpload`/offset persistence, not just `WriteChunk` — plus
hook validation, preallocate, group ownership, reaper) + ~60 lines TypeScript
(pure boundary function, stable group id, metadata) + tests around the store
using `net/http/httptest` with tusd's handler, for which
`concat_test.go:181-271` is the working template. Keep the legacy assembly path
as the fallback. Try the reflink ioctl first only if the server FS turns out to
support it, since it is a fraction of the work.
