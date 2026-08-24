<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useTusUpload } from '../../composables/useTusUpload'
import { usePackages, type VerificationMethod } from '../../composables/usePackages'
import { getUserId } from '../../composables/useUserId'
import type { UploadRecord } from '../../types'
import PackageFileRow from './PackageFileRow.vue'
import RecipientChipInput from './RecipientChipInput.vue'
import VerificationMethodPicker from './VerificationMethodPicker.vue'

const emit = defineEmits<{ sent: [packageId: string] }>()

const { uploads, addFiles, cancelUpload, forgetUpload, addServerUpload } = useTusUpload()
const { createPackage } = usePackages()

// Send shows no target picker, so it names its destination symbolically and the
// tus pre-create hook resolves it (S3 when configured, else a real target).
// Must not be an empty string: Home submits that when a user has no grants.
const SEND_TARGET = 'send'
const SEND_DRAFT_UPLOADS_KEY = 'filebox-send-draft-uploads:'

function draftStorageKey(): string {
  return `${SEND_DRAFT_UPLOADS_KEY}${getUserId()}`
}

function readDraftUploadIds(): string[] {
  try {
    const value: unknown = JSON.parse(localStorage.getItem(draftStorageKey()) || '[]')
    if (!Array.isArray(value)) return []
    return [...new Set(value.filter((id): id is string => typeof id === 'string' && id.length > 0))]
  } catch {
    return []
  }
}

function saveDraftUploadIds(ids: string[]) {
  try {
    if (ids.length) localStorage.setItem(draftStorageKey(), JSON.stringify([...new Set(ids)]))
    else localStorage.removeItem(draftStorageKey())
  } catch {
    // Private browsing/storage policies may disable localStorage.
  }
}

const initialDraftUploadIds = readDraftUploadIds()
let hydratingDraft = initialDraftUploadIds.length > 0
const restoringDraft = ref(initialDraftUploadIds.length > 0)
const serverUploads = ref<UploadRecord[]>([])
const serverUploadsLoaded = ref(false)
const serverUploadsLoading = ref(false)
const serverUploadsError = ref<string | null>(null)
const unresolvedDraftUploadIds = ref<string[]>([])

const isDragging = ref(false)
const filePicker = ref<HTMLInputElement | null>(null)

function onDrop(e: DragEvent) {
  isDragging.value = false
  if (e.dataTransfer?.files?.length) addFiles(e.dataTransfer.files, SEND_TARGET)
}
function onDragOver(e: DragEvent) {
  e.preventDefault()
  isDragging.value = true
}
function onFilesPicked(e: Event) {
  const input = e.target as HTMLInputElement
  if (input.files?.length) addFiles(input.files, SEND_TARGET)
  input.value = ''
}

const totalSize = computed(() => uploads.value.reduce((s, f) => s + f.bytesTotal, 0))

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

const name = ref('')
const recipients = ref<string[]>([])
const message = ref('')
const expiresInDays = ref(7)
const maxDownloads = ref<number | ''>('')
const verify = ref<VerificationMethod>('none')
const password = ref('')
const notify = ref(true)
const sending = ref(false)
const sendError = ref<string | null>(null)

const isEmail = (r: string) => /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(r)
const validRecipients = computed(() => recipients.value.filter(isEmail))
// A typo used to be dropped silently: no mail, and no way for that person to ask
// for the package later, since only its recipients can.
const badRecipients = computed(() => recipients.value.filter((r) => !isEmail(r)))
// Blank means unlimited, anything else must be at least 1.
const maxDownloadsValid = computed(() => maxDownloads.value === '' || maxDownloads.value >= 1)
const completedUploads = computed(() => uploads.value.filter((u) => u.status === 'completed' && u.uploadId))
const stillUploading = computed(() => uploads.value.some((u) => u.status === 'uploading' || u.status === 'pending'))
const restoredUploadCount = computed(() => uploads.value.filter((u) => u.restored).length)

