<script setup lang="ts">
import { computed } from 'vue'
import type { UploadItem } from '../types'

const props = defineProps<{
  item: UploadItem
}>()

const emit = defineEmits<{
  pause: [item: UploadItem]
  resume: [item: UploadItem]
  retry: [item: UploadItem]
  cancel: [item: UploadItem]
}>()

function formatSize(bytes: number): string {
  if (bytes === 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(1024))
  return `${(bytes / Math.pow(1024, i)).toFixed(i > 0 ? 1 : 0)} ${units[i]}`
}

function formatSpeed(bytesPerSec: number): string {
  if (bytesPerSec <= 0) return ''
  return `${formatSize(bytesPerSec)}/s`
}

function formatETA(item: UploadItem): string {
  if (item.speed <= 0 || item.status !== 'uploading') return ''
  const remaining = item.bytesTotal - item.bytesUploaded
  const seconds = Math.round(remaining / item.speed)
  if (seconds < 60) return `${seconds}s`
  if (seconds < 3600) return `${Math.floor(seconds / 60)}m ${seconds % 60}s`
  return `${Math.floor(seconds / 3600)}h ${Math.floor((seconds % 3600) / 60)}m`
}

// Upload status drives both the badge tint and the progress fill; keep the two
// mappings together so a new status can't be styled inconsistently.
const statusBadge = computed(() => ({
  uploading: 'badge-accent',
  paused: 'badge-warn',
  completed: 'badge-ok',
  failed: 'badge-danger',
  pending: '',
}[props.item.status] ?? ''))

const fillTone = computed(() => ({
  uploading: '',
  paused: 'warn',
  completed: 'ok',
  failed: 'danger',
  pending: 'idle',
}[props.item.status] ?? ''))
</script>

<template>
  <div class="card upload-row">
    <div class="upload-top">
      <div class="upload-id">
        <div class="fname">{{ item.displayName }}</div>
        <div class="fmeta">
          <span>{{ formatSize(item.bytesUploaded) }} / {{ formatSize(item.bytesTotal) }}</span>
          <span v-if="formatSpeed(item.speed)">&mdash; {{ formatSpeed(item.speed) }}</span>
          <span v-if="formatETA(item)">&mdash; {{ formatETA(item) }} remaining</span>
        </div>
      </div>
      <span class="badge" :class="statusBadge">{{ item.status }}</span>
    </div>

    <div
      class="progress"
      role="progressbar"
      :aria-valuenow="Math.round(item.progress)"
      aria-valuemin="0"
      aria-valuemax="100"
    >
      <div class="fill" :class="fillTone" :style="{ width: `${item.progress}%` }" />
    </div>

    <div class="upload-actions">
      <button v-if="item.status === 'uploading'" class="btn btn-sm btn-ghost" @click="emit('pause', item)">
        Pause
      </button>
      <button v-if="item.status === 'paused'" class="btn btn-sm btn-primary" @click="emit('resume', item)">
        Resume
      </button>
      <button v-if="item.status === 'failed'" class="btn btn-sm btn-primary" @click="emit('retry', item)">
        Retry
      </button>
      <button v-if="item.status !== 'completed'" class="btn btn-sm btn-danger" @click="emit('cancel', item)">
        Cancel
      </button>
      <p v-if="item.error" class="upload-error">{{ item.error }}</p>
    </div>
  </div>
</template>

<style scoped>
.upload-row { padding: 14px 16px; }

.upload-top {
  display: flex; align-items: flex-start; justify-content: space-between;
  gap: 16px; margin-bottom: 10px;
}

.upload-id { flex: 1; min-width: 0; }
.fname {
  font-size: 13.5px; color: var(--color-ink);
  overflow: hidden; text-overflow: ellipsis; white-space: nowrap;
}
.fmeta {
  display: flex; flex-wrap: wrap; gap: 6px;
  margin-top: 3px; font-size: 11.5px; color: var(--color-ink-3);
}

.upload-actions { display: flex; align-items: center; gap: 8px; margin-top: 12px; flex-wrap: wrap; }
.upload-error { margin: 0; font-size: 11.5px; color: var(--color-danger); }
</style>
