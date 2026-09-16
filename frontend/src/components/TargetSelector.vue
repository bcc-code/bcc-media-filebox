<script setup lang="ts">
import type { TargetInfo } from '../types'

defineProps<{
  modelValue: string
  targets: TargetInfo[]
}>()

const emit = defineEmits<{
  'update:modelValue': [name: string]
}>()

function select(name: string) {
  emit('update:modelValue', name)
}
</script>

<template>
  <div class="target-grid">
    <button
      v-for="t in targets"
      :key="t.name"
      type="button"
      class="target-card"
      :class="{ selected: t.name === modelValue }"
      :aria-pressed="t.name === modelValue"
      @click="select(t.name)"
    >
      <div class="target-top">
        <!-- Generic disk icon -->
        <span class="target-ic">
          <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
            <ellipse cx="12" cy="5" rx="8" ry="3" />
            <path d="M4 5v6c0 1.66 3.58 3 8 3s8-1.34 8-3V5" />
            <path d="M4 11v6c0 1.66 3.58 3 8 3s8-1.34 8-3v-6" />
          </svg>
        </span>

        <!-- Selection indicator -->
        <span v-if="t.name === modelValue" class="target-check">
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3" stroke-linecap="round" stroke-linejoin="round">
            <path d="M5 13l4 4L19 7" />
          </svg>
        </span>
        <span v-else class="target-radio" />
      </div>

      <div class="target-name">{{ t.name }}</div>
    </button>
  </div>
</template>

<style scoped>
.target-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
  gap: 12px;
}

.target-card {
  display: flex; flex-direction: column; gap: 12px;
  padding: 14px; border-radius: 12px; text-align: left; cursor: pointer;
  background: var(--color-surface-2);
  border: 1px solid var(--color-line-2);
  color: var(--color-ink);
  font: inherit;
  transition: border-color .15s ease, background .15s ease;
}
.target-card:hover { border-color: color-mix(in oklch, var(--color-accent), transparent 40%); }
.target-card:focus-visible {
  outline: none;
  box-shadow: 0 0 0 3px color-mix(in oklch, var(--color-accent), transparent 80%);
}
.target-card.selected {
  border-color: var(--color-accent);
  background: color-mix(in oklch, var(--color-accent), transparent 90%);
}

.target-top { display: flex; align-items: flex-start; justify-content: space-between; }

.target-ic {
  display: grid; place-items: center;
  width: 40px; height: 40px; border-radius: 10px;
  background: var(--color-surface-3);
  border: 1px solid var(--color-line-2);
  color: var(--color-ink-3);
  transition: color .15s ease, background .15s ease;
}
.target-card.selected .target-ic {
  background: color-mix(in oklch, var(--color-accent), transparent 82%);
  border-color: color-mix(in oklch, var(--color-accent), transparent 55%);
  color: var(--color-accent);
}

.target-check {
  display: grid; place-items: center;
  width: 24px; height: 24px; border-radius: 50%;
  background: var(--color-accent);
  color: var(--color-accent-ink);
  flex-shrink: 0;
}
.target-radio {
  width: 24px; height: 24px; border-radius: 50%;
  border: 2px solid var(--color-line-2);
  flex-shrink: 0;
}

.target-name { font-size: 14px; font-weight: 600; color: var(--color-ink); }
</style>
