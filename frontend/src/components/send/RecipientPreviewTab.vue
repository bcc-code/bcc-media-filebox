<script setup lang="ts">
import { onMounted, ref, watch } from 'vue'
import { usePackages } from '../../composables/usePackages'
import { usePackagePreview } from '../../composables/usePackagePreview'
import AppLogo from '../AppLogo.vue'
import PackageVerifyScreen from './PackageVerifyScreen.vue'
import PackageDownloadScreen from './PackageDownloadScreen.vue'
import PackageUnavailableScreen from './PackageUnavailableScreen.vue'
import PackageAccessRequestForm from './PackageAccessRequestForm.vue'

const props = defineProps<{ selectedPackageId?: string }>()

const { packages, fetchPackages } = usePackages()
const selected = ref(props.selectedPackageId ?? '')
const { preview, loading, error, unavailable, verifying, verifyError, load, verifyPassword, allDownloadsExhausted } =
  usePackagePreview()

onMounted(async () => {
  if (!packages.value.length) await fetchPackages()
  if (!selected.value && packages.value.length) selected.value = packages.value[0].packageId
})

watch(
  () => props.selectedPackageId,
  (id) => {
    if (id) selected.value = id
  },
)

watch(
  selected,
  (id) => {
    if (id) load(id)
  },
  { immediate: true },
)

function signInBcc() {
  const returnTo = encodeURIComponent(window.location.pathname + window.location.search)
  window.location.href = `/auth/login/bcc?returnTo=${returnTo}`
}
</script>

<template>
  <div class="preview-bar">
    <span class="pl">
      <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M2 12s3.5-7 10-7 10 7 10 7-3.5 7-10 7-10-7-10-7z"/><circle cx="12" cy="12" r="3"/></svg>
      This is what your recipient sees
    </span>
    <span class="spacer"></span>
    <div v-if="packages.length" class="sel-wrap" style="width: 260px">
      <select v-model="selected" class="inp">
        <option v-for="p in packages" :key="p.packageId" :value="p.packageId">{{ p.name }}</option>
      </select>
      <svg class="chev" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M6 9l6 6 6-6"/></svg>
    </div>
  </div>

  <div v-if="!packages.length" class="empty">Send a package first to preview it here.</div>
  <div v-else-if="loading" class="empty">Loading…</div>
  <div v-else class="public-stage">
    <div class="public-card fb-fade" style="text-align: center">
      <template v-if="error">
        <PackageUnavailableScreen :heading="error" />
        <!-- interactive=false: this is the author previewing their own package,
             so the form renders but cannot mail them their own request. -->
        <div v-if="unavailable" style="text-align: left">
          <PackageAccessRequestForm
            :reason="unavailable.reason"
            :sender-name="unavailable.senderName"
            :recipients-only="unavailable.recipientsOnly"
            :interactive="false"
          />
        </div>
      </template>
      <template v-else-if="preview">
        <div class="public-brand" style="justify-content: center"><AppLogo class="mark" /><span class="name">FileBox</span></div>
        <div style="text-align: left">
          <PackageVerifyScreen
            v-if="!preview.verified"
            :verification-method="preview.verificationMethod"
            :submitting="verifying"
            :error="verifyError"
            @submit-password="(pw) => verifyPassword(selected, pw)"
            @sign-in-bcc="signInBcc"
          />
          <template v-else-if="allDownloadsExhausted">
            <PackageUnavailableScreen
              heading="Download limit reached"
              message="Every file in this package has reached its download limit."
            />
            <PackageAccessRequestForm
              reason="limit_reached"
              :sender-name="preview.senderName"
              :recipients-only="preview.recipientsOnly"
              :interactive="false"
            />
          </template>
          <PackageDownloadScreen
            v-else
            :package-name="preview.name"
            :sender-name="preview.senderName"
            :message="preview.message"
            :files="preview.files ?? []"
            :expires-at="preview.expiresAt"
            :max-downloads="preview.maxDownloads"
            :interactive="false"
          />
        </div>
      </template>
    </div>
  </div>
</template>
