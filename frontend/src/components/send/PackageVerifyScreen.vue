<script setup lang="ts">
import { ref } from 'vue'
import type { VerificationMethod } from '../../composables/usePackages'

const props = defineProps<{
  verificationMethod: VerificationMethod
  submitting?: boolean
  error?: string | null
}>()

const emit = defineEmits<{
  submitPassword: [password: string]
  signInBcc: []
}>()

const password = ref('')

function submit() {
  if (!password.value || props.submitting) return
  emit('submitPassword', password.value)
}
</script>

<template>
  <div class="verify-icon">
    <svg v-if="verificationMethod === 'password'" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round"><rect x="5" y="11" width="14" height="10" rx="2"/><path d="M8 11V7a4 4 0 0 1 8 0v4"/></svg>
    <svg v-else-if="verificationMethod === 'bcc_login'" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="8" r="3.5"/><path d="M4 20c1.6-4 4.5-5.5 8-5.5s6.4 1.5 8 5.5"/></svg>
    <svg v-else width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round"><rect x="2" y="4" width="20" height="16" rx="2"/><path d="m22 7-10 5L2 7"/></svg>
  </div>

  <template v-if="verificationMethod === 'password'">
    <h2 class="verify-h">Password required</h2>
    <p class="verify-p">This package is password-protected. Enter the password you were given to continue.</p>
    <input
      v-model="password"
      class="inp pw-verify"
      type="password"
      placeholder="Enter password"
      :disabled="submitting"
      @keydown.enter="submit"
    />
    <div v-if="error" class="verify-error">{{ error }}</div>
    <button class="btn btn-primary btn-block" style="margin-top: 12px" :disabled="!password || submitting" @click="submit">
      {{ submitting ? 'Checking…' : 'Unlock files' }}
    </button>
  </template>

  <template v-else-if="verificationMethod === 'bcc_login'">
    <h2 class="verify-h">Sign in required</h2>
    <p class="verify-p">This package requires you to be signed in with a <b>BCC Login</b> account to access it.</p>
    <div v-if="error" class="verify-error">{{ error }}</div>
    <button class="btn btn-primary btn-block" @click="emit('signInBcc')">Continue with BCC Login</button>
  </template>

  <template v-else>
    <h2 class="verify-h">Verification not available</h2>
    <p class="verify-p">This package uses a verification method that isn't supported yet.</p>
  </template>
</template>
