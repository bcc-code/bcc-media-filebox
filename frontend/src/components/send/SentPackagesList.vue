<script setup lang="ts">
import { ref } from 'vue'
import { usePackages } from '../../composables/usePackages'
import SentPackageCard from './SentPackageCard.vue'

const emit = defineEmits<{ preview: [packageId: string] }>()

const { packages, loading, revokePackage, extendPackage, dismissAccessRequest } = usePackages()
// Per-package so one slow extend doesn't disable every other card's button.
const extendingId = ref<string | null>(null)
const toast = ref('')
let toastTimer: number | null = null

function flash(text: string) {
  toast.value = text
  if (toastTimer) window.clearTimeout(toastTimer)
  toastTimer = window.setTimeout(() => (toast.value = ''), 2200)
}

function copyLink(packageId: string) {
  const url = `${location.origin}/s/${packageId}`
  navigator.clipboard?.writeText(url).catch(() => {})
  flash('Download link copied')
}

async function revoke(packageId: string, name: string) {
  if (!confirm(`Revoke "${name}"? The link will stop working immediately.`)) return
  try {
    await revokePackage(packageId)
    flash('Package revoked')
  } catch (e) {
    flash((e as Error).message || 'Failed to revoke package')
  }
}

async function extend(packageId: string, expiresInDays: number, maxDownloads: number | undefined) {
  // Counted before the call: extending grants every pending request, so the
  // refreshed package comes back with none left to count.
  const granted = packages.value.find((p) => p.packageId === packageId)?.pendingRequests.length ?? 0
  extendingId.value = packageId
  try {
    await extendPackage(packageId, { expiresInDays, maxDownloads })
    flash(granted ? `Package extended — ${granted} requester${granted === 1 ? '' : 's'} notified` : 'Package extended')
  } catch (e) {
    flash((e as Error).message || 'Failed to extend package')
  } finally {
    extendingId.value = null
  }
}

async function dismiss(packageId: string, requestId: string) {
  try {
    await dismissAccessRequest(packageId, requestId)
    flash('Request dismissed')
  } catch (e) {
    flash((e as Error).message || 'Failed to dismiss request')
  }
}

// No fetch on mount: usePackages() is a shared singleton, and Send.vue already
// fetches on mount and after each send.
</script>

<template>
  <div v-if="loading && !packages.length" class="empty">Loading…</div>
  <div v-else-if="!packages.length" class="empty">No packages sent yet.</div>
  <div v-else class="pkg-list">
    <SentPackageCard
      v-for="p in packages"
      :key="p.packageId"
      :pkg="p"
      :extending="extendingId === p.packageId"
      @copy-link="copyLink(p.packageId)"
      @preview="emit('preview', p.packageId)"
      @revoke="revoke(p.packageId, p.name)"
      @extend="(days, max) => extend(p.packageId, days, max)"
      @dismiss-request="(id) => dismiss(p.packageId, id)"
    />
  </div>
  <div v-if="toast" class="toast fb-pop"><span class="ok">✓</span>{{ toast }}</div>
</template>
