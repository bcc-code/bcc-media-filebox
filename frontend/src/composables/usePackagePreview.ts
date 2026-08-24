import { computed, onScopeDispose, ref } from 'vue'
import type { AccessRequestReason, PackagePreview, PackageSourceFile } from './usePackages'

const preparationPollIntervalMs = 2_000

// The 410 body GetPackagePreview returns for a dead package. Carries the reason
// and sender so the page can offer to ask that person to reopen it.
export interface PackageUnavailable {
  error: string
  reason: AccessRequestReason
  name: string
  senderName: string
  canRequestAccess: boolean
  recipientsOnly: boolean
  // Only set for the package's own author: what was in it, so they can judge
  // whether it's worth extending without that requiring recipient verification.
  files?: PackageSourceFile[]
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
  let pollTimer: number | null = null
  let loadGeneration = 0
  let disposed = false

  function clearPollTimer() {
    if (pollTimer != null) {
      window.clearTimeout(pollTimer)
      pollTimer = null
    }
  }

  function schedulePreparationPoll(packageId: string, generation: number) {
    clearPollTimer()
    if (disposed || generation !== loadGeneration || preview.value?.preparationStatus !== 'processing') return
    pollTimer = window.setTimeout(() => void fetchPreview(packageId, generation, true), preparationPollIntervalMs)
  }

  async function fetchPreview(packageId: string, generation: number, polling: boolean) {
    if (!polling) {
      loading.value = true
      error.value = null
      unavailable.value = null
    }

    try {
      const res = await fetch(`/api/packages/${encodeURIComponent(packageId)}/preview`, {
        credentials: 'same-origin',
      })
      if (disposed || generation !== loadGeneration) return
      if (!res.ok) {
        const body = await res.json().catch(() => null)
        if (disposed || generation !== loadGeneration) return
        if (polling && res.status >= 500) return
        error.value = body?.error || `Request failed (${res.status})`
        // 410 means it existed and stopped working. Anything else leaves this
        // null, so no form is offered for a link that may never have been valid.
        if (res.status === 410 && body?.canRequestAccess) unavailable.value = body as PackageUnavailable
        preview.value = null
        return
      }
      const nextPreview = (await res.json()) as PackagePreview
      if (disposed || generation !== loadGeneration) return
      preview.value = nextPreview
      error.value = null
      unavailable.value = null
    } catch {
      if (disposed || generation !== loadGeneration) return
      // A transient polling failure should not replace useful progress with an
      // error page. Keep the last response visible and try again.
      if (!polling) {
        error.value = 'Failed to load package'
        preview.value = null
      }
    } finally {
      if (!disposed && generation === loadGeneration) {
        if (!polling) loading.value = false
        schedulePreparationPoll(packageId, generation)
      }
    }
  }

  async function load(packageId: string) {
    clearPollTimer()
    const generation = ++loadGeneration
    await fetchPreview(packageId, generation, false)
  }

  function recordDownload(artifactId: string) {
    const artifact = preview.value?.downloads?.find((item) => item.id === artifactId)
    if (artifact) artifact.accessCount++
  }

  // Download limits apply independently to prepared artifacts, not to source
  // manifest rows. This remains a display concern; the server enforces it.
  const allDownloadsExhausted = computed(() => {
    const p = preview.value
    if (
      !p ||
      p.preparationStatus !== 'ready' ||
      p.maxDownloads == null ||
      !p.downloads ||
      p.downloads.length === 0
    ) {
      return false
    }
    return p.downloads.every((item) => item.accessCount >= p.maxDownloads!)
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

  onScopeDispose(() => {
    disposed = true
    loadGeneration++
    clearPollTimer()
  })

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
