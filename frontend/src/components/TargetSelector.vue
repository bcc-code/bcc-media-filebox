<script setup lang="ts">
import { computed } from 'vue'
import type { TargetInfo } from '../types'
import UiRadioGroup, { type UiRadioOption } from './ui/UiRadioGroup.vue'

const props = defineProps<{
  modelValue: string
  targets: TargetInfo[]
}>()

const emit = defineEmits<{
  'update:modelValue': [name: string]
}>()

// The target name is the value, so no id mapping is needed.
const options = computed<UiRadioOption<string>[]>(() =>
  props.targets.map((t) => ({ value: t.name, label: t.name })),
)
</script>

<template>
  <UiRadioGroup
    :model-value="modelValue"
    :options="options"
    layout="grid"
    indicator="check"
    aria-label="Upload target"
    @update:model-value="emit('update:modelValue', $event)"
  >
    <!--
      One icon slot per option, by the `icon-<value>` convention UiTabs and
      UiRadioGroup share. Every target gets the same generic disk tile; the
      per-option slot is what lets a future target carry its own.
    -->
    <template v-for="t in targets" #[`icon-${t.name}`] :key="t.name">
      <span class="target-ic">
        <svg
          width="20"
          height="20"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          stroke-width="1.8"
          stroke-linecap="round"
          stroke-linejoin="round"
        >
          <ellipse cx="12" cy="5" rx="8" ry="3" />
          <path d="M4 5v6c0 1.66 3.58 3 8 3s8-1.34 8-3V5" />
          <path d="M4 11v6c0 1.66 3.58 3 8 3s8-1.34 8-3v-6" />
        </svg>
      </span>
    </template>
  </UiRadioGroup>
</template>

<style scoped>
/* The icon tile is this component's own decoration, not part of the shared
   card. */
.target-ic {
  display: grid;
  place-items: center;
  width: 40px;
  height: 40px;
  border-radius: 10px;
  background: var(--color-surface-3);
  border: 1px solid var(--color-line-2);
  color: var(--color-ink-3);
  transition:
    color 0.15s ease,
    background 0.15s ease,
    border-color 0.15s ease;
}

/* The tile reacts to the enclosing item's checked state. That item is rendered
   by UiRadioGroup, but the tile is our own slot content, and scoped CSS tags
   only the last compound selector — so a plain descendant selector reaches it
   without :deep(). */
[data-part='item'][data-state='checked'] .target-ic {
  background: color-mix(in oklch, var(--color-accent), transparent 82%);
  border-color: color-mix(in oklch, var(--color-accent), transparent 55%);
  color: var(--color-accent-2);
}
</style>
