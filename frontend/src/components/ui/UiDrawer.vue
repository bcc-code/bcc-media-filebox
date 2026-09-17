<script setup lang="ts">
import * as dialog from '@zag-js/dialog'
import { normalizeProps, useMachine } from '@zag-js/vue'
import { computed, useId } from 'vue'
import UiButton from './UiButton.vue'

const props = withDefaults(
  defineProps<{
    /** Edge the panel is anchored to and slides in from. */
    side?: 'right' | 'left'
    /** Panel width. Any CSS length — it still shrinks on a narrow viewport. */
    width?: string
    /** Controlled open state. Defaults to open-on-mount. */
    open?: boolean
    /** Accessible name, for when the caller has no heading to bind `titleProps` to. */
    label?: string
  }>(),
  {
    side: 'right',
    width: 'min(960px, 92vw)',
    open: true,
    label: undefined,
  },
)

const emit = defineEmits<{ (e: 'close'): void }>()

// Resolved once at setup — useId() is instance-bound and must not run inside a
// computed, or every recompute would hand the machine a different id.
const id = useId()

const service = useMachine(
  dialog.machine,
  // A getter keeps the machine in step with prop changes (Vue adapter takes a MaybeRef).
  computed(() => ({
    id,
    open: props.open,
    onOpenChange: (details: dialog.OpenChangeDetails) => {
      if (!details.open) emit('close')
    },
  })),
)

const api = computed(() => dialog.connect(service, normalizeProps))

// Zag types the close trigger's `disabled` as Booleanish, which does not line
// up with UiButton's boolean prop. The trigger is never disabled here, so the
// attributes pass through as a plain record.
const closeTriggerProps = computed(
  () => api.value.getCloseTriggerProps() as Record<string, unknown>,
)
</script>

<template>
  <!-- Always <body>: a transformed ancestor would otherwise become the
       containing block for the fixed-position backdrop and panel. -->
  <Teleport to="body">
    <template v-if="api.open">
      <div v-bind="api.getBackdropProps()" class="drawer-backdrop" />
      <div
        v-bind="api.getPositionerProps()"
        class="drawer-positioner"
        :data-side="side"
      >
        <div
          v-bind="api.getContentProps()"
          class="drawer-panel"
          :data-side="side"
          :style="{ width }"
          :aria-label="label"
        >
          <div class="drawer-head">
            <UiButton
              v-bind="closeTriggerProps"
              icon
              variant="ghost"
              aria-label="Close"
            >
              <svg
                width="16"
                height="16"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                stroke-width="2"
                stroke-linecap="round"
              >
                <path d="M18 6L6 18M6 6l12 12" />
              </svg>
            </UiButton>
            <slot name="head" />
          </div>

          <div class="drawer-body">
            <!-- The caller binds `titleProps` to its own heading, which is what
                 names the dialog for a screen reader. -->
            <slot :title-props="api.getTitleProps()" />
          </div>
        </div>
      </div>
    </template>
  </Teleport>
</template>

<style scoped>
.drawer-backdrop {
  position: fixed;
  inset: 0;
  z-index: var(--z-index-drawer-backdrop);
  background: color-mix(in oklch, var(--color-surface), transparent 40%);
  backdrop-filter: blur(4px);
  animation: drawer-backdrop-in 0.15s ease both;
}

.drawer-positioner {
  position: fixed;
  inset: 0;
  z-index: var(--z-index-drawer);
  display: flex;
}
.drawer-positioner[data-side='right'] {
  justify-content: flex-end;
}
.drawer-positioner[data-side='left'] {
  justify-content: flex-start;
}

.drawer-panel {
  max-width: 100%;
  height: 100%;
  background: var(--color-surface);
  box-shadow: -30px 0 80px rgb(0 0 0 / 0.5);
  /* The panel is the scroll container, so the head below can stick to it. */
  overflow-y: auto;
}
.drawer-panel[data-side='right'] {
  border-left: 1px solid var(--color-line);
  animation: drawer-in-right 0.22s ease both;
}
.drawer-panel[data-side='left'] {
  border-right: 1px solid var(--color-line);
  box-shadow: 30px 0 80px rgb(0 0 0 / 0.5);
  animation: drawer-in-left 0.22s ease both;
}
.drawer-panel:focus-visible {
  outline: none;
}

.drawer-head {
  position: sticky;
  top: 0;
  z-index: 2;
  display: flex;
  align-items: center;
  gap: 4px;
  padding: 12px 18px;
  background: color-mix(in oklch, var(--color-surface), transparent 10%);
  backdrop-filter: blur(10px);
  border-bottom: 1px solid var(--color-line);
}

.drawer-body {
  padding: 28px 32px 60px;
}

@keyframes drawer-backdrop-in {
  from {
    opacity: 0;
  }
  to {
    opacity: 1;
  }
}

/* `to: none` rather than `translateX(0)` on purpose: a lingering transform would
   make the panel a containing block for any `position: fixed` descendant. */
@keyframes drawer-in-right {
  from {
    transform: translateX(100%);
  }
  to {
    transform: none;
  }
}
@keyframes drawer-in-left {
  from {
    transform: translateX(-100%);
  }
  to {
    transform: none;
  }
}

@media (prefers-reduced-motion: reduce) {
  .drawer-backdrop,
  .drawer-panel {
    animation: none;
  }
}
</style>
