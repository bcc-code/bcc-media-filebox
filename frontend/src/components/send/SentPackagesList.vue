<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { usePackages } from '../../composables/usePackages'
import SentPackageCard from './SentPackageCard.vue'

const props = defineProps<{ focusPackageId?: string }>()
const emit = defineEmits<{ preview: [packageId: string]; focusConsumed: [] }>()

const {
  packages,
  total,
  loading,
  loadMorePackages,
  revokePackage,
  extendPackage,
  dismissAccessRequest,
  setNotifyOnDownload,
  startPreparationPolling,
  stopPreparationPolling,
} = usePackages()

// The "reopen this" email link names a package that may be many pages back
// (list is created_at DESC, and a reopen request implies an old, dead one).
// Keep paging in the background until it turns up, instead of making the
// author click "Load more" themselves to find it.
const seekingFocus = computed(
  () =>
    !!props.focusPackageId &&
    !packages.value.some((p) => p.packageId === props.focusPackageId) &&
    packages.value.length < total.value,
)
watch(
  [packages, total],
  () => {
    if (seekingFocus.value && !loading.value) void loadMorePackages()
  },
  { immediate: true },
)
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

async function revoke(packageId: string) {
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

async function setNotify(packageId: string, notifyOnDownload: boolean) {
  try {
    await setNotifyOnDownload(packageId, notifyOnDownload)
    flash(notifyOnDownload ? 'Download notifications on' : 'Download notifications off')
  } catch (e) {
    flash((e as Error).message || 'Failed to update notifications')
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
onMounted(startPreparationPolling)
onUnmounted(stopPreparationPolling)
</script>

<template>
  <div v-if="loading && !packages.length" class="empty">Loading…</div>
  <div v-else-if="!packages.length" class="empty">No packages sent yet.</div>
  <div v-else class="pkg-list">
    <p v-if="seekingFocus" class="ask-hint">Loading more packages to find the one from your link…</p>
    <SentPackageCard
      v-for="p in packages"
      :key="p.packageId"
      :pkg="p"
      :extending="extendingId === p.packageId"
      :auto-open="p.packageId === focusPackageId"
      @auto-focused="emit('focusConsumed')"
      @copy-link="copyLink(p.packageId)"
      @preview="emit('preview', p.packageId)"
      @revoke="revoke(p.packageId)"
      @extend="(days, max) => extend(p.packageId, days, max)"
      @dismiss-request="(id) => dismiss(p.packageId, id)"
      @set-notify="(on) => setNotify(p.packageId, on)"
    />
    <button v-if="packages.length < total" class="btn" :disabled="loading" @click="loadMorePackages">
      {{ loading ? 'Loading…' : `Load more (${packages.length}/${total})` }}
    </button>
  </div>
  <div v-if="toast" class="toast fb-pop"><span class="ok">✓</span>{{ toast }}</div>
</template>
