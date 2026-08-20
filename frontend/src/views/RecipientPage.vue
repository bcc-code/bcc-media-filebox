<script setup lang="ts">
import { onMounted } from 'vue'
import { useRoute } from 'vue-router'
import AppLogo from '../components/AppLogo.vue'
import PackageVerifyScreen from '../components/send/PackageVerifyScreen.vue'
import PackageDownloadScreen from '../components/send/PackageDownloadScreen.vue'
import PackageUnavailableScreen from '../components/send/PackageUnavailableScreen.vue'
import PackageAccessRequestForm from '../components/send/PackageAccessRequestForm.vue'
import { usePackagePreview } from '../composables/usePackagePreview'
import '../assets/send.css'

const route = useRoute()
const packageId = route.params.packageId as string

const {
  preview,
  loading,
  error,
  unavailable,
  verifying,
  verifyError,
  requesting,
  requestError,
  requestSent,
  load,
  verifyPassword,
  requestAccess,
  recordDownload,
  allDownloadsExhausted,
} = usePackagePreview()

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

        <template v-else-if="error">
          <PackageUnavailableScreen heading="This link isn't available" :message="error" />
          <!-- Only a package that existed and stopped working can be reopened, so
               the form rides on `unavailable` rather than on `error`. -->
          <PackageAccessRequestForm
            v-if="unavailable"
            :reason="unavailable.reason"
            :sender-name="unavailable.senderName"
            :recipients-only="unavailable.recipientsOnly"
            :submitting="requesting"
            :error="requestError"
            :sent="requestSent"
            :interactive="true"
            @submit="(email, message) => requestAccess(packageId, email, message)"
          />
        </template>

        <template v-else-if="preview">
          <PackageVerifyScreen
            v-if="!preview.verified"
            :verification-method="preview.verificationMethod"
            :submitting="verifying"
            :error="verifyError"
            @submit-password="(pw) => verifyPassword(packageId, pw)"
            @sign-in-bcc="signInBcc"
          />
          <template v-else>
            <PackageDownloadScreen
              :package-name="preview.name"
              :sender-name="preview.senderName"
              :message="preview.message"
              :files="preview.files ?? []"
              :downloads="preview.downloads ?? []"
              :expires-at="preview.expiresAt"
              :max-downloads="preview.maxDownloads"
              :preparation-status="preview.preparationStatus"
              :preparation-bytes-done="preview.preparationBytesDone"
              :preparation-bytes-total="preview.preparationBytesTotal"
              :preparation-progress="preview.preparationProgress"
              :preparation-error="preview.preparationError"
              :interactive="true"
              @downloaded="recordDownload"
            />
            <PackageAccessRequestForm
              v-if="allDownloadsExhausted"
              reason="limit_reached"
              :sender-name="preview.senderName"
              :recipients-only="preview.recipientsOnly"
              :submitting="requesting"
              :error="requestError"
              :sent="requestSent"
              :interactive="true"
              @submit="(email, message) => requestAccess(packageId, email, message)"
            />
          </template>
        </template>
      </div>
    </div>
  </div>
</template>
