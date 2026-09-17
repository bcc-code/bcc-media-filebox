<script setup lang="ts">
import { computed } from 'vue'
import type {
  PackageArtifact,
  PackageSourceFile,
  PreparationStatus,
} from '../../composables/usePackages'
import PackageFileManifest from './PackageFileManifest.vue'

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

const progress = computed(() =>
  Math.min(100, Math.max(0, props.preparationProgress || 0)),
)
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
  return (
    props.maxDownloads != null && artifact.accessCount >= props.maxDownloads
  )
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
    <template v-else> You've received files </template>
  </div>
  <h2 class="public-pkgname">{{ packageName }}</h2>
  <div v-if="message" class="public-msg">{{ message }}</div>

  <PackageFileManifest :files="files" />

  <div
    v-if="preparationStatus === 'processing'"
    class="preparation-state"
    aria-live="polite"
  >
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
        {{ fmtBytes(preparationBytesDone) }} of
        {{ fmtBytes(preparationBytesTotal) }} processed
      </span>
      <span v-else>Starting preparation…</span>
    </div>
    <p>
      Your files are safe. This page updates automatically while the download
      files are being prepared.
    </p>
  </div>

  <div
    v-else-if="preparationStatus === 'failed'"
    class="preparation-state failed"
    role="alert"
  >
    <div class="preparation-heading">We couldn't prepare the downloads</div>
    <p>
      {{
        preparationError ||
        'Something went wrong while preparing this package. Please ask the sender to try again.'
      }}
    </p>
  </div>

  <section v-else class="public-section downloads-section">
    <div class="public-section-head">
      <span>Downloads</span>
      <span>{{ downloads.length }}</span>
    </div>
    <div v-if="downloads.length" class="artifact-list">
      <div
        v-for="artifact in downloads"
        :key="artifact.id"
        class="artifact-row"
      >
        <span class="artifact-icon" :class="artifact.kind">
          <svg
            v-if="artifact.kind === 'zip'"
            width="17"
            height="17"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="1.7"
            stroke-linecap="round"
            stroke-linejoin="round"
          >
            <path
              d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"
            />
            <path d="M14 2v6h6M10 6h2M10 10h2M10 14h2M10 18h2" />
          </svg>
          <svg
            v-else
            width="17"
            height="17"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="1.7"
            stroke-linecap="round"
            stroke-linejoin="round"
          >
            <path
              d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"
            />
            <path d="M14 2v6h6" />
          </svg>
        </span>
        <span class="artifact-body">
          <span class="artifact-name">{{ artifact.filename }}</span>
          <span class="artifact-meta">
            {{ artifactKindText(artifact) }} · {{ fmtBytes(artifact.size) }} ·
            {{ artifact.accessCount
            }}{{ maxDownloads != null ? '/' + maxDownloads : '' }} downloads
            <template v-if="isExhausted(artifact)"> · limit reached</template>
          </span>
        </span>
        <span
          v-if="isExhausted(artifact)"
          class="artifact-dl inert"
          title="This download has reached its limit"
        >
          Unavailable
        </span>
        <a
          v-else-if="interactive"
          class="artifact-dl"
          :href="`/api/artifacts/${encodeURIComponent(artifact.id)}`"
          :download="artifact.filename"
          @click="emit('downloaded', artifact.id)"
        >
          <svg
            width="14"
            height="14"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="2"
            stroke-linecap="round"
            stroke-linejoin="round"
          >
            <path d="M12 3v12M7 10l5 5 5-5M5 21h14" />
          </svg>
          Download
        </a>
        <span
          v-else
          class="artifact-dl inert"
          title="Preview only — download disabled"
        >
          <svg
            width="14"
            height="14"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="2"
            stroke-linecap="round"
            stroke-linejoin="round"
          >
            <path d="M12 3v12M7 10l5 5 5-5M5 21h14" />
          </svg>
          Download
        </span>
      </div>
    </div>
    <div v-else class="preparation-state failed">
      <div class="preparation-heading">No downloads are available</div>
      <p>
        The package finished preparing, but it doesn't contain a downloadable
        item.
      </p>
    </div>
    <div v-if="allArtifactsExhausted" class="download-limit-note">
      Every download item has reached its limit. You can ask the sender to make
      the package available again.
    </div>
  </section>

  <div class="public-note">
    {{ expiryText(expiresAt)
    }}<template v-if="maxDownloads != null">
      · {{ maxDownloads }} downloads allowed per item</template
    >
  </div>
