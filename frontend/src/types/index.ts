export interface UploadItem {
  id: string
  file: File
  tusUpload: import('tus-js-client').Upload | null
  status: 'pending' | 'uploading' | 'paused' | 'completed' | 'failed'
  progress: number
  bytesUploaded: number
  bytesTotal: number
  speed: number
  error: string | null
}

export interface UploadRecord {
  id: string
  filename: string
  size: number
  offset: number
  contentType: string | null
  status: string
  createdAt: string
  completedAt: string | null
}
