<script setup lang="ts">
import type { VerificationMethod } from '../../composables/usePackages'
import UiRadioGroup, { type UiRadioOption } from '../ui/UiRadioGroup.vue'
import UiInput from '../ui/UiInput.vue'

defineProps<{
  modelValue: VerificationMethod
  password: string
}>()

const emit = defineEmits<{
  'update:modelValue': [value: VerificationMethod]
  'update:password': [value: string]
}>()

// email_otp/magic_link are excluded: packageVerified can't satisfy them yet, so
// offering them would let a sender create a package nobody can open.
const options: UiRadioOption<VerificationMethod>[] = [
  {
    value: 'none',
    label: 'No verification',
    description: 'Anyone with the link can download.',
  },
  {
    value: 'bcc_login',
    label: 'BCC login',
    description: 'Recipient must sign in with a BCC account.',
  },
  {
    value: 'password',
    label: 'Password',
    description: 'You set a password and share it separately.',
  },
]
</script>

<template>
  <UiRadioGroup
    :model-value="modelValue"
    :options="options"
    aria-label="Verification"
    @update:model-value="emit('update:modelValue', $event)"
  />
  <div v-if="modelValue === 'password'" class="pw-reveal fb-fade">
    <UiInput
      :model-value="password"
      type="text"
      placeholder="Set a password to share out-of-band"
      @update:model-value="emit('update:password', $event)"
    />
  </div>
</template>
