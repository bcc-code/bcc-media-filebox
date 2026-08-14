<script setup lang="ts">
import { onMounted } from 'vue'
import { useRoute } from 'vue-router'
import AppLogo from '../components/AppLogo.vue'
import PackageVerifyScreen from '../components/send/PackageVerifyScreen.vue'
import PackageDownloadScreen from '../components/send/PackageDownloadScreen.vue'
import PackageUnavailableScreen from '../components/send/PackageUnavailableScreen.vue'
import { usePackagePreview } from '../composables/usePackagePreview'
import '../assets/send.css'

const route = useRoute()
const packageId = route.params.packageId as string

const { preview, loading, error, verifying, verifyError, load, verifyPassword, recordDownload, allDownloadsExhausted } =
  usePackagePreview()

onMounted(() => load(packageId))

function signInBcc() {
  const returnTo = encodeURIComponent(window.location.pathname + window.location.search)
  window.location.href = `/auth/login/bcc?returnTo=${returnTo}`
}
</script>

<template>
  <div class="send-root">
    <div class="public-stage" style="min-height: 100vh; border-radius: 0; border: none">
      <div class="public-card fb-fade">
        <div class="public-brand"><AppLogo class="mark" /><span class="name">FileBox</span></div>

        <div v-if="loading" style="text-align: center; color: var(--ink-3); padding: 20px 0">Loading…</div>

        <PackageUnavailableScreen v-else-if="error" heading="This link isn't available" :message="error" />

        <template v-else-if="preview">
          <PackageVerifyScreen
            v-if="!preview.verified"
            :verification-method="preview.verificationMethod"
            :submitting="verifying"
            :error="verifyError"
            @submit-password="(pw) => verifyPassword(packageId, pw)"
            @sign-in-bcc="signInBcc"
          />
          <PackageUnavailableScreen
            v-else-if="allDownloadsExhausted"
            heading="Download limit reached"
            message="Every file in this package has reached its download limit."
          />
          <PackageDownloadScreen
            v-else
            :package-name="preview.name"
            :sender-name="preview.senderName"
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
