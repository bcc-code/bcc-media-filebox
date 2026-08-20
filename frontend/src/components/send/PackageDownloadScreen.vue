<script setup lang="ts">
import { computed } from 'vue'
import type { PackageArtifact, PackageSourceFile, PreparationStatus } from '../../composables/usePackages'

const props = defineProps<{
  packageName: string
  senderName: string
  message: string
  files: PackageSourceFile[]
  downloads: PackageArtifact[]
  expiresAt: string
  maxDownloads: number | null
  preparationStatus: PreparationStatus
  preparationBytesDone: number
  preparationBytesTotal: number
  preparationProgress: number
  preparationError?: string | null
  interactive: boolean
}>()

// The browser streams artifacts directly. The server records an access as soon
// as the request arrives, so updating the visible count on click is the closest
// useful reflection of it here.
const emit = defineEmits<{ downloaded: [artifactId: string] }>()

const totalSize = computed(() => props.files.reduce((sum, file) => sum + file.size, 0))
const progress = computed(() => Math.min(100, Math.max(0, props.preparationProgress || 0)))
const allArtifactsExhausted = computed(
  () =>
    props.preparationStatus === 'ready' &&
    props.maxDownloads != null &&
    props.downloads.length > 0 &&
    props.downloads.every((artifact) => isExhausted(artifact)),
)

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

function isExhausted(artifact: PackageArtifact): boolean {
  return props.maxDownloads != null && artifact.accessCount >= props.maxDownloads
}

function artifactKindText(artifact: PackageArtifact): string {
  if (artifact.kind === 'file') return 'Original file'
  return `ZIP archive · ${artifact.fileCount} file${artifact.fileCount === 1 ? '' : 's'}`
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

  <div v-if="preparationStatus === 'processing'" class="preparation-state" aria-live="polite">
    <div class="preparation-heading">
      <span>Preparing your downloads</span>
      <span class="mono">{{ Math.round(progress) }}%</span>
    </div>
    <div
      class="preparation-track"
      role="progressbar"
      aria-label="Preparing downloads"
      aria-valuemin="0"
      aria-valuemax="100"
      :aria-valuenow="Math.round(progress)"
    >
      <span class="preparation-fill" :style="{ width: `${progress}%` }"></span>
    </div>
    <div class="preparation-meta">
      <span v-if="preparationBytesTotal > 0">
        {{ fmtBytes(preparationBytesDone) }} of {{ fmtBytes(preparationBytesTotal) }} processed
      </span>
      <span v-else>Starting preparation…</span>
    </div>
    <p>Your files are safe. This page updates automatically while the download files are being prepared.</p>
  </div>

  <div v-else-if="preparationStatus === 'failed'" class="preparation-state failed" role="alert">
    <div class="preparation-heading">We couldn't prepare the downloads</div>
    <p>{{ preparationError || 'Something went wrong while preparing this package. Please ask the sender to try again.' }}</p>
  </div>

  <section v-else class="public-section downloads-section">
    <div class="public-section-head">
      <span>Downloads</span>
      <span>{{ downloads.length }}</span>
    </div>
    <div v-if="downloads.length" class="artifact-list">
      <div v-for="artifact in downloads" :key="artifact.id" class="artifact-row">
        <span class="artifact-icon" :class="artifact.kind">
          <svg v-if="artifact.kind === 'zip'" width="17" height="17" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><path d="M14 2v6h6M10 6h2M10 10h2M10 14h2M10 18h2"/></svg>
          <svg v-else width="17" height="17" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><path d="M14 2v6h6"/></svg>
        </span>
        <span class="artifact-body">
          <span class="artifact-name">{{ artifact.filename }}</span>
          <span class="artifact-meta">
            {{ artifactKindText(artifact) }} · {{ fmtBytes(artifact.size) }} ·
            {{ artifact.accessCount }}{{ maxDownloads != null ? '/' + maxDownloads : '' }} downloads
            <template v-if="isExhausted(artifact)"> · limit reached</template>
          </span>
        </span>
        <span v-if="isExhausted(artifact)" class="artifact-dl inert" title="This download has reached its limit">
          Unavailable
        </span>
        <a
          v-else-if="interactive"
          class="artifact-dl"
          :href="`/api/artifacts/${encodeURIComponent(artifact.id)}`"
          :download="artifact.filename"
          @click="emit('downloaded', artifact.id)"
        >
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 3v12M7 10l5 5 5-5M5 21h14"/></svg>
          Download
        </a>
        <span v-else class="artifact-dl inert" title="Preview only — download disabled">
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 3v12M7 10l5 5 5-5M5 21h14"/></svg>
          Download
        </span>
      </div>
    </div>
    <div v-else class="preparation-state failed">
      <div class="preparation-heading">No downloads are available</div>
      <p>The package finished preparing, but it doesn't contain a downloadable item.</p>
    </div>
    <div v-if="allArtifactsExhausted" class="download-limit-note">
      Every download item has reached its limit. You can ask the sender to make the package available again.
    </div>
  </section>

  <div class="public-note">
    {{ expiryText(expiresAt) }}<template v-if="maxDownloads != null"> · {{ maxDownloads }} downloads allowed per item</template>
  </div>
</template>
