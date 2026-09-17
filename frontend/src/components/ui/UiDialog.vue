<script setup lang="ts">
import * as dialog from '@zag-js/dialog'
import { normalizeProps, useMachine } from '@zag-js/vue'
import { computed, useId } from 'vue'

const props = withDefaults(
  defineProps<{
    /** Accessible name. Rendered as the panel heading unless the `title` slot is used. */
    title?: string
    /** Supporting line under the title; also becomes the dialog's description for screen readers. */
    description?: string
    /** Panel width. Any CSS length — the panel still shrinks on narrow viewports. */
    width?: string
    /** `alertdialog` for destructive confirmations that must not be dismissed casually. */
    role?: 'dialog' | 'alertdialog'
    /** Controlled open state. Defaults to open-on-mount. */
    open?: boolean
    /** Allow closing by clicking the backdrop. Off automatically for `alertdialog`. */
    closeOnInteractOutside?: boolean
    /** Allow closing with Escape. Off automatically for `alertdialog`. */
    closeOnEscape?: boolean
    /** Show an × button in the top corner. */
    dismissible?: boolean
  }>(),
  {
    title: undefined,
    description: undefined,
    width: '520px',
    role: 'dialog',
    open: true,
    closeOnInteractOutside: undefined,
    closeOnEscape: undefined,
    dismissible: false,
  },
)

const emit = defineEmits<{ (e: 'close'): void }>()

// An alertdialog demands a deliberate choice, so casual dismissal is off unless
// the caller asks for it back.
const isAlert = computed(() => props.role === 'alertdialog')
const dismissOutside = computed(
  () => props.closeOnInteractOutside ?? !isAlert.value,
)
const dismissEscape = computed(() => props.closeOnEscape ?? !isAlert.value)

// Resolved once at setup — useId() is instance-bound and must not run inside a
// computed, or every recompute would hand the machine a different id.
const id = useId()

const service = useMachine(
  dialog.machine,
  // A getter keeps the machine in step with prop changes (Vue adapter takes a MaybeRef).
  computed(() => ({
    id,
    open: props.open,
    role: props.role,
    closeOnInteractOutside: dismissOutside.value,
    closeOnEscape: dismissEscape.value,
    onOpenChange: (details: dialog.OpenChangeDetails) => {
      if (!details.open) emit('close')
    },
  })),
)

const api = computed(() => dialog.connect(service, normalizeProps))
</script>

<template>
  <Teleport to="body">
    <template v-if="api.open">
      <div v-bind="api.getBackdropProps()" class="dialog-backdrop" />
      <div v-bind="api.getPositionerProps()" class="dialog-positioner">
        <div
          v-bind="api.getContentProps()"
          class="dialog-panel gradient-border"
          :style="{ width }"
        >
          <button
            v-if="dismissible"
            v-bind="api.getCloseTriggerProps()"
            class="dialog-close"
            aria-label="Close dialog"
          >
            <svg
              width="14"
              height="14"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              stroke-width="2.4"
              stroke-linecap="round"
            >
              <path d="M18 6 6 18M6 6l12 12" />
            </svg>
          </button>

          <h2
            v-if="title || $slots.title"
            v-bind="api.getTitleProps()"
            class="dialog-title"
          >
            <slot name="title">{{ title }}</slot>
          </h2>
          <p
            v-if="description || $slots.description"
            v-bind="api.getDescriptionProps()"
            class="dialog-description"
          >
            <slot name="description">{{ description }}</slot>
          </p>

          <slot />

          <div v-if="$slots.actions" class="dialog-actions">
            <slot name="actions" />
          </div>
        </div>
      </div>
    </template>
  </Teleport>
</template>

<style scoped>
.dialog-backdrop {
  position: fixed;
  inset: 0;
  z-index: var(--z-index-dialog-backdrop);
  background: color-mix(in oklch, var(--color-surface), transparent 30%);
  backdrop-filter: blur(6px);
  animation: dialog-backdrop-in 0.15s ease both;
}

.dialog-positioner {
  position: fixed;
  inset: 0;
  z-index: var(--z-index-dialog);
  display: grid;
  place-items: center;
  /* Let a tall panel scroll the overlay rather than overflow the viewport. */
  padding: 24px 16px;
  overflow-y: auto;
}

.dialog-panel {
  /* No border: the gradient-border band is the edge. Keeping both drew a
     solid hairline with a second, lighter one just inside it. */
  max-width: 100%;
  padding: 24px 24px 20px;
  border-radius: var(--radius-surface);
  background: var(--color-surface-2);
  box-shadow: var(--shadow-floating);
  animation: dialog-panel-in 0.18s ease both;
}
.dialog-panel:focus-visible {
  outline: none;
}

.dialog-title {
  margin: 0 0 4px;
  font-size: var(--text-title-1);
  line-height: var(--text-title-1--line-height);
  font-weight: var(--text-title-1--font-weight);
  color: var(--color-ink);
}

.dialog-description {
  margin: 0 0 18px;
  font-size: var(--text-body-3);
  line-height: var(--text-body-3--line-height);
  font-weight: var(--text-body-3--font-weight);
  letter-spacing: var(--text-body-3--letter-spacing);
  color: var(--color-ink-2);
}

/* With no description the title still needs separating from the first field. */
.dialog-title:last-of-type {
  margin-bottom: 18px;
}

.dialog-actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  margin-top: 18px;
  padding-top: 16px;
  border-top: 1px solid var(--color-line);
}

.dialog-close {
  float: right;
  display: grid;
  place-items: center;
  width: 28px;
  height: 28px;
  margin: -6px -6px 0 0;
  border: none;
  border-radius: 6px;
  background: transparent;
  color: var(--color-ink-3);
  cursor: pointer;
}
.dialog-close:hover {
  background: var(--color-surface-3);
  color: var(--color-ink);
}

@keyframes dialog-backdrop-in {
  from {
    opacity: 0;
  }
  to {
    opacity: 1;
  }
}

@keyframes dialog-panel-in {
  from {
    opacity: 0;
    transform: translateY(-6px) scale(0.98);
  }
  to {
    opacity: 1;
    transform: none;
  }
}

@media (prefers-reduced-motion: reduce) {
  .dialog-backdrop,
  .dialog-panel {
    animation: none;
  }
}
</style>
