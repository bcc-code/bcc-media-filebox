<script lang="ts">
/** A selectable option. `value` keeps the caller's own type. */
export type UiSelectOption<T = string | number> = {
  value: T
  label: string
  disabled?: boolean
}

/** A labelled set of options, rendered like <optgroup>. */
export type UiSelectGroup<T = string | number> = {
  label: string
  options: UiSelectOption<T>[]
}

export type UiSelectEntry<T = string | number> =
  UiSelectOption<T> | UiSelectGroup<T>

const isGroup = <T,>(entry: UiSelectEntry<T>): entry is UiSelectGroup<T> =>
  'options' in entry
</script>

<script setup lang="ts" generic="T extends string | number">
import * as select from '@zag-js/select'
import { normalizeProps, useMachine } from '@zag-js/vue'
import { computed, useId } from 'vue'

const props = withDefaults(
  defineProps<{
    modelValue: T | null | undefined
    /** Options, optionally wrapped in groups. */
    options: UiSelectEntry<T>[]
    /** Shown on the trigger while nothing is selected. */
    placeholder?: string
    disabled?: boolean
    /** Draws the error border, for failed validation. */
    invalid?: boolean
    /** Accessible name for the trigger, when the visible label sits outside. */
    ariaLabel?: string
  }>(),
  {
    placeholder: 'Select…',
    disabled: false,
    invalid: false,
    ariaLabel: undefined,
  },
)

const emit = defineEmits<{ 'update:modelValue': [value: T] }>()

const id = useId()

// Keyboard navigation and typeahead run over the flat list; grouping is purely
// how it renders.
const flat = computed(() =>
  props.options.flatMap((e) => (isGroup(e) ? e.options : [e])),
)

// Only treat the model value as a selection when an option actually matches it.
// GrantModal starts at '' with no '' option, which would otherwise read as
// "selected" and render an empty trigger instead of the placeholder — while
// TargetModal's '' *is* a real option and must still resolve.
const selectedValues = computed(() => {
  if (props.modelValue == null) return []
  const v = String(props.modelValue)
  return flat.value.some((o) => String(o.value) === v) ? [v] : []
})

const hasSelection = computed(() => selectedValues.value.length > 0)

const collection = computed(() =>
  select.collection({
    items: flat.value,
    // Zag addresses items by string; `value` may be a number, so map both ways.
    itemToValue: (item) => String(item.value),
    itemToString: (item) => item.label,
    isItemDisabled: (item) => !!item.disabled,
  }),
)

const service = useMachine(
  select.machine,
  computed(() => ({
    id,
    collection: collection.value,
    value: selectedValues.value,
    disabled: props.disabled,
    invalid: props.invalid,
    // Match the trigger's width so the list lines up with the field.
    positioning: { sameWidth: true, gutter: 6 },
    onValueChange: ({ items }: select.ValueChangeDetails) => {
      // Emit the caller's original value, so a numeric option stays numeric.
      const picked = items[0] as UiSelectOption<T> | undefined
      if (picked) emit('update:modelValue', picked.value)
    },
  })),
)

const api = computed(() => select.connect(service, normalizeProps))
</script>

<template>
  <div v-bind="api.getRootProps()" class="select-root">
    <div v-bind="api.getControlProps()">
      <button
        v-bind="api.getTriggerProps()"
        class="select-trigger"
        :class="{ 'select-invalid': invalid }"
        :aria-label="ariaLabel"
      >
        <span
          v-bind="api.getValueTextProps()"
          class="select-value"
          :class="{ 'select-placeholder': !hasSelection }"
        >
          {{ hasSelection ? api.valueAsString : placeholder }}
        </span>
        <span v-bind="api.getIndicatorProps()" class="select-indicator">
          <svg
            width="14"
            height="14"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="2.2"
            stroke-linecap="round"
            stroke-linejoin="round"
          >
            <path d="m6 9 6 6 6-6" />
          </svg>
        </span>
      </button>
    </div>

    <Teleport to="body">
      <div v-bind="api.getPositionerProps()" class="select-positioner">
        <div v-bind="api.getContentProps()" class="select-content">
          <ul v-bind="api.getListProps()" class="select-list">
            <template v-for="(entry, i) in options" :key="i">
              <li
                v-if="isGroup(entry)"
                v-bind="api.getItemGroupProps({ id: `g${i}` })"
              >
                <div
                  v-bind="api.getItemGroupLabelProps({ htmlFor: `g${i}` })"
                  class="select-group-label"
                >
                  {{ entry.label }}
                </div>
                <div
                  v-for="opt in entry.options"
                  :key="String(opt.value)"
                  v-bind="api.getItemProps({ item: opt })"
                  class="select-item"
                >
                  <span v-bind="api.getItemTextProps({ item: opt })">{{
                    opt.label
                  }}</span>
                  <span
                    v-bind="api.getItemIndicatorProps({ item: opt })"
                    class="select-check"
                    >✓</span
                  >
                </div>
              </li>
              <li
                v-else
                v-bind="api.getItemProps({ item: entry })"
                class="select-item"
              >
                <span v-bind="api.getItemTextProps({ item: entry })">{{
                  entry.label
                }}</span>
                <span
                  v-bind="api.getItemIndicatorProps({ item: entry })"
                  class="select-check"
                  >✓</span
                >
              </li>
            </template>
          </ul>
        </div>
      </div>
    </Teleport>
  </div>
</template>
