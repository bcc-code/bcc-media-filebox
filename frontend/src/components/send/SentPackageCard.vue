<script setup lang="ts">
import { computed, ref } from 'vue'
import type { AccessRequest, PackageInfo, VerificationMethod } from '../../composables/usePackages'

const props = defineProps<{ pkg: PackageInfo; extending?: boolean }>()
const emit = defineEmits<{
  copyLink: []
  preview: []
  revoke: []
  extend: [expiresInDays: number, maxDownloads: number | undefined]
  dismissRequest: [requestId: string]
  setNotify: [notifyOnDownload: boolean]
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
const verifyShort = computed(() => verifyShortLabels[props.pkg.verificationMethod] ?? props.pkg.verificationMethod)

const displayStatus = computed(() => {
  if (props.pkg.status === 'revoked') return 'revoked'
  if (props.pkg.isExpired) return 'expired'
  return 'active'
})

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
  emit('extend', extendDays.value, extendMaxDownloads.value === '' ? undefined : extendMaxDownloads.value)
  showExtend.value = false
}
</script>

<template>
  <div class="pkg-card">
    <div class="pkg-top">
      <span class="pkg-ic">
        <svg width="19" height="19" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"><path d="M21 8v13H3V8M1 3h22v5H1zM10 12h4"/></svg>
      </span>
      <div class="pkg-main">
        <div class="pkg-name">{{ pkg.name }}</div>
        <div class="pkg-recips">{{ pkg.recipients.length ? `To ${pkg.recipients.join(', ')}` : 'No recipients — link only' }}</div>
      </div>
      <div class="pkg-badges">
        <span v-if="pkg.pendingRequests.length" class="chip-sm ask">
          <svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z"/></svg>
          {{ pkg.pendingRequests.length }} request{{ pkg.pendingRequests.length === 1 ? '' : 's' }}
        </span>
        <span v-if="pkg.isDownloadLimitHit && displayStatus === 'active'" class="chip-sm warn">
          <svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M10.29 3.86 1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0Z"/><path d="M12 9v4M12 17h.01"/></svg>
          Limit reached
        </span>
        <span v-if="pkg.verificationMethod !== 'none'" class="chip-sm lock">
          <svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="5" y="11" width="14" height="10" rx="2"/><path d="M8 11V7a4 4 0 0 1 8 0v4"/></svg>
          {{ verifyShort }}
        </span>
        <span class="pill" :class="displayStatus">{{ displayStatus }}</span>
      </div>
    </div>
    <div class="pkg-meta">
      <span><span class="mk">{{ pkg.fileCount }}</span> file{{ pkg.fileCount === 1 ? '' : 's' }} · {{ fmtBytes(pkg.totalSize) }}</span>
      <span><span class="mk">{{ pkg.downloadCount }}{{ pkg.maxDownloads ? '/' + pkg.maxDownloads : '' }}</span> downloads</span>
      <span v-if="displayStatus === 'revoked'">revoked</span>
      <span v-else>{{ expiryText(pkg.expiresAt) }}</span>
    </div>

    <!-- Pending access requests. On the card rather than only in the author's
         inbox, so a request that arrived while mail was down still gets seen. -->
    <div v-if="pkg.pendingRequests.length" class="pkg-asks">
      <div v-for="req in pkg.pendingRequests" :key="req.id" class="ask-row">
        <div class="ask-body">
          <div class="ask-line">
            <span class="ask-who">{{ req.email }}</span>
            asked to <span class="ask-kind">reopen this</span>
            <span class="ask-when">· {{ reasonLabels[req.reason] }} · {{ agoText(req.createdAt) }}</span>
          </div>
          <p v-if="req.message" class="ask-msg">{{ req.message }}</p>
        </div>
        <button
          class="ask-x"
          title="Dismiss request"
          :aria-label="`Dismiss request from ${req.email}`"
          @click="emit('dismissRequest', req.id)"
        >
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M18 6 6 18M6 6l12 12"/></svg>
        </button>
      </div>
      <p class="ask-hint">
        Extending answers every request here at once and emails each person the working link.
      </p>
    </div>

    <div v-if="showExtend" class="pkg-extend">
      <div class="ex-fields">
        <label class="ex-f">
          <span class="ex-lab">More days</span>
          <input v-model.number="extendDays" class="inp" type="number" min="1" max="30" step="1" />
        </label>
        <label class="ex-f">
          <span class="ex-lab">Downloads per file</span>
          <input v-model.number="extendMaxDownloads" class="inp" type="number" min="1" step="1" placeholder="Unlimited" />
        </label>
      </div>
      <p class="ex-note">
        Counted from today, so the link is live for {{ extendValid ? extendDays : '—' }} more
        day{{ extendDays === 1 ? '' : 's' }}. Leave downloads blank for unlimited; this replaces the current limit
        rather than adding to it.<template v-if="displayStatus === 'revoked'"> This also un-revokes the package.</template>
      </p>
      <div class="ex-actions">
        <button class="btn sm btn-primary" :disabled="!extendValid || extending" @click="submitExtend">
          {{ extending ? 'Extending…' : 'Confirm' }}
        </button>
        <button class="btn sm" :disabled="extending" @click="showExtend = false">Cancel</button>
      </div>
    </div>

    <div class="pkg-foot">
      <button class="btn sm" @click="emit('copyLink')">
        <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M10 13a5 5 0 0 0 7 0l3-3a5 5 0 0 0-7-7l-1 1"/><path d="M14 11a5 5 0 0 0-7 0l-3 3a5 5 0 0 0 7 7l1-1"/></svg>
        Copy link
      </button>
      <button class="btn sm" @click="emit('preview')">Preview</button>
      <span class="spacer"></span>
      <button
        v-if="!showExtend"
        class="btn sm"
        :class="{ 'btn-primary': pkg.pendingRequests.length > 0 }"
        @click="openExtend"
      >
        {{ displayStatus === 'active' ? 'Extend' : 'Reopen' }}
      </button>
      <button
        class="btn sm"
        :class="{ notifying: pkg.notifyOnDownload }"
        :title="pkg.notifyOnDownload ? 'Stop emailing me when this is downloaded' : 'Email me when this is downloaded'"
        :aria-pressed="pkg.notifyOnDownload"
        @click="emit('setNotify', !pkg.notifyOnDownload)"
      >
        <svg v-if="pkg.notifyOnDownload" width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M18 8a6 6 0 0 0-12 0c0 7-3 9-3 9h18s-3-2-3-9"/><path d="M13.73 21a2 2 0 0 1-3.46 0"/></svg>
        <svg v-else width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M8.7 3A6 6 0 0 1 18 8c0 7 3 9 3 9H6"/><path d="M13.73 21a2 2 0 0 1-3.46 0"/><path d="M2 2l20 20"/></svg>
        Notify {{ pkg.notifyOnDownload ? 'on' : 'off' }}
      </button>
      <button v-if="displayStatus === 'active'" class="btn sm btn-danger" @click="emit('revoke')">Revoke</button>
    </div>
  </div>
</template>
