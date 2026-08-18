<script setup lang="ts">
import { computed } from 'vue'
import type { PackageFile } from '../../composables/usePackages'

const props = defineProps<{
  packageName: string
  senderName: string
  message: string
  files: PackageFile[]
  expiresAt: string
  maxDownloads: number | null
  // When false, file rows are inert — the sender's "Recipient preview" tab, so
  // trying it out never increments the real access counts.
  interactive: boolean
}>()

// The download is a plain <a href download> so the browser streams it natively,
// which means nothing here sees it finish. The server increments access_count as
// soon as the request arrives, so emitting on click matches it closely enough.
const emit = defineEmits<{ downloaded: [shareId: string] }>()

const totalSize = computed(() => props.files.reduce((sum, f) => sum + f.size, 0))

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

function expiryText(iso: string): string {
  const days = Math.ceil((new Date(iso).getTime() - Date.now()) / 86400000)
  if (days <= 0) return 'expires today'
  return `expires in ${days} day${days === 1 ? '' : 's'}`
}

function isExhausted(f: PackageFile): boolean {
  return props.maxDownloads != null && f.accessCount >= props.maxDownloads
}

// "Download all" with no backend endpoint: fire each file's own link. Fine for a
// handful; zipping many small files is a separate, unbuilt piece.
function downloadAll() {
  if (!props.interactive) return
  for (const f of props.files) {
    if (isExhausted(f)) continue
    const a = document.createElement('a')
    a.href = `/api/shares/${encodeURIComponent(f.shareId)}`
    a.download = f.filename
    document.body.appendChild(a)
    a.click()
    a.remove()
    emit('downloaded', f.shareId)
  }
}
</script>

<template>
  <div class="public-from">
    <template v-if="senderName">
      <strong>{{ senderName }}</strong> sent you a package
    </template>
    <template v-else>
      You've received files
    </template>
  </div>
  <h2 class="public-pkgname">{{ packageName }}</h2>
  <div v-if="message" class="public-msg">{{ message }}</div>

  <div class="public-files">
    <div v-for="f in files" :key="f.shareId" class="public-file">
      <span class="fic"><svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><path d="M14 2v6h6"/></svg></span>
      <span class="fn">{{ f.filename }}</span>
      <span class="fs">{{ fmtBytes(f.size) }}</span>
      <span class="fs">{{ f.accessCount }}{{ maxDownloads ? '/' + maxDownloads : '' }} downloads<template v-if="isExhausted(f)"> · limit reached</template></span>
      <span v-if="isExhausted(f)" class="file-dl inert" title="This file has reached its download limit">
        <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 3v12M7 10l5 5 5-5M5 21h14"/></svg>
      </span>
      <a
        v-else-if="interactive"
        class="file-dl"
        :href="`/api/shares/${encodeURIComponent(f.shareId)}`"
        :download="f.filename"
        title="Download this file"
        @click="emit('downloaded', f.shareId)"
      >
        <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 3v12M7 10l5 5 5-5M5 21h14"/></svg>
      </a>
      <span v-else class="file-dl inert" title="Preview only — download disabled">
        <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 3v12M7 10l5 5 5-5M5 21h14"/></svg>
      </span>
    </div>
  </div>

  <div class="public-total"><span>{{ files.length }} file{{ files.length === 1 ? '' : 's' }}</span><span class="mono">{{ fmtBytes(totalSize) }}</span></div>

  <div class="public-dl">
    <button class="btn btn-primary btn-block" :disabled="!interactive" @click="downloadAll">
      <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 3v12M7 10l5 5 5-5M5 21h14"/></svg>
      Download all · {{ fmtBytes(totalSize) }}
    </button>
  </div>

  <div class="public-note">
    {{ expiryText(expiresAt) }}<template v-if="maxDownloads"> · {{ maxDownloads }} downloads allowed</template>
  </div>
</template>
