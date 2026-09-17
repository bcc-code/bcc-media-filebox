<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import type {
  AccessRequest,
  PackageInfo,
  VerificationMethod,
} from '../../composables/usePackages'

import UiNumberInput from '../ui/UiNumberInput.vue'
import UiButton from '../ui/UiButton.vue'
import UiBadge from '../ui/UiBadge.vue'

const props = defineProps<{
  pkg: PackageInfo
  extending?: boolean
  autoOpen?: boolean
}>()
const emit = defineEmits<{
  copyLink: []
  preview: []
  revoke: []
  extend: [expiresInDays: number, maxDownloads: number | undefined]
  dismissRequest: [requestId: string]
  setNotify: [notifyOnDownload: boolean]
  autoFocused: []
}>()

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
  if (days <= 0) return 'expired'
  return `expires in ${days} day${days === 1 ? '' : 's'}`
}

function dateText(iso: string): string {
  return new Date(iso).toLocaleDateString(undefined, {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
  })
}

const permanentDeleteWarningDays = 14

function daysUntilFilesDeleted(iso: string): number {
  return Math.ceil((new Date(iso).getTime() - Date.now()) / 86400000)
}

function permanentDeleteText(days: number): string {
  return `files deleted in ${days} day${days === 1 ? '' : 's'}`
}

function agoText(iso: string): string {
  const mins = Math.round((Date.now() - new Date(iso).getTime()) / 60000)
  if (mins < 1) return 'just now'
  if (mins < 60) return `${mins}m ago`
  const hours = Math.round(mins / 60)
  if (hours < 24) return `${hours}h ago`
  const days = Math.round(hours / 24)
  return `${days}d ago`
}

const reasonLabels: Record<AccessRequest['reason'], string> = {
  '': 'no longer available',
  expired: 'link had expired',
  revoked: 'package was revoked',
  limit_reached: 'download limit reached',
}

const verifyShortLabels: Partial<Record<VerificationMethod, string>> = {
  bcc_login: 'BCC',
  password: 'Password',
  email_otp: 'Code',
  magic_link: 'Link',
}
const verifyShort = computed(
  () =>
    verifyShortLabels[props.pkg.verificationMethod] ??
    props.pkg.verificationMethod,
)

const isLive = computed(
  () =>
    !props.pkg.permanentlyExpired &&
    props.pkg.status !== 'revoked' &&
    !props.pkg.isExpired,
)
const downloadsReady = computed(() => props.pkg.preparationStatus === 'ready')
const preparationProgress = computed(() =>
  Math.min(100, Math.max(0, props.pkg.preparationProgress || 0)),
)

const displayStatus = computed(() => {
  if (props.pkg.permanentlyExpired) return 'deleted'
  if (props.pkg.status === 'revoked') return 'revoked'
  if (props.pkg.isExpired) return 'expired'
  if (props.pkg.preparationStatus === 'processing') return 'preparing'
  if (props.pkg.preparationStatus === 'failed') return 'failed'
  return 'active'
})

const canNotify = computed(
  () => isLive.value && downloadsReady.value && !props.pkg.isDownloadLimitHit,
)

// Extend form, prefilled with a week and the current budget, since "same limits,
// more time" is the usual answer. Blank downloads means unlimited.
const showExtend = ref(false)
const extendDays = ref(7)
const extendMaxDownloads = ref<number | ''>(props.pkg.maxDownloads ?? '')

function openExtend() {
  extendDays.value = 7
  extendMaxDownloads.value = props.pkg.maxDownloads ?? ''
  showExtend.value = true
}

// Blank downloads means unlimited, anything else must be at least 1.
const extendValid = computed(
  () =>
    extendDays.value >= 1 &&
    extendDays.value <= 30 &&
    (extendMaxDownloads.value === '' || extendMaxDownloads.value >= 1),
)

function submitExtend() {
  if (!extendValid.value || props.extending) return
  emit(
    'extend',
    extendDays.value,
    extendMaxDownloads.value === '' ? undefined : extendMaxDownloads.value,
  )
  showExtend.value = false
}

// Landed here from the "reopen this" email link: bring the card into view and,
// unless it's past permanent deletion (nothing to reopen), open the panel too.
const cardRoot = ref<HTMLElement | null>(null)
onMounted(() => {
  if (!props.autoOpen) return
  cardRoot.value?.scrollIntoView({ block: 'center', behavior: 'smooth' })
  if (!props.pkg.permanentlyExpired) openExtend()
  emit('autoFocused')
})
</script>

