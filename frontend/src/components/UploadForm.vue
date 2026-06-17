<script setup lang="ts">
import { computed } from 'vue'
import { buildFilename, type Form, type Option } from '../forms'

const props = defineProps<{
  form: Form
  modelValue: Record<string, string>
  // Options for select fields whose optionsSource is DB-backed (keyed by field key).
  dynamicOptions?: Record<string, Option[]>
  // Autocomplete suggestions for free-text fields (keyed by field key).
  suggestions?: Record<string, string[]>
}>()

const emit = defineEmits<{
  'update:modelValue': [values: Record<string, string>]
}>()

function setField(key: string, value: string) {
  emit('update:modelValue', { ...props.modelValue, [key]: value })
}

function optionsFor(fieldKey: string, source: string | undefined, staticOpts: Option[] | undefined): Option[] {
  if (source) return props.dynamicOptions?.[fieldKey] ?? []
  return staticOpts ?? []
}

function tooShort(field: { minLength?: number }, value: string): boolean {
  return !!field.minLength && value.trim() !== '' && value.trim().length < field.minLength
}

// Live preview of the derived filename. The backend re-derives it
// authoritatively on upload; this is purely informational.
const previewName = computed(() => buildFilename(props.form, props.modelValue, '.ext'))
</script>

<template>
  <div class="rounded-xl border border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-800 p-5">
    <div class="flex items-center gap-2 mb-4">
      <span class="inline-flex items-center gap-1.5 rounded-full bg-blue-50 dark:bg-blue-500/10 px-2.5 py-1 text-xs font-medium text-blue-700 dark:text-blue-300">
        <span class="h-1.5 w-1.5 rounded-full bg-blue-500" />
        {{ form.label }}
      </span>
      <span class="text-sm text-gray-600 dark:text-gray-400">{{ form.description }}</span>
    </div>

    <div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
      <div v-for="field in form.fields" :key="field.key" class="flex flex-col gap-1.5">
        <label class="flex items-center gap-1.5 text-sm font-medium text-gray-700 dark:text-gray-300">
          {{ field.label }}
          <span v-if="field.required" class="text-blue-500">*</span>
          <span v-else class="text-[10px] uppercase tracking-wide text-gray-400">Optional</span>
          <span v-if="field.maxLength" class="ml-auto text-xs text-gray-400">
            {{ (modelValue[field.key] ?? '').length }}/{{ field.maxLength }}
          </span>
        </label>

        <select
          v-if="field.type === 'select'"
          :value="modelValue[field.key] ?? ''"
          class="rounded-lg border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-900 px-3 py-2 text-sm text-gray-900 dark:text-gray-100 focus:outline-none focus:ring-2 focus:ring-blue-500"
          @change="setField(field.key, ($event.target as HTMLSelectElement).value)"
        >
          <option v-if="field.placeholder" value="">{{ field.placeholder }}</option>
          <option v-for="opt in optionsFor(field.key, field.optionsSource, field.options)" :key="opt.code" :value="opt.code">{{ opt.label }}</option>
        </select>

        <template v-else>
          <input
            :type="field.type === 'number' ? 'number' : 'text'"
            :value="modelValue[field.key] ?? ''"
            :placeholder="field.placeholder"
            :maxlength="field.maxLength || undefined"
            :list="field.suggest ? `${form.key}-${field.key}-list` : undefined"
            class="rounded-lg border bg-white dark:bg-gray-900 px-3 py-2 text-sm text-gray-900 dark:text-gray-100 focus:outline-none focus:ring-2 focus:ring-blue-500"
            :class="tooShort(field, modelValue[field.key] ?? '') ? 'border-amber-400' : 'border-gray-300 dark:border-gray-600'"
            @input="setField(field.key, ($event.target as HTMLInputElement).value)"
          />
          <datalist v-if="field.suggest" :id="`${form.key}-${field.key}-list`">
            <option v-for="s in suggestions?.[field.key] ?? []" :key="s" :value="s" />
          </datalist>
          <span v-if="tooShort(field, modelValue[field.key] ?? '')" class="text-xs text-amber-600 dark:text-amber-400">
            At least {{ field.minLength }} characters.
          </span>
        </template>
      </div>
    </div>

    <div class="mt-5 rounded-lg bg-gray-50 dark:bg-gray-900/60 px-4 py-3 flex items-center gap-3">
      <span class="text-[10px] font-semibold uppercase tracking-wider text-gray-400">Resulting filename</span>
      <span class="font-mono text-sm text-gray-700 dark:text-gray-300">{{ previewName }}</span>
    </div>
  </div>
</template>
