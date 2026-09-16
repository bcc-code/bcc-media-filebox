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
