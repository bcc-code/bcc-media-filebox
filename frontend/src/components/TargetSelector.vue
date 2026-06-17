<script setup lang="ts">
import type { TargetInfo } from '../types'

defineProps<{
  modelValue: string
  targets: TargetInfo[]
}>()

const emit = defineEmits<{
  'update:modelValue': [name: string]
}>()

function select(name: string) {
  emit('update:modelValue', name)
}
</script>

<template>
  <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
    <button
      v-for="t in targets"
      :key="t.name"
      type="button"
      :aria-pressed="t.name === modelValue"
      class="group flex flex-col gap-3 rounded-xl border p-4 text-left transition focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-500"
      :class="t.name === modelValue
        ? 'border-blue-500 bg-blue-50 dark:bg-blue-500/10 ring-1 ring-blue-500'
        : 'border-gray-300 dark:border-gray-600 hover:border-blue-400 bg-white dark:bg-gray-800'"
      @click="select(t.name)"
    >
      <div class="flex items-start justify-between">
        <!-- Generic disk icon -->
        <span
          class="flex h-10 w-10 items-center justify-center rounded-lg transition"
          :class="t.name === modelValue
            ? 'bg-blue-500/15 text-blue-600 dark:text-blue-400'
            : 'bg-gray-100 dark:bg-gray-700 text-gray-500 dark:text-gray-400'"
        >
          <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
            <ellipse cx="12" cy="5" rx="8" ry="3" />
            <path d="M4 5v6c0 1.66 3.58 3 8 3s8-1.34 8-3V5" />
            <path d="M4 11v6c0 1.66 3.58 3 8 3s8-1.34 8-3v-6" />
          </svg>
        </span>

        <!-- Selection indicator -->
        <span
          v-if="t.name === modelValue"
          class="flex h-6 w-6 items-center justify-center rounded-full bg-blue-500 text-white"
        >
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3" stroke-linecap="round" stroke-linejoin="round">
            <path d="M5 13l4 4L19 7" />
          </svg>
        </span>
        <span
          v-else
          class="h-6 w-6 rounded-full border-2 border-gray-300 dark:border-gray-600"
        />
      </div>

      <div class="font-semibold text-gray-900 dark:text-gray-100">{{ t.name }}</div>
    </button>
  </div>
</template>
