export interface TargetInfo {
  name: string
  formKey: string | null
}

export interface UploadItem {
  id: string
  // Restored Send-draft entries already live on the server, so the browser no
  // longer has (or needs) their original File object.
  file: File | null
  displayName: string
  tusUpload: import('tus-js-client').Upload | null
  status: 'pending' | 'uploading' | 'paused' | 'completed' | 'failed'
  progress: number
  bytesUploaded: number
  bytesTotal: number
  speed: number
  error: string | null
  // Server-assigned upload ID, set once the tus upload completes. Needed to
  // reference this file elsewhere (e.g. bundling it into a Send package).
  uploadId: string | null
  // True when this row was reconstructed from GET /api/uploads rather than
  // selected from the browser during the current component lifetime.
  restored: boolean
}

export interface UploadRecord {
  id: string
  filename: string
  size: number
  offset: number
  contentType: string | null
  status: string
  durationMs: number | null
  avgBandwidth: number | null
  sha256: string | null
  createdAt: string
  completedAt: string | null
}