</template>

<style scoped>
/* Colocated from send.css: these classes are used only by this
   component. Shared primitives stay in assets/components.css — several
   components need them, and scoped CSS cannot be shared. */
.public-msg {
  margin-top: 16px;
  padding: 13px 15px;
  background: var(--color-surface);
  border: 1px solid var(--color-line);
  border-radius: 10px;
  font-size: 13.5px;
  color: var(--color-ink-2);
  line-height: 1.55;
}
/* Async package preparation and server-produced download artifacts. */
.preparation-state {
  margin-top: 22px;
  padding: 15px;
  background: color-mix(in oklch, var(--color-info), transparent 92%);
  border: 1px solid color-mix(in oklch, var(--color-info), transparent 64%);
  border-radius: 11px;
}
.preparation-heading {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  color: var(--color-ink);
  font-size: 13.5px;
  font-weight: 600;
}
.preparation-meta {
  margin-top: 7px;
  color: var(--color-ink-3);
  font-size: 11.5px;
  font-family: 'JetBrains Mono', monospace;
}
.preparation-state p {
  margin: 9px 0 0;
  color: var(--color-ink-2);
  font-size: 12.5px;
  line-height: 1.5;
}
.downloads-section {
  padding-top: 20px;
  border-top: 1px solid var(--color-line);
}
.artifact-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin-top: 10px;
}
.artifact-row {
  display: flex;
  align-items: center;
  gap: 11px;
  padding: 11px;
  background: var(--color-surface);
  border: 1px solid var(--color-line);
  border-radius: 10px;
}
.artifact-icon {
  width: 34px;
  height: 34px;
  display: grid;
  place-items: center;
  flex-shrink: 0;
  color: var(--color-ink-2);
  background: var(--color-surface-3);
  border: 1px solid var(--color-line-2);
  border-radius: 8px;
}
.artifact-icon.zip {
  color: var(--color-accent-2);
  border-color: color-mix(in oklch, var(--color-accent), transparent 65%);
}
.artifact-body {
  display: flex;
  flex: 1;
  min-width: 0;
  flex-direction: column;
  gap: 3px;
}
.artifact-name {
  overflow: hidden;
  color: var(--color-ink);
  font-size: 13px;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.artifact-meta {
  color: var(--color-ink-3);
  font-size: 10.5px;
  line-height: 1.4;
}
.artifact-dl {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 5px;
  flex-shrink: 0;
  padding: 7px 9px;
  color: var(--color-accent-2);
  background: color-mix(in oklch, var(--color-accent), transparent 90%);
  border: 1px solid color-mix(in oklch, var(--color-accent), transparent 62%);
  border-radius: 7px;
  font-size: 11.5px;
  font-weight: 600;
  transition:
    background 0.12s,
    border-color 0.12s;
}
.artifact-dl:hover {
  background: color-mix(in oklch, var(--color-accent), transparent 84%);
  border-color: color-mix(in oklch, var(--color-accent), transparent 42%);
}
.artifact-dl.inert {
  color: var(--color-ink-3);
  background: var(--color-surface-3);
  border-color: var(--color-line-2);
  cursor: default;
  opacity: 0.65;
}
.download-limit-note {
  margin-top: 12px;
  padding: 10px 12px;
  color: oklch(0.86 0.11 75);
  background: color-mix(in oklch, var(--color-warn), transparent 90%);
  border: 1px solid color-mix(in oklch, var(--color-warn), transparent 62%);
  border-radius: 9px;
  font-size: 12px;
  line-height: 1.5;
}

/* Its own responsive override. This has to live beside the base rule: left
   in send.css it tied on specificity with the scoped rule here and the winner
   came down to bundle order. */
@media (max-width: 600px) {
  .artifact-row {
    align-items: flex-start;
    flex-wrap: wrap;
  }
  .artifact-dl {
    width: 100%;
    margin-top: 2px;
  }
}
/* Failed-state modifiers of the scoped .preparation-state above. */
.preparation-state.failed .preparation-heading {
  color: oklch(0.86 0.1 25);
}
.preparation-state.failed {
  background: color-mix(in oklch, var(--color-danger), transparent 91%);
  border-color: color-mix(in oklch, var(--color-danger), transparent 60%);
}
</style>
