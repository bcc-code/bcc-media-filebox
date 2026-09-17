<script setup lang="ts">
const props = defineProps<{
  modelValue: string
  /** Marks the field invalid for assistive tech as well as styling. */
  invalid?: boolean
}>()

const emit = defineEmits<{ 'update:modelValue': [value: string] }>()
</script>

<template>
  <textarea
    :class="['inp', invalid && 'inp-invalid']"
    :value="modelValue"
    :aria-invalid="invalid || undefined"
    @input="
      emit('update:modelValue', ($event.target as HTMLTextAreaElement).value)
    "
  ></textarea>
</template>
