<script setup lang="ts">
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
</script>

<template>
  <div class="border border-gray-200 dark:border-gray-700 rounded-lg p-4">
    <div class="flex items-center justify-between mb-2">
      <div class="flex-1 min-w-0 mr-4">
        <p class="text-sm font-medium text-gray-900 dark:text-gray-100 truncate">
          {{ item.file.name }}
        </p>
        <p class="text-xs text-gray-500 dark:text-gray-400">
          {{ formatSize(item.bytesUploaded) }} / {{ formatSize(item.bytesTotal) }}
          <span v-if="formatSpeed(item.speed)" class="ml-2">
            &mdash; {{ formatSpeed(item.speed) }}
          </span>
          <span v-if="formatETA(item)" class="ml-2">
            &mdash; {{ formatETA(item) }} remaining
          </span>
        </p>
      </div>
      <div class="flex items-center gap-2">
        <span
          class="text-xs font-medium px-2 py-0.5 rounded-full"
          :class="{
            'bg-blue-100 text-blue-700 dark:bg-blue-900 dark:text-blue-300': item.status === 'uploading',
            'bg-yellow-100 text-yellow-700 dark:bg-yellow-900 dark:text-yellow-300': item.status === 'paused',
            'bg-green-100 text-green-700 dark:bg-green-900 dark:text-green-300': item.status === 'completed',
            'bg-red-100 text-red-700 dark:bg-red-900 dark:text-red-300': item.status === 'failed',
            'bg-gray-100 text-gray-700 dark:bg-gray-800 dark:text-gray-300': item.status === 'pending',
          }"
        >
          {{ item.status }}
        </span>
      </div>
    </div>

    <div class="w-full bg-gray-200 dark:bg-gray-700 rounded-full h-2 mb-3">
      <div
        class="h-2 rounded-full transition-all duration-300"
        :class="{
          'bg-blue-500': item.status === 'uploading',
          'bg-yellow-500': item.status === 'paused',
          'bg-green-500': item.status === 'completed',
          'bg-red-500': item.status === 'failed',
          'bg-gray-400': item.status === 'pending',
        }"
        :style="{ width: `${item.progress}%` }"
      />
    </div>

    <div class="flex items-center gap-2">
      <button
        v-if="item.status === 'uploading'"
        @click="emit('pause', item)"
        class="text-xs px-3 py-1 rounded bg-gray-100 hover:bg-gray-200 dark:bg-gray-700 dark:hover:bg-gray-600 text-gray-700 dark:text-gray-300 cursor-pointer"
      >
        Pause
      </button>
      <button
        v-if="item.status === 'paused'"
        @click="emit('resume', item)"
        class="text-xs px-3 py-1 rounded bg-blue-100 hover:bg-blue-200 dark:bg-blue-900 dark:hover:bg-blue-800 text-blue-700 dark:text-blue-300 cursor-pointer"
      >
        Resume
      </button>
      <button
        v-if="item.status === 'failed'"
        @click="emit('retry', item)"
        class="text-xs px-3 py-1 rounded bg-blue-100 hover:bg-blue-200 dark:bg-blue-900 dark:hover:bg-blue-800 text-blue-700 dark:text-blue-300 cursor-pointer"
      >
        Retry
      </button>
      <button
        v-if="item.status !== 'completed'"
        @click="emit('cancel', item)"
        class="text-xs px-3 py-1 rounded bg-red-100 hover:bg-red-200 dark:bg-red-900 dark:hover:bg-red-800 text-red-700 dark:text-red-300 cursor-pointer"
      >
        Cancel
      </button>
      <p v-if="item.error" class="text-xs text-red-500 ml-2">{{ item.error }}</p>
    </div>
  </div>
</template>
