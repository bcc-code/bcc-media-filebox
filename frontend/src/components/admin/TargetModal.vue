<script setup lang="ts">
import { reactive, watch, computed } from 'vue'
import type { Target } from '../../composables/useAdmin'
import { registry } from '../../forms'

const props = defineProps<{ target: Target | null }>()
const emit = defineEmits<{
  (e: 'cancel'): void
  (e: 'save', body: { name: string; path: string; formKey: string | null }): void
}>()

const draft = reactive({ name: '', path: '', formKey: '' })

const formOptions = computed(() => Object.values(registry).map((f) => ({ key: f.key, label: f.label })))

watch(
  () => props.target,
  (t) => {
    draft.name = t?.name ?? ''
    draft.path = t?.path ?? ''
    draft.formKey = t?.formKey ?? ''
  },
  { immediate: true },
)

const isEdit = computed(() => !!props.target)
const valid = computed(() => draft.name.trim() && draft.path.trim())

function onSave() {
  if (!valid.value) return
  emit('save', {
    name: draft.name.trim(),
    path: draft.path.trim(),
    formKey: draft.formKey || null,
  })
}
</script>

<template>
  <div class="modal-bg" @click.self="emit('cancel')">
    <div class="modal fb-fade">
      <h2>{{ isEdit ? 'Edit target' : 'New upload target' }}</h2>
      <div class="sub">Maps a friendly name visible to uploaders to a real folder path on the storage backend.</div>

      <div class="field">
        <label>Display name</label>
        <input v-model="draft.name" placeholder="e.g. Upload to BCC Media (Isilon)" autofocus />
      </div>

      <div class="field">
        <label>Folder path</label>
        <input class="mono" v-model="draft.path" placeholder="/mnt/isilon/filebox/incoming" />
        <div class="hint">Path must exist and be writable on the server's filesystem.</div>
      </div>

      <div class="field">
        <label>Upload form</label>
        <select v-model="draft.formKey">
          <option value="">None — free upload</option>
          <option v-for="f in formOptions" :key="f.key" :value="f.key">{{ f.label }}</option>
        </select>
        <div class="hint">Forms collect structured details and derive the filename from them.</div>
      </div>

      <div class="modal-actions">
        <button class="btn btn-ghost" @click="emit('cancel')">Cancel</button>
        <button class="btn btn-primary" :disabled="!valid" @click="onSave">
          {{ isEdit ? 'Save changes' : 'Create target' }}
        </button>
      </div>
    </div>
  </div>
</template>
