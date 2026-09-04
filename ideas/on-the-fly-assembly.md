# On-the-fly assembly of parallel uploads

Status: design note, not implemented. Branch `explore/on-the-fly-assembly`.

## Problem

tus-js-client's `parallelUploads` splits a file into N parts, uploads each as
its own tus upload (`Upload-Concat: partial`), then sends one final POST
(`Upload-Concat: final;<urls>`). Since 58cc134 the final POST returns
immediately and the server assembles the file in the background by appending
every partial into the final file. For a 264 GB upload that is a full extra
read + write pass over the data (minutes to an hour depending on the disk),
during which the file is nowhere visible, plus a second full read if a SHA-256
was supplied. Peak temp disk is file size plus one partial.

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

- tusd `PreUploadCreateCallback` returns `FileInfoChanges`, which may set the
  upload `ID` and `Storage` map. tusd's filestore honours
  `Storage["Path"]` in `NewUpload` as the binary path, and opens it with
  `O_CREATE|O_WRONLY` without truncation, so an existing preallocated file
  survives.
- filestore's `WriteChunk` is `O_APPEND` to the upload's own binary path, so a
  positioned write needs a custom `handler.DataStore` (or a wrapper around
  filestore) for partials. `GetInfo`/`.info` sidecar handling can stay as is.
- We already own the concater (`DeferredConcater`) and the finalizer
  (`finalizeUpload`), so skipping assembly when partials were positioned is a
  local change.
- Space reservation: `fallocate(2)` on Linux (`unix.Fallocate`), `F_PREALLOCATE`
  on macOS (`unix.FcntlFstore`). `golang.org/x/sys` is already an indirect
  dependency. Reservation makes ENOSPC fail at the first PATCH instead of
  after hours of upload, and avoids fragmentation from N interleaved writers.

## Design

### Client

Keep tus-js-client, keep its built-in parallel mode and resume handling, but
give the server what it needs:

1. Generate a `group` id (ULID) per file and compute part boundaries ourselves
   (`parallelUploadBoundaries`), aligned to 4 KiB (see reflink option below).
2. `metadataForPartialUploads = { group, total, boundaries: JSON }` — static,
   which the option supports.
3. The final upload keeps today's metadata (filename, target, formdata, ...)
   plus `group`.

The server then knows the boundary list at partial creation. It still has to
map *this* partial to *one* boundary, and `Upload-Length` is the only per-part
signal. Make it unambiguous: choose boundaries so every part has a distinct
length (equal split, then shift each boundary by its index in 4 KiB units).
Sizes stay within a few KiB of equal, and the server matches
`Upload-Length` against the boundary list exactly. If the match fails the
partial is rejected at creation (400), so a mismatch can never corrupt data.

Alternative without the size trick: replace tus-js-client's built-in parallel
mode with our own orchestration (N `tus.Upload` instances, one per part,
`metadata: { group, index, offset }`). Cleaner protocol, but we would re-own
resume (part URLs in localStorage), pause/resume fan-out, and error
aggregation, which tus-js-client does for us today. Start with the size trick;
move to explicit orchestration only if it proves fragile.

### Server

1. **Pre-create hook, partial with `group`**: validate `total`, boundaries
   (contiguous, sum == total, all distinct lengths, each <= total), and that
   `group` belongs to the calling user (first-come binds `group -> userid` in
   a small table or in-memory map with TTL; a second user on the same group
   is rejected). Match `Upload-Length` to a boundary -> `partOffset`. Set
   `FileInfoChanges.Storage = { Group: <group>, PartOffset: <n> }`.
2. **Group file**: on first partial for a group, create `tempDir/<group>` and
   preallocate `total` bytes. Idempotent; concurrent partials race on
   `O_CREATE|O_EXCL` and the loser just proceeds.
3. **Positioned store**: a `DataStore` wrapper. For uploads whose `Storage`
   has `Group`, `WriteChunk(offset, src)` opens `tempDir/<group>` and writes at
   `PartOffset + offset` (`pwrite` / `Seek` then `io.Copy`), updating the
   partial's `.info` offset exactly as filestore does. No per-partial binary
   file. `Terminate` of a partial does nothing to the group file. Everything
   else delegates to filestore.
4. **Pre-create hook, final with `group`**: set `FileInfoChanges.ID = group`
   so the final upload's binary path *is* the group file (`tempDir/<group>`),
   which is what `finalizeUpload` and `RecoverPending` already expect at
   `tempDir/<id>`. tusd's `sizeOfUploads` still verifies every partial is
   complete before emitting the final upload.
5. **Finalizer**: when the final's `Storage` marks it positioned, skip
   `assembleConcat`; the file is already whole. Run the existing path:
   optional SHA-256 verification, rename into the target, sidecar, webhook,
   `FinalizeUploadStorage`, cleanup of partial `.info` files and rows.
6. **Legacy clients / no group metadata**: fall through to today's
   `assembleConcat` path unchanged, so old tabs mid-upload keep working.

### Failure and recovery

- Client aborts mid-way: group file sits in `tempDir` alongside partial
  `.info` files, same as today's orphaned partials. Existing temp cleanup
  applies; add the group file to it.
- Server restart: partials resume via HEAD on their own URLs (offsets live in
  `.info`). Nothing to recover for the group file itself.
- A corrupt boundary list from a hostile client can at worst write inside its
  *own* group file (offset + length are checked against `total` on every
  `WriteChunk`). It cannot touch another user's group.
- ENOSPC surfaces at preallocation (first PATCH) instead of at assembly.

### What it buys

| | today | positioned |
|---|---|---|
| assembly pass | full read + write | none |
| peak temp disk | size + one partial | size |
| time to "file in target" after last byte | minutes to an hour | SHA-256 pass only (or immediate without hash) |
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

Implement the positioned store with the distinct-length boundary trick. Rough
size: ~300 lines Go (store wrapper, hook validation, preallocate, group
ownership) + ~40 lines TypeScript (boundaries, metadata) + tests around the
store using `net/http/httptest` with tusd's handler. Keep the legacy assembly
path as the fallback. Try the reflink ioctl first only if the server FS turns
out to support it, since it is a fraction of the work.
