<script setup lang="ts">
import { reactive, ref, watch } from 'vue'
import { useAdmin, type Arrangement } from '../../composables/useAdmin'
import SubEventImportModal from './SubEventImportModal.vue'
import UiCollapsible from '../ui/UiCollapsible.vue'
import { confirmAction } from '../../composables/useConfirm'

const emit = defineEmits<{
  (e: 'new'): void
  (e: 'edit', a: Arrangement): void
}>()
const {
  arrangements,
  deleteArrangement,
  createSubEvent,
  updateSubEvent,
  deleteSubEvent,
} = useAdmin()

// Arrangement the bulk-import modal is currently open for.
const importFor = ref<Arrangement | null>(null)
// Inline-edit drafts for existing sub events, keyed by sub-event id.
const editById = reactive<Record<number, { name: string; code: string }>>({})
// "Add sub event" drafts, keyed by arrangement id.
const newSub = reactive<Record<number, { name: string; code: string }>>({})

// Keep an editable draft for every sub-event (including newly added ones)
// without clobbering edits already in progress.
watch(
  arrangements,
  (arrs) => {
    for (const a of arrs) {
      if (!newSub[a.id]) newSub[a.id] = { name: '', code: '' }
      for (const s of a.subEvents) {
        if (!editById[s.id]) editById[s.id] = { name: s.name, code: s.code }
      }
    }
  },
  { immediate: true, deep: true },
)

const codeOk = (c: string) => /^[A-Za-z0-9_-]+$/.test(c.trim())

async function addSub(a: Arrangement) {
  const d = newSub[a.id]
  if (!d || !d.name.trim() || !codeOk(d.code)) return
  await createSubEvent(a.id, { name: d.name.trim(), code: d.code.trim() })
  newSub[a.id] = { name: '', code: '' }
}

async function saveSub(a: Arrangement, id: number) {
  const d = editById[id]
  if (!d || !d.name.trim() || !codeOk(d.code)) return
  await updateSubEvent(a.id, id, { name: d.name.trim(), code: d.code.trim() })
}

async function onDeleteSubEvent(a: Arrangement, id: number, name: string) {
  const ok = await confirmAction({
    title: `Delete sub event “${name}”?`,
    body: 'Uploads that already used it keep their filenames; new uploads cannot pick it.',
    confirmLabel: 'Delete sub event',
  })
  if (!ok) return
  await deleteSubEvent(a.id, id)
}

async function onDeleteArrangement(a: Arrangement) {
  const count = a.subEvents.length
  const ok = await confirmAction({
    title: `Delete arrangement “${a.name}”?`,
    body: count
      ? `Its ${count} sub event${count === 1 ? '' : 's'} will be deleted too.`
      : undefined,
    confirmLabel: 'Delete arrangement',
  })
  if (!ok) return
  await deleteArrangement(a.id)
}
</script>

