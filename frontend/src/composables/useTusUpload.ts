import { reactive, ref } from 'vue'
import * as tus from 'tus-js-client'
import { ulid } from 'ulid'
import type { UploadItem, UploadRecord } from '../types'
import { getUserId } from './useUserId'

let idCounter = 0
let detectedParallelUploads: number | null = null

// Upload speed (and therefore ETA) is averaged over this window.
const SPEED_WINDOW_MS = 30_000
// No progress for this long means the transfer was paused or stalled; the
// window is restarted so the dead time doesn't sink the average.
const STALL_RESET_MS = 5_000
// Don't publish a rate from a window shorter than this — too noisy.
const MIN_SPEED_SPAN_MS = 1_000

function sanitizeFilename(name: string): { name: string; error: string | null } {
  if (name === '' || name === '.' || name === '..') {
    return { name: '', error: 'Invalid filename' }
  }
  const chars = Array.from(name)
  let lastDot = -1
  // Start at index 1 so a leading dot is never treated as an extension separator.
  for (let i = chars.length - 1; i >= 1; i--) {
    if (chars[i] === '.') {
      lastDot = i
      break
    }
  }
  let out = ''
  for (let i = 0; i < chars.length; i++) {
    const ch = chars[i]
    if (/^[A-Za-z0-9_-]$/.test(ch)) {
      out += ch
    } else if (ch === '.' && i === lastDot) {
      out += '.'
    } else {
      out += '_'
    }
  }
  return { name: out, error: null }
}

// Part boundaries are aligned to this, which keeps the server's reflink option
// open (FICLONERANGE needs every part but the last to be a block multiple).
const BOUNDARY_ALIGN = 4096

export type Boundary = { start: number; end: number }

// boundariesFor splits a file so that the server can position each part's bytes
// as they arrive, instead of concatenating the parts afterwards.
//
// Two properties matter, and both come from how tus-js-client works:
//
//   - The lengths must all differ. Every part is handed the same metadata
//     object and the same headers (`metadataForPartialUploads` is one static
//     value), so Upload-Length is the only per-part signal the server gets.
//     The split is therefore equal-ish but deliberately uneven: each of the
//     first N-1 parts gets one more alignment unit than the one before, which
//     makes them strictly increasing, and the remainder — the last part — comes
//     out smaller than all of them.
//   - It must be a pure function of (size, partCount). On resume tus-js-client
//     takes the part count from the stored part URLs and recomputes the split
//     from this option, so a different answer would make a resumed part write
//     at the wrong offset.
//
// Returns null when the file is too small to split this way, in which case the
// caller must not use parallel mode.
//
// The Go reference implementation of this same rule is distinctBoundaries() in
// internal/tus/positioned_test.go, which asserts the server accepts what this
// produces. Change the two together.
export function boundariesFor(size: number, partCount: number): Boundary[] | null {
  if (partCount <= 1 || size <= 0 || !Number.isFinite(size)) return null

  const base = Math.floor(Math.floor(size / partCount) / BOUNDARY_ALIGN) * BOUNDARY_ALIGN
  if (base <= 0) return null

  // The first N-1 parts consume (N-1)*base plus one, two, ... alignment units.
  const shift = (BOUNDARY_ALIGN * partCount * (partCount - 1)) / 2
  const last = size - ((partCount - 1) * base + shift)
  // last < base + BOUNDARY_ALIGN is what proves the lengths are distinct: it
  // keeps the remainder strictly below the smallest of the interior parts.
  if (last <= 0 || last >= base + BOUNDARY_ALIGN) return null

  const parts: Boundary[] = []
  let start = 0
  for (let i = 0; i < partCount - 1; i++) {
    const end = start + base + (i + 1) * BOUNDARY_ALIGN
    parts.push({ start, end })
    start = end
  }
  parts.push({ start, end: size })
  return parts
}

// Two things have to survive a page reload for a resumed upload to land in the
// right places:
//
//   - the group id, which names the server-side file every part writes into and
//     which the final upload is created under. A fresh id would abandon the
//     partly-written file and start a second one.
//   - the part count, because tus-js-client takes it from the stored part URLs
//     on resume while the boundaries still come from our option. If the two
//     disagree — the protocol detection can differ between sessions — it would
//     resume a subset of the parts and concatenate the wrong file.
type GroupRecord = { id: string; parts: number }

const GROUP_KEY_PREFIX = 'filebox-upload-group:'

function groupKey(file: File, target: string): string {
  return `${GROUP_KEY_PREFIX}${file.name}/${file.size}/${file.lastModified}/${target}`
}

