<script setup lang="ts">
import { computed } from 'vue'

const props = withDefaults(
  defineProps<{
    variant?: 'default' | 'accent' | 'ok' | 'warn' | 'danger'
    /** Leading dot. It inherits the badge's text colour, so it needs no tinting. */
    dot?: boolean
    /** Monospace face, for codes and ids. */
    mono?: boolean
    /**
     * Colour for a value with its own identity — an auth provider, say. Drives
     * the text and border, and the dot follows via currentColor. Takes
     * precedence over `variant`.
     */
    tone?: string
  }>(),
  { variant: 'default', dot: false, mono: false, tone: undefined },
)

const classes = computed(() => [
  'badge',
  props.variant !== 'default' && `badge-${props.variant}`,
  props.mono && 'mono',
])

const toneStyle = computed(() =>
  props.tone
    ? {
        color: props.tone,
        borderColor: `color-mix(in oklch, ${props.tone}, transparent 60%)`,
      }
    : undefined,
)
</script>

<template>
  <span :class="classes" :style="toneStyle">
    <span v-if="dot" class="badge-dot" />
    <slot />
  </span>
</template>

<style scoped>
@layer components {
  /* The whole .badge family lives here, including the variants. Splitting it
     would put a global modifier against this scoped base, which scoping raises
     above it — see docs/frontend-styling.md. `.badges`, the row container, is
     the caller's element and stays in components.css. */
  .badge {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    padding: 3px 8px;
    border-radius: var(--radius-item);
    font-size: var(--text-caption-1);
    line-height: var(--text-caption-1--line-height);
    font-weight: var(--text-caption-1--font-weight);
    letter-spacing: var(--text-caption-1--letter-spacing);
    white-space: nowrap;
    flex-shrink: 0;
    max-width: 100%;
    background: var(--color-surface-3);
    border: 1px solid var(--color-line-2);
    color: var(--color-ink-2);
  }
  .badge-accent {
    background: color-mix(in oklch, var(--color-accent), transparent 88%);
    border-color: color-mix(in oklch, var(--color-accent), transparent 55%);
    color: var(--color-accent-2);
  }
  .badge-ok {
    background: color-mix(in oklch, var(--color-ok), transparent 85%);
    border-color: color-mix(in oklch, var(--color-ok), transparent 60%);
    color: var(--color-ok);
  }
  .badge-warn {
    background: color-mix(in oklch, var(--color-warn), transparent 85%);
    border-color: color-mix(in oklch, var(--color-warn), transparent 55%);
    color: var(--color-warn);
  }
  .badge-danger {
    background: color-mix(in oklch, var(--color-danger), transparent 88%);
    border-color: color-mix(in oklch, var(--color-danger), transparent 60%);
    color: var(--color-danger);
  }
  .badge-dot {
    width: 6px;
    height: 6px;
    border-radius: 50%;
    background: currentColor;
    flex-shrink: 0;
  }
}
</style>
