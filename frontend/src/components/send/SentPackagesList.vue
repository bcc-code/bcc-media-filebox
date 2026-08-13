<script setup lang="ts">
import { ref } from 'vue'
import { usePackages } from '../../composables/usePackages'
import SentPackageCard from './SentPackageCard.vue'

const emit = defineEmits<{ preview: [packageId: string] }>()

const { packages, loading, fetchPackages, revokePackage } = usePackages()
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

// No fetch on mount here — usePackages() is a shared singleton, and Send.vue
// (this component's only parent) already fetches on its own mount, before
// this tab can even be selected. `loading`/`packages` below are the same
// shared refs, so the "Loading…" state still renders correctly either way.
defineExpose({ refresh: fetchPackages })
</script>

<template>
  <div v-if="loading && !packages.length" class="empty">Loading…</div>
  <div v-else-if="!packages.length" class="empty">No packages sent yet.</div>
  <div v-else class="pkg-list">
    <SentPackageCard
      v-for="p in packages"
      :key="p.packageId"
      :pkg="p"
      @copy-link="copyLink(p.packageId)"
      @preview="emit('preview', p.packageId)"
      @revoke="revoke(p.packageId, p.name)"
    />
  </div>
  <div v-if="toast" class="toast fb-pop"><span class="ok">✓</span>{{ toast }}</div>
</template>