function loadGroup(file: File, target: string): GroupRecord | null {
  try {
    const raw = localStorage.getItem(groupKey(file, target))
    if (!raw) return null
    const parsed = JSON.parse(raw) as Partial<GroupRecord> | null
    if (typeof parsed?.id === 'string' && parsed.id !== '' && typeof parsed?.parts === 'number' && parsed.parts > 1) {
      return { id: parsed.id, parts: parsed.parts }
    }
  } catch {
    // Blocked storage, or a record written by an older version. Either way,
    // start fresh rather than resuming against a guess.
  }
  return null
}

function saveGroup(file: File, target: string, record: GroupRecord) {
  try {
    localStorage.setItem(groupKey(file, target), JSON.stringify(record))
  } catch {
    // Private mode or blocked storage. tus-js-client's own resume support uses
    // localStorage too, so resume is already unavailable here; the upload still
    // completes, it just cannot be picked up again.
  }
}

function forgetGroup(file: File, target: string) {
  try {
    localStorage.removeItem(groupKey(file, target))
  } catch {
    // Nothing to clean up if we could never write it.
  }
}

async function detectParallelUploads(): Promise<number> {
  if (detectedParallelUploads !== null) return detectedParallelUploads

  try {
    // OPTIONS, not HEAD: tusd's collection endpoint allows only POST, so HEAD
    // 405s noisily. OPTIONS answers 204 and still fills the timing entry.
    await fetch('/files/', { method: 'OPTIONS' })
    const entry = performance
      .getEntriesByType('resource')
      .reverse()
      .find((e) => e.name.includes('/files/')) as PerformanceResourceTiming | undefined
    if (entry?.nextHopProtocol === 'h2' || entry?.nextHopProtocol === 'h3') {
      detectedParallelUploads = 6
    } else {
      // HTTP/1.1: browsers allow ~6 connections per origin, reserve some headroom
      detectedParallelUploads = 3
    }
  } catch {
    detectedParallelUploads = 3
  }

  return detectedParallelUploads
}