<template>
  <div
    ref="cardRoot"
    class="pkg-card"
    :class="{ gone: displayStatus === 'deleted' }"
  >
    <div class="pkg-top">
      <span class="pkg-ic">
        <svg
          width="19"
          height="19"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          stroke-width="1.6"
          stroke-linecap="round"
          stroke-linejoin="round"
        >
          <path d="M21 8v13H3V8M1 3h22v5H1zM10 12h4" />
        </svg>
      </span>
      <div class="pkg-main">
        <div class="pkg-name">{{ pkg.name }}</div>
        <div class="pkg-recips">
          {{
            pkg.recipients.length
              ? `To ${pkg.recipients.join(', ')}`
              : 'No recipients — link only'
          }}
        </div>
      </div>
      <div class="pkg-badges">
        <UiBadge v-if="pkg.pendingRequests.length" variant="warn"
          ><svg
            width="11"
            height="11"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="2"
            stroke-linecap="round"
            stroke-linejoin="round"
          >
            <path
              d="M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z"
            />
          </svg>
          {{ pkg.pendingRequests.length }} request{{
            pkg.pendingRequests.length === 1 ? '' : 's'
          }}</UiBadge
        >
        <UiBadge
          v-if="pkg.isDownloadLimitHit && displayStatus === 'active'"
          variant="warn"
          ><svg
            width="11"
            height="11"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="2"
            stroke-linecap="round"
            stroke-linejoin="round"
          >
            <path
              d="M10.29 3.86 1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0Z"
            />
            <path d="M12 9v4M12 17h.01" />
          </svg>
          Limit reached</UiBadge
        >
        <UiBadge v-if="pkg.verificationMethod !== 'none'" variant="accent"
          ><svg
            width="11"
            height="11"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="2"
            stroke-linecap="round"
            stroke-linejoin="round"
          >
            <rect x="5" y="11" width="14" height="10" rx="2" />
            <path d="M8 11V7a4 4 0 0 1 8 0v4" />
          </svg>
          {{ verifyShort }}</UiBadge
        >
        <span class="pill" :class="displayStatus">{{ displayStatus }}</span>
      </div>
    </div>
    <div class="pkg-meta">
      <span
        ><span class="mk">{{ pkg.fileCount }}</span> file{{
          pkg.fileCount === 1 ? '' : 's'
        }}
        · {{ fmtBytes(pkg.totalSize) }}</span
      >
      <span v-if="pkg.preparationStatus === 'ready'">
        <span class="mk">{{ pkg.artifactCount }}</span> artifact{{
          pkg.artifactCount === 1 ? '' : 's'
        }}
      </span>
      <span v-if="pkg.preparationStatus === 'ready'">
        Most used:
        <span class="mk"
          >{{ pkg.downloadCount
          }}{{ pkg.maxDownloads != null ? '/' + pkg.maxDownloads : '' }}</span
        >
      </span>
      <span v-if="displayStatus === 'deleted'">files permanently deleted</span>
      <span v-else-if="displayStatus === 'revoked'">revoked</span>
      <span v-else>{{ expiryText(pkg.expiresAt) }}</span>
      <span
        v-if="
          displayStatus !== 'deleted' &&
          daysUntilFilesDeleted(pkg.filesDeletedAt) <=
            permanentDeleteWarningDays
        "
        class="warn"
      >
        {{ permanentDeleteText(daysUntilFilesDeleted(pkg.filesDeletedAt)) }}
      </span>
    </div>

    <div
      v-if="pkg.preparationStatus === 'processing'"
      class="pkg-preparation"
      aria-live="polite"
    >
      <div class="pkg-preparation-head">
        <span>Preparing download files</span>
        <span class="mono">{{ Math.round(preparationProgress) }}%</span>
      </div>
      <div
        class="preparation-track"
        role="progressbar"
        aria-label="Preparing package downloads"
        aria-valuemin="0"
        aria-valuemax="100"
        :aria-valuenow="Math.round(preparationProgress)"
      >
        <span
          class="preparation-fill"
          :style="{ width: `${preparationProgress}%` }"
        ></span>
      </div>
      <div class="pkg-preparation-meta">
        <template v-if="pkg.preparationBytesTotal > 0">
          {{ fmtBytes(pkg.preparationBytesDone) }} of
          {{ fmtBytes(pkg.preparationBytesTotal) }} processed
        </template>
        <template v-else>Starting preparation…</template>
      </div>
    </div>
    <div
      v-else-if="pkg.preparationStatus === 'failed'"
      class="pkg-preparation failed"
      role="alert"
    >
      <div class="pkg-preparation-head">Download preparation failed</div>
      <div class="pkg-preparation-meta">
        {{
          pkg.preparationError ||
          'The original files are still safe, but the download files could not be prepared.'
        }}
      </div>
    </div>

    <!-- Pending access requests. On the card rather than only in the author's
         inbox, so a request that arrived while mail was down still gets seen. -->
    <div v-if="pkg.pendingRequests.length" class="pkg-asks">
      <div v-for="req in pkg.pendingRequests" :key="req.id" class="ask-row">
        <div class="ask-body">
          <div class="ask-line">
            <span class="ask-who">{{ req.email }}</span>
            asked to <span class="ask-kind">reopen this</span>
            <span class="ask-when"
              >· {{ reasonLabels[req.reason] }} ·
              {{ agoText(req.createdAt) }}</span
            >
          </div>
          <p v-if="req.message" class="ask-msg">{{ req.message }}</p>
        </div>
        <button
          class="ask-x"
          title="Dismiss request"
          :aria-label="`Dismiss request from ${req.email}`"
          @click="emit('dismissRequest', req.id)"
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
            <path d="M18 6 6 18M6 6l12 12" />
          </svg>
        </button>
      </div>
      <p v-if="displayStatus === 'deleted'" class="ask-hint">
        These files have been permanently deleted from storage — dismiss
        requests you can no longer fulfill.
      </p>
      <p v-else class="ask-hint">
        Extending answers every request here at once and emails each person the
        working link.
      </p>
    </div>

    <div v-if="showExtend" class="pkg-extend">
      <div class="ex-fields">
        <label class="ex-f">
          <span class="ex-lab">More days</span>
          <UiNumberInput
            v-model="extendDays"
            :min="1"
            :max="30"
            aria-label="More days"
          />
        </label>
        <label class="ex-f">
          <span class="ex-lab">Downloads per artifact</span>
          <UiNumberInput
            v-model="extendMaxDownloads"
            :min="1"
            placeholder="Unlimited"
            aria-label="Downloads per artifact"
          />
        </label>
      </div>
      <p class="ex-note">
        Counted from today, so the link is live for
        {{ extendValid ? extendDays : '—' }} more day{{
          extendDays === 1 ? '' : 's'
        }}. Leave downloads blank for unlimited; this replaces the current limit
        rather than adding to it.<template v-if="displayStatus === 'revoked'">
          This also un-revokes the package.</template
        >
        Files are permanently deleted from storage on
        {{ dateText(pkg.filesDeletedAt) }} — extending won't be possible after
        that.
      </p>
      <div class="ex-actions">
        <UiButton
          size="sm"
          variant="primary"
          :disabled="!extendValid"
          :loading="extending"
          loading-label="Extending…"
          @click="submitExtend"
        >
          Confirm
        </UiButton>
        <UiButton size="sm" :disabled="extending" @click="showExtend = false">
          Cancel
        </UiButton>
      </div>
    </div>

    <div class="pkg-foot">
      <UiButton
        size="sm"
        :disabled="!downloadsReady"
        :title="
          downloadsReady
            ? 'Copy recipient link'
            : 'Available after download preparation finishes'
        "
        @click="emit('copyLink')"
      >
        <svg
          width="13"
          height="13"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          stroke-width="1.8"
          stroke-linecap="round"
          stroke-linejoin="round"
        >
          <path d="M10 13a5 5 0 0 0 7 0l3-3a5 5 0 0 0-7-7l-1 1" />
          <path d="M14 11a5 5 0 0 0-7 0l-3 3a5 5 0 0 0 7 7l1-1" />
        </svg>
        Copy link
      </UiButton>
      <UiButton
        size="sm"
        :disabled="!downloadsReady"
        :title="
          downloadsReady
            ? 'Preview recipient page'
            : 'Available after download preparation finishes'
        "
        @click="emit('preview')"
      >
        Preview
      </UiButton>
      <span class="spacer"></span>
      <span v-if="displayStatus === 'deleted'" class="pkg-gone-note"
        >Files deleted — can't be renewed</span
      >
      <UiButton
        v-else-if="!showExtend"
        size="sm"
        :variant="pkg.pendingRequests.length > 0 ? 'primary' : 'default'"
        @click="openExtend"
      >
        {{ isLive ? 'Extend' : 'Reopen' }}
      </UiButton>
      <UiButton
        v-if="canNotify"
        size="sm"
        :active="pkg.notifyOnDownload"
        :title="
          pkg.notifyOnDownload
            ? 'Stop emailing me when this is downloaded'
            : 'Email me when this is downloaded'
        "
        :aria-pressed="pkg.notifyOnDownload"
        @click="emit('setNotify', !pkg.notifyOnDownload)"
      >
        <svg
          v-if="pkg.notifyOnDownload"
          width="13"
          height="13"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          stroke-width="1.8"
          stroke-linecap="round"
          stroke-linejoin="round"
        >
          <path d="M18 8a6 6 0 0 0-12 0c0 7-3 9-3 9h18s-3-2-3-9" />
          <path d="M13.73 21a2 2 0 0 1-3.46 0" />
        </svg>
        <svg
          v-else
          width="13"
          height="13"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          stroke-width="1.8"
          stroke-linecap="round"
          stroke-linejoin="round"
        >
          <path d="M8.7 3A6 6 0 0 1 18 8c0 7 3 9 3 9H6" />
          <path d="M13.73 21a2 2 0 0 1-3.46 0" />
          <path d="M2 2l20 20" />
        </svg>
        Notify {{ pkg.notifyOnDownload ? 'on' : 'off' }}
      </UiButton>
      <UiButton
        v-if="isLive"
        size="sm"
        variant="danger"
        @click="emit('revoke')"
      >
        Revoke
      </UiButton>
    </div>
  </div>
