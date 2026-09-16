<script setup lang="ts">
import UiDialog from '../ui/UiDialog.vue'
import { reactive, watch, computed } from 'vue'
import type { Grant, Target, Group } from '../../composables/useAdmin'
import UiSelect, { type UiSelectEntry } from '../ui/UiSelect.vue'

const props = defineProps<{
  grant: Grant | null
  prefillEmail?: string
  targets: Target[]
  builtinGroups: Group[]
  customGroups: Group[]
}>()
const emit = defineEmits<{
  (e: 'cancel'): void
  (
    e: 'save',
    body: {
      principalKind: 'user' | 'group'
      principalValue: string
      admin: boolean
      allTargets: boolean
      targetIds: number[]
    },
  ): void
  (e: 'goToGroups'): void
}>()

const draft = reactive({
  kind: 'user' as 'user' | 'group',
  name: '',
  admin: false,
  allTargets: false,
  targetIds: [] as number[],
})

watch(
  () => [props.grant, props.prefillEmail] as const,
  ([g, prefill]) => {
    if (g) {
      draft.kind = g.principalKind
      draft.name = g.principalValue
      draft.admin = g.admin
      draft.allTargets = g.allTargets
      draft.targetIds = [...g.targetIds]
    } else {
      draft.kind = prefill ? 'user' : 'user'
      draft.name = prefill ?? ''
      draft.admin = false
      draft.allTargets = false
      draft.targetIds = []
    }
  },
  { immediate: true },
)

const groupOptions = computed<UiSelectEntry<string>[]>(() => {
  const entries: UiSelectEntry<string>[] = [
    {
      label: 'Built-in directory groups',
      options: props.builtinGroups.map((gr) => ({
        value: gr.name,
        label: gr.name,
      })),
    },
  ]
  if (props.customGroups.length) {
    entries.push({
      label: 'Custom groups',
      options: props.customGroups.map((gr) => ({
        value: gr.name,
        label: gr.name,
      })),
    })
  }
  return entries
})

const isEdit = computed(() => !!props.grant)
const valid = computed(() => draft.name.trim().length > 0)

const selectedGroup = computed(
  () =>
    [...props.builtinGroups, ...props.customGroups].find(
      (g) => g.name === draft.name,
    ) ?? null,
)

function setKind(k: 'user' | 'group') {
  draft.kind = k
  draft.name = ''
}

function toggleTarget(id: number) {
  const i = draft.targetIds.indexOf(id)
  if (i >= 0) draft.targetIds.splice(i, 1)
  else draft.targetIds.push(id)
}

function onSave() {
  if (!valid.value) return
  emit('save', {
    principalKind: draft.kind,
    principalValue: draft.name.trim(),
    admin: draft.admin,
    allTargets: draft.allTargets,
    targetIds: draft.admin || draft.allTargets ? [] : [...draft.targetIds],
  })
}
</script>

