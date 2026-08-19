import { ref } from 'vue'

export type VerificationMethod = 'none' | 'password' | 'bcc_login' | 'email_otp' | 'magic_link'

// Mirrors package_access_requests: a recipient's unanswered ask to reopen a dead
// package. What they need goes in `message`; `reason` is server-derived.
export type AccessRequestReason = '' | 'expired' | 'revoked' | 'limit_reached'

export interface AccessRequest {
  id: string
  email: string
  reason: AccessRequestReason
  message: string
  createdAt: string
}

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
  notifyOnDownload: boolean
  pendingRequests: AccessRequest[]
}

export interface ExtendPackageInput {
  expiresInDays: number
  // Replaces the stored per-file budget rather than adding to it; omit for
  // unlimited. Always send it, or an extension silently lifts the limit.
  maxDownloads?: number
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
  senderName: string
  message: string
  verificationMethod: VerificationMethod
  expiresAt: string
  maxDownloads: number | null
  downloadCount: number
  verified: boolean
  // Whether an access request has to come from one of the addresses the package
  // was mailed to. Only words the request form's copy.
  recipientsOnly: boolean
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

// extendPackage pushes expiry out and resets the download budget, granting every
// outstanding request. Returns the refreshed package, so no re-fetch is needed.
async function extendPackage(packageId: string, input: ExtendPackageInput): Promise<PackageInfo> {
  const updated = await jsonFetch<PackageInfo>(`/api/packages/${encodeURIComponent(packageId)}/extend`, {
    method: 'POST',
    body: JSON.stringify(input),
  })
  const i = packages.value.findIndex((p) => p.packageId === packageId)
  if (i !== -1) packages.value[i] = updated
  return updated
}

async function dismissAccessRequest(packageId: string, requestId: string) {
  await jsonFetch<void>(
    `/api/packages/${encodeURIComponent(packageId)}/access-requests/${encodeURIComponent(requestId)}`,
    { method: 'DELETE' },
  )
  const pkg = packages.value.find((p) => p.packageId === packageId)
  if (pkg) pkg.pendingRequests = pkg.pendingRequests.filter((r) => r.id !== requestId)
}

// Download reports are coalesced server-side, so this only decides whether the
// author hears about the next window — an open one still reports what it has.
async function setNotifyOnDownload(packageId: string, notifyOnDownload: boolean) {
  await jsonFetch<{ notifyOnDownload: boolean }>(`/api/packages/${encodeURIComponent(packageId)}/notify`, {
    method: 'PATCH',
    body: JSON.stringify({ notifyOnDownload }),
  })
  const pkg = packages.value.find((p) => p.packageId === packageId)
  if (pkg) pkg.notifyOnDownload = notifyOnDownload
}

// The opt-out behind the link in a download report. Authorised by the token
// alone, so it works from a mail client with no session.
export async function muteNotifications(token: string): Promise<{ packageName: string }> {
  return jsonFetch<{ packageName: string }>(`/api/notifications/mute/${encodeURIComponent(token)}`, {
    method: 'POST',
  })
}

async function revokePackage(packageId: string) {
  await jsonFetch<void>(`/api/packages/${encodeURIComponent(packageId)}`, { method: 'DELETE' })
  const pkg = packages.value.find((p) => p.packageId === packageId)
  if (pkg) pkg.status = 'revoked'
}

export function usePackages() {
  return {
    packages,
    total,
    loading,
    lastError,
    fetchPackages,
    createPackage,
    revokePackage,
    setNotifyOnDownload,
    extendPackage,
    dismissAccessRequest,
  }
}
