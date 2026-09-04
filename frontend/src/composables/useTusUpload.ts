import { reactive, ref } from 'vue'
import * as tus from 'tus-js-client'
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
    const parallel = await detectParallelUploads()

    const upload = new tus.Upload(file, {
      endpoint: '/files/',
      chunkSize: 50 * 1024 * 1024,
      parallelUploads: parallel,
      retryDelays: [0, 1000, 3000, 5000, 10000],
      removeFingerprintOnSuccess: true,
      metadata: {
        filename: item.displayName,
        filetype: file.type || 'application/octet-stream',
        userid: getUserId(),
        target: target,
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
