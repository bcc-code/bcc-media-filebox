<script lang="ts">
import * as toast from '@zag-js/toast'
import { normalizeProps, useMachine } from '@zag-js/vue'
import { computed, defineComponent, h, type PropType } from 'vue'

// Each toast owns its own machine — that is what drives its dismiss timer,
// pause-on-hover and swipe — so the group renders one instance per entry.
const ToastItem = defineComponent({
  name: 'UiToastItem',
  props: {
    toast: { type: Object as PropType<toast.Options>, required: true },
    index: { type: Number, required: true },
    parent: { type: Object as PropType<toast.GroupService>, required: true },
  },
  setup(props) {
    const service = useMachine(
      toast.machine,
      computed(() => ({
        ...props.toast,
        index: props.index,
        parent: props.parent,
      })),
    )
    const api = computed(() => toast.connect(service, normalizeProps))

    return () =>
      h('div', { ...api.value.getRootProps(), class: 'toast' }, [
        h('span', { class: 'toast-dot' }),
        h(
          'div',
          { ...api.value.getTitleProps(), class: 'toast-title' },
          api.value.title,
        ),
        api.value.description
          ? h(
              'div',
              {
                ...api.value.getDescriptionProps(),
                class: 'toast-description',
              },
              api.value.description,
            )
          : null,
        h(
          'button',
          {
            ...api.value.getCloseTriggerProps(),
            class: 'toast-close',
            'aria-label': 'Dismiss notification',
          },
          '×',
        ),
      ])
  },
})
</script>

<script setup lang="ts">
import { toastStore } from '../../composables/useToast'

const service = useMachine(toast.group.machine, { store: toastStore })
const api = computed(() => toast.group.connect(service, normalizeProps))
</script>

<template>
  <Teleport to="body">
    <div v-bind="api.getGroupProps()">
      <ToastItem
        v-for="(t, i) in api.getToasts()"
        :key="t.id"
        :toast="t"
        :index="i"
        :parent="service"
      />
    </div>
  </Teleport>
</template>

<style scoped>
/* Colocated from components.css: used only by this component. The @layer
   wrapper is kept so precedence against Tailwind utilities is unchanged.
   Shared primitives stay in assets/components.css. */
@layer components {
  .toast {
    display: grid;
    grid-template-columns: auto 1fr auto;
    align-items: center;
    gap: 10px;
    width: var(--width, auto);
    min-width: 220px;
    padding: 11px 14px 11px 18px;
    border-radius: 10px;
    font-size: 13.5px;
    color: var(--color-ink);
    background: var(--color-surface-4);
    border: 1px solid var(--color-line-2);
    box-shadow: 0 14px 40px rgb(0 0 0 / 0.5);
    /* Zag animates opacity/translate off these data attributes. */
    translate: var(--x) var(--y);
    scale: var(--scale);
    z-index: var(--z-index);
    height: var(--height);
    opacity: var(--opacity);
    will-change: translate, opacity, scale;
    transition:
      translate 0.3s ease,
      scale 0.3s ease,
      opacity 0.2s ease;
  }
  .toast[data-state='closed'] {
    opacity: 0;
  }
  .toast-dot {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    background: var(--color-ok);
    flex-shrink: 0;
  }
  .toast[data-type='error'] .toast-dot {
    background: var(--color-danger);
  }
  .toast[data-type='warning'] .toast-dot {
    background: var(--color-warn);
  }
  .toast[data-type='info'] .toast-dot {
    background: var(--color-accent);
  }
  .toast-title {
    font-weight: 500;
  }
  .toast-description {
    grid-column: 2;
    font-size: 12.5px;
    color: var(--color-ink-2);
  }
  .toast-close {
    display: grid;
    place-items: center;
    width: 22px;
    height: 22px;
    border: none;
    border-radius: 6px;
    background: transparent;
    color: var(--color-ink-3);
    font-size: 16px;
    line-height: 1;
    cursor: pointer;
  }
  .toast-close:hover {
    background: var(--color-surface-3);
    color: var(--color-ink);
  }
}
</style>
