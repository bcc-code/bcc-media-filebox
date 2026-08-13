<script setup lang="ts">
import type { VerificationMethod } from '../../composables/usePackages'

defineProps<{
  modelValue: VerificationMethod
  password: string
}>()

const emit = defineEmits<{
  'update:modelValue': [value: VerificationMethod]
  'update:password': [value: string]
}>()

// email_otp/magic_link are deliberately excluded — nothing can satisfy them
// yet (see internal/api/package_delivery.go's packageVerified), so offering
// them here would let a sender create a package nobody can ever open.
const options: { id: VerificationMethod; name: string; desc: string }[] = [
  { id: 'none', name: 'No verification', desc: 'Anyone with the link can download.' },
  { id: 'bcc_login', name: 'BCC login', desc: 'Recipient must sign in with a BCC account.' },
  { id: 'password', name: 'Password', desc: 'You set a password and share it separately.' },
]
</script>

<template>
  <div class="verify-opts">
    <button
      v-for="o in options"
      :key="o.id"
      type="button"
      class="verify-card"
      :class="{ selected: modelValue === o.id }"
      @click="emit('update:modelValue', o.id)"
    >
      <span class="radio"></span>
      <div class="vbody">
        <div class="vname">{{ o.name }}</div>
        <div class="vdesc">{{ o.desc }}</div>
      </div>
    </button>
  </div>
  <div v-if="modelValue === 'password'" class="pw-reveal fb-fade">
    <input
      class="inp"
      type="text"
      :value="password"
      placeholder="Set a password to share out-of-band"
      @input="emit('update:password', ($event.target as HTMLInputElement).value)"
    />
  </div>
</template>
