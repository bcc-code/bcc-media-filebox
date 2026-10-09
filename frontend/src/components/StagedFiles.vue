<script setup lang="ts">
import { computed } from 'vue'
import { expectedFilename, type Form } from '../forms'
import { formatBytes } from '../composables/adminHelpers'
import UiButton from './ui/UiButton.vue'

// Files picked but not yet uploading. Shows what each will be stored as so the
// uploader can check before anything is sent; the parent owns the list.
const props = withDefaults(
  defineProps<{
    files: File[]
    /** The target's form, when the stored name is built from its fields. */
    form?: Form | null
    values?: Record<string, string>
    /** Why Upload is unavailable; empty when it can go. */
    blockedReason?: string
  }>(),
  { form: null, values: () => ({}), blockedReason: '' },
)

const emit = defineEmits<{
  remove: [index: number]
  clear: []
  /** Only the files the server would accept. */
  upload: [files: File[]]
}>()

const rows = computed(() =>
  props.files.map((file) => ({
    file,
    ...expectedFilename(file, props.form, props.values),
  })),
)
const valid = computed(() => rows.value.filter((r) => !r.error))
const totalSize = computed(() =>
  valid.value.reduce((sum, r) => sum + r.file.size, 0),
)

function size(n: number): string {
  return n === 0 ? '0 B' : formatBytes(n)
}
</script>

<template>
  <div v-if="files.length" class="card staged">
    <div class="staged-head">
      <span class="staged-title">Ready to upload</span>
      <span class="staged-note">Nothing is sent until you press Upload.</span>
    </div>

    <ul class="staged-list">
      <li v-for="(row, i) in rows" :key="i" class="staged-row">
        <div class="names">
          <template v-if="row.error">
            <div class="orig">{{ row.file.name }}</div>
            <div class="err">{{ row.error }} — will be skipped</div>
          </template>
          <template v-else>
            <div class="orig" :title="row.file.name">
              <span class="tag">Original</span>{{ row.file.name }}
            </div>
            <div class="dest mono" :title="row.name">
              <span class="tag">Saved as</span>{{ row.name }}
            </div>
          </template>
        </div>
        <span class="size">{{ size(row.file.size) }}</span>
        <button
          type="button"
          class="rm"
          :title="`Remove ${row.file.name}`"
          :aria-label="`Remove ${row.file.name}`"
          @click="emit('remove', i)"
        >
          <svg
            width="15"
            height="15"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="2"
            stroke-linecap="round"
            stroke-linejoin="round"
          >
            <path d="M18 6 6 18M6 6l12 12" />
          </svg>
        </button>
      </li>
    </ul>

    <div class="staged-foot">
      <span class="total">
        {{ valid.length }} file{{ valid.length === 1 ? '' : 's' }} ·
        {{ size(totalSize) }}
      </span>
      <span v-if="blockedReason" class="blocked">{{ blockedReason }}</span>
      <span class="actions">
        <UiButton size="sm" variant="ghost" @click="emit('clear')">
          Clear
        </UiButton>
        <UiButton
          size="sm"
          variant="primary"
          :disabled="!!blockedReason || valid.length === 0"
          @click="emit('upload', valid.map((r) => r.file))"
        >
          Upload
        </UiButton>
      </span>
    </div>
  </div>
</template>

<style scoped>
.staged {
  margin-top: 14px;
  padding: 14px 16px;
}

.staged-head {
  display: flex;
  align-items: baseline;
  gap: 10px;
  flex-wrap: wrap;
  margin-bottom: 8px;
}
.staged-title {
  font-size: 13.5px;
  font-weight: 600;
  color: var(--color-ink);
}
.staged-note {
  font-size: 11.5px;
  color: var(--color-ink-3);
}

.staged-list {
  list-style: none;
  margin: 0;
  padding: 0;
}
.staged-row {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 10px 0;
  border-top: 1px solid var(--color-line);
}
.names {
  flex: 1;
  min-width: 0;
}
.orig,
.dest,
.err {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.orig {
  font-size: 12px;
  color: var(--color-ink-3);
}
.dest {
  margin-top: 2px;
  font-size: 13px;
  color: var(--color-ink);
}
.err {
  margin-top: 2px;
  font-size: 12px;
  color: var(--color-danger);
}
.tag {
  display: inline-block;
  width: 64px;
  font-family: var(--font-sans);
  font-size: 10px;
  font-weight: 600;
  color: var(--color-ink-3);
}
.size {
  flex-shrink: 0;
  font-size: 12.5px;
  font-variant-numeric: tabular-nums;
  color: var(--color-ink-2);
}
.rm {
  flex-shrink: 0;
  display: grid;
  place-items: center;
  padding: 4px;
  border: 0;
  border-radius: 6px;
  background: transparent;
  color: var(--color-ink-3);
  cursor: pointer;
}
.rm:hover {
  color: var(--color-danger);
  background: var(--color-surface-3);
}

.staged-foot {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
  padding-top: 12px;
  border-top: 1px solid var(--color-line);
}
.total {
  font-size: 12.5px;
  color: var(--color-ink-2);
}
.blocked {
  font-size: 12px;
  color: var(--color-warn);
}
.actions {
  display: flex;
  gap: 8px;
  margin-left: auto;
}
</style>
