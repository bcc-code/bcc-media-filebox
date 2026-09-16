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
