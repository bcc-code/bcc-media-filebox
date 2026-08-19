<script setup lang="ts">
import { ref } from 'vue'
import { useRoute } from 'vue-router'
import AppLogo from '../components/AppLogo.vue'
import { muteNotifications } from '../composables/usePackages'
import '../assets/send.css'

// Public: the token in the link is the only credential, since the author's mail
// client may have no FileBox session. A button, not an on-load call — mail
// scanners follow links, and one must not silence anything by itself.
const route = useRoute()
const token = route.params.token as string

const submitting = ref(false)
const packageName = ref<string | null>(null)
const error = ref<string | null>(null)

async function confirm() {
  submitting.value = true
  error.value = null
  try {
    packageName.value = (await muteNotifications(token)).packageName
  } catch (e) {
    error.value = (e as Error).message
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <div class="send-root">
    <div class="public-stage" style="min-height: 100vh; border-radius: 0; border: none">
      <div class="public-card fb-fade">
        <div class="public-brand"><AppLogo class="mark" /><span class="name">FileBox</span></div>

        <template v-if="packageName !== null">
          <div class="public-from">Download notifications are off</div>
          <div class="public-pkgname">{{ packageName || 'This package' }}</div>
          <p class="public-note" style="margin-top: 14px">
            You won't be emailed about this package again. The link still works for its recipients — turn
            notifications back on from the package's card under Sent packages.
          </p>
          <a class="btn btn-primary btn-block" href="/send?tab=sent" style="margin-top: 18px">Open Sent packages</a>
        </template>

        <template v-else>
          <div class="public-from">Stop download notifications?</div>
          <p class="public-note" style="margin-top: 12px">
            FileBox will stop emailing you when this package is downloaded. Nothing else changes: the package stays
            live and its recipients keep their access.
          </p>
          <div v-if="error" class="verify-error" style="margin-top: 14px">{{ error }}</div>
          <button class="btn btn-primary btn-block" :disabled="submitting" style="margin-top: 18px" @click="confirm">
            {{ submitting ? 'Turning off…' : 'Stop these notifications' }}
          </button>
        </template>
      </div>
    </div>
  </div>
</template>
