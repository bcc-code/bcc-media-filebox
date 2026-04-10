<script setup lang="ts">
import { ref, watch } from 'vue'
import { useTusUpload } from './composables/useTusUpload'
import FileUploader from './components/FileUploader.vue'
import UploadProgress from './components/UploadProgress.vue'
import UploadList from './components/UploadList.vue'

const { uploads, addFiles, pauseUpload, resumeUpload, retryUpload, cancelUpload } = useTusUpload()
const uploadList = ref<InstanceType<typeof UploadList> | null>(null)

function onFiles(files: FileList) {
  addFiles(files)
}

// Refresh the upload list when any upload completes
watch(
  () => uploads.value.filter(u => u.status === 'completed').length,
  () => {
    uploadList.value?.refresh()
  },
)
</script>

<template>
  <div class="min-h-screen bg-gray-50 dark:bg-gray-900">
    <div class="max-w-3xl mx-auto px-4 py-12">
      <h1 class="text-3xl font-bold text-gray-900 dark:text-gray-100 mb-8">File Pusher</h1>

      <FileUploader @files="onFiles" />

      <div v-if="uploads.length > 0" class="mt-8 space-y-3">
        <h2 class="text-lg font-semibold text-gray-900 dark:text-gray-100 mb-4">Active Uploads</h2>
        <UploadProgress
          v-for="item in uploads"
          :key="item.id"
          :item="item"
          @pause="pauseUpload"
          @resume="resumeUpload"
          @retry="retryUpload"
          @cancel="cancelUpload"
        />
      </div>

      <div class="mt-12">
        <UploadList ref="uploadList" />
      </div>
    </div>
  </div>
</template>
