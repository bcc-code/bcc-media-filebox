<script setup lang="ts">
import { reactive, ref, watch, computed } from 'vue'
import type { Group } from '../../composables/useAdmin'

const props = defineProps<{ group: Group | null }>()
const emit = defineEmits<{
  (e: 'cancel'): void
  (e: 'save', body: { name: string; description: string; members: string[] }): void
}>()

const draft = reactive<{ name: string; description: string; members: string[] }>({
  name: '',
  description: '',
  members: [],
})
const memberInput = ref('')

watch(
  () => props.group,
  (g) => {
    draft.name = g?.name ?? ''
    draft.description = g?.description ?? ''
    draft.members = g ? [...g.members] : []
    memberInput.value = ''
  },
  { immediate: true },
)

const isEdit = computed(() => !!props.group)
const valid = computed(() => draft.name.trim().length > 0)

function addMember() {
  const raw = memberInput.value.trim().replace(/,$/, '').trim()
  if (!raw) return
  if (!draft.members.includes(raw)) draft.members.push(raw)
  memberInput.value = ''
}

function removeMember(i: number) {
  draft.members.splice(i, 1)
}

function onBackspace() {
  if (memberInput.value === '' && draft.members.length) draft.members.pop()
}

function onSave() {
  if (!valid.value) return
  if (memberInput.value.trim()) addMember()
  emit('save', {
    name: draft.name.trim(),
    description: draft.description.trim(),
    members: [...draft.members],
  })
}
</script>

<template>
  <div class="modal-bg" @click.self="emit('cancel')">
    <div class="modal fb-fade" style="width: 560px">
      <h2>{{ isEdit ? 'Edit group' : 'New custom group' }}</h2>
      <div class="sub">A custom group is a named bundle of users. Use it when you want to grant the same target access to several specific people at once.</div>

      <div class="field">
        <label>Group name</label>
        <input v-model="draft.name" placeholder="e.g. Camera dept." autofocus />
      </div>

      <div class="field">
        <label>Description <span style="text-transform:none;letter-spacing:0;color:var(--color-ink-3)">(optional)</span></label>
        <input v-model="draft.description" placeholder="Short note about who this group is for" />
      </div>

      <div class="field">
        <label>Members</label>
        <div class="token-input">
          <span v-for="(m, i) in draft.members" :key="m + i" class="token">
            {{ m }}
            <button class="x" @click="removeMember(i)" aria-label="Remove">×</button>
          </span>
          <input
            v-model="memberInput"
            @keydown.enter.prevent="addMember"
            @keydown.,.prevent="addMember"
            @keydown.delete="onBackspace"
            @blur="addMember"
            :placeholder="draft.members.length ? '' : 'someone@bcc.media, another@bcc.no'"
          />
        </div>
        <div class="hint">Press Enter or comma to add. Users must sign in once via BCC Login or Azure AD before they can be matched.</div>
      </div>

      <div class="modal-actions">
        <button class="btn btn-ghost" @click="emit('cancel')">Cancel</button>
        <button class="btn btn-primary" :disabled="!valid" @click="onSave">
          {{ isEdit ? 'Save changes' : 'Create group' }}
        </button>
      </div>
    </div>
  </div>
</template>