<template>
  <div class="fb-fade">
    <div class="section-head">
      <div>
        <h1>Arrangements</h1>
        <div class="sub">
          Arrangements and their sub events power the Oslofjord Delivery upload
          form. Each has a display name and a short code used in the resulting
          filename. Expand a row to manage its sub events.
        </div>
      </div>
      <button class="btn btn-primary" @click="emit('new')">
        <svg
          width="14"
          height="14"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          stroke-width="2.5"
          stroke-linecap="round"
        >
          <path d="M12 5v14M5 12h14" />
        </svg>
        New arrangement
      </button>
    </div>

    <div v-if="arrangements.length === 0" class="empty">
      No arrangements yet. Add one so people can select it in the upload form.
    </div>

    <UiCollapsible
      v-for="a in arrangements"
      :key="a.id"
      v-slot="{ trigger, content, open, visible }"
      class="card arr-card"
    >
      <div class="arr-head">
        <button
          v-bind="trigger"
          class="chev"
          :class="{ open }"
          aria-label="Toggle sub events"
        >
          <svg
            width="14"
            height="14"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="2.5"
            stroke-linecap="round"
            stroke-linejoin="round"
          >
            <path d="M9 6l6 6-6 6" />
          </svg>
        </button>
        <div class="primary">{{ a.name }}</div>
        <span class="badge mono">{{ a.code }}</span>
        <span class="secondary"
          >{{ a.subEvents.length }} sub event{{
            a.subEvents.length === 1 ? '' : 's'
          }}</span
        >
        <div class="arr-actions">
          <button class="btn btn-sm btn-ghost" @click="emit('edit', a)">
            Edit
          </button>
          <button class="btn btn-sm btn-danger" @click="onDeleteArrangement(a)">
            Delete
          </button>
        </div>
      </div>

      <div v-bind="content" class="sub-list">
        <template v-if="visible">
          <div class="sub-row head">
            <span class="sub-name">Sub event name</span>
            <span class="sub-code">Code</span>
            <span class="sub-spacer"></span>
          </div>

          <div v-for="s in a.subEvents" :key="s.id" class="sub-row">
            <input
              v-if="editById[s.id]"
              v-model="editById[s.id].name"
              class="inline-edit sub-name"
              placeholder="e.g. Åpning"
            />
            <input
              v-if="editById[s.id]"
              v-model="editById[s.id].code"
              class="inline-edit mono sub-code"
              placeholder="CODE"
            />
            <button class="btn btn-sm btn-ghost" @click="saveSub(a, s.id)">
              Save
            </button>
            <button
              class="btn btn-sm btn-danger"
              @click="onDeleteSubEvent(a, s.id, s.name)"
            >
              Delete
            </button>
          </div>

          <div v-if="a.subEvents.length === 0" class="sub-empty">
            No sub events yet — add one below.
          </div>

          <div v-if="newSub[a.id]" class="sub-row add">
            <input
              v-model="newSub[a.id].name"
              class="inline-edit sub-name"
              placeholder="New sub event name"
              @keyup.enter="addSub(a)"
            />
            <input
              v-model="newSub[a.id].code"
              class="inline-edit mono sub-code"
              placeholder="CODE"
              @keyup.enter="addSub(a)"
            />
            <button class="btn btn-sm btn-primary" @click="addSub(a)">
              Add
            </button>
            <button class="btn btn-sm btn-ghost" @click="importFor = a">
              Import list…
            </button>
          </div>
        </template>
      </div>
    </UiCollapsible>
  </div>

  <SubEventImportModal
    v-if="importFor"
    :arrangement="importFor"
    @close="importFor = null"
  />
</template>

<style scoped>
.arr-card {
  padding: 0;
  margin-bottom: 10px;
}
.arr-head {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px 14px;
}
.arr-head .primary {
  font-weight: 600;
}
.arr-head .secondary {
  color: var(--color-ink-3);
  font-size: 12.5px;
}
.arr-actions {
  margin-left: auto;
  display: flex;
  gap: 6px;
}
.chev {
  background: none;
  border: none;
  color: var(--color-ink-2);
  cursor: pointer;
  display: flex;
  transition: transform 0.15s;
  padding: 2px;
}
.chev.open {
  transform: rotate(90deg);
}
.sub-list {
  border-top: 1px solid var(--color-line);
  padding: 10px 14px 14px 40px;
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.sub-row {
  display: flex;
  align-items: center;
  gap: 8px;
}
.sub-name {
  flex: 1 1 auto;
  min-width: 0;
  width: auto;
}
.sub-code {
  flex: 0 0 150px;
  width: auto;
}
.sub-spacer {
  flex: 0 0 130px;
}
.sub-row.head {
  font-size: 11px;
  text-transform: uppercase;
  letter-spacing: 0.04em;
  color: var(--color-ink-3);
}
.sub-row.add {
  padding-top: 10px;
  margin-top: 2px;
  border-top: 1px dashed var(--color-line);
}
.sub-empty {
  font-size: 12.5px;
  color: var(--color-ink-3);
  padding: 2px 0;
}
</style>