export function useTusUpload() {
  const uploads = ref<UploadItem[]>([])

  function addFiles(files: FileList | File[], target: string, formData?: Record<string, string>) {
    for (const file of files) {
      const { name: displayName, error: reason } = sanitizeFilename(file.name)
      const item = reactive<UploadItem>({
        id: `upload-${++idCounter}`,
        file,
        displayName: reason ? file.name : displayName,
        tusUpload: null,
        status: reason ? 'failed' : 'pending',
        progress: 0,
        bytesUploaded: 0,
        bytesTotal: file.size,
        speed: 0,
        error: reason,
        uploadId: null,
        restored: false,
      })
      uploads.value.push(item)
      if (!reason) startUpload(item, target, formData)
    }
  }

  async function startUpload(item: UploadItem, target: string, formData?: Record<string, string>) {
    const file = item.file
    if (!file) return

    // Speed is a rolling average over the last SPEED_WINDOW_MS. Parallel
    // 50 MB chunks make raw progress deltas lumpy, so a single-sample rate
    // swings by an order of magnitude between events.
    let samples: { t: number; bytes: number }[] = []

    // A resumed upload keeps the part count it started with; only a fresh one
    // asks how many connections the protocol affords.
    const saved = loadGroup(file, target)
    const parallel = saved?.parts ?? (await detectParallelUploads())

    // With boundaries the server writes each part straight into its final
    // position, so the upload is done the moment the last byte lands. Without
    // them — a file too small to split unevenly — the server falls back to
    // concatenating the parts afterwards, which is what it did before.
    const boundaries = boundariesFor(file.size, parallel)
    const group = boundaries ? (saved?.id ?? ulid()) : null
    if (group) saveGroup(file, target, { id: group, parts: parallel })
    const userid = getUserId()

    const upload = new tus.Upload(file, {
      endpoint: '/files/',
      chunkSize: 50 * 1024 * 1024,
      parallelUploads: parallel,
      // null is the option's own default: no boundaries, so tus-js-client
      // splits the file evenly and the server concatenates afterwards.
      parallelUploadBoundaries: boundaries,
      retryDelays: [0, 1000, 3000, 5000, 10000],
      removeFingerprintOnSuccess: true,
      // One static object shared by every part — that is all the option
      // supports, and it is why the parts are told apart by length. userid has
      // to be here: without it the server mints a fresh guest id per part and
      // could never check that the whole group belongs to one user.
      metadataForPartialUploads:
        group && boundaries
          ? {
              userid,
              group,
              total: String(file.size),
              boundaries: JSON.stringify(boundaries),
            }
          : {},
      metadata: {
        filename: item.displayName,
        filetype: file.type || 'application/octet-stream',
        userid,
        target: target,
        // The final upload is created under the group id, so the assembled
        // file is already where finalization expects it.
        ...(group ? { group, total: String(file.size) } : {}),
        ...(formData ? { formdata: JSON.stringify(formData) } : {}),
      },
      onProgress(bytesUploaded: number, bytesTotal: number) {
        const now = Date.now()
        const last = samples[samples.length - 1]
        // A gap (pause/resume, stall, retry) would drag the average down for
        // the next 30 s, so start a fresh window instead.
        if (last && now - last.t > STALL_RESET_MS) samples = []
        samples.push({ t: now, bytes: bytesUploaded })
        while (samples.length > 1 && samples[1].t <= now - SPEED_WINDOW_MS) samples.shift()
        const first = samples[0]
        const spanMs = now - first.t
        if (spanMs >= MIN_SPEED_SPAN_MS) {
          item.speed = ((bytesUploaded - first.bytes) * 1000) / spanMs
        }
        item.bytesUploaded = bytesUploaded
        item.bytesTotal = bytesTotal
        item.progress = bytesTotal > 0 ? (bytesUploaded / bytesTotal) * 100 : 0
        item.status = 'uploading'
      },
      onSuccess() {
        item.status = 'completed'
        item.progress = 100
        item.speed = 0
        // The group record only exists to survive a reload mid-upload.
        if (group) forgetGroup(file, target)
        // The URL's last path segment is the server-assigned upload ID. Parsed
        // via pathname, so a query string or hash can't leak into it.
        const segments = upload.url
          ? new URL(upload.url, location.origin).pathname.split('/').filter(Boolean)
          : []
        item.uploadId = segments.length ? segments[segments.length - 1] : null
      },
      onError(error: Error) {
        item.status = 'failed'
        item.error = error.message
        item.speed = 0
      },
    })

    item.tusUpload = upload

    upload.findPreviousUploads().then((previousUploads) => {
      if (previousUploads.length > 0) {
        upload.resumeFromPreviousUpload(previousUploads[0])
      }
      upload.start()
      item.status = 'uploading'
    })
  }

  // Reconstruct a completed upload after the Send form was unmounted or the
  // browser reloaded. Identity is the server upload ID: filename and size are
  // not safe deduplication keys because two distinct files may share both.
  function addServerUpload(record: UploadRecord): boolean {
    if (!record.id || record.status !== 'completed') return false
    if (uploads.value.some((item) => item.uploadId === record.id)) return false

    uploads.value.push(
      reactive<UploadItem>({
        id: `server-${record.id}`,
        file: null,
        displayName: record.filename,
        tusUpload: null,
        status: 'completed',
        progress: 100,
        bytesUploaded: record.size,
        bytesTotal: record.size,
        speed: 0,
        error: null,
        uploadId: record.id,
        restored: true,
      }),
    )
    return true
  }

  function pauseUpload(item: UploadItem) {
    if (item.tusUpload && item.status === 'uploading') {
      item.tusUpload.abort()
      item.status = 'paused'
      item.speed = 0
    }
  }

  function resumeUpload(item: UploadItem) {
    if (item.tusUpload && item.status === 'paused') {
      item.tusUpload.start()
      item.status = 'uploading'
    }
  }

  function retryUpload(item: UploadItem) {
    if (item.tusUpload && item.status === 'failed') {
      item.error = null
      item.tusUpload.start()
      item.status = 'uploading'
    }
  }

  function cancelUpload(item: UploadItem) {
    // Only terminate an upload being abandoned. A completed one is finalized and
    // may already back a package, so aborting it is wrong — use forgetUpload.
    if (item.tusUpload && item.status !== 'completed') {
      item.tusUpload.abort(true).catch(() => {})
    }
    const idx = uploads.value.indexOf(item)
    if (idx !== -1) {
      uploads.value.splice(idx, 1)
    }
  }

  // Drops an upload from the local list only, never contacting the server: for
  // clearing the form once its files are packaged and must stay put.
  function forgetUpload(item: UploadItem) {
    const idx = uploads.value.indexOf(item)
    if (idx !== -1) {
      uploads.value.splice(idx, 1)
    }
  }

  return {
    uploads,
    addFiles,
    pauseUpload,
    resumeUpload,
    retryUpload,
    cancelUpload,
    forgetUpload,
    addServerUpload,
  }
}
