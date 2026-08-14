import { computed, ref } from 'vue'
import type { PackagePreview } from './usePackages'

// Per-call state (like useTusUpload, unlike the shared usePackages singleton)
// — each viewer of a package's preview (the real public page, and the
// sender's own "Recipient preview" tab) gets its own independent state.
export function usePackagePreview() {
  const preview = ref<PackagePreview | null>(null)
  const loading = ref(false)
  const error = ref<string | null>(null)
  const verifying = ref(false)
  const verifyError = ref<string | null>(null)

  async function load(packageId: string) {
    loading.value = true
    error.value = null
    try {
      const res = await fetch(`/api/packages/${encodeURIComponent(packageId)}/preview`, {
        credentials: 'same-origin',
      })
      if (!res.ok) {
        const body = await res.json().catch(() => null)
        error.value = body?.error || `Request failed (${res.status})`
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

  // Whether every file has already used up its download budget. Only
  // meaningful once files are loaded (post-verification) and a limit is
  // actually set — an unlimited package (maxDownloads === null) can never
  // be "exhausted". The server intentionally keeps serving the preview in
  // this state (see GetPackagePreview), so this is purely a display concern.
  const allDownloadsExhausted = computed(() => {
    const p = preview.value
    if (!p || p.maxDownloads == null || !p.files || p.files.length === 0) return false
    return p.files.every((f) => f.accessCount >= p.maxDownloads!)
  })

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

  return { preview, loading, error, verifying, verifyError, load, verifyPassword, recordDownload, allDownloadsExhausted }
}
