import { ref } from 'vue'
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

  return { preview, loading, error, verifying, verifyError, load, verifyPassword }
}
