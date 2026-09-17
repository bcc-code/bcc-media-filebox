<script lang="ts">
/** One tab. `count` renders as a badge beside the label. */
export type UiTabEntry = {
  value: string
  label: string
  count?: number
}
</script>

<script setup lang="ts">
import * as zagTabs from '@zag-js/tabs'
import { normalizeProps, useMachine } from '@zag-js/vue'
import { computed, useId } from 'vue'

const props = withDefaults(
  defineProps<{
    modelValue: string
    tabs: UiTabEntry[]
    /** Extra classes for the tab list, for surface-specific padding. */
    listClass?: string
    /** Extra classes for each panel, e.g. a page-width wrapper. */
    panelClass?: string
  }>(),
  { listClass: undefined, panelClass: undefined },
)

const emit = defineEmits<{ 'update:modelValue': [value: string] }>()

const id = useId()

const service = useMachine(
  zagTabs.machine,
  computed(() => ({
    id,
    value: props.modelValue,
    onValueChange: ({ value }: zagTabs.ValueChangeDetails) =>
      emit('update:modelValue', value),
  })),
)

const api = computed(() => zagTabs.connect(service, normalizeProps))
</script>

<template>
  <div v-bind="api.getRootProps()">
    <div v-bind="api.getListProps()" class="tab-list" :class="listClass">
      <button
        v-for="tab in tabs"
        :key="tab.value"
        v-bind="api.getTriggerProps({ value: tab.value })"
        class="tab"
      >
        <slot :name="`icon-${tab.value}`" />
        {{ tab.label }}
        <span v-if="tab.count !== undefined" class="count">{{
          tab.count
        }}</span>
      </button>
    </div>

    <div
      v-for="tab in tabs"
      :key="tab.value"
      v-bind="api.getContentProps({ value: tab.value })"
      :class="panelClass"
    >
      <!--
        The panel element always exists so the trigger's aria-controls resolves,
        but its content mounts only while selected. Rendering all panels at once
        would mount every admin tab's component up front, firing their fetches
        and watchers — the v-if/v-else-if chain this replaces did not.
      -->
      <slot v-if="modelValue === tab.value" :name="tab.value" />
    </div>
  </div>
</template>

<style scoped>
/* Colocated from components.css: used only by this component. The @layer
   wrapper is kept so precedence against Tailwind utilities is unchanged.
   Shared primitives stay in assets/components.css. */
@layer components {
  .tab-list {
    display: flex;
    gap: 2px;
    border-bottom: 1px solid var(--color-line);
  }
  .tab {
    display: inline-flex;
    align-items: center;
    gap: 8px;
    padding: 13px 14px;
    margin-bottom: -1px;
    border: none;
    border-bottom: 2px solid transparent;
    background: transparent;
    color: var(--color-ink-2);
    font-family: inherit;
    font-size: 14px;
    font-weight: 500;
    cursor: pointer;
    transition: color 0.15s ease;
  }
  .tab:hover {
    color: var(--color-ink-2);
  }
  .tab[data-selected] {
    color: var(--color-ink);
    border-bottom-color: var(--color-accent);
  }
  .tab:focus-visible {
    outline: none;
    color: var(--color-ink);
    background: var(--color-surface-2);
  }
}
/* Layout variants callers select through `list-class`. They belong here rather
   than with the caller: the class is applied to this component's element, so
   only this component's styles can reach it. */
.tab-list-inset {
  padding: 0 28px;
  background: var(--color-surface);
}
.tab-list-spaced {
  margin-bottom: 26px;
}
</style>
