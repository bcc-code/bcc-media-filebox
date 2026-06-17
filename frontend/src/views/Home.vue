<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useTusUpload } from '../composables/useTusUpload'
import FileUploader from '../components/FileUploader.vue'
import UploadForm from '../components/UploadForm.vue'
import UploadProgress from '../components/UploadProgress.vue'
import UploadList from '../components/UploadList.vue'
import AppLogo from '../components/AppLogo.vue'
import AuthMenu from '../components/AuthMenu.vue'
import TargetSelector from '../components/TargetSelector.vue'
import type { TargetInfo } from '../types'
import { getForm, isFormValid, type Option } from '../forms'

const { uploads, addFiles, pauseUpload, resumeUpload, retryUpload, cancelUpload } = useTusUpload()
const uploadList = ref<InstanceType<typeof UploadList> | null>(null)
const targets = ref<TargetInfo[]>([])
const target = ref('')
// Per-target form field values, keyed by target name so switching back and
// forth keeps what the user typed.
const formValues = ref<Record<string, Record<string, string>>>({})
// Top-level DB catalogs (no scope), keyed by optionsSource name.
const catalogs = ref<Record<string, Option[]>>({})
// Dependent (scoped) options, keyed by field key.
const scopedOptions = ref<Record<string, Option[]>>({})
// Last scope value fetched per dependent field, to avoid refetching.
const scopeLoaded: Record<string, string> = {}
// Autocomplete suggestions for the current scope, keyed by field key.
const suggestions = ref<Record<string, string[]>>({})

// Endpoints for catalog-backed select options.
const catalogEndpoints: Record<string, string> = {
  projects: '/api/projects',
  arrangements: '/api/arrangements',
}
const scopedEndpoints: Record<string, (code: string) => string> = {
  subEvents: (code) => `/api/arrangements/${encodeURIComponent(code)}/sub-events`,
}

function toOptions(rows: { name: string; code: string }[]): Option[] {
  return rows.map((r) => ({ code: r.code, label: r.name }))
}

async function loadCatalog(source: string) {
  if (catalogs.value[source] || !catalogEndpoints[source]) return
  try {
    const res = await fetch(catalogEndpoints[source])
    catalogs.value = { ...catalogs.value, [source]: toOptions(await res.json()) }
  } catch {
    /* leave empty on failure */
  }
}

onMounted(async () => {
  const res = await fetch('/api/targets')
  targets.value = await res.json()
  target.value = targets.value[0]?.name ?? ''
})

const selectedTarget = computed(() => targets.value.find((t) => t.name === target.value) ?? null)
const activeForm = computed(() => getForm(selectedTarget.value?.formKey))
const currentValues = computed(() => formValues.value[target.value] ?? {})

// Supply select options: scoped fields use their fetched per-scope list, plain
// catalog fields use the top-level catalog.
const dynamicOptions = computed<Record<string, Option[]>>(() => {
  const map: Record<string, Option[]> = {}
  for (const f of activeForm.value?.fields ?? []) {
    if (!f.optionsSource) continue
    map[f.key] = f.optionsScope ? (scopedOptions.value[f.key] ?? []) : (catalogs.value[f.optionsSource] ?? [])
  }
  return map
})

// The field key whose value scopes the autocomplete (e.g. "project").
const scopeFieldKey = computed(() => activeForm.value?.fields.find((f) => f.suggest)?.suggestScope ?? null)

function setValues(values: Record<string, string>) {
  formValues.value = { ...formValues.value, [target.value]: values }
}

// When the active form changes, load its top-level catalogs and reset scoped state.
watch(
  activeForm,
  (f) => {
    scopedOptions.value = {}
    for (const k of Object.keys(scopeLoaded)) delete scopeLoaded[k]
    for (const field of f?.fields ?? []) {
      if (field.optionsSource && !field.optionsScope) loadCatalog(field.optionsSource)
    }
  },
  { immediate: true },
)

// Fetch dependent options when their scope field's value changes, and clear the
// dependent field so a stale child selection can't survive a parent change.
watch(
  [activeForm, currentValues],
  () => {
    const f = activeForm.value
    if (!f) return
    for (const field of f.fields) {
      if (!field.optionsSource || !field.optionsScope) continue
      const scopeVal = currentValues.value[field.optionsScope] ?? ''
      if (scopeLoaded[field.key] === scopeVal) continue
      scopeLoaded[field.key] = scopeVal
      if (currentValues.value[field.key]) {
        setValues({ ...currentValues.value, [field.key]: '' })
      }
      if (!scopeVal) {
        scopedOptions.value = { ...scopedOptions.value, [field.key]: [] }
        continue
      }
      const ep = scopedEndpoints[field.optionsSource]
      if (!ep) continue
      fetch(ep(scopeVal))
        .then((r) => r.json())
        .then((rows) => {
          scopedOptions.value = { ...scopedOptions.value, [field.key]: toOptions(rows) }
        })
        .catch(() => {})
    }
  },
  { immediate: true },
)

// Refetch season/episode suggestions whenever the scoping project changes.
watch(
  () => (scopeFieldKey.value ? currentValues.value[scopeFieldKey.value] : undefined),
  async (code) => {
    if (!code) {
      suggestions.value = {}
      return
    }
    try {
      const res = await fetch(`/api/projects/${encodeURIComponent(code)}/suggestions`)
      const data: { seasons: string[]; episodes: string[] } = await res.json()
      suggestions.value = { season: data.seasons, episode: data.episodes }
    } catch {
      suggestions.value = {}
    }
  },
)

// For form targets the picker is gated until required fields are valid.
const canUpload = computed(() => !activeForm.value || isFormValid(activeForm.value, currentValues.value))

function onFiles(files: FileList) {
  if (!canUpload.value) return
  const form = activeForm.value
  // Snapshot the values so resetting the form below can't race the upload's
  // metadata, which is read asynchronously when the tus upload starts.
  const snapshot = { ...currentValues.value }
  addFiles(files, target.value, form ? snapshot : undefined)
  if (form?.resetFields?.length) {
    const next = { ...snapshot }
    for (const k of form.resetFields) delete next[k]
    setValues(next)
  }
}

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
      <div class="flex gap-4 items-center text-gray-900 dark:text-gray-100 mb-8">
        <AppLogo class="w-10 h-10" />
        <h1 class="text-3xl font-bold">FileBox</h1>
        <div class="ml-auto">
          <AuthMenu />
        </div>
      </div>

      <div class="mb-6">
        <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">Target</label>
        <TargetSelector v-model="target" :targets="targets" />
      </div>

      <div v-if="activeForm" class="mb-6">
        <UploadForm
          :form="activeForm"
          :model-value="currentValues"
          :dynamic-options="dynamicOptions"
          :suggestions="suggestions"
          @update:model-value="setValues"
        />
        <p v-if="!canUpload" class="mt-2 text-sm text-amber-600 dark:text-amber-400">
          Fill in the required fields above before uploading.
        </p>
      </div>

      <FileUploader
        :max-files="activeForm?.maxFiles ?? 0"
        :disabled="!canUpload"
        @files="onFiles"
      />

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
