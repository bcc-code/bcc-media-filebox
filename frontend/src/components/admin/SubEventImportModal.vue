<script setup lang="ts">
import { computed, ref } from 'vue'
import { useAdmin, type Arrangement } from '../../composables/useAdmin'

const props = defineProps<{ arrangement: Arrangement }>()
const emit = defineEmits<{ (e: 'close'): void }>()

const { importSubEvents } = useAdmin()

const raw = ref('')
const importing = ref(false)

const codeOk = (c: string) => /^[A-Za-z0-9_-]+$/.test(c)

interface Row {
  name: string
  code: string
  status: 'new' | 'skip' | 'error'
  error?: string
}

const parsed = computed<{ rows: Row[]; trailing: string | null }>(() => {
  const lines = raw.value
    .split(/\r?\n/)
    .map(l => l.trim())
    .filter(l => l !== '')

  // Pair up lines: "name<TAB>code" on one line, otherwise alternating
  // name/code lines.
  const pairs: { name: string; code: string }[] = []
  let pendingName: string | null = null
  for (const line of lines) {
    const tab = line.indexOf('\t')
    if (tab >= 0 && pendingName === null) {
      pairs.push({ name: line.slice(0, tab).trim(), code: line.slice(tab + 1).trim() })
    } else if (pendingName === null) {
      pendingName = line
    } else {
      pairs.push({ name: pendingName, code: line })
      pendingName = null
    }
  }

  const existingNames = new Set(props.arrangement.subEvents.map(s => s.name.toLowerCase()))
  const existingCodes = new Set(props.arrangement.subEvents.map(s => s.code.toLowerCase()))
  const seenNames = new Set<string>()
  const seenCodes = new Set<string>()

  const rows: Row[] = pairs.map(({ name, code }) => {
    let error: string | undefined
    if (!name) error = 'Name is missing'
    else if (!codeOk(code)) error = "Code may contain only letters, digits, '-' and '_'"
    else if (seenNames.has(name.toLowerCase())) error = 'Duplicate name in the pasted list'
    else if (seenCodes.has(code.toLowerCase())) error = 'Duplicate code in the pasted list'
    seenNames.add(name.toLowerCase())
    seenCodes.add(code.toLowerCase())
    if (error) return { name, code, status: 'error', error }
    if (existingNames.has(name.toLowerCase()) || existingCodes.has(code.toLowerCase())) {
      return { name, code, status: 'skip' }
    }
    return { name, code, status: 'new' }
  })

  return { rows, trailing: pendingName }
})

const newCount = computed(() => parsed.value.rows.filter(r => r.status === 'new').length)
const skipCount = computed(() => parsed.value.rows.filter(r => r.status === 'skip').length)
const hasErrors = computed(() => parsed.value.trailing !== null || parsed.value.rows.some(r => r.status === 'error'))
const canImport = computed(() => !hasErrors.value && newCount.value > 0 && !importing.value)

async function onImport() {
  if (!canImport.value) return
  importing.value = true
  const items = parsed.value.rows.filter(r => r.status === 'new').map(r => ({ name: r.name, code: r.code }))
  const ok = await importSubEvents(props.arrangement.id, items)
  importing.value = false
  if (ok) emit('close')
}
</script>

<template>
  <div class="modal-bg" @click.self="emit('close')">
    <div class="modal fb-fade">
      <h2>Import sub events</h2>
      <div class="sub">
        Paste a list for “{{ arrangement.name }}”: one line with the sub event name followed by one line with its
        code (or name and code separated by a tab). Blank lines are ignored; entries that already exist are skipped.
      </div>

      <div class="field">
        <label>Pasted list</label>
        <textarea
          v-model="raw"
          class="paste-area mono"
          rows="8"
          autofocus
          placeholder="Åpningsmøte&#10;OPENING_JULY&#10;LLB kick-off&#10;LLB"
        ></textarea>
      </div>

      <div v-if="parsed.rows.length > 0 || parsed.trailing" class="preview">
        <div class="prev-row head">
          <span class="prev-name">Name</span>
          <span class="prev-code">Code</span>
          <span class="prev-status"></span>
        </div>
        <div v-for="(r, i) in parsed.rows" :key="i" class="prev-row" :class="r.status">
          <span class="prev-name">{{ r.name }}</span>
          <span class="prev-code mono">{{ r.code }}</span>
          <span class="prev-status">
            <template v-if="r.status === 'new'">new</template>
            <template v-else-if="r.status === 'skip'">already exists — skipped</template>
            <template v-else>{{ r.error }}</template>
          </span>
        </div>
        <div v-if="parsed.trailing" class="prev-row error">
          <span class="prev-name">{{ parsed.trailing }}</span>
          <span class="prev-code"></span>
          <span class="prev-status">Name without a code line</span>
        </div>
      </div>

      <div class="modal-actions">
        <span v-if="parsed.rows.length" class="summary">
          {{ newCount }} new<template v-if="skipCount"> · {{ skipCount }} skipped</template>
        </span>
        <button class="btn btn-ghost" @click="emit('close')">Cancel</button>
        <button class="btn btn-primary" :disabled="!canImport" @click="onImport">
          {{ importing ? 'Importing…' : `Import ${newCount} sub event${newCount === 1 ? '' : 's'}` }}
        </button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.paste-area { width: 100%; resize: vertical; }
.preview { border: 1px solid var(--line); border-radius: 8px; max-height: 240px; overflow-y: auto; margin-bottom: 14px; }
.prev-row { display: flex; gap: 10px; align-items: baseline; padding: 5px 10px; border-top: 1px solid var(--line); font-size: 13px; }
.prev-row:first-child { border-top: none; }
.prev-row.head { font-size: 11px; text-transform: uppercase; letter-spacing: 0.04em; color: var(--ink-3); position: sticky; top: 0; background: var(--bg-2); }
.prev-name { flex: 1 1 auto; min-width: 0; }
.prev-code { flex: 0 0 140px; }
.prev-status { flex: 0 0 45%; font-size: 12px; color: var(--ink-3); }
.prev-row.skip .prev-status { color: var(--ink-3); font-style: italic; }
.prev-row.error .prev-status { color: var(--danger); }
.prev-row.new .prev-status { color: var(--ok); }
.summary { margin-right: auto; font-size: 12.5px; color: var(--ink-3); }
</style>
