<script setup lang="ts">
import { computed, ref } from 'vue'
import { useTusUpload } from '../../composables/useTusUpload'
import { usePackages, type VerificationMethod } from '../../composables/usePackages'
import PackageFileRow from './PackageFileRow.vue'
import RecipientChipInput from './RecipientChipInput.vue'
import VerificationMethodPicker from './VerificationMethodPicker.vue'

const emit = defineEmits<{ sent: [packageId: string] }>()

const { uploads, addFiles, cancelUpload, forgetUpload } = useTusUpload()
const { createPackage } = usePackages()

// Send never shows the caller a target picker, so it names its destination
// symbolically: SEND_TARGET says "wherever Send files belong" and the backend's
// tus pre-create hook resolves it (S3 when configured, else a real target).
// It must be this explicit value rather than an empty string — Home also
// submits an empty target when the signed-in user has no granted targets, and
// the backend cannot tell the two apart.
const SEND_TARGET = 'send'

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
const maxDownloads = ref('')
const verify = ref<VerificationMethod>('none')
const password = ref('')
const notify = ref(true)
const sending = ref(false)
const sendError = ref<string | null>(null)

const validRecipients = computed(() => recipients.value.filter((r) => /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(r)))
const completedUploads = computed(() => uploads.value.filter((u) => u.status === 'completed' && u.uploadId))
const stillUploading = computed(() => uploads.value.some((u) => u.status === 'uploading' || u.status === 'pending'))

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
    (verify.value !== 'password' || password.value.trim().length > 0) &&
    !sending.value,
)

const blockReason = computed(() => {
  if (uploads.value.length === 0) return 'Add at least one file.'
  if (stillUploading.value) return 'Wait for files to finish uploading.'
  if (completedUploads.value.length === 0) return 'At least one file must finish uploading.'
  if (!name.value.trim()) return 'Give the package a name.'
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
      maxDownloads: maxDownloads.value ? parseInt(maxDownloads.value, 10) : undefined,
      verificationMethod: verify.value,
      password: verify.value === 'password' ? password.value : undefined,
      notifyOnDownload: notify.value,
    })
    emit('sent', result.packageId)
    // reset — completed uploads are now owned by the package and must stay on
    // the server; anything else here (e.g. a stray paused upload) never made
    // it into the package and should actually be cleaned up.
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
          <input v-model="maxDownloads" class="inp" type="number" min="1" step="1" placeholder="Max downloads (optional)" />
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
            <div class="tdesc">Email me each time a recipient downloads this package.</div>
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
      <div class="sum-line"><span class="k">Downloads</span><span class="v mono">{{ maxDownloads ? `max ${maxDownloads}` : 'unlimited' }}</span></div>
      <div class="sum-line"><span class="k">Verification</span><span class="v">{{ verifyName }}</span></div>
      <div class="sum-line"><span class="k">Notify</span><span class="v">{{ notify ? 'On' : 'Off' }}</span></div>
      <button class="btn btn-primary btn-block" style="margin-top: 16px" :disabled="!canSend" @click="send">
        {{ sending ? 'Sending…' : 'Send package' }}
      </button>
      <div v-if="!canSend" style="font-size: 11.5px; color: var(--ink-3); margin-top: 10px; text-align: center; line-height: 1.5">{{ blockReason }}</div>
    </aside>
  </div>
</template>
