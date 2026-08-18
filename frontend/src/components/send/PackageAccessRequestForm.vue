<script setup lang="ts">
import { computed, ref } from 'vue'
import type { AccessRequestReason } from '../../composables/usePackages'

const props = withDefaults(
  defineProps<{
    // Why the package stopped working. Only words the copy — the server records
    // the reason itself, from the package, not from the requester.
    reason: AccessRequestReason
    senderName?: string
    submitting?: boolean
    error?: string | null
    sent?: boolean
    // false is the sender's "Recipient preview" tab: shown, but must not mail
    // them their own request. The default below is explicit because Vue casts an
    // absent Boolean prop to false, which would make the real page inert.
    interactive?: boolean
  }>(),
  { interactive: true, senderName: '', error: null, sent: false, submitting: false },
)

const emit = defineEmits<{ submit: [email: string, message: string] }>()

const email = ref('')
const message = ref('')

const who = computed(() => props.senderName || 'the sender')
// What the recipient ran into, so the copy names it rather than making them
// work out which limit stopped them.
const blockedBy = computed(() => {
  switch (props.reason) {
    case 'limit_reached':
      return 'These files have hit their download limit.'
    case 'revoked':
      return 'The sender closed this link.'
    default:
      return 'This link has run out of time.'
  }
})
const emailValid = computed(() => /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email.value.trim()))

// Why the button is dead, or null. Shown next to it: a disabled submit with no
// stated reason reads as broken, and neither reason is visible otherwise.
const disabledReason = computed(() => {
  if (!props.interactive) return 'Preview only — nothing is sent from this tab.'
  if (!emailValid.value) return 'Enter your email address so the sender can reach you.'
  return null
})

function submit() {
  if (disabledReason.value || props.submitting) return
  emit('submit', email.value.trim(), message.value.trim())
}
</script>

<template>
  <div v-if="sent" class="req-done">
    <div class="req-done-ic">
      <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M20 6 9 17l-5-5"/></svg>
    </div>
    <div>
      <div class="req-done-h">Request sent to {{ who }}</div>
      <p class="req-done-p">
        You'll get an email as soon as {{ who }} reopens the link. Nothing more to do here.
      </p>
    </div>
  </div>

  <div v-else class="req-box">
    <div class="req-h">Ask {{ who }} to reopen it</div>
    <p class="req-p">
      {{ blockedBy }} Leave an address to reach you at, say what you need, and we'll pass the request on to
      {{ who }}.
    </p>

    <input
      v-model="email"
      class="inp"
      type="email"
      autocomplete="email"
      placeholder="your@email.com"
      :disabled="submitting"
      @keydown.enter="submit"
    />
    <textarea
      v-model="message"
      class="inp req-msg"
      rows="3"
      maxlength="1000"
      placeholder="Optional — what you need, e.g. a few more days or another download"
      :disabled="submitting"
    ></textarea>

    <div v-if="error" class="verify-error">{{ error }}</div>

    <button
      class="btn btn-primary btn-block"
      style="margin-top: 12px"
      :disabled="!!disabledReason || submitting"
      :title="disabledReason ?? ''"
      @click="submit"
    >
      {{ submitting ? 'Sending…' : 'Send request' }}
    </button>
    <p class="req-note" :class="{ blocked: disabledReason }">
      {{ disabledReason ?? `Only ${who} sees this. It doesn't reopen the link on its own — they decide.` }}
    </p>
  </div>
</template>