// Persist only completed server IDs. File objects cannot be reconstructed after
// a reload, while an upload ID can be safely revalidated against the caller's
// server-side upload history.
watch(
  () => completedUploads.value.map((upload) => upload.uploadId as string),
  (ids) => {
    if (!hydratingDraft) saveDraftUploadIds([...unresolvedDraftUploadIds.value, ...ids])
  },
  { flush: 'sync' },
)

async function loadServerUploads(force = false): Promise<boolean> {
  if (serverUploadsLoaded.value && !force) return true
  if (serverUploadsLoading.value) return false

  serverUploadsLoading.value = true
  serverUploadsError.value = null
  try {
    const response = await fetch(`/api/uploads?user_id=${encodeURIComponent(getUserId())}`)
    if (!response.ok) throw new Error(`Upload history returned ${response.status}`)
    const records: unknown = await response.json()
    if (!Array.isArray(records)) throw new Error('Upload history returned an invalid response')
    serverUploads.value = records as UploadRecord[]
    serverUploadsLoaded.value = true
    return true
  } catch {
    serverUploadsError.value = 'Could not load files that are already on the server.'
    return false
  } finally {
    serverUploadsLoading.value = false
  }
}

async function restoreDraftUploads(force = false) {
  const draftIds = readDraftUploadIds()
  if (!draftIds.length) {
    hydratingDraft = false
    restoringDraft.value = false
    return
  }

  hydratingDraft = true
  restoringDraft.value = true
  const loaded = await loadServerUploads(force)
  if (loaded) {
    const byId = new Map(serverUploads.value.map((record) => [record.id, record]))
    const unresolved: string[] = []
    for (const id of draftIds) {
      const record = byId.get(id)
      if (record) addServerUpload(record)
      else unresolved.push(id)
    }
    unresolvedDraftUploadIds.value = unresolved
  } else {
    unresolvedDraftUploadIds.value = draftIds
  }
  hydratingDraft = false
  restoringDraft.value = false

  const selected = completedUploads.value.map((upload) => upload.uploadId as string)
  // On a transient fetch failure retain the old IDs as well as any upload that
  // completed meanwhile, so Retry can still recover the entire draft.
  saveDraftUploadIds([...unresolvedDraftUploadIds.value, ...selected])
}

function forgetUnresolvedDraftUploads() {
  unresolvedDraftUploadIds.value = []
  saveDraftUploadIds(completedUploads.value.map((upload) => upload.uploadId as string))
}

onMounted(() => void restoreDraftUploads())

const verifyLabels: Partial<Record<VerificationMethod, string>> = {
  none: 'No verification',
  bcc_login: 'BCC login',
  password: 'Password',
}
const verifyName = computed(() => verifyLabels[verify.value] ?? verify.value)

const canSend = computed(
  () =>
    completedUploads.value.length > 0 &&
    !stillUploading.value &&
    name.value.trim().length > 0 &&
    badRecipients.value.length === 0 &&
    maxDownloadsValid.value &&
    (verify.value !== 'password' || password.value.trim().length > 0) &&
    !restoringDraft.value &&
    unresolvedDraftUploadIds.value.length === 0 &&
    !sending.value,
)

const blockReason = computed(() => {
  if (restoringDraft.value) return 'Restoring files already uploaded to the server.'
  if (unresolvedDraftUploadIds.value.length) {
    return `Wait for ${unresolvedDraftUploadIds.value.length} saved upload${unresolvedDraftUploadIds.value.length === 1 ? '' : 's'} to become available, or forget the missing selection.`
  }
  if (uploads.value.length === 0) return 'Add at least one file.'
  if (stillUploading.value) return 'Wait for files to finish uploading.'
  if (completedUploads.value.length === 0) return 'At least one file must finish uploading.'
  if (!name.value.trim()) return 'Give the package a name.'
  if (badRecipients.value.length) return `Fix or remove ${badRecipients.value.join(', ')} — not a valid email address.`
  if (!maxDownloadsValid.value) return 'Max downloads per artifact must be 1 or more, or blank for unlimited.'
  if (verify.value === 'password' && !password.value.trim()) return 'Set a password.'
  return ''
})

