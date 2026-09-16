<script setup lang="ts">
import * as numberInput from '@zag-js/number-input'
import { normalizeProps, useMachine } from '@zag-js/vue'
import { computed, useId } from 'vue'

const props = withDefaults(
  defineProps<{
    /** `''` means unset — two call sites use that for "unlimited". */
    modelValue: number | ''
    min?: number
    max?: number
    step?: number
    placeholder?: string
    disabled?: boolean
    /** Accessible name, when the visible label sits outside. */
    ariaLabel?: string
  }>(),
  {
    min: undefined,
    max: undefined,
    step: 1,
    placeholder: undefined,
    disabled: false,
    ariaLabel: undefined,
  },
)

const emit = defineEmits<{ 'update:modelValue': [value: number | ''] }>()

const id = useId()

const service = useMachine(
  numberInput.machine,
  computed(() => ({
    id,
    value: props.modelValue === '' ? '' : String(props.modelValue),
    min: props.min,
    max: props.max,
    step: props.step,
    disabled: props.disabled,
    // Off by default: scrolling a page should never silently change a value.
    allowMouseWheel: false,
    onValueChange: ({
      value,
      valueAsNumber,
    }: numberInput.ValueChangeDetails) => {
      // Preserve the caller's empty-means-unset contract rather than emitting NaN.
      emit(
        'update:modelValue',
        value === '' || Number.isNaN(valueAsNumber) ? '' : valueAsNumber,
      )
    },
  })),
)

const api = computed(() => numberInput.connect(service, normalizeProps))
</script>

<template>
  <div v-bind="api.getRootProps()" class="num-root">
    <div v-bind="api.getControlProps()" class="num-control">
      <input
        v-bind="api.getInputProps()"
        class="num-input"
        :placeholder="placeholder"
        :aria-label="ariaLabel"
      />
      <span class="num-steppers">
        <button
          v-bind="api.getIncrementTriggerProps()"
          class="num-step"
          aria-label="Increase"
        >
          <svg
            width="10"
            height="10"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="3"
            stroke-linecap="round"
            stroke-linejoin="round"
          >
            <path d="m6 15 6-6 6 6" />
          </svg>
        </button>
        <button
          v-bind="api.getDecrementTriggerProps()"
          class="num-step"
          aria-label="Decrease"
        >
          <svg
            width="10"
            height="10"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="3"
            stroke-linecap="round"
            stroke-linejoin="round"
          >
            <path d="m6 9 6 6 6-6" />
          </svg>
        </button>
      </span>
    </div>
  </div>
</template>