</template>

<style scoped>
/* Colocated from send.css: these classes are used only by this
   component. Shared primitives stay in assets/components.css — several
   components need them, and scoped CSS cannot be shared. */
.pkg-card {
  background: var(--color-surface-2);
  border: 1px solid var(--color-line);
  border-radius: 13px;
  padding: 16px 18px;
}
.pkg-gone-note {
  font-size: 12px;
  color: var(--color-ink-3);
}
.pkg-top {
  display: flex;
  align-items: flex-start;
  gap: 14px;
}
.pkg-ic {
  width: 40px;
  height: 40px;
  border-radius: 10px;
  background: var(--color-surface-3);
  border: 1px solid var(--color-line-2);
  color: var(--color-ink-2);
  display: grid;
  place-items: center;
  flex-shrink: 0;
}
.pkg-main {
  flex: 1;
  min-width: 0;
}
.pkg-name {
  font-size: 15px;
  font-weight: 600;
  color: var(--color-ink);
}
.pkg-recips {
  font-size: 12.5px;
  color: var(--color-ink-2);
  margin-top: 3px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.pkg-meta {
  display: flex;
  flex-wrap: wrap;
  gap: 6px 8px;
  margin-top: 12px;
  font-size: 12px;
  color: var(--color-ink-3);
  font-family: 'JetBrains Mono', monospace;
}
.pkg-meta .mk {
  color: var(--color-ink-2);
}
.pkg-meta .warn {
  color: oklch(0.85 0.13 75);
  font-weight: 600;
}
.pkg-meta > span:not(:first-child)::before {
  content: '|';
  margin-right: 8px;
  color: var(--color-line-2);
}
.pkg-badges {
  display: flex;
  gap: 7px;
  flex-shrink: 0;
  align-items: center;
}
.pkg-foot {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-top: 14px;
  padding-top: 14px;
  border-top: 1px solid var(--color-line);
}
/* On is the state the author chose, so it reads as active rather than as the default; off falls back to the plain button. */
.pill {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 4px 11px;
  border-radius: 999px;
  font-size: 12px;
  font-weight: 500;
  white-space: nowrap;
}
.pill.revoked {
  background: color-mix(in oklch, var(--color-danger), transparent 80%);
  color: oklch(0.82 0.13 25);
  border: 1px solid color-mix(in oklch, var(--color-danger), transparent 58%);
}
.pill.deleted {
  background: var(--color-surface-3);
  color: var(--color-ink-3);
  border: 1px solid var(--color-line-2);
}
.pkg-preparation {
  margin-top: 13px;
  padding: 12px 13px;
  border: 1px solid color-mix(in oklch, var(--color-info), transparent 65%);
  background: color-mix(in oklch, var(--color-info), transparent 92%);
  border-radius: 10px;
}
.pkg-preparation-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  font-size: 12.5px;
  font-weight: 600;
  color: var(--color-ink);
}
.pkg-preparation-meta {
  margin-top: 7px;
  font-size: 11.5px;
  color: var(--color-ink-3);
  line-height: 1.45;
}
/* Pending access requests and the extend form, on the author's package card */
.pkg-asks {
  margin-top: 14px;
  padding-top: 13px;
  border-top: 1px solid var(--color-line);
  display: flex;
  flex-direction: column;
  gap: 9px;
}
.ask-row {
  display: flex;
  align-items: flex-start;
  gap: 10px;
}
.ask-body {
  flex: 1;
  min-width: 0;
}
.ask-line {
  font-size: 12.5px;
  color: var(--color-ink-2);
  line-height: 1.5;
}
.ask-line .ask-who {
  color: var(--color-ink);
  font-weight: 500;
  word-break: break-all;
}
.ask-line .ask-kind {
  color: oklch(0.85 0.13 75);
}
.ask-line .ask-when {
  color: var(--color-ink-3);
}
.ask-msg {
  margin: 6px 0 0;
  padding: 9px 11px;
  background: var(--color-surface);
  border: 1px solid var(--color-line);
  border-radius: 8px;
  font-size: 12.5px;
  color: var(--color-ink-2);
  line-height: 1.5;
  white-space: pre-wrap;
}
.ask-x {
  background: transparent;
  border: none;
  color: var(--color-ink-3);
  cursor: pointer;
  padding: 4px;
  border-radius: 6px;
  display: grid;
  place-items: center;
  flex-shrink: 0;
}
.ask-x:hover {
  color: var(--color-ink);
  background: var(--color-surface-3);
}
.pkg-extend {
  margin-top: 14px;
  padding: 14px;
  background: var(--color-surface);
  border: 1px solid var(--color-line-2);
  border-radius: 11px;
}
.ex-fields {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 10px;
}
.ex-f {
  display: flex;
  flex-direction: column;
  gap: 6px;
  min-width: 0;
}
.ex-lab {
  font-size: 11.5px;
  color: var(--color-ink-3);
}
.ex-note {
  margin: 11px 0 0;
  font-size: 11.5px;
  color: var(--color-ink-3);
  line-height: 1.5;
}
.ex-actions {
  display: flex;
  gap: 8px;
  margin-top: 12px;
}

