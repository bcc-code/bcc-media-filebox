<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useAdmin } from '../../composables/useAdmin'
import { formatBytes, relTime } from '../../composables/adminHelpers'

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
        <div class="sub">Recent completed form uploads. Re-trigger fires the destination target's webhook again with the same sidecar payload as the original upload.</div>
      </div>
      <button class="btn btn-ghost" @click="loadAdminUploads()">
        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21 12a9 9 0 1 1-2.6-6.4M21 3v6h-6"/></svg>
        Refresh
      </button>
    </div>

    <div v-if="adminUploads.length === 0" class="empty">
      No form uploads yet.
    </div>

    <div v-else class="card">
      <table>
        <thead>
          <tr>
            <th>File</th>
            <th>Target</th>
            <th>Uploader</th>
            <th style="text-align:right">Size</th>
            <th>When</th>
            <th></th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="u in adminUploads" :key="u.id">
            <td><span class="mono" style="font-size:12.5px">{{ u.filename }}</span></td>
            <td>{{ u.targetName }}</td>
            <td><span style="color:var(--ink-2)">{{ u.uploaderEmail || '—' }}</span></td>
            <td style="text-align:right"><span class="mono" style="font-size:12.5px">{{ formatBytes(u.size) }}</span></td>
            <td><span style="font-size:13px;color:var(--ink-2)">{{ relTime(u.when) }}</span></td>
            <td class="actions">
              <button
                class="btn btn-sm btn-ghost"
                :disabled="!u.webhookConfigured || sending === u.id"
                :title="u.webhookConfigured ? 'Re-send the webhook for this upload' : 'No webhook configured on this target'"
                @click="onRetrigger(u.id)"
              >
                {{ sending === u.id ? 'Sending…' : '↻ Webhook' }}
              </button>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>
