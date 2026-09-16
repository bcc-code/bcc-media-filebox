<script setup lang="ts">
import * as editable from '@zag-js/editable'
import { normalizeProps, useMachine } from '@zag-js/vue'
import { computed, ref, useId, watch } from 'vue'

const props = withDefaults(
  defineProps<{
    value: string
    /** Classes for the read-only preview, so callers keep their own typography. */
    previewClass?: string
    /** Classes for the input shown while editing. */
    inputClass?: string
    placeholder?: string
    disabled?: boolean
    /** Accessible name for the edit field. */
    ariaLabel?: string
  }>(),
  {
    previewClass: undefined,
    inputClass: undefined,
    placeholder: undefined,
    disabled: false,
    ariaLabel: undefined,
  },
)

const emit = defineEmits<{
  /** Fires only on commit — Escape cancels without emitting. */
  commit: [value: string]
  /** Whether the field is currently open for editing. */
  editChange: [editing: boolean]
}>()

const id = useId()

// The machine's value is controlled, so typing must be mirrored back or the
// edit buffer stays at the prop and Enter commits an unchanged value.
const draft = ref(props.value)
watch(
  () => props.value,
  (next) => {
    draft.value = next
  },
)

const service = useMachine(
  editable.machine,
  computed(() => ({
    id,
    value: draft.value,
    disabled: props.disabled,
    placeholder: props.placeholder,
    // Click to edit, Enter or blur to commit — matching the behaviour this
    // replaces. `focus` activation would open the editor on tab-through.
    activationMode: 'click' as const,
    submitMode: 'both' as const,
    onValueChange: ({ value }: editable.ValueChangeDetails) => {
      draft.value = value
    },
    onEditChange: ({ edit }: editable.EditChangeDetails) => {
      // Escape reverts the buffer; the machine has already restored its value.
      if (!edit) draft.value = props.value
      emit('editChange', edit)
    },
    onValueCommit: ({ value }: editable.ValueChangeDetails) => {
      const next = value.trim()
      // An empty field means "no change", as the hand-rolled version had it.
      if (next && next !== props.value) emit('commit', next)
    },
  })),
)

const api = computed(() => editable.connect(service, normalizeProps))
</script>

<template>
  <div v-bind="api.getRootProps()">
    <div v-bind="api.getAreaProps()">
      <input
        v-bind="api.getInputProps()"
        class="inline-edit"
        :class="inputClass"
        :aria-label="ariaLabel"
      />
      <span v-bind="api.getPreviewProps()" :class="previewClass">
        {{ value || placeholder }}
      </span>
    </div>
  </div>
</template>
