<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import AppLogo from '../components/AppLogo.vue'
import SendUserMenu from '../components/send/SendUserMenu.vue'
import PackageComposeForm from '../components/send/PackageComposeForm.vue'
import SentPackagesList from '../components/send/SentPackagesList.vue'
import RecipientPreviewTab from '../components/send/RecipientPreviewTab.vue'
import { usePackages } from '../composables/usePackages'
import '../assets/send.css'

type View = 'compose' | 'sent' | 'preview'
const route = useRoute()

// ?tab=sent is what an access-request email links to (mail.ManageURL), landing
// the author on the package they were asked about rather than the compose form.
const initialView: View = route.query.tab === 'sent' || route.query.tab === 'preview' ? (route.query.tab as View) : 'compose'
const view = ref<View>(initialView)
const { total, fetchPackages } = usePackages()
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
      <div class="header">
        <router-link to="/" class="brand">
          <AppLogo class="mark" />
          <span class="name">FileBox</span>
        </router-link>
        <nav class="nav">
          <router-link to="/">Upload</router-link>
          <router-link to="/send" class="active">Send</router-link>
        </nav>
        <span class="spacer"></span>
        <SendUserMenu />
      </div>

      <h1 class="page-title">Send files</h1>
      <p class="page-sub">Bundle files into a single package and send a secure download link to anyone by email.</p>

      <div class="view-tabs">
        <button :class="{ active: view === 'compose' }" @click="view = 'compose'">New package</button>
        <button :class="{ active: view === 'sent' }" @click="view = 'sent'">
          Sent packages <span class="count">{{ total }}</span>
        </button>
        <button :class="{ active: view === 'preview' }" @click="view = 'preview'">Recipient preview</button>
      </div>

      <PackageComposeForm v-if="view === 'compose'" @sent="onSent" />
      <SentPackagesList v-else-if="view === 'sent'" @preview="onPreview" />
      <RecipientPreviewTab v-else :selected-package-id="previewPackageId" />
    </div>
  </div>
</template>
