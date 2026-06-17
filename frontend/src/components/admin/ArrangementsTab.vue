<script setup lang="ts">
import { reactive, ref, watch } from 'vue'
import { useAdmin, type Arrangement } from '../../composables/useAdmin'

const emit = defineEmits<{ (e: 'new'): void; (e: 'edit', a: Arrangement): void }>()
const { arrangements, deleteArrangement, createSubEvent, updateSubEvent, deleteSubEvent } = useAdmin()

const expanded = ref<Set<number>>(new Set())
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

function toggle(id: number) {
  const s = new Set(expanded.value)
  s.has(id) ? s.delete(id) : s.add(id)
  expanded.value = s
}

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

async function onDeleteArrangement(a: Arrangement) {
  if (!confirm(`Delete arrangement “${a.name}” and its ${a.subEvents.length} sub event(s)?`)) return
  await deleteArrangement(a.id)
}
</script>

<template>
  <div class="fb-fade">
    <div class="section-head">
      <div>
        <h1>Arrangements</h1>
        <div class="sub">Arrangements and their sub events power the Oslofjord Delivery upload form. Each has a display name and a short code used in the resulting filename. Expand a row to manage its sub events.</div>
      </div>
      <button class="btn btn-primary" @click="emit('new')">
        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round"><path d="M12 5v14M5 12h14"/></svg>
        New arrangement
      </button>
    </div>

    <div v-if="arrangements.length === 0" class="empty">
      No arrangements yet. Add one so people can select it in the upload form.
    </div>

    <div v-for="a in arrangements" :key="a.id" class="card arr-card">
      <div class="arr-head">
        <button class="chev" :class="{ open: expanded.has(a.id) }" @click="toggle(a.id)" aria-label="Toggle sub events">
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><path d="M9 6l6 6-6 6"/></svg>
        </button>
        <div class="primary">{{ a.name }}</div>
        <span class="chip mono">{{ a.code }}</span>
        <span class="secondary">{{ a.subEvents.length }} sub event{{ a.subEvents.length === 1 ? '' : 's' }}</span>
        <div class="arr-actions">
          <button class="btn btn-sm btn-ghost" @click="emit('edit', a)">Edit</button>
          <button class="btn btn-sm btn-danger" @click="onDeleteArrangement(a)">Delete</button>
        </div>
      </div>

      <div v-if="expanded.has(a.id)" class="sub-list">
        <div class="sub-row head">
          <span class="sub-name">Sub event name</span>
          <span class="sub-code">Code</span>
          <span class="sub-spacer"></span>
        </div>

        <div v-for="s in a.subEvents" :key="s.id" class="sub-row">
          <input v-if="editById[s.id]" v-model="editById[s.id].name" class="inline-edit sub-name" placeholder="e.g. Åpning" />
          <input v-if="editById[s.id]" v-model="editById[s.id].code" class="inline-edit mono sub-code" placeholder="CODE" />
          <button class="btn btn-sm btn-ghost" @click="saveSub(a, s.id)">Save</button>
          <button class="btn btn-sm btn-danger" @click="deleteSubEvent(a.id, s.id)">Delete</button>
        </div>

        <div v-if="a.subEvents.length === 0" class="sub-empty">No sub events yet — add one below.</div>

        <div v-if="newSub[a.id]" class="sub-row add">
          <input v-model="newSub[a.id].name" class="inline-edit sub-name" placeholder="New sub event name" @keyup.enter="addSub(a)" />
          <input v-model="newSub[a.id].code" class="inline-edit mono sub-code" placeholder="CODE" @keyup.enter="addSub(a)" />
          <button class="btn btn-sm btn-primary" @click="addSub(a)">Add</button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.arr-card { padding: 0; margin-bottom: 10px; }
.arr-head { display: flex; align-items: center; gap: 12px; padding: 12px 14px; }
.arr-head .primary { font-weight: 600; }
.arr-head .secondary { color: var(--ink-3); font-size: 12.5px; }
.arr-actions { margin-left: auto; display: flex; gap: 6px; }
.chev { background: none; border: none; color: var(--ink-2); cursor: pointer; display: flex; transition: transform 0.15s; padding: 2px; }
.chev.open { transform: rotate(90deg); }
.sub-list { border-top: 1px solid var(--line); padding: 10px 14px 14px 40px; display: flex; flex-direction: column; gap: 8px; }
.sub-row { display: flex; align-items: center; gap: 8px; }
.sub-name { flex: 1 1 auto; min-width: 0; width: auto; }
.sub-code { flex: 0 0 150px; width: auto; }
.sub-spacer { flex: 0 0 130px; }
.sub-row.head { font-size: 11px; text-transform: uppercase; letter-spacing: 0.04em; color: var(--ink-3); }
.sub-row.add { padding-top: 10px; margin-top: 2px; border-top: 1px dashed var(--line); }
.sub-empty { font-size: 12.5px; color: var(--ink-3); padding: 2px 0; }
</style>
