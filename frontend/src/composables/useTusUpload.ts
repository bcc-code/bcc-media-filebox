import { reactive, ref } from 'vue'
import * as tus from 'tus-js-client'
import type { UploadItem } from '../types'
import { getUserId } from './useUserId'

let idCounter = 0
let detectedParallelUploads: number | null = null

async function detectParallelUploads(): Promise<number> {
  if (detectedParallelUploads !== null) return detectedParallelUploads

  try {
    await fetch('/files/', { method: 'HEAD' })
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

  function addFiles(files: FileList | File[], target: string) {
    for (const file of files) {
      const item = reactive<UploadItem>({
        id: `upload-${++idCounter}`,
        file,
        tusUpload: null,
        status: 'pending',
        progress: 0,
        bytesUploaded: 0,
        bytesTotal: file.size,
        speed: 0,
        error: null,
      })
      uploads.value.push(item)
      startUpload(item, target)
    }
  }

  async function startUpload(item: UploadItem, target: string) {
    let lastBytes = 0
    let lastTime = Date.now()
    const parallel = await detectParallelUploads()

    const upload = new tus.Upload(item.file, {
      endpoint: '/files/',
      chunkSize: 50 * 1024 * 1024,
      parallelUploads: parallel,
      retryDelays: [0, 1000, 3000, 5000, 10000],
      removeFingerprintOnSuccess: true,
      metadata: {
        filename: item.file.name,
        filetype: item.file.type || 'application/octet-stream',
        userid: getUserId(),
        target: target,
      },
      onProgress(bytesUploaded: number, bytesTotal: number) {
        const now = Date.now()
        const elapsed = (now - lastTime) / 1000
        if (elapsed > 0.5) {
          item.speed = (bytesUploaded - lastBytes) / elapsed
          lastBytes = bytesUploaded
          lastTime = now
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
    if (item.tusUpload) {
      item.tusUpload.abort(true)
    }
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
  }
}
