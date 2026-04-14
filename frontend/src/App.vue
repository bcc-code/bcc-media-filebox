<script setup lang="ts">
import { onMounted, ref, watch } from 'vue'
import { useTusUpload } from './composables/useTusUpload'
import FileUploader from './components/FileUploader.vue'
import UploadProgress from './components/UploadProgress.vue'
import UploadList from './components/UploadList.vue'

const { uploads, addFiles, pauseUpload, resumeUpload, retryUpload, cancelUpload } = useTusUpload()
const uploadList = ref<InstanceType<typeof UploadList> | null>(null)
const targets = ref<string[]>([])
const target = ref('')

onMounted(async () => {
  const res = await fetch('/api/targets')
  targets.value = await res.json()
  target.value = targets.value[0] ?? ''
})

function onFiles(files: FileList) {
  addFiles(files, target.value)
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

      <div class="mb-6">
        <label for="target" class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Target</label>
        <select
          id="target"
          v-model="target"
          class="block w-full rounded-lg border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 px-3 py-2 shadow-sm focus:border-blue-500 focus:ring-blue-500"
        >
          <option v-for="t in targets" :key="t" :value="t">{{ t }}</option>
        </select>
      </div>

      <FileUploader @files="onFiles" />

      <div v-if="uploads.length > 0" class="mt-8 space-y-3">
        <h2 class="text-lg font-semibold text-gray-900 dark:text-gray-100 mb-4">Active Uploads</h2>
        <UploadProgress
          v-for="item in [...uploads].reverse()"
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
