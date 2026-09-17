<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useTusUpload } from '../composables/useTusUpload'
import UiFileUpload from '../components/ui/UiFileUpload.vue'
import { notifyError } from '../composables/useToast'
import UploadForm from '../components/UploadForm.vue'
import UploadProgress from '../components/UploadProgress.vue'
import UploadList from '../components/UploadList.vue'
import AppHeader from '../components/AppHeader.vue'
import TargetSelector from '../components/TargetSelector.vue'
import type { TargetInfo } from '../types'
import { getForm, isFormValid, type Option } from '../forms'

const {
  uploads,
  addFiles,
  pauseUpload,
  resumeUpload,
  retryUpload,
  cancelUpload,
} = useTusUpload()
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
  subEvents: (code) =>
    `/api/arrangements/${encodeURIComponent(code)}/sub-events`,
}

function toOptions(rows: { name: string; code: string }[]): Option[] {
  return rows.map((r) => ({ code: r.code, label: r.name }))
}

async function loadCatalog(source: string) {
  if (catalogs.value[source] || !catalogEndpoints[source]) return
  try {
    const res = await fetch(catalogEndpoints[source])
    catalogs.value = {
      ...catalogs.value,
      [source]: toOptions(await res.json()),
    }
  } catch {
    /* leave empty on failure */
  }
}

onMounted(async () => {
  const res = await fetch('/api/targets')
  targets.value = await res.json()
  target.value = targets.value[0]?.name ?? ''
})

const selectedTarget = computed(
  () => targets.value.find((t) => t.name === target.value) ?? null,
)
const activeForm = computed(() => getForm(selectedTarget.value?.formKey))
const currentValues = computed(() => formValues.value[target.value] ?? {})

// Supply select options: scoped fields use their fetched per-scope list, plain
// catalog fields use the top-level catalog.
const dynamicOptions = computed<Record<string, Option[]>>(() => {
  const map: Record<string, Option[]> = {}
  for (const f of activeForm.value?.fields ?? []) {
    if (!f.optionsSource) continue
    map[f.key] = f.optionsScope
      ? (scopedOptions.value[f.key] ?? [])
      : (catalogs.value[f.optionsSource] ?? [])
  }
  return map
})

// The field key whose value scopes the autocomplete (e.g. "project").
const scopeFieldKey = computed(
  () => activeForm.value?.fields.find((f) => f.suggest)?.suggestScope ?? null,
)

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
      if (field.optionsSource && !field.optionsScope)
        loadCatalog(field.optionsSource)
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
          scopedOptions.value = {
            ...scopedOptions.value,
            [field.key]: toOptions(rows),
          }
        })
        .catch(() => {})
    }
  },
  { immediate: true },
)

// Refetch season/episode suggestions whenever the scoping project changes.
watch(
  () =>
    scopeFieldKey.value ? currentValues.value[scopeFieldKey.value] : undefined,
  async (code) => {
    if (!code) {
      suggestions.value = {}
      return
    }
    try {
      const res = await fetch(
        `/api/projects/${encodeURIComponent(code)}/suggestions`,
      )
      const data: { seasons: string[]; episodes: string[] } = await res.json()
      suggestions.value = { season: data.seasons, episode: data.episodes }
    } catch {
      suggestions.value = {}
    }
  },
)

// For form targets the picker is gated until required fields are valid.
const canUpload = computed(
  () => !activeForm.value || isFormValid(activeForm.value, currentValues.value),
)

function onFiles(files: File[]) {
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
  () => uploads.value.filter((u) => u.status === 'completed').length,
  () => {
    uploadList.value?.refresh()
  },
)
</script>

<template>
  <div class="upload-root">
    <div class="page-wrap">
      <AppHeader />

      <h1 class="page-title">Upload files</h1>
      <p class="page-sub">
        Pick a target, fill in what it needs, then drop your files.
      </p>

      <div class="field">
        <label>Target</label>
        <TargetSelector v-model="target" :targets="targets" />
      </div>

      <div v-if="activeForm" class="upload-section">
        <UploadForm
          :form="activeForm"
          :model-value="currentValues"
          :dynamic-options="dynamicOptions"
          :suggestions="suggestions"
          @update:model-value="setValues"
        />
        <p v-if="!canUpload" class="form-gate-note">
          Fill in the required fields above before uploading.
        </p>
      </div>

      <UiFileUpload
        :max-files="activeForm?.maxFiles ?? 0"
        :disabled="!canUpload"
        hint="Supports files up to 300 GB with resumable upload"
        @files="onFiles"
        @reject="notifyError"
      />

      <div v-if="uploads.length > 0" class="upload-section">
        <h2 class="section-title">Active uploads</h2>
        <div class="upload-stack">
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
      </div>

      <div class="upload-section">
        <UploadList ref="uploadList" />
      </div>
    </div>
  </div>
</template>

<style scoped>
.upload-root {
  min-height: 100vh;
}
.upload-root :deep(*),
.upload-root :deep(*::before),
.upload-root :deep(*::after) {
  box-sizing: border-box;
}

.upload-section {
  margin-top: 32px;
}

.section-title {
  margin: 0 0 14px;
  font-size: var(--text-title-2);
  line-height: var(--text-title-2--line-height);
  font-weight: var(--text-title-2--font-weight);
  color: var(--color-ink-2);
}

.upload-stack {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.form-gate-note {
  margin: 10px 0 0;
  font-size: 13px;
  color: var(--color-warn);
}
</style>
