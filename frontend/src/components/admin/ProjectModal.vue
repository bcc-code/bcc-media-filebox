<script setup lang="ts">
import { reactive, watch, computed } from 'vue'
import type { Project } from '../../composables/useAdmin'

const props = defineProps<{ project: Project | null }>()
const emit = defineEmits<{
  (e: 'cancel'): void
  (e: 'save', body: { name: string; code: string }): void
}>()

const draft = reactive({ name: '', code: '' })

watch(
  () => props.project,
  (p) => {
    draft.name = p?.name ?? ''
    draft.code = p?.code ?? ''
  },
  { immediate: true },
)

const isEdit = computed(() => !!props.project)
const codeValid = computed(() => /^[A-Za-z0-9_-]+$/.test(draft.code.trim()))
const valid = computed(() => draft.name.trim() !== '' && codeValid.value)

function onSave() {
  if (!valid.value) return
  emit('save', { name: draft.name.trim(), code: draft.code.trim() })
}
</script>

<template>
  <div class="modal-bg" @click.self="emit('cancel')">
    <div class="modal fb-fade">
      <h2>{{ isEdit ? 'Edit project' : 'New project' }}</h2>
      <div class="sub">The display name is what uploaders pick; the code is embedded in the resulting filename.</div>

      <div class="field">
        <label>Display name</label>
        <input v-model="draft.name" placeholder="e.g. Summer Camp 2026" autofocus />
      </div>

      <div class="field">
        <label>Code</label>
        <input class="mono" v-model="draft.code" placeholder="e.g. SMR26" />
        <div class="hint">
          Used verbatim in filenames. Letters, digits, '-' and '_' only.
          <span v-if="draft.code && !codeValid" style="color:var(--danger)">Invalid characters.</span>
        </div>
      </div>

      <div class="modal-actions">
        <button class="btn btn-ghost" @click="emit('cancel')">Cancel</button>
        <button class="btn btn-primary" :disabled="!valid" @click="onSave">
          {{ isEdit ? 'Save changes' : 'Create project' }}
        </button>
      </div>
    </div>
  </div>
</template>
