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
    /** `card` renders labelled cards; `segmented` is a compact inline switch. */
    variant?: 'card' | 'segmented'
    /**
     * How `card` options are arranged. `grid` tiles them and moves the control
     * onto a top row beside the icon, so the label gets its own line. Ignored
     * by the `segmented` variant.
     */
    layout?: 'stack' | 'grid'
    /**
     * Selected-state glyph for the `card` variant. `check` is a filled circle
     * with a tick — a firmer "confirmed" than the radio dot, for a choice with
     * consequences.
     */
    indicator?: 'dot' | 'check'
    disabled?: boolean
    /** Accessible name for the group, when the visible label sits outside. */
    ariaLabel?: string
  }>(),
  {
    variant: 'card',
    layout: 'stack',
    indicator: 'dot',
    disabled: false,
    ariaLabel: undefined,
  },
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
const isGrid = computed(() => isCard.value && props.layout === 'grid')
const isCheck = computed(() => props.indicator === 'check')

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
    :class="[isCard ? 'radio-cards' : 'seg', isGrid && 'radio-cards-grid']"
    :aria-label="ariaLabel"
  >
    <label
      v-for="option in options"
      :key="option.value"
      v-bind="itemProps(option)"
      :class="isCard ? 'radio-card' : 'seg-item'"
    >
      <!-- The control is the visual radio dot; the card variant shows it, the
           segmented one relies on the selected background instead. A grid card
           pairs it with the icon on a top row instead of inline with the text. -->
      <span v-if="isGrid" class="radio-card-top">
        <slot :name="`icon-${option.value}`" />
        <span
          v-bind="api.getItemControlProps(itemArgs(option))"
          :class="isCheck ? 'radio-check' : 'radio-dot'"
        >
          <svg
            v-if="isCheck"
            width="14"
            height="14"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="3"
            stroke-linecap="round"
            stroke-linejoin="round"
          >
            <path d="M5 13l4 4L19 7" />
          </svg>
        </span>
      </span>
      <span
        v-else-if="isCard"
        v-bind="api.getItemControlProps(itemArgs(option))"
        :class="isCheck ? 'radio-check' : 'radio-dot'"
      >
        <svg
          v-if="isCheck"
          width="14"
          height="14"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          stroke-width="3"
          stroke-linecap="round"
          stroke-linejoin="round"
        >
          <path d="M5 13l4 4L19 7" />
        </svg>
      </span>
      <span v-bind="api.getItemTextProps(itemArgs(option))" class="radio-body">
        <span class="radio-label">
          <slot v-if="!isGrid" :name="`icon-${option.value}`" />
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

<style scoped>
/* Colocated from components.css: used only by this component. The @layer
   wrapper is kept so precedence against Tailwind utilities is unchanged.
   Shared primitives stay in assets/components.css. */
@layer components {
  .seg {
    display: flex;
    gap: 0;
    padding: 3px;
    border: 1px solid var(--color-line-2);
    border-radius: 8px;
    background: var(--color-surface);
  }
  .seg button,
  .seg .seg-item {
    display: inline-flex;
    flex: 1;
    align-items: center;
    justify-content: center;
    gap: 6px;
    padding: 7px 10px;
    border: none;
    border-radius: 6px;
    background: transparent;
    color: var(--color-ink-2);
    font-family: inherit;
    font-size: 13px;
    cursor: pointer;
    user-select: none;
  }
  .seg .seg-item:has(:focus-visible) {
    outline: 2px solid color-mix(in oklch, var(--color-accent), transparent 50%);
    outline-offset: -2px;
  }
  .seg .seg-item[data-disabled] {
    opacity: 0.5;
    cursor: not-allowed;
  }
}

/* Colocated from components.css: used only by this component. The @layer
   wrapper is kept so precedence against Tailwind utilities is unchanged.
   Shared primitives stay in assets/components.css. */