<template>
  <UiDialog
    :title="isEdit ? 'Edit grant' : 'Grant access'"
    description="Pick a person or a group, choose their role, and select which targets they can upload to."
    width="560px"
    @close="emit('cancel')"
  >
    <div class="field">
      <label>Principal type</label>
      <div class="seg">
        <button
          :class="{ active: draft.kind === 'user' }"
          @click="setKind('user')"
        >
          <svg
            width="13"
            height="13"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="2"
          >
            <circle cx="12" cy="8" r="3.2" />
            <path d="M5 20c1.5-3.6 4-5 7-5s5.5 1.4 7 5" />
          </svg>
          Individual user
        </button>
        <button
          :class="{ active: draft.kind === 'group' }"
          @click="setKind('group')"
        >
          <svg
            width="13"
            height="13"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="2"
          >
            <circle cx="9" cy="8" r="3" />
            <circle cx="17" cy="9" r="2.5" />
            <path d="M3 19c1-3 3.5-4.5 6-4.5s5 1.5 6 4.5" />
            <path d="M15 19c.5-2 2-3 3.5-3s3 1 3.5 3" />
          </svg>
          Group
        </button>
      </div>
    </div>

    <div v-if="draft.kind === 'user'" class="field">
      <label>Email address</label>
      <input v-model="draft.name" placeholder="someone@bcc.media" autofocus />
      <div class="hint">
        User must have signed in once via BCC Login or Azure AD to be matched.
      </div>
    </div>

    <div v-else class="field">
      <label>Group</label>
      <UiSelect
        v-model="draft.name"
        :options="groupOptions"
        placeholder="Choose a group…"
        aria-label="Group"
      />
      <div class="hint" v-if="selectedGroup">
        {{ selectedGroup.description }}
      </div>
      <div class="hint" v-else-if="customGroups.length === 0">
        No custom groups yet.
        <a
          href="#"
          @click.prevent="emit('goToGroups')"
          style="color: var(--color-accent)"
          >Create one →</a
        >
      </div>
    </div>

    <div class="field">
      <label>Role</label>
      <label
        style="
          display: flex;
          align-items: center;
          gap: 10px;
          padding: 10px 12px;
          border: 1px solid var(--color-line-2);
          border-radius: 8px;
          text-transform: none;
          letter-spacing: 0;
          color: var(--color-ink);
          font-size: 13.5px;
          cursor: pointer;
          background: var(--color-surface);
        "
        :style="
          draft.admin
            ? 'border-color:var(--color-accent);background:color-mix(in oklch,var(--color-accent),transparent 88%)'
            : ''
        "
      >
        <input
          type="checkbox"
          v-model="draft.admin"
          style="accent-color: var(--color-accent)"
        />
        <div style="flex: 1">
          <div style="font-weight: 500">Grant admin access</div>
          <div
            style="font-size: 12px; color: var(--color-ink-3); margin-top: 2px"
          >
            Can manage targets and other people's access. Implies access to all
            targets.
          </div>
        </div>
      </label>
    </div>

    <div class="field" v-if="!draft.admin">
      <label>Allowed upload targets</label>
      <div class="target-pick">
        <label class="all-toggle" :class="{ checked: draft.allTargets }">
          <input type="checkbox" v-model="draft.allTargets" />
          <span style="font-weight: 500"
            >All targets, including future ones</span
          >
        </label>
        <template v-if="!draft.allTargets">
          <label
            v-for="t in targets"
            :key="t.id"
            :class="{ checked: draft.targetIds.includes(t.id) }"
          >
            <input
              type="checkbox"
              :checked="draft.targetIds.includes(t.id)"
              @change="toggleTarget(t.id)"
            />
            <span>{{ t.name }}</span>
            <span class="meta">{{ t.path }}</span>
          </label>
        </template>
      </div>
      <div
        class="hint"
        v-if="!draft.allTargets && draft.targetIds.length === 0"
      >
        If you grant no targets, this person won't be able to upload anything —
        only sign in.
      </div>
    </div>

    <template #actions>
      <button class="btn btn-ghost" @click="emit('cancel')">Cancel</button>
      <button class="btn btn-primary" :disabled="!valid" @click="onSave">
        {{ isEdit ? 'Save changes' : 'Add grant' }}
      </button>
    </template>
  </UiDialog>
</template>

<style scoped>
/* Target checkbox list — specific to this dialog, so it travels with the
   component rather than living in the admin surface sheet. */
.target-pick {
  display: flex;
  flex-direction: column;
  gap: 6px;
  max-height: 220px;
  overflow-y: auto;
  padding: 4px 2px;
}
.target-pick label {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 9px 11px;
  border: 1px solid var(--color-line-2);
  border-radius: 8px;
  cursor: pointer;
  text-transform: none;
  letter-spacing: 0;
  color: var(--color-ink);
  font-size: 13.5px;
  background: var(--color-surface);
  transition:
    border-color 0.12s,
    background 0.12s;
}
.target-pick label:hover {
  border-color: var(--color-ink-3);
}
.target-pick label.checked {
  border-color: var(--color-accent);
  background: color-mix(in oklch, var(--color-accent), transparent 88%);
}
.target-pick input[type='checkbox'] {
  accent-color: var(--color-accent);
}
.target-pick .all-toggle {
  background: var(--color-surface-3);
  border-style: dashed;
}
.target-pick .all-toggle.checked {
  border-style: solid;
}
.target-pick label .meta {
  color: var(--color-ink-3);
  font-family: 'JetBrains Mono', monospace;
  font-size: 11.5px;
  margin-left: auto;
}
</style>
