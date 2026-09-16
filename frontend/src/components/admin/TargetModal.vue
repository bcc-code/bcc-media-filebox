<script setup lang="ts">
import UiDialog from '../ui/UiDialog.vue'
import { reactive, watch, computed } from 'vue'
import type { Target } from '../../composables/useAdmin'
import { registry } from '../../forms'
import UiSelect from '../ui/UiSelect.vue'

const props = defineProps<{ target: Target | null }>()
const emit = defineEmits<{
  (e: 'cancel'): void
  (
    e: 'save',
    body: {
      name: string
      path: string
      formKey: string | null
      webhookUrl: string | null
    },
  ): void
}>()

const draft = reactive({ name: '', path: '', formKey: '', webhookUrl: '' })

const formOptions = computed(() => [
  // '' is a real choice here, not a placeholder — it means "no form".
  { value: '', label: 'None — free upload' },
  ...Object.values(registry).map((f) => ({ value: f.key, label: f.label })),
])

watch(
  () => props.target,
  (t) => {
    draft.name = t?.name ?? ''
    draft.path = t?.path ?? ''
    draft.formKey = t?.formKey ?? ''
    draft.webhookUrl = t?.webhookUrl ?? ''
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
    webhookUrl: draft.webhookUrl.trim() || null,
  })
}
</script>

<template>
  <UiDialog
    :title="isEdit ? 'Edit target' : 'New upload target'"
    description="Maps a friendly name visible to uploaders to a real folder path on the storage backend."
    @close="emit('cancel')"
  >
    <div class="field">
      <label>Display name</label>
      <input
        v-model="draft.name"
        placeholder="e.g. Upload to BCC Media (Isilon)"
        autofocus
      />
    </div>

    <div class="field">
      <label>Folder path</label>
      <input
        class="mono"
        v-model="draft.path"
        placeholder="/mnt/isilon/filebox/incoming"
      />
      <div class="hint">
        Path must exist and be writable on the server's filesystem.
      </div>
    </div>

    <div class="field">
      <label>Upload form</label>
      <UiSelect
        v-model="draft.formKey"
        :options="formOptions"
        aria-label="Upload form"
      />
      <div class="hint">
        Forms collect structured details and derive the filename from them.
      </div>
    </div>

    <div class="field">
      <label>Webhook URL</label>
      <input
        class="mono"
        v-model="draft.webhookUrl"
        placeholder="https://example.com/hook"
      />
      <div class="hint">
        POSTed a JSON body with the sidecar name and path when an upload
        completes. Only fires for targets bound to a form, which produce a JSON
        sidecar.
      </div>
    </div>

    <template #actions>
      <button class="btn btn-ghost" @click="emit('cancel')">Cancel</button>
      <button class="btn btn-primary" :disabled="!valid" @click="onSave">
        {{ isEdit ? 'Save changes' : 'Create target' }}
      </button>
    </template>
  </UiDialog>
</template>
