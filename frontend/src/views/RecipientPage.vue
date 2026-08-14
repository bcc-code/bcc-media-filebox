<script setup lang="ts">
import { onMounted } from 'vue'
import { useRoute } from 'vue-router'
import AppLogo from '../components/AppLogo.vue'
import PackageVerifyScreen from '../components/send/PackageVerifyScreen.vue'
import PackageDownloadScreen from '../components/send/PackageDownloadScreen.vue'
import { usePackagePreview } from '../composables/usePackagePreview'
import '../assets/send.css'

const route = useRoute()
const packageId = route.params.packageId as string

const { preview, loading, error, verifying, verifyError, load, verifyPassword, recordDownload } = usePackagePreview()

onMounted(() => load(packageId))

function signInBcc() {
  window.location.href = '/auth/login/bcc'
}
</script>

<template>
  <div class="send-root">
    <div class="public-stage" style="min-height: 100vh; border-radius: 0; border: none">
      <div class="public-card fb-fade">
        <div class="public-brand"><AppLogo class="mark" /><span class="name">FileBox</span></div>

        <div v-if="loading" style="text-align: center; color: var(--ink-3); padding: 20px 0">Loading…</div>

        <div v-else-if="error" style="text-align: center">
          <div class="terminal-icon" style="margin: 0 auto 20px">
            <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="9"/><path d="M12 8v5M12 16h.01"/></svg>
          </div>
          <h2 class="verify-h">This link isn't available</h2>
          <p class="verify-p">{{ error }}</p>
        </div>

        <template v-else-if="preview">
          <PackageVerifyScreen
            v-if="!preview.verified"
            :verification-method="preview.verificationMethod"
            :submitting="verifying"
            :error="verifyError"
            @submit-password="(pw) => verifyPassword(packageId, pw)"
            @sign-in-bcc="signInBcc"
          />
          <PackageDownloadScreen
            v-else
            :package-name="preview.name"
            :message="preview.message"
            :files="preview.files ?? []"
            :expires-at="preview.expiresAt"
            :max-downloads="preview.maxDownloads"
            :interactive="true"
            @downloaded="recordDownload"
          />
        </template>
      </div>
    </div>
  </div>
</template>
