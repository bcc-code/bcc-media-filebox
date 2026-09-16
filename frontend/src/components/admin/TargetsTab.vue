<script setup lang="ts">
import { ref } from 'vue'
import { useAdmin, type Target } from '../../composables/useAdmin'
import { confirmAction } from '../../composables/useConfirm'
import UiEditable from '../ui/UiEditable.vue'
import UiButton from '../ui/UiButton.vue'

const emit = defineEmits<{
  (e: 'new'): void
  (e: 'edit', t: Target): void
  (e: 'open', t: Target): void
}>()
const { targets, grants, duplicateTarget, deleteTarget, reorderTargets } =
  useAdmin()

// A row must not be draggable while its name or path is open for editing,
// or the drag fights text selection.
const editingId = ref<number | null>(null)

const dragId = ref<number | null>(null)
const dragOverId = ref<number | null>(null)

async function onDelete(t: Target) {
  // Deleting a target only drops the DB row — the folder and everything already
  // uploaded into it stay put. grant_targets cascades, so grants survive but
  // lose this target.
  const affected = grants.value.filter(
    (g) => !g.admin && !g.allTargets && g.targetIds.includes(t.id),
  ).length
  const ok = await confirmAction({
    title: `Delete upload target “${t.name}”?`,
    body:
      'The folder and any files already uploaded to it are left untouched.' +
      (affected
        ? ` ${affected} grant${affected === 1 ? '' : 's'} will lose access to it.`
        : ''),
    confirmLabel: 'Delete target',
  })
  if (!ok) return
  await deleteTarget(t.id)
}

function countGrantsForTarget(id: number) {
  return grants.value.filter(
    (g) => g.admin || g.allTargets || g.targetIds.includes(id),
  ).length
}

function onDragStart(t: Target, e: DragEvent) {
  if (editingId.value === t.id) {
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
  const ids = targets.value.map((t) => t.id)
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
        <div class="sub">
          Destinations users can upload to. Each target maps a friendly name to
          a folder path on the storage backend. Click the name or path to rename
          inline. Drag the handle to reorder — the first target a user can
          access becomes their default.
        </div>
      </div>
      <UiButton variant="primary" @click="emit('new')">
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
        New target
      </UiButton>
    </div>

    <div v-if="targets.length === 0" class="empty">
      No targets yet. Add one to let people start uploading.
    </div>

    <div v-else class="card">
      <table>
        <thead>
          <tr>
            <th style="width: 24px"></th>
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
            :draggable="editingId !== t.id"
            :class="{
              'drag-source': dragId === t.id,
              'drag-over': dragOverId === t.id,
            }"
            @dragstart="onDragStart(t, $event)"
            @dragover="onDragOver(t, $event)"
            @dragleave="onDragLeave(t)"
            @drop="onDrop(t, $event)"
            @dragend="onDragEnd"
          >
            <td
              class="drag-handle"
              title="Drag to reorder"
              aria-label="Drag to reorder"
            >
              <svg
                width="14"
                height="14"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                stroke-width="2"
                stroke-linecap="round"
              >
                <path d="M4 8h16M4 16h16" />
              </svg>
            </td>
            <td>
              <div class="name-cell">
                <div class="swatch">
                  <svg
                    width="16"
                    height="16"
                    viewBox="0 0 24 24"
                    fill="none"
                    stroke="currentColor"
                    stroke-width="2"
                    stroke-linecap="round"
                    stroke-linejoin="round"
                  >
                    <path
                      d="M3 7a2 2 0 0 1 2-2h4l2 2h8a2 2 0 0 1 2 2v8a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z"
                    />
                  </svg>
                </div>
                <div>
                  <UiEditable
                    :value="t.name"
                    preview-class="primary editable-preview"
                    aria-label="Target name"
                    @commit="(name) => emit('edit', { ...t, name })"
                    @edit-change="(on) => (editingId = on ? t.id : null)"
                  />
                  <div class="secondary">
                    {{ countGrantsForTarget(t.id) }}
                    {{ countGrantsForTarget(t.id) === 1 ? 'grant' : 'grants' }}
                  </div>
                </div>
              </div>
            </td>
            <td>
              <UiEditable
                :value="t.path"
                preview-class="path editable-preview"
                input-class="mono"
                aria-label="Folder path"
                @commit="(path) => emit('edit', { ...t, path })"
                @edit-change="(on) => (editingId = on ? t.id : null)"
              />
            </td>
            <td>
              <span
                class="badge"
                v-if="countGrantsForTarget(t.id) === 0"
                style="color: var(--color-ink-3)"
                >No one</span
              >
              <span v-else class="badge badge-ok"
                >{{ countGrantsForTarget(t.id) }} principals</span
              >
            </td>
            <td class="actions">
              <UiButton size="sm" variant="ghost" @click="emit('open', t)">
                Edit
              </UiButton>
              <UiButton size="sm" variant="ghost" @click="duplicateTarget(t)">
                Duplicate
              </UiButton>
              <UiButton size="sm" variant="danger" @click="onDelete(t)">
                Delete
              </UiButton>
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
  color: var(--color-ink-3);
  text-align: center;
  width: 24px;
  user-select: none;
}
.drag-handle:active {
  cursor: grabbing;
}
tr.drag-source {
  opacity: 0.4;
}
tr.drag-over td {
  box-shadow: inset 0 2px 0 0 var(--color-accent);
}
</style>
