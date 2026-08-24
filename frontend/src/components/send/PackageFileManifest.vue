<script setup lang="ts">
import { computed } from 'vue'
import type { PackageSourceFile } from '../../composables/usePackages'

const props = defineProps<{ files: PackageSourceFile[] }>()
const totalSize = computed(() => props.files.reduce((sum, file) => sum + file.size, 0))

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
  <section class="public-section">
    <div class="public-section-head">
      <span>Files in this package</span>
      <span>{{ files.length }}</span>
    </div>
    <div class="public-files source-manifest">
      <div v-for="file in files" :key="file.id" class="public-file">
        <span class="fic"><svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><path d="M14 2v6h6"/></svg></span>
        <span class="fn">{{ file.filename }}</span>
        <span class="fs">{{ fmtBytes(file.size) }}</span>
      </div>
      <div v-if="!files.length" class="public-files-empty">No source files are available.</div>
    </div>
    <div class="public-total">
      <span>{{ files.length }} file{{ files.length === 1 ? '' : 's' }}</span>
      <span class="mono">{{ fmtBytes(totalSize) }}</span>
    </div>
  </section>
</template>
