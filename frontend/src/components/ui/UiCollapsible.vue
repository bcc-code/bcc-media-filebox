<script setup lang="ts">
import * as collapsible from '@zag-js/collapsible'
import { normalizeProps, useMachine } from '@zag-js/vue'
import { computed, useId } from 'vue'

const props = withDefaults(
  defineProps<{
    /** Controlled open state. Omit to let the component own it. */
    open?: boolean
    disabled?: boolean
  }>(),
  { open: undefined, disabled: false },
)

const emit = defineEmits<{ 'update:open': [open: boolean] }>()

const id = useId()

const service = useMachine(
  collapsible.machine,
  computed(() => ({
    id,
    open: props.open,
    disabled: props.disabled,
    onOpenChange: ({ open }: collapsible.OpenChangeDetails) =>
      emit('update:open', open),
  })),
)

const api = computed(() => collapsible.connect(service, normalizeProps))
</script>

<template>
  <!--
    A single scoped slot rather than fixed trigger/content slots: the trigger
    usually sits among other controls in a header row (names, badges, Edit and
    Delete buttons), which a fixed structure could not accommodate without
    nesting those inside the trigger.
  -->
  <div v-bind="api.getRootProps()">
    <slot
      :trigger="api.getTriggerProps()"
      :content="api.getContentProps()"
      :open="api.open"
      :visible="api.visible"
    />
  </div>
</template>
