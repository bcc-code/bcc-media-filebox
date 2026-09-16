<script setup lang="ts">
import { ref, onMounted } from 'vue'
import type { UploadRecord } from '../types'
import { getUserId } from '../composables/useUserId'

const records = ref<UploadRecord[]>([])
const loading = ref(false)

async function fetchUploads() {
  loading.value = true
  try {
    const res = await fetch(`/api/uploads?user_id=${encodeURIComponent(getUserId())}`)
    records.value = await res.json()
  } catch {
    // silently fail
  } finally {
    loading.value = false
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

defineExpose({ refresh: fetchUploads })
</script>

<template>
  <div>
    <div class="list-head">
      <h2 class="section-title">Completed uploads</h2>
      <button class="btn btn-sm btn-ghost" @click="fetchUploads">Refresh</button>
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
            <span v-if="record.durationMs">&mdash; {{ formatDuration(record.durationMs) }}</span>
            <span v-if="record.avgBandwidth">&mdash; avg {{ formatSize(record.avgBandwidth) }}/s</span>
            <span v-if="record.completedAt">&mdash; {{ formatDate(record.completedAt) }}</span>
          </div>
        </div>
        <span class="badge badge-ok">
          <span class="badge-dot" />
          completed
        </span>
      </div>
    </div>
  </div>
</template>

<style scoped>
.list-head {
  display: flex; align-items: center; justify-content: space-between;
  gap: 14px; margin-bottom: 14px;
}

.section-title {
  margin: 0;
  font-size: 14px; font-weight: 600;
  text-transform: uppercase; letter-spacing: 0.08em;
  color: var(--color-ink-2);
}

.list-status { margin: 0; font-size: 13px; color: var(--color-ink-3); }

/* The metadata line packs several optional spans; space them consistently
   instead of relying on a margin on each one. */
.file-item .fsize { display: flex; flex-wrap: wrap; gap: 6px; }
</style>
