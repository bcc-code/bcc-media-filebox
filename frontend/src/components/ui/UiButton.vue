<script setup lang="ts">
import { computed } from 'vue'
import { RouterLink } from 'vue-router'

const props = withDefaults(
  defineProps<{
    variant?: 'default' | 'primary' | 'ghost' | 'danger'
    size?: 'sm' | 'md' | 'lg'
    /** Full width, for stacked form actions. */
    block?: boolean
    /** Square, for a bare icon. */
    icon?: boolean
    /** Accent-tinted "on" state, for toggles. */
    active?: boolean
    /** Shows `loadingLabel` and blocks the click while a request is in flight. */
    loading?: boolean
    loadingLabel?: string
    disabled?: boolean
    /** Defaults to `button` so a button inside a form never submits by accident. */
    type?: 'button' | 'submit' | 'reset'
    /** Renders a plain `<a>` instead — for a button-shaped link out of the app. */
    href?: string
    /** Renders a `<router-link>` instead — for in-app navigation. */
    to?: string
  }>(),
  {
    variant: 'default',
    size: 'md',
    block: false,
    icon: false,
    active: false,
    loading: false,
    loadingLabel: undefined,
    disabled: false,
    type: 'button',
    href: undefined,
    to: undefined,
  },
)

const classes = computed(() => [
  'btn',
  props.variant !== 'default' && `btn-${props.variant}`,
  props.size !== 'md' && `btn-${props.size}`,
  props.block && 'btn-block',
  props.icon && 'btn-icon',
  props.active && 'btn-active',
])

// A link stays a link: middle-click, Cmd-click and "copy address" are browser
// behaviour a <button> with a click handler cannot reproduce.
const tag = computed(() =>
  props.to ? RouterLink : props.href ? 'a' : 'button',
)

const tagProps = computed(() => {
  const off = props.disabled || props.loading
  // `disabled` and `type` are button-only attributes. A link ignores `disabled`
  // outright, so a disabled one is marked for assistive tech and taken out of
  // the tab order instead — otherwise it stays clickable while looking dead.
  const link = props.to
    ? { to: props.to }
    : props.href
      ? { href: props.href }
      : null
  if (link) return off ? { ...link, 'aria-disabled': true, tabindex: -1 } : link
  return { type: props.type, disabled: off }
})
</script>

<template>
  <component :is="tag" v-bind="tagProps" :class="classes">
    <!-- A pending label replaces the content rather than sitting beside it, so
         the button does not change width mid-request. -->
    <template v-if="loading && loadingLabel">{{ loadingLabel }}</template>
    <slot v-else />
  </component>
</template>

<style scoped>
/* Colocated from components.css: used only by this component. The @layer
   wrapper is kept so precedence against Tailwind utilities is unchanged.
   Shared primitives stay in assets/components.css. */
@layer components {
  .btn {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    gap: 8px;
    /* .btn also dresses <a>/<router-link>, which arrive underlined. */
    text-decoration: none;
    padding: 9px 14px;
    border-radius: 8px;
    font-family: inherit;
    font-size: 13px;
    font-weight: 500;
    cursor: pointer;
    white-space: nowrap;
    flex-shrink: 0;
    border: 1px solid var(--color-line-2);
    background: var(--color-surface-2);
    color: var(--color-ink);
    transition:
      border-color 0.15s ease,
      background 0.15s ease,
      opacity 0.15s ease;
  }
  .btn:hover {
    border-color: var(--color-ink-3);
    background: var(--color-surface-3);
  }
  .btn-icon {
    padding: 6px;
    width: 30px;
    height: 30px;
  }
  .btn-block {
    width: 100%;
  }
  .btn-active {
    color: oklch(0.8 0.1 250);
    border-color: color-mix(in oklch, var(--color-accent), transparent 55%);
    background: color-mix(in oklch, var(--color-accent), transparent 88%);
  }
  .btn:disabled {
    opacity: 0.45;
    cursor: not-allowed;
  }
  .btn:disabled:hover {
    border-color: var(--color-line-2);
    background: var(--color-surface-2);
  }

  /* Scales and colours. These must live in the same scope as `.btn` above: a
     scoped base selector carries a [data-v] attribute, so a global `.btn-sm`
     (0,1,0) loses to the scoped `.btn` (0,2,0) and neither size nor variant
     applies. */
  .btn-sm {
    padding: 5px 10px;
    font-size: 12px;
  }
  .btn-lg {
    padding: 11px 18px;
    border-radius: 9px;
    font-size: 14px;
    font-weight: 600;
  }
  .btn-primary:hover {
    background: var(--color-accent-hover);
    border-color: var(--color-accent-hover);
  }
  .btn-primary:disabled:hover {
    background: var(--color-accent);
    border-color: var(--color-accent);
  }
  .btn-primary {
    background: var(--color-accent);
    border-color: var(--color-accent);
    color: var(--color-accent-ink);
    font-weight: 600;
  }
  .btn-ghost:hover {
    background: var(--color-surface-2);
  }
  .btn-ghost {
    background: transparent;
  }
  .btn-danger:hover {
    background: color-mix(in oklch, var(--color-danger), transparent 88%);
    border-color: var(--color-danger);
  }
  .btn-danger {
    background: transparent;
    color: var(--color-danger);
    border-color: color-mix(in oklch, var(--color-danger), transparent 70%);
  }
}
</style>
