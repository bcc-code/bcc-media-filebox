<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import type { UploadRecord } from '../types'
import { getUserId } from '../composables/useUserId'
import UiButton from './ui/UiButton.vue'
import UiBadge from './ui/UiBadge.vue'

const records = ref<UploadRecord[]>([])
const loading = ref(false)

// While any upload is still being assembled or moved into storage, keep
// refreshing so the badge flips to "completed" without a manual reload.
const PENDING_POLL_MS = 5_000
let pollTimer: ReturnType<typeof setTimeout> | null = null

async function fetchUploads() {
  loading.value = true
  try {
    const res = await fetch(
      `/api/uploads?user_id=${encodeURIComponent(getUserId())}`,
    )
    records.value = await res.json()
  } catch {
    // silently fail
  } finally {
    loading.value = false
  }
  schedulePoll()
}

function schedulePoll() {
  if (pollTimer) clearTimeout(pollTimer)
  pollTimer = null
  if (records.value.some((r) => r.storageStatus === 'pending')) {
    pollTimer = setTimeout(fetchUploads, PENDING_POLL_MS)
  }
}

function badge(record: UploadRecord): {
  text: string
  variant: 'ok' | 'warn' | 'danger'
  title: string
} {
  switch (record.storageStatus) {
    case 'pending':
      return {
        text: 'assembling',
        variant: 'warn',
        title:
          'All bytes have arrived; the file is being assembled and moved into its target.',
      }
    case 'failed':
      return {
        text: 'failed',
        variant: 'danger',
        title: 'The file could not be moved into storage.',
      }
    default:
      return {
        text: 'completed',
        variant: 'ok',
        title: 'The file is in its target location.',
      }
  }
}

function formatSize(bytes: number): string {
  if (bytes === 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(1024))
  return `${(bytes / Math.pow(1024, i)).toFixed(i > 0 ? 1 : 0)} ${units[i]}`
}

function formatDuration(ms: number): string {
  const seconds = Math.round(ms / 1000)
  if (seconds < 60) return `${seconds}s`
  if (seconds < 3600) return `${Math.floor(seconds / 60)}m ${seconds % 60}s`
  return `${Math.floor(seconds / 3600)}h ${Math.floor((seconds % 3600) / 60)}m`
}

function formatDate(iso: string): string {
  return new Date(iso).toLocaleString()
}

onMounted(fetchUploads)
onUnmounted(() => {
  if (pollTimer) clearTimeout(pollTimer)
})

defineExpose({ refresh: fetchUploads })
</script>

<template>
  <div>
    <div class="list-head">
      <h2 class="section-title">Completed uploads</h2>
      <UiButton size="sm" variant="ghost" @click="fetchUploads"
        >Refresh</UiButton
      >
    </div>

    <p v-if="loading" class="list-status">Loading…</p>

    <div v-else-if="!records || records.length === 0" class="empty">
      No completed uploads yet.
    </div>

    <div v-else class="file-list">
      <div v-for="record in records" :key="record.id" class="file-item">
        <div class="fbody">
          <div class="fname">{{ record.filename }}</div>
          <div class="fsize">
            {{ formatSize(record.size) }}
            <span v-if="record.durationMs"
              >&mdash; {{ formatDuration(record.durationMs) }}</span
            >
            <span v-if="record.avgBandwidth"
              >&mdash; avg {{ formatSize(record.avgBandwidth) }}/s</span
            >
            <span v-if="record.completedAt"
              >&mdash; {{ formatDate(record.completedAt) }}</span
            >
          </div>
        </div>
        <UiBadge
          :variant="badge(record).variant"
          :title="badge(record).title"
          dot
          >{{ badge(record).text }}</UiBadge
        >
      </div>
    </div>
  </div>
</template>

<style scoped>
.list-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 14px;
  margin-bottom: 14px;
}

.section-title {
  margin: 0;
  font-size: var(--text-title-2);
  line-height: var(--text-title-2--line-height);
  font-weight: var(--text-title-2--font-weight);
  color: var(--color-ink-2);
}

.list-status {
  margin: 0;
  font-size: 13px;
  color: var(--color-ink-3);
}

/* The metadata line packs several optional spans; space them consistently
   instead of relying on a margin on each one. */
.file-item .fsize {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}
</style>