/* Its own responsive override. This has to live beside the base rule: left
   in send.css it tied on specificity with the scoped rule here and the winner
   came down to bundle order. */
@media (max-width: 600px) {
  .pkg-top {
    flex-wrap: wrap;
  }
  .pkg-badges {
    width: 100%;
    padding-left: 54px;
    flex-wrap: wrap;
  }
  .pkg-foot {
    flex-wrap: wrap;
  }
}
/* State modifiers for this card. They stayed in send.css because each one
   mixes in a shared modifier class (.gone, .active, .failed, .spacer), but
   every element they target is rendered here — and left global they tied
   with the scoped base rules. */
.pkg-card.gone {
  opacity: 0.55;
}
.pkg-foot .spacer {
  flex: 1;
}
.pill .dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: currentColor;
}
.pill.active {
  background: color-mix(in oklch, var(--color-ok), transparent 80%);
  color: oklch(0.82 0.11 160);
  border: 1px solid color-mix(in oklch, var(--color-ok), transparent 60%);
}
.pill.expired {
  background: var(--color-surface-3);
  color: var(--color-ink-3);
  border: 1px solid var(--color-line-2);
}
.pill.preparing {
  background: color-mix(in oklch, var(--color-info), transparent 82%);
  color: oklch(0.82 0.1 240);
  border: 1px solid color-mix(in oklch, var(--color-info), transparent 58%);
}
.pill.failed {
  background: color-mix(in oklch, var(--color-danger), transparent 80%);
  color: oklch(0.84 0.11 25);
  border: 1px solid color-mix(in oklch, var(--color-danger), transparent 58%);
}
.pkg-preparation.failed .pkg-preparation-head {
  color: oklch(0.86 0.1 25);
}
.pkg-preparation.failed {
  border-color: color-mix(in oklch, var(--color-danger), transparent 62%);
  background: color-mix(in oklch, var(--color-danger), transparent 91%);
}
</style>
