<script setup lang="ts">
import { ref, onMounted } from 'vue'
import type { UploadRecord } from '../types'

const records = ref<UploadRecord[]>([])
const loading = ref(false)

async function fetchUploads() {
  loading.value = true
  try {
    const res = await fetch('/api/uploads')
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

function formatDate(iso: string): string {
  return new Date(iso).toLocaleString()
}

onMounted(fetchUploads)

defineExpose({ refresh: fetchUploads })
</script>

<template>
  <div>
    <div class="flex items-center justify-between mb-4">
      <h2 class="text-lg font-semibold text-gray-900 dark:text-gray-100">Completed Uploads</h2>
      <button
        @click="fetchUploads"
        class="text-sm px-3 py-1 rounded bg-gray-100 hover:bg-gray-200 dark:bg-gray-700 dark:hover:bg-gray-600 text-gray-700 dark:text-gray-300 cursor-pointer"
      >
        Refresh
      </button>
    </div>

    <div v-if="loading" class="text-sm text-gray-500 dark:text-gray-400">Loading...</div>

    <div v-else-if="!records || records.length === 0" class="text-sm text-gray-500 dark:text-gray-400">
      No completed uploads yet.
    </div>

    <div v-else class="space-y-2">
      <div
        v-for="record in records"
        :key="record.id"
        class="flex items-center justify-between p-3 border border-gray-200 dark:border-gray-700 rounded-lg"
      >
        <div class="min-w-0 flex-1">
          <p class="text-sm font-medium text-gray-900 dark:text-gray-100 truncate">
            {{ record.filename }}
          </p>
          <p class="text-xs text-gray-500 dark:text-gray-400">
            {{ formatSize(record.size) }}
            <span v-if="record.completedAt" class="ml-2">&mdash; {{ formatDate(record.completedAt) }}</span>
          </p>
        </div>
        <span
          class="text-xs font-medium px-2 py-0.5 rounded-full ml-4"
          :class="{
            'bg-green-100 text-green-700 dark:bg-green-900 dark:text-green-300': record.status === 'completed',
            'bg-blue-100 text-blue-700 dark:bg-blue-900 dark:text-blue-300': record.status === 'uploading',
            'bg-red-100 text-red-700 dark:bg-red-900 dark:text-red-300': record.status === 'failed',
          }"
        >
          {{ record.status }}
        </span>
      </div>
    </div>
  </div>
</template>
