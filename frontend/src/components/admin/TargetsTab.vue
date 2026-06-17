<script setup lang="ts">
import { ref } from 'vue'
import { useAdmin, type Target } from '../../composables/useAdmin'

const emit = defineEmits<{ (e: 'new'): void; (e: 'edit', t: Target): void; (e: 'open', t: Target): void }>()
const { targets, grants, duplicateTarget, deleteTarget, reorderTargets } = useAdmin()

const inlineEditId = ref<number | null>(null)
const inlineEditField = ref<'name' | 'path' | null>(null)

const dragId = ref<number | null>(null)
const dragOverId = ref<number | null>(null)

function countGrantsForTarget(id: number) {
  return grants.value.filter(g => g.admin || g.allTargets || g.targetIds.includes(id)).length
}

function startEdit(id: number, field: 'name' | 'path') {
  inlineEditId.value = id
  inlineEditField.value = field
}

function finishEdit() {
  inlineEditId.value = null
  inlineEditField.value = null
}

function commitInline(t: Target, e: Event) {
  const value = (e.target as HTMLInputElement).value.trim()
  if (!value) {
    finishEdit()
    return
  }
  if (inlineEditField.value === 'name' && value !== t.name) {
    emit('edit', { ...t, name: value })
  } else if (inlineEditField.value === 'path' && value !== t.path) {
    emit('edit', { ...t, path: value })
  }
  finishEdit()
}

function onDragStart(t: Target, e: DragEvent) {
  if (inlineEditId.value === t.id) {
    e.preventDefault()
    return
  }
  dragId.value = t.id
  if (e.dataTransfer) {
    e.dataTransfer.effectAllowed = 'move'
    // Some browsers require non-empty data to actually fire drop events.
    e.dataTransfer.setData('text/plain', String(t.id))
  }
}

function onDragOver(t: Target, e: DragEvent) {
  if (dragId.value === null || dragId.value === t.id) return
  e.preventDefault()
  if (e.dataTransfer) e.dataTransfer.dropEffect = 'move'
  dragOverId.value = t.id
}

function onDragLeave(t: Target) {
  if (dragOverId.value === t.id) dragOverId.value = null
}

function onDrop(target: Target, e: DragEvent) {
  e.preventDefault()
  const moving = dragId.value
  dragId.value = null
  dragOverId.value = null
  if (moving === null || moving === target.id) return
  const ids = targets.value.map(t => t.id)
  const from = ids.indexOf(moving)
  const to = ids.indexOf(target.id)
  if (from < 0 || to < 0) return
  ids.splice(from, 1)
  // Insert before the drop target's current slot. After splice-removal of `from`,
  // the drop target's index shifts left by 1 if it was after the moved row.
  const adjusted = from < to ? to - 1 : to
  ids.splice(adjusted, 0, moving)
  reorderTargets(ids)
}

function onDragEnd() {
  dragId.value = null
  dragOverId.value = null
}
</script>

<template>
  <div class="fb-fade">
    <div class="section-head">
      <div>
        <h1>Upload targets</h1>
        <div class="sub">Destinations users can upload to. Each target maps a friendly name to a folder path on the storage backend. Click the name or path to rename inline. Drag the handle to reorder — the first target a user can access becomes their default.</div>
      </div>
      <button class="btn btn-primary" @click="emit('new')">
        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round"><path d="M12 5v14M5 12h14"/></svg>
        New target
      </button>
    </div>

    <div v-if="targets.length === 0" class="empty">
      No targets yet. Add one to let people start uploading.
    </div>

    <div v-else class="card">
      <table>
        <thead>
          <tr>
            <th style="width:24px"></th>
            <th>Name</th>
            <th>Folder path</th>
            <th>Access</th>
            <th></th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="t in targets"
            :key="t.id"
            :draggable="inlineEditId !== t.id"
            :class="{ 'drag-source': dragId === t.id, 'drag-over': dragOverId === t.id }"
            @dragstart="onDragStart(t, $event)"
            @dragover="onDragOver(t, $event)"
            @dragleave="onDragLeave(t)"
            @drop="onDrop(t, $event)"
            @dragend="onDragEnd"
          >
            <td class="drag-handle" title="Drag to reorder" aria-label="Drag to reorder">
              <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><path d="M4 8h16M4 16h16"/></svg>
            </td>
            <td>
              <div class="name-cell">
                <div class="swatch">
                  <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M3 7a2 2 0 0 1 2-2h4l2 2h8a2 2 0 0 1 2 2v8a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z"/></svg>
                </div>
                <div>
                  <input
                    v-if="inlineEditId === t.id && inlineEditField === 'name'"
                    class="inline-edit"
                    :value="t.name"
                    @blur="commitInline(t, $event)"
                    @keyup.enter="commitInline(t, $event)"
                    @keyup.escape="finishEdit"
                    ref="inlineInput"
                    autofocus
                  />
                  <div v-else class="primary" @click="startEdit(t.id, 'name')" style="cursor:text">{{ t.name }}</div>
                  <div class="secondary">
                    {{ countGrantsForTarget(t.id) }} {{ countGrantsForTarget(t.id) === 1 ? 'grant' : 'grants' }}
                  </div>
                </div>
              </div>
            </td>
            <td>
              <input
                v-if="inlineEditId === t.id && inlineEditField === 'path'"
                class="inline-edit mono"
                :value="t.path"
                @blur="commitInline(t, $event)"
                @keyup.enter="commitInline(t, $event)"
                @keyup.escape="finishEdit"
                autofocus
              />
              <span v-else class="path" @click="startEdit(t.id, 'path')" style="cursor:text">{{ t.path }}</span>
            </td>
            <td>
              <span class="chip" v-if="countGrantsForTarget(t.id) === 0" style="color:var(--ink-3)">No one</span>
              <span v-else class="chip ok">{{ countGrantsForTarget(t.id) }} principals</span>
            </td>
            <td class="actions">
              <button class="btn btn-sm btn-ghost" @click="emit('open', t)">Edit</button>
              <button class="btn btn-sm btn-ghost" @click="duplicateTarget(t)">Duplicate</button>
              <button class="btn btn-sm btn-danger" @click="deleteTarget(t.id)">Delete</button>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>

<style scoped>
.drag-handle {
  cursor: grab;
  color: var(--ink-3);
  text-align: center;
  width: 24px;
  user-select: none;
}
.drag-handle:active { cursor: grabbing; }
tr.drag-source { opacity: 0.4; }
tr.drag-over td { box-shadow: inset 0 2px 0 0 var(--accent); }
</style>
