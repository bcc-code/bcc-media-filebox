<script setup lang="ts">
import { computed, ref } from 'vue'

const props = withDefaults(
  defineProps<{
    // 0 = unlimited; 1 = single file (form targets that derive one filename).
    maxFiles?: number
    disabled?: boolean
  }>(),
  { maxFiles: 0, disabled: false },
)

const emit = defineEmits<{
  files: [files: FileList]
}>()

const isDragging = ref(false)
const fileInput = ref<HTMLInputElement | null>(null)
const allowMultiple = computed(() => props.maxFiles !== 1)

// When the target caps the upload at one file, keep only the first dropped/
// selected file so a multi-select can't sneak past the single-file form.
function limit(files: FileList): FileList {
  if (props.maxFiles === 1 && files.length > 1) {
    const dt = new DataTransfer()
    dt.items.add(files[0])
    return dt.files
  }
  return files
}

function onDrop(e: DragEvent) {
  isDragging.value = false
  if (props.disabled) return
  if (e.dataTransfer?.files?.length) {
    emit('files', limit(e.dataTransfer.files))
  }
}

function onDragOver(e: DragEvent) {
  e.preventDefault()
  if (props.disabled) return
  isDragging.value = true
}

function onDragLeave() {
  isDragging.value = false
}

function openFilePicker() {
  if (props.disabled) return
  fileInput.value?.click()
}

function onFileSelected(e: Event) {
  const input = e.target as HTMLInputElement
  if (input.files?.length) {
    emit('files', limit(input.files))
    input.value = ''
  }
}
</script>

<template>
  <div
    @drop.prevent="onDrop"
    @dragover="onDragOver"
    @dragleave="onDragLeave"
    @click="openFilePicker"
    class="border-2 border-dashed rounded-xl p-12 text-center transition-colors"
    :class="[
      disabled
        ? 'border-gray-200 dark:border-gray-700 opacity-50 cursor-not-allowed'
        : 'cursor-pointer',
      !disabled && isDragging
        ? 'border-blue-500 bg-blue-50 dark:bg-blue-950'
        : !disabled
          ? 'border-gray-300 dark:border-gray-600 hover:border-gray-400 dark:hover:border-gray-500'
          : '',
    ]"
  >
    <input
      ref="fileInput"
      type="file"
      :multiple="allowMultiple"
      class="hidden"
      @change="onFileSelected"
    />
    <div class="text-gray-500 dark:text-gray-400">
      <svg class="mx-auto h-12 w-12 mb-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
        <path stroke-linecap="round" stroke-linejoin="round" d="M3 16.5v2.25A2.25 2.25 0 005.25 21h13.5A2.25 2.25 0 0021 18.75V16.5m-13.5-9L12 3m0 0l4.5 4.5M12 3v13.5" />
      </svg>
      <p class="text-lg font-medium">
        <span v-if="isDragging">Drop files here</span>
        <span v-else>Drag & drop files here, or click to browse</span>
      </p>
      <p class="text-sm mt-1">Supports files up to 300 GB with resumable upload</p>
    </div>
  </div>
</template>
