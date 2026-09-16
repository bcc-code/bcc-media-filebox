<script setup lang="ts">
import * as tagsInput from '@zag-js/tags-input'
import { normalizeProps, useMachine } from '@zag-js/vue'
import { computed, useId } from 'vue'

const props = withDefaults(
  defineProps<{
    modelValue: string[]
    placeholder?: string
    disabled?: boolean
    /** Splits typed and pasted input. */
    delimiter?: string | RegExp
    /** Marks a tag visually without rejecting it — e.g. a malformed address. */
    isInvalid?: (value: string) => boolean
    /** Accessible name, when the visible label sits outside. */
    ariaLabel?: string
  }>(),
  {
    placeholder: undefined,
    disabled: false,
    delimiter: () => /[,\s;]+/,
    isInvalid: undefined,
    ariaLabel: undefined,
  },
)

const emit = defineEmits<{ 'update:modelValue': [value: string[]] }>()

const id = useId()

const service = useMachine(
  tagsInput.machine,
  computed(() => ({
    id,
    value: props.modelValue,
    disabled: props.disabled,
    delimiter: props.delimiter,
    addOnPaste: true,
    // Commit whatever is half-typed when focus leaves, so a value is never
    // silently lost by clicking away.
    blurBehavior: 'add' as const,
    editable: true,
    onValueChange: ({ value }: tagsInput.ValueChangeDetails) =>
      emit('update:modelValue', value),
  })),
)

const api = computed(() => tagsInput.connect(service, normalizeProps))
</script>

<template>
  <div v-bind="api.getRootProps()">
    <div v-bind="api.getControlProps()" class="token-input">
      <span
        v-for="(value, index) in api.value"
        :key="`${value}-${index}`"
        v-bind="api.getItemProps({ index, value })"
      >
        <span
          v-bind="api.getItemPreviewProps({ index, value })"
          class="token"
          :class="{ 'token-invalid': isInvalid?.(value) }"
        >
          <span v-bind="api.getItemTextProps({ index, value })">{{
            value
          }}</span>
          <button
            v-bind="api.getItemDeleteTriggerProps({ index, value })"
            :aria-label="`Remove ${value}`"
          >
            <svg
              width="12"
              height="12"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              stroke-width="2.4"
              stroke-linecap="round"
            >
              <path d="M18 6 6 18M6 6l12 12" />
            </svg>
          </button>
        </span>
        <input
          v-bind="api.getItemInputProps({ index, value })"
          class="token-edit"
        />
      </span>

      <input
        v-bind="api.getInputProps()"
        :placeholder="placeholder"
        :aria-label="ariaLabel"
      />
    </div>
    <input v-bind="api.getHiddenInputProps()" />
  </div>
</template>
