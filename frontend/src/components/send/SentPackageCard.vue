<script setup lang="ts">
import { computed } from 'vue'
import type { PackageInfo, VerificationMethod } from '../../composables/usePackages'

const props = defineProps<{ pkg: PackageInfo }>()
const emit = defineEmits<{ copyLink: []; preview: []; revoke: [] }>()

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
      <span v-if="displayStatus === 'active'">{{ expiryText(pkg.expiresAt) }}</span>
      <span v-else-if="displayStatus === 'revoked'">revoked</span>
      <span v-else>{{ expiryText(pkg.expiresAt) }}</span>
    </div>
    <div class="pkg-foot">
      <button class="btn sm" @click="emit('copyLink')">
        <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M10 13a5 5 0 0 0 7 0l3-3a5 5 0 0 0-7-7l-1 1"/><path d="M14 11a5 5 0 0 0-7 0l-3 3a5 5 0 0 0 7 7l1-1"/></svg>
        Copy link
      </button>
      <button class="btn sm" @click="emit('preview')">Preview</button>
      <span class="spacer"></span>
      <button v-if="displayStatus === 'active'" class="btn sm btn-danger" @click="emit('revoke')">Revoke</button>
      <button v-else class="btn sm" disabled style="opacity: .5; cursor: default">{{ displayStatus === 'revoked' ? 'Revoked' : 'Expired' }}</button>
    </div>
  </div>
</template>
