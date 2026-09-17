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
        <div
          v-bind="api.getContentProps()"
          class="select-content gradient-border"
        >
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

<style scoped>
/* Colocated from components.css: used only by this component. The @layer
   wrapper is kept so precedence against Tailwind utilities is unchanged.
   Shared primitives stay in assets/components.css. */
@layer components {
  .select-root {
    width: 100%;
  }
  /* Layout only: the field chrome — ground, border, radius, body-3 text and the
     focus ring — comes from the shared .inp rule, so the trigger reads as one
     of the fields around it. admin-web raises its select instead (surface-raise
     + band, because their text inputs are transparent); the *panel* follows
     them, the trigger follows its neighbours here. */
  .select-trigger {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 8px;
    text-align: left;
    cursor: pointer;
  }
  .select-trigger[data-disabled] {
    opacity: 0.5;
    cursor: not-allowed;
  }
  .select-invalid {
    border-color: var(--color-danger);
  }
  .select-value {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .select-placeholder {
    color: var(--color-ink-3);
  }
  .select-indicator {
    display: grid;
    place-items: center;
    color: var(--color-ink-3);
    flex-shrink: 0;
    transition: transform 0.15s ease;
  }
  .select-trigger[data-state='open'] .select-indicator {
    transform: rotate(180deg);
  }
  .select-positioner {
    isolation: isolate;
  }
  .select-content {
    /* No border: the gradient-border band is the edge. Keeping both drew a
       solid hairline with a second, lighter one just inside it. */
    z-index: var(--z-index-dropdown);
    padding: 6px;
    background: var(--color-surface-raise);
    border-radius: var(--radius-surface);
    box-shadow: var(--shadow-floating);
    animation: fb-pop 0.15s ease both;
  }
  .select-content:focus-visible {
    outline: none;
  }
  .select-list {
    list-style: none;
    margin: 0;
    padding: 0;
    max-height: 280px;
    overflow-y: auto;
  }
  .select-item {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 10px;
    padding: 8px 10px;
    border-radius: var(--radius-item);
    font-size: 13.5px;
    color: var(--color-ink);
    cursor: pointer;
    user-select: none;
  }
  .select-item[data-highlighted] {
    background: var(--color-surface-indent);
  }
  .select-item[data-state='checked'] {
    color: var(--color-accent-2);
  }
  .select-item[data-disabled] {
    color: var(--color-ink-3);
    cursor: not-allowed;
  }
  .select-check {
    color: var(--color-accent-2);
    font-size: 12px;
  }
  .select-item[data-state='unchecked'] .select-check {
    visibility: hidden;
  }
  .select-group-label {
    padding: 8px 10px 4px;
    font-size: 10.5px;
    font-weight: 500;
    text-transform: uppercase;
    letter-spacing: 0.08em;
    color: var(--color-ink-3);
  }
}
</style>
