<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import AppHeader from '../components/AppHeader.vue'
import PackageComposeForm from '../components/share/PackageComposeForm.vue'
import SharedPackagesList from '../components/share/SharedPackagesList.vue'
import RecipientPreviewTab from '../components/share/RecipientPreviewTab.vue'
import { usePackages } from '../composables/usePackages'
import { useAuth } from '../composables/useAuth'
import '../assets/share.css'
import UiTabs, { type UiTabEntry } from '../components/ui/UiTabs.vue'

type View = 'compose' | 'shared' | 'preview'
const route = useRoute()

// ?tab=shared is what an access-request email links to (mail.ManageURL),
// landing the author on the package they were asked about rather than the
// compose form. Mail sent before the Send → Share rename spells it tab=sent,
// and those links never expire from an inbox, so both are accepted.
const tab = route.query.tab
const initialView: View =
  tab === 'shared' || tab === 'sent'
    ? 'shared'
    : tab === 'preview'
      ? 'preview'
      : 'compose'
const view = ref<View>(initialView)
const { total, fetchPackages } = usePackages()

const viewTabs = computed<UiTabEntry[]>(() => [
  { value: 'compose', label: 'New package' },
  { value: 'shared', label: 'Shared packages', count: total.value },
  { value: 'preview', label: 'Recipient preview' },
])
// Sharing is unavailable to guest sessions — the backend rejects both the
// package-creation call and share-flow uploads; this just explains why.
const { state: authState } = useAuth()
const isGuest = computed(() => authState.provider === 'guest')
const previewPackageId = ref<string | undefined>(
  typeof route.query.package === 'string' ? route.query.package : undefined,
)

onMounted(fetchPackages)

function onShared() {
  previewPackageId.value = undefined
  view.value = 'shared'
  fetchPackages()
}

function onPreview(packageId: string) {
  previewPackageId.value = packageId
  view.value = 'preview'
}
</script>

<template>
  <div class="share-root">
    <div class="page-wrap">
      <AppHeader />

      <h1 class="page-title">Share with people</h1>

      <template v-if="isGuest">
        <p class="page-sub">
          Sending files externally is not available for guest accounts. Sign in
          with your organisation account.
        </p>
        <p class="page-note">
          <router-link to="/" class="link">Back to Upload</router-link>
        </p>
      </template>

      <template v-else>
        <p class="page-sub">
          For sending material out to people outside BCC Media. They get an
          email with a secure download link — nothing is filed into the archive.
        </p>
        <p class="page-note">
          Handing material in instead?
          <router-link to="/" class="link">Upload to BCC Media</router-link>.
        </p>
        <p class="page-note">
          Files are permanently deleted from storage 90 days after upload —
          packages can no longer be renewed after that.
        </p>

        <UiTabs v-model="view" :tabs="viewTabs" list-class="tab-list-spaced">
          <template #compose>
            <PackageComposeForm @shared="onShared" />
          </template>
          <template #shared>
            <SharedPackagesList
              :focus-package-id="previewPackageId"
              @preview="onPreview"
              @focus-consumed="previewPackageId = undefined"
            />
          </template>
          <template #preview>
            <RecipientPreviewTab :selected-package-id="previewPackageId" />
          </template>
        </UiTabs>
      </template>
    </div>
  </div>
</template>
