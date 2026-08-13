<script setup lang="ts">
import type { UploadItem } from '../../types'

defineProps<{ item: UploadItem }>()
const emit = defineEmits<{ remove: [] }>()

function fmtBytes(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`
  const units = ['KB', 'MB', 'GB', 'TB']
  let i = -1
  let v = bytes
  do {
    v /= 1024
    i++
  } while (v >= 1024 && i < units.length - 1)
  return `${v.toFixed(v < 10 ? 2 : 1)} ${units[i]}`
}
</script>

<template>
  <div class="file-item fb-fade">
    <span class="fic">
      <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><path d="M14 2v6h6"/></svg>
    </span>
    <div class="fbody">
      <div class="fname">{{ item.displayName }}</div>
      <div class="fsize">
        {{ fmtBytes(item.bytesTotal) }}
        <template v-if="item.status === 'uploading'"> — uploading {{ Math.round(item.progress) }}%</template>
        <template v-else-if="item.status === 'pending'"> — queued</template>
        <template v-else-if="item.status === 'failed'" style="color: var(--danger)"> — {{ item.error || 'upload failed' }}</template>
        <template v-else-if="item.status === 'completed'"> — ready</template>
      </div>
    </div>
    <button class="frm" title="Remove" @click="emit('remove')">
      <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M18 6 6 18M6 6l12 12"/></svg>
    </button>
  </div>
</template>
