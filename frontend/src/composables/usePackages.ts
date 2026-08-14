import { ref } from 'vue'

export type VerificationMethod = 'none' | 'password' | 'bcc_login' | 'email_otp' | 'magic_link'

export interface PackageInfo {
  packageId: string
  name: string
  message: string
  verificationMethod: VerificationMethod
  recipients: string[]
  fileCount: number
  totalSize: number
  downloadCount: number
  maxDownloads: number | null
  expiresAt: string
  isExpired: boolean
  isDownloadLimitHit: boolean
  status: string
  createdAt: string
}

export interface CreatePackageInput {
  name: string
  message?: string
  uploadIds: string[]
  recipients?: string[]
  expiresInDays: number
  maxDownloads?: number
  verificationMethod?: VerificationMethod
  password?: string
  notifyOnDownload?: boolean
}

export interface CreatePackageResult {
  packageId: string
  packageUrl: string
  expiresAt: string
}

export interface PackageFile {
  shareId: string
  filename: string
  size: number
  accessCount: number
}

export interface PackagePreview {
  name: string
  message: string
  verificationMethod: VerificationMethod
  expiresAt: string
  maxDownloads: number | null
  downloadCount: number
  verified: boolean
  files?: PackageFile[]
}

async function jsonFetch<T>(input: string, init?: RequestInit): Promise<T> {
  const res = await fetch(input, {
    credentials: 'same-origin',
    ...init,
    headers: { 'Content-Type': 'application/json', ...init?.headers },
  })
  if (!res.ok) {
    let msg = `Request failed (${res.status})`
    try {
      const body = await res.json()
      if (body?.error) msg = body.error
    } catch {
      /* ignore */
    }
    throw new Error(msg)
  }
  if (res.status === 204) return undefined as T
  return res.json() as Promise<T>
}

const packages = ref<PackageInfo[]>([])
const total = ref(0)
const loading = ref(false)
const lastError = ref<string | null>(null)

async function fetchPackages(page = 1, pageSize = 20) {
  loading.value = true
  lastError.value = null
  try {
    const data = await jsonFetch<{ packages: PackageInfo[]; page: number; pageSize: number; total: number }>(
      `/api/packages?page=${page}&pageSize=${pageSize}`,
    )
    packages.value = data.packages
    total.value = data.total
  } catch (e) {
    lastError.value = (e as Error).message
  } finally {
    loading.value = false
  }
}

async function createPackage(input: CreatePackageInput): Promise<CreatePackageResult> {
  return jsonFetch<CreatePackageResult>('/api/packages', {
    method: 'POST',
    body: JSON.stringify(input),
  })
}

async function revokePackage(packageId: string) {
  await jsonFetch<void>(`/api/packages/${encodeURIComponent(packageId)}`, { method: 'DELETE' })
  const pkg = packages.value.find((p) => p.packageId === packageId)
  if (pkg) pkg.status = 'revoked'
}

export function usePackages() {
  return { packages, total, loading, lastError, fetchPackages, createPackage, revokePackage }
}
