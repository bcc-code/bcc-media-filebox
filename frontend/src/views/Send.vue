<script setup lang="ts">
import { onMounted, ref } from 'vue'
import AppLogo from '../components/AppLogo.vue'
import SendUserMenu from '../components/send/SendUserMenu.vue'
import PackageComposeForm from '../components/send/PackageComposeForm.vue'
import SentPackagesList from '../components/send/SentPackagesList.vue'
import RecipientPreviewTab from '../components/send/RecipientPreviewTab.vue'
import { usePackages } from '../composables/usePackages'
import '../assets/send.css'

type View = 'compose' | 'sent' | 'preview'
const view = ref<View>('compose')
const { total, fetchPackages } = usePackages()
const sentList = ref<InstanceType<typeof SentPackagesList> | null>(null)
const previewPackageId = ref<string | undefined>(undefined)

onMounted(fetchPackages)

function onSent() {
  previewPackageId.value = undefined
  view.value = 'sent'
  sentList.value?.refresh()
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
      <SentPackagesList v-else-if="view === 'sent'" ref="sentList" @preview="onPreview" />
      <RecipientPreviewTab v-else :selected-package-id="previewPackageId" />
    </div>
  </div>
</template>
