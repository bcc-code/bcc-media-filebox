<script lang="ts">
/** One entry in a UiMenu — an action, or a divider between groups. */
export type UiMenuEntry =
  | {
      type?: 'item'
      label: string
      onSelect: () => void
      disabled?: boolean
      /** Marks destructive actions (red label). */
      danger?: boolean
    }
  | { type: 'separator' }
</script>

<script setup lang="ts">
import * as menu from '@zag-js/menu'
import { normalizeProps, useMachine } from '@zag-js/vue'
import { computed, useId } from 'vue'

const props = withDefaults(
  defineProps<{
    items: UiMenuEntry[]
    /** Where the menu sits relative to its trigger. */
    placement?: menu.PositioningOptions['placement']
    /** Wrap keyboard navigation from the last item back to the first. */
    loopFocus?: boolean
    /** Menu width. Defaults to sizing from content. */
    width?: string
  }>(),
  { placement: 'bottom-end', loopFocus: true, width: undefined },
)

const emit = defineEmits<{ (e: 'openChange', open: boolean): void }>()

const id = useId()

/** Index-keyed so separators can sit in the same list without taking a value. */
const actions = computed(() =>
  props.items.flatMap((entry, i) =>
    entry.type === 'separator' ? [] : [{ ...entry, value: String(i) }],
  ),
)

const service = useMachine(
  menu.machine,
  computed(() => ({
    id,
    loopFocus: props.loopFocus,
    positioning: { placement: props.placement, gutter: 8 },
    // Zag routes pointer and keyboard selection through this one callback.
    onSelect: ({ value }: menu.SelectionDetails) =>
      actions.value.find((a) => a.value === value)?.onSelect(),
    onOpenChange: ({ open }: menu.OpenChangeDetails) =>
      emit('openChange', open),
  })),
)

const api = computed(() => menu.connect(service, normalizeProps))

defineExpose({
  /** Close the menu from outside, e.g. after navigating away. */
  close: () => api.value.setOpen(false),
})
</script>

<template>
  <!-- A real <button> so Zag owns its ARIA state; the slot carries the styling. -->
  <button v-bind="api.getTriggerProps()" class="menu-trigger-reset">
    <slot name="trigger" :open="api.open" />
  </button>

  <Teleport to="body">
    <div
      v-if="api.open"
      v-bind="api.getPositionerProps()"
      class="menu-positioner"
    >
      <div
        v-bind="api.getContentProps()"
        class="menu-content"
        :style="width ? { width } : undefined"
      >
        <div v-if="$slots.header" class="menu-header">
          <slot name="header" />
        </div>

        <template v-for="(entry, i) in items" :key="i">
          <div
            v-if="entry.type === 'separator'"
            v-bind="api.getSeparatorProps()"
            class="menu-separator"
          />
          <div
            v-else
            v-bind="
              api.getItemProps({
                value: String(i),
                disabled: entry.disabled,
                valueText: entry.label,
              })
            "
            class="menu-item"
            :class="{ 'menu-item-danger': entry.danger }"
          >
            {{ entry.label }}
          </div>
        </template>
      </div>
    </div>
  </Teleport>
</template>

<style scoped>
/* Colocated from components.css: used only by this component. The @layer
   wrapper is kept so precedence against Tailwind utilities is unchanged.
   Shared primitives stay in assets/components.css. */
@layer components {
  .menu-positioner {
    isolation: isolate;
  }
  .menu-content {
    z-index: var(--z-index-dropdown);
    min-width: 200px;
    padding: 6px;
    background: var(--color-surface-2);
    border: 1px solid var(--color-line-2);
    border-radius: 12px;
    box-shadow: 0 18px 50px rgb(0 0 0 / 0.55);
    animation: fb-pop 0.15s ease both;
  }
  .menu-content:focus-visible {
    outline: none;
  }
  .menu-item {
    display: flex;
    align-items: center;
    gap: 8px;
    width: 100%;
    padding: 8px 12px;
    border-radius: 6px;
    font-size: 14px;
    color: var(--color-ink);
    cursor: pointer;
    user-select: none;
  }
  .menu-item[data-highlighted] {
    background: var(--color-surface-3);
  }
  .menu-item[data-disabled] {
    color: var(--color-ink-3);
    cursor: not-allowed;
  }
  .menu-item[data-disabled][data-highlighted] {
    background: transparent;
  }
  .menu-item-danger {
    color: var(--color-danger);
  }
  .menu-item-danger[data-highlighted] {
    background: color-mix(in oklch, var(--color-danger), transparent 88%);
  }
  .menu-separator {
    height: 1px;
    margin: 4px 0;
    background: var(--color-line);
  }
  .menu-header {
    padding: 10px 12px 12px;
    margin-bottom: 4px;
    border-bottom: 1px solid var(--color-line);
  }
}

.menu-trigger-reset {
  padding: 0;
  border: none;
  background: none;
  font: inherit;
  color: inherit;
  cursor: pointer;
}
</style>
