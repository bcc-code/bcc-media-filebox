<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useAdmin } from '../../composables/useAdmin'
import {
  formatBytes,
  relTime,
  storageLabel,
} from '../../composables/adminHelpers'
import UiTooltip from '../ui/UiTooltip.vue'
import UiButton from '../ui/UiButton.vue'
import UiBadge from '../ui/UiBadge.vue'

const { adminUploads, loadAdminUploads, retriggerWebhook } = useAdmin()

// Track the upload currently being re-triggered so its button can show a
// pending state and we don't fire twice.
const sending = ref<string | null>(null)

onMounted(() => {
  loadAdminUploads()
})

async function onRetrigger(id: string) {
  if (sending.value) return
  sending.value = id
  try {
    await retriggerWebhook(id)
  } finally {
    sending.value = null
  }
}
</script>

<template>
  <div class="fb-fade">
    <div class="section-head">
      <div>
        <h1>Uploads</h1>
        <div class="sub">
          Recent completed uploads across all users. "Assembling" means the
          bytes have arrived but the file is still being put together and moved
          into its target, so it is not on disk yet. Re-trigger fires the
          destination target's webhook again with the same sidecar payload as
          the original upload.
        </div>
      </div>
      <UiButton variant="ghost" @click="loadAdminUploads()">
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
          <path d="M21 12a9 9 0 1 1-2.6-6.4M21 3v6h-6" />
        </svg>
        Refresh
      </UiButton>
    </div>

    <div v-if="adminUploads.length === 0" class="empty">No uploads yet.</div>

    <div v-else class="card">
      <table>
        <thead>
          <tr>
            <th>File</th>
            <th>Target</th>
            <th>Uploader</th>
            <th style="text-align: right">Size</th>
            <th>When</th>
            <th>Status</th>
            <th></th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="u in adminUploads" :key="u.id">
            <td>
              <span class="mono" style="font-size: 12.5px">{{
                u.filename
              }}</span>
            </td>
            <td>{{ u.targetName }}</td>
            <td>
              <span style="color: var(--color-ink-2)">{{
                u.uploaderEmail || '—'
              }}</span>
            </td>
            <td style="text-align: right">
              <span class="mono" style="font-size: 12.5px">{{
                formatBytes(u.size)
              }}</span>
            </td>
            <td>
              <span style="font-size: 13px; color: var(--color-ink-2)">{{
                relTime(u.when)
              }}</span>
            </td>
            <td>
              <UiBadge :variant="storageLabel(u.storageStatus).variant">{{
                storageLabel(u.storageStatus).text
              }}</UiBadge>
            </td>
            <td class="actions">
              <UiTooltip
                :label="
                  u.webhookConfigured
                    ? 'Re-send the webhook for this upload'
                    : u.storageStatus !== 'ready'
                      ? 'Available once the upload is in storage'
                      : 'No webhook or sidecar for this upload'
                "
              >
                <UiButton
                  size="sm"
                  variant="ghost"
                  :disabled="!u.webhookConfigured"
                  :loading="sending === u.id"
                  loading-label="Sending…"
                  @click="onRetrigger(u.id)"
                >
                  ↻ Webhook
                </UiButton>
              </UiTooltip>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>
