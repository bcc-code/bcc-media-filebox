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
