<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import AppLogo from '../components/AppLogo.vue'
import UserMenu from '../components/UserMenu.vue'
import PackageComposeForm from '../components/send/PackageComposeForm.vue'
import SentPackagesList from '../components/send/SentPackagesList.vue'
import RecipientPreviewTab from '../components/send/RecipientPreviewTab.vue'
import { usePackages } from '../composables/usePackages'
import { useAuth } from '../composables/useAuth'
import '../assets/send.css'
import UiTabs, { type UiTabEntry } from '../components/ui/UiTabs.vue'

type View = 'compose' | 'sent' | 'preview'
const route = useRoute()

// ?tab=sent is what an access-request email links to (mail.ManageURL), landing
// the author on the package they were asked about rather than the compose form.
const initialView: View =
  route.query.tab === 'sent' || route.query.tab === 'preview'
    ? (route.query.tab as View)
    : 'compose'
const view = ref<View>(initialView)
const { total, fetchPackages } = usePackages()

const viewTabs = computed<UiTabEntry[]>(() => [
  { value: 'compose', label: 'New package' },
  { value: 'sent', label: 'Sent packages', count: total.value },
  { value: 'preview', label: 'Recipient preview' },
])
// Send is unavailable to guest sessions — the backend rejects both the
// package-creation call and Send-flow uploads; this just explains why.
const { state: authState } = useAuth()
const isGuest = computed(() => authState.provider === 'guest')
const previewPackageId = ref<string | undefined>(
  typeof route.query.package === 'string' ? route.query.package : undefined,
)

onMounted(fetchPackages)

function onSent() {
  previewPackageId.value = undefined
  view.value = 'sent'
  fetchPackages()
}

function onPreview(packageId: string) {
  previewPackageId.value = packageId
  view.value = 'preview'
}
</script>

<template>
  <div class="send-root">
    <div class="page-wrap">
      <div class="app-header">
        <router-link to="/" class="app-brand">
          <AppLogo class="mark" />
          <span class="name">FileBox</span>
        </router-link>
        <nav class="app-nav">
          <router-link to="/">Upload</router-link>
          <router-link to="/send" class="active">Send</router-link>
        </nav>
        <span class="spacer"></span>
        <UserMenu />
      </div>

      <h1 class="page-title">Send files</h1>

      <template v-if="isGuest">
        <p class="page-sub">
          Send is not available for guest accounts. Sign in with your
          organisation account to send packages.
        </p>
        <p class="page-note">
          <router-link to="/">Back to Upload</router-link>
        </p>
      </template>

      <template v-else>
        <p class="page-sub">
          Bundle files into a single package and send a secure download link to
          anyone by email.
        </p>
        <p class="page-note">
          Files are permanently deleted from storage 90 days after upload —
          packages can no longer be renewed after that.
        </p>

        <UiTabs v-model="view" :tabs="viewTabs" list-class="tab-list-spaced">
          <template #compose>
            <PackageComposeForm @sent="onSent" />
          </template>
          <template #sent>
            <SentPackagesList
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
