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

<style scoped>
/* Colocated from components.css: used only by this component. The @layer
   wrapper is kept so precedence against Tailwind utilities is unchanged.
   Shared primitives stay in assets/components.css. */
@layer components {
  .token-input {
    display: flex;
    flex-wrap: wrap;
    gap: 7px;
    align-items: center;
    /* Matches .inp exactly — same background token, radius and text inset — so
  a stacked form does not show one field on a different surface. The
  vertical padding is smaller because the chips inside carry their own. */
    padding: 8px 12px;
    min-height: 42px;
    background: var(--color-surface);
    border: 1px solid var(--color-line-2);
    border-radius: 8px;
    transition:
      border-color 0.15s ease,
      box-shadow 0.15s ease;
  }
}

/* Colocated from components.css: used only by this component. The @layer
   wrapper is kept so precedence against Tailwind utilities is unchanged.
   Shared primitives stay in assets/components.css. */
@layer components {
  .token-input:focus-within {
    border-color: var(--color-accent);
    box-shadow: 0 0 0 3px
      color-mix(in oklch, var(--color-accent), transparent 88%);
  }
  .token-input input {
    flex: 1;
    min-width: 140px;
    width: auto;
    /* 2px keeps an empty control at .inp's 42px: 8+8 outer + 2+2 here + a
  20.25px line + 2px border. A chip row is taller, which is correct. */
    padding: 2px 0;
    border: none;
    border-radius: 0;
    background: transparent;
    outline: none;
    font-family: inherit;
    font-size: 13.5px;
    color: var(--color-ink);
  }
  .token-input input::placeholder {
    color: var(--color-ink-3);
  }
  .token {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    padding: 3px 4px 3px 10px;
    border-radius: 6px;
    font-family: var(--font-mono);
    font-size: 12.5px;
    background: var(--color-surface-4);
    border: 1px solid var(--color-line-2);
    color: var(--color-ink);
  }
  /* Zag's editable tags swap the chip for an input on double-click; size it like the chip so the row does not jump. */
  .token-edit {
    width: auto;
    min-width: 80px;
    padding: 3px 8px;
    border: 1px solid var(--color-accent);
    border-radius: 6px;
    background: var(--color-surface);
    color: var(--color-ink);
    font-family: var(--font-mono);
    font-size: 12.5px;
    outline: none;
  }
  .token-invalid {
    background: color-mix(in oklch, var(--color-danger), transparent 84%);
    border-color: color-mix(in oklch, var(--color-danger), transparent 45%);
    color: oklch(0.85 0.1 25);
  }
  /* Remove affordance — `.token button` so both `<button class="x">` and a bare button work without a second class. */
  .token button {
    display: grid;
    place-items: center;
    padding: 0 4px;
    border: none;
    border-radius: 4px;
    background: transparent;
    color: var(--color-ink-3);
    font-size: 14px;
    line-height: 1;
    cursor: pointer;
  }
  .token button:hover {
    color: var(--color-danger);
    background: color-mix(in oklch, var(--color-danger), transparent 85%);
  }
}
</style>
