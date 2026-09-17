<script setup lang="ts">
const props = defineProps<{
  modelValue: string
  /** Monospace face, for codes and paths. */
  mono?: boolean
  /** Marks the field invalid for assistive tech as well as styling. */
  invalid?: boolean
}>()

const emit = defineEmits<{ 'update:modelValue': [value: string] }>()
</script>

<template>
  <input
    :class="['inp', mono && 'mono', invalid && 'inp-invalid']"
    :value="modelValue"
    :aria-invalid="invalid || undefined"
    @input="
      emit('update:modelValue', ($event.target as HTMLInputElement).value)
    "
  />
</template>
