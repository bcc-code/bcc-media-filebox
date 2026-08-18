import { computed, ref } from 'vue'
import type { AccessRequestReason, PackagePreview } from './usePackages'

// The 410 body GetPackagePreview returns for a dead package. Carries the reason
// and sender so the page can offer to ask that person to reopen it.
export interface PackageUnavailable {
  error: string
  reason: AccessRequestReason
  name: string
  senderName: string
  canRequestAccess: boolean
}

// Per-call state, not a singleton like usePackages: the public page and the
// sender's "Recipient preview" tab each get their own.
export function usePackagePreview() {
  const preview = ref<PackagePreview | null>(null)
  const loading = ref(false)
  const error = ref<string | null>(null)
  const verifying = ref(false)
  const verifyError = ref<string | null>(null)
  const unavailable = ref<PackageUnavailable | null>(null)
  const requesting = ref(false)
  const requestError = ref<string | null>(null)
  const requestSent = ref(false)

  async function load(packageId: string) {
    loading.value = true
    error.value = null
    unavailable.value = null
    try {
      const res = await fetch(`/api/packages/${encodeURIComponent(packageId)}/preview`, {
        credentials: 'same-origin',
      })
      if (!res.ok) {
        const body = await res.json().catch(() => null)
        error.value = body?.error || `Request failed (${res.status})`
        // 410 means it existed and stopped working. Anything else leaves this
        // null, so no form is offered for a link that may never have been valid.
        if (res.status === 410 && body?.canRequestAccess) unavailable.value = body as PackageUnavailable
        preview.value = null
        return
      }
      preview.value = await res.json()
    } catch {
      error.value = 'Failed to load package'
      preview.value = null
    } finally {
      loading.value = false
    }
  }

  function recordDownload(shareId: string) {
    const file = preview.value?.files?.find((f) => f.shareId === shareId)
    if (file) file.accessCount++
  }

  // Whether every file has used its budget. Only meaningful once files are
  // loaded and a limit is set; the server still serves the preview in this
  // state, so this is purely a display concern.
  const allDownloadsExhausted = computed(() => {
    const p = preview.value
    if (!p || p.maxDownloads == null || !p.files || p.files.length === 0) return false
    return p.files.every((f) => f.accessCount >= p.maxDownloads!)
  })

  // requestAccess asks the author to reopen the package. Unauthenticated by
  // necessity: whoever needs it is someone the package won't let in.
  async function requestAccess(packageId: string, email: string, message: string) {
    requesting.value = true
    requestError.value = null
    try {
      const res = await fetch(`/api/packages/${encodeURIComponent(packageId)}/access-request`, {
        method: 'POST',
        credentials: 'same-origin',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ email, message }),
      })
      if (!res.ok) {
        const body = await res.json().catch(() => null)
        requestError.value = body?.error || `Request failed (${res.status})`
        return
      }
      requestSent.value = true
    } catch {
      requestError.value = 'Failed to send. Check your connection and try again.'
    } finally {
      requesting.value = false
    }
  }

  async function verifyPassword(packageId: string, password: string) {
    verifying.value = true
    verifyError.value = null
    try {
      const res = await fetch(`/api/packages/${encodeURIComponent(packageId)}/verify`, {
        method: 'POST',
        credentials: 'same-origin',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ password }),
      })
      if (!res.ok) {
        const body = await res.json().catch(() => null)
        verifyError.value = body?.error || `Request failed (${res.status})`
        return
      }
      await load(packageId)
    } catch {
      verifyError.value = 'Failed to verify. Check your connection and try again.'
    } finally {
      verifying.value = false
    }
  }

  return {
    preview,
    loading,
    error,
    unavailable,
    verifying,
    verifyError,
    requesting,
    requestError,
    requestSent,
    load,
    verifyPassword,
    requestAccess,
    recordDownload,
    allDownloadsExhausted,
  }
}