async function send() {
  if (!canSend.value) return
  sending.value = true
  sendError.value = null
  try {
    const result = await createPackage({
      name: name.value.trim(),
      message: message.value.trim() || undefined,
      uploadIds: completedUploads.value.map((u) => u.uploadId as string),
      recipients: validRecipients.value.length ? validRecipients.value : undefined,
      expiresInDays: expiresInDays.value,
      maxDownloads: maxDownloads.value === '' ? undefined : maxDownloads.value,
      verificationMethod: verify.value,
      password: verify.value === 'password' ? password.value : undefined,
      notifyOnDownload: notify.value,
    })
    emit('sent', result.packageId)
    // Completed uploads now belong to the package and must stay on the server;
    // anything else here never made it in and should be cleaned up.
    for (const u of [...uploads.value]) {
      if (u.status === 'completed') forgetUpload(u)
      else cancelUpload(u)
    }
    name.value = ''
    recipients.value = []
    message.value = ''
    expiresInDays.value = 7
    maxDownloads.value = ''
    verify.value = 'none'
    password.value = ''
    notify.value = true
  } catch (e) {
    sendError.value = (e as Error).message
  } finally {
    sending.value = false
  }
}
</script>

<template>
  <div class="compose">
    <div class="col-main">
      <!-- Files -->
      <div class="block">
        <div class="block-label"><span class="num" :class="{ done: uploads.length }">1</span> Files</div>
        <div
          class="dropzone"
          :class="{ dragover: isDragging }"
          @click="filePicker?.click()"
          @dragenter.prevent="isDragging = true"
          @dragover="onDragOver"
          @dragleave.prevent="isDragging = false"
          @drop.prevent="onDrop"
        >
          <svg class="ic" width="34" height="34" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><path d="M12 16V4M6 10l6-6 6 6M4 20h16"/></svg>
          <div class="t">Drag &amp; drop files, or click to browse</div>
          <div class="s">Add as many as you like — they'll be sent as one package</div>
          <input ref="filePicker" type="file" multiple hidden @change="onFilesPicked" />
        </div>
        <div v-if="restoringDraft || restoredUploadCount" class="draft-restore-status" aria-live="polite">
          <span v-if="restoringDraft">Restoring your uploaded files…</span>
          <span v-else>
            {{ restoredUploadCount }} file{{ restoredUploadCount === 1 ? '' : 's' }} restored after reload
          </span>
        </div>

        <div v-if="serverUploadsError && initialDraftUploadIds.length" class="draft-restore-error" role="alert">
          <span>{{ serverUploadsError }} Your draft IDs are still saved.</span>
          <button type="button" @click="restoreDraftUploads(true)">Retry</button>
        </div>
        <div v-else-if="unresolvedDraftUploadIds.length" class="draft-restore-error pending" role="status">
          <span>
            {{ unresolvedDraftUploadIds.length }} saved upload{{ unresolvedDraftUploadIds.length === 1 ? '' : 's' }}
            {{ unresolvedDraftUploadIds.length === 1 ? 'is' : 'are' }} still being finalized or no longer available.
          </span>
          <span class="draft-restore-actions">
            <button type="button" @click="restoreDraftUploads(true)">Retry</button>
            <button type="button" @click="forgetUnresolvedDraftUploads">Forget missing</button>
          </span>
        </div>
        <div v-if="uploads.length" class="file-list">
          <PackageFileRow v-for="item in uploads" :key="item.id" :item="item" @remove="cancelUpload(item)" />
        </div>
        <div v-if="uploads.length" class="files-total">{{ uploads.length }} file{{ uploads.length === 1 ? '' : 's' }} · {{ fmtBytes(totalSize) }}</div>
      </div>

      <!-- Package name -->
      <div class="block">
        <div class="block-label"><span class="num" :class="{ done: name.trim() }">2</span> Package name</div>
        <input v-model="name" class="inp" placeholder="e.g. Sommerstevne 2026 — masters" maxlength="80" />
      </div>

      <!-- Recipients -->
      <div class="block">
        <div class="block-label"><span class="num" :class="{ done: validRecipients.length }">3</span> Recipients <span class="opt">optional</span></div>
        <RecipientChipInput v-model="recipients" />
      </div>

      <!-- Message -->
      <div class="block">
        <div class="block-label">Message <span class="opt">optional</span></div>
        <textarea v-model="message" class="inp" placeholder="Add a short note for your recipients…" maxlength="500"></textarea>
      </div>

      <!-- Expiration -->
      <div class="block">
        <div class="block-label"><span class="num done">4</span> Expiration</div>
        <div class="grid-2">
          <div class="sel-wrap">
            <select v-model.number="expiresInDays" class="inp">
              <option :value="1">Expires in 1 day</option>
              <option :value="7">Expires in 7 days</option>
              <option :value="14">Expires in 14 days</option>
              <option :value="30">Expires in 30 days</option>
            </select>
            <svg class="chev" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M6 9l6 6 6-6"/></svg>
          </div>
          <input v-model="maxDownloads" class="inp" type="number" min="1" step="1" placeholder="Max downloads per item" />
        </div>
      </div>

      <!-- Verification -->
      <div class="block">
        <div class="block-label"><span class="num done">5</span> Verification</div>
        <VerificationMethodPicker v-model="verify" v-model:password="password" />
      </div>

      <!-- Notify -->
      <div class="block">
        <div class="toggle-row">
          <div class="tbody">
            <div class="tname">Notify me on download</div>
            <div class="tdesc">
              Email me what was downloaded. Downloads close together are collected into one email, and you can
              turn this off later from the package's card.
            </div>
          </div>
          <button
            type="button"
            class="switch"
            :class="{ on: notify }"
            role="switch"
            :aria-checked="notify"
            aria-label="Notify me on download"
            @click="notify = !notify"
          ><span class="knob"></span></button>
        </div>
      </div>

      <div v-if="sendError" class="verify-error">{{ sendError }}</div>

      <div class="compose-actions">
        <button class="btn btn-primary btn-block" :disabled="!canSend" @click="send">
          {{ sending ? 'Sending…' : 'Send package' }}
        </button>
      </div>
    </div>

    <!-- Summary rail -->
    <aside class="summary">
      <h3>Summary</h3>
      <div class="sum-line"><span class="k">Files</span><span class="v mono">{{ uploads.length }} · {{ fmtBytes(totalSize) }}</span></div>
      <div class="sum-line"><span class="k">Name</span><span class="v">{{ name.trim() || '—' }}</span></div>
      <div class="sum-line"><span class="k">Recipients</span><span class="v">{{ validRecipients.length || '—' }}</span></div>
      <div class="sum-line"><span class="k">Expires</span><span class="v">In {{ expiresInDays }} day{{ expiresInDays === 1 ? '' : 's' }}</span></div>
      <div class="sum-line"><span class="k">Downloads / item</span><span class="v mono">{{ maxDownloads === '' ? 'unlimited' : `max ${maxDownloads}` }}</span></div>
      <div class="sum-line"><span class="k">Verification</span><span class="v">{{ verifyName }}</span></div>
      <div class="sum-line"><span class="k">Notify</span><span class="v">{{ notify ? 'On' : 'Off' }}</span></div>
      <button class="btn btn-primary btn-block" style="margin-top: 16px" :disabled="!canSend" @click="send">
        {{ sending ? 'Sending…' : 'Send package' }}
      </button>
      <div v-if="!canSend" style="font-size: 11.5px; color: var(--ink-3); margin-top: 10px; text-align: center; line-height: 1.5">{{ blockReason }}</div>
    </aside>
  </div>
</template>
