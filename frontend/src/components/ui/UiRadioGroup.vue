<script lang="ts">
/** One choice. `description` only renders in the `card` variant. */
export type UiRadioOption<T = string> = {
  value: T
  label: string
  description?: string
  disabled?: boolean
}
</script>

<script setup lang="ts" generic="T extends string">
import * as radio from '@zag-js/radio-group'
import { normalizeProps, useMachine } from '@zag-js/vue'
import { computed, useId } from 'vue'

const props = withDefaults(
  defineProps<{
    modelValue: T
    options: UiRadioOption<T>[]
    /** `card` stacks labelled cards; `segmented` is a compact inline switch. */
    variant?: 'card' | 'segmented'
    disabled?: boolean
    /** Accessible name for the group, when the visible label sits outside. */
    ariaLabel?: string
  }>(),
  { variant: 'card', disabled: false, ariaLabel: undefined },
)

const emit = defineEmits<{ 'update:modelValue': [value: T] }>()

const id = useId()

const service = useMachine(
  radio.machine,
  computed(() => ({
    id,
    value: props.modelValue,
    disabled: props.disabled,
    onValueChange: ({ value }: radio.ValueChangeDetails) =>
      emit('update:modelValue', value as T),
  })),
)

const api = computed(() => radio.connect(service, normalizeProps))

const isCard = computed(() => props.variant === 'card')

/** Zag derives per-item state from these, so they must reach every getter. */
const itemArgs = (option: UiRadioOption<T>) => ({
  value: option.value,
  disabled: option.disabled,
})
const itemProps = (option: UiRadioOption<T>) =>
  api.value.getItemProps(itemArgs(option))
</script>

<template>
  <div
    v-bind="api.getRootProps()"
    :class="isCard ? 'radio-cards' : 'seg'"
    :aria-label="ariaLabel"
  >
    <label
      v-for="option in options"
      :key="option.value"
      v-bind="itemProps(option)"
      :class="isCard ? 'radio-card' : 'seg-item'"
    >
      <!-- The control is the visual radio dot; the card variant shows it, the
           segmented one relies on the selected background instead. -->
      <span
        v-if="isCard"
        v-bind="api.getItemControlProps(itemArgs(option))"
        class="radio-dot"
      />
      <span v-bind="api.getItemTextProps(itemArgs(option))" class="radio-body">
        <span class="radio-label">
          <slot :name="`icon-${option.value}`" />
          {{ option.label }}
        </span>
        <span v-if="isCard && option.description" class="radio-description">
          {{ option.description }}
        </span>
      </span>
      <input v-bind="api.getItemHiddenInputProps(itemArgs(option))" />
    </label>
  </div>
</template>
