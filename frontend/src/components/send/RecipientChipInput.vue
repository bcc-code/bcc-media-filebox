<script setup lang="ts">
import { ref } from 'vue'

const props = defineProps<{ modelValue: string[] }>()
const emit = defineEmits<{ 'update:modelValue': [value: string[]] }>()

const input = ref('')
const focus = ref(false)
const inputRef = ref<HTMLInputElement | null>(null)

function isEmail(s: string): boolean {
  return /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(s)
}

function commit() {
  const parts = input.value.split(/[,\s;]+/).map((s) => s.trim()).filter(Boolean)
  if (parts.length) {
    const next = [...props.modelValue]
    for (const p of parts) if (!next.includes(p)) next.push(p)
    emit('update:modelValue', next)
  }
  input.value = ''
}

function onKeydown(e: KeyboardEvent) {
  if (e.key === ',' || e.key === ';' || e.key === ' ') {
    e.preventDefault()
    commit()
  } else if (e.key === 'Backspace' && !input.value && props.modelValue.length) {
    emit('update:modelValue', props.modelValue.slice(0, -1))
  }
}

function onPaste(e: ClipboardEvent) {
  e.preventDefault()
  input.value = e.clipboardData?.getData('text') ?? ''
  commit()
}

function remove(r: string) {
  emit('update:modelValue', props.modelValue.filter((x) => x !== r))
}
</script>

<template>
  <div class="chips" :class="{ focus }" @click="inputRef?.focus()">
    <span v-for="r in modelValue" :key="r" class="chip" :class="{ invalid: !isEmail(r) }">
      {{ r }}
      <button type="button" @click.stop="remove(r)">
        <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.4" stroke-linecap="round"><path d="M18 6 6 18M6 6l12 12"/></svg>
      </button>
    </span>
    <input
      ref="inputRef"
      v-model="input"
      placeholder="Add email and press Enter…"
      @keydown.enter.prevent="commit"
      @keydown="onKeydown"
      @paste="onPaste"
      @focus="focus = true"
      @blur="focus = false; commit()"
    />
  </div>
</template>
