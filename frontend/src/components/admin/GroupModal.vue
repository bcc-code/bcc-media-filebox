<script setup lang="ts">
import UiDialog from '../ui/UiDialog.vue'
import UiTagsInput from '../ui/UiTagsInput.vue'
import { reactive, watch, computed } from 'vue'
import type { Group } from '../../composables/useAdmin'
import UiButton from '../ui/UiButton.vue'

const props = defineProps<{ group: Group | null }>()
const emit = defineEmits<{
  (e: 'cancel'): void
  (
    e: 'save',
    body: { name: string; description: string; members: string[] },
  ): void
}>()

const draft = reactive<{
  name: string
  description: string
  members: string[]
}>({
  name: '',
  description: '',
  members: [],
})
watch(
  () => props.group,
  (g) => {
    draft.name = g?.name ?? ''
    draft.description = g?.description ?? ''
    draft.members = g ? [...g.members] : []
  },
  { immediate: true },
)

const isEdit = computed(() => !!props.group)
const valid = computed(() => draft.name.trim().length > 0)

function onSave() {
  if (!valid.value) return
  emit('save', {
    name: draft.name.trim(),
    description: draft.description.trim(),
    members: [...draft.members],
  })
}
</script>

<template>
  <UiDialog
    :title="isEdit ? 'Edit group' : 'New custom group'"
    description="A custom group is a named bundle of users. Use it when you want to grant the same target access to several specific people at once."
    width="560px"
    @close="emit('cancel')"
  >
    <div class="field">
      <label>Group name</label>
      <input v-model="draft.name" placeholder="e.g. Camera dept." autofocus />
    </div>

    <div class="field">
      <label
        >Description
        <span
          style="
            text-transform: none;
            letter-spacing: 0;
            color: var(--color-ink-3);
          "
          >(optional)</span
        ></label
      >
      <input
        v-model="draft.description"
        placeholder="Short note about who this group is for"
      />
    </div>

    <div class="field">
      <label>Members</label>
      <UiTagsInput
        v-model="draft.members"
        :placeholder="
          draft.members.length ? '' : 'someone@bcc.media, another@bcc.no'
        "
        aria-label="Members"
      />
      <div class="hint">
        Press Enter or comma to add. Users must sign in once via BCC Login or
        Azure AD before they can be matched.
      </div>
    </div>

    <template #actions>
      <UiButton variant="ghost" @click="emit('cancel')">Cancel</UiButton>
      <UiButton variant="primary" :disabled="!valid" @click="onSave">
        {{ isEdit ? 'Save changes' : 'Create group' }}
      </UiButton>
    </template>
  </UiDialog>
</template>