/* ============================================================ Radio group — stacked labelled cards, or the compact `.seg` switch above. Zag drives selection from data-state, so the same rules cover pointer and keyboard. ============================================================ */
@layer components {
  .radio-cards {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .radio-card {
    display: flex;
    align-items: flex-start;
    gap: 12px;
    padding: 13px 15px;
    border: 1.5px solid var(--color-line);
    border-radius: 11px;
    background: var(--color-surface-2);
    color: var(--color-ink);
    font-family: inherit;
    text-align: left;
    cursor: pointer;
    transition:
      border-color 0.15s,
      background 0.15s;
  }
  .radio-card:hover {
    border-color: var(--color-ink-3);
    background: var(--color-surface-3);
  }
  .radio-card[data-state='checked'] {
    border-color: var(--color-accent);
    background: color-mix(in oklch, var(--color-accent), transparent 90%);
  }
  .radio-card:has(:focus-visible) {
    outline: 2px solid color-mix(in oklch, var(--color-accent), transparent 50%);
    outline-offset: 2px;
  }
  .radio-card[data-disabled] {
    opacity: 0.5;
    cursor: not-allowed;
  }

  /* Grid cards: a tile of choices rather than a stacked list. */
  .radio-cards-grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
    gap: 12px;
  }
  .radio-cards-grid .radio-card {
    flex-direction: column;
    gap: 12px;
    padding: 14px;
  }
  /* The label owns its own line here, so the body must not share a flex row. */
  .radio-cards-grid .radio-body {
    flex: none;
    width: 100%;
  }
  /* Grid cards carry more weight than a stacked list item, and hold up better
against a label that wraps to two lines. */
  .radio-cards-grid .radio-label {
    font-weight: 600;
  }

  .radio-card-top {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    width: 100%;
  }

  .radio-dot {
    display: grid;
    place-items: center;
    width: 18px;
    height: 18px;
    margin-top: 1px;
    border: 1.5px solid var(--color-line-2);
    border-radius: 50%;
    flex-shrink: 0;
    transition: border-color 0.15s;
  }
  .radio-card[data-state='checked'] .radio-dot {
    border-color: var(--color-accent);
  }
  .radio-card[data-state='checked'] .radio-dot::after {
    content: '';
    width: 9px;
    height: 9px;
    border-radius: 50%;
    background: var(--color-accent);
  }

  /* The `check` indicator: an empty ring until selected, then a filled accent
circle with a tick. The tick is hidden by colour rather than by absence, so
the glyph never reflows the row. */
  .radio-check {
    display: grid;
    place-items: center;
    width: 24px;
    height: 24px;
    border: 2px solid var(--color-line-2);
    border-radius: 50%;
    flex-shrink: 0;
    color: transparent;
    transition:
      background 0.15s,
      border-color 0.15s,
      color 0.15s;
  }
  .radio-card[data-state='checked'] .radio-check {
    background: var(--color-accent);
    border-color: var(--color-accent);
    /* Same token .btn-primary uses for content sitting on accent. */
    color: var(--color-accent-ink);
  }

  .radio-body {
    flex: 1;
    min-width: 0;
  }
  .radio-label {
    display: flex;
    align-items: center;
    gap: 8px;
    font-size: 14px;
    font-weight: 500;
  }
  .radio-description {
    display: block;
    margin-top: 3px;
    font-size: 12.5px;
    line-height: 1.45;
    color: var(--color-ink-3);
  }
  .radio-card[data-state='checked'] .radio-label svg {
    color: var(--color-accent);
  }
  .radio-label svg {
    color: var(--color-ink-2);
  }

  /* Checked state for the segmented variant. Left in components.css it was
     (0,3,0) — exactly the same as the scoped `.seg .seg-item` above — so which
     rule won came down to bundle order. */
  .seg .seg-item[data-state='checked'] {
    background: var(--color-surface-3);
    color: var(--color-ink);
  }
}
</style>
