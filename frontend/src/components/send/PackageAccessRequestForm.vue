<script setup lang="ts">
import { computed, ref } from 'vue'
import type { AccessRequestReason } from '../../composables/usePackages'
import UiTooltip from '../ui/UiTooltip.vue'
import UiButton from '../ui/UiButton.vue'
import UiInput from '../ui/UiInput.vue'
import UiTextarea from '../ui/UiTextarea.vue'

const props = withDefaults(
  defineProps<{
    // Why the package stopped working. Only words the copy — the server records
    // the reason itself, from the package, not from the requester.
    reason: AccessRequestReason
    senderName?: string
    // Whether the server will only take a request from an address the package
    // was mailed to. A link-only package has no list, so anyone holding the link
    // may ask — and being told to use "the address this was sent to" would be
    // wrong there.
    recipientsOnly?: boolean
    submitting?: boolean
    error?: string | null
    sent?: boolean
    // false is the sender's "Recipient preview" tab: shown, but must not mail
    // them their own request. The default below is explicit because Vue casts an
    // absent Boolean prop to false, which would make the real page inert.
    interactive?: boolean
  }>(),
  {
    interactive: true,
    senderName: '',
    recipientsOnly: false,
    error: null,
    sent: false,
    submitting: false,
  },
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
const emailValid = computed(() =>
  /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email.value.trim()),
)

// Why the button is dead, or null. Shown next to it: a disabled submit with no
// stated reason reads as broken, and neither reason is visible otherwise.
const disabledReason = computed(() => {
  if (!props.interactive) return 'Preview only — nothing is sent from this tab.'
  if (!emailValid.value) {
    return props.recipientsOnly
      ? 'Enter the address this package was sent to.'
      : 'Enter your email address so the sender can reach you.'
  }
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
      <svg
        width="20"
        height="20"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        stroke-width="2"
        stroke-linecap="round"
        stroke-linejoin="round"
      >
        <path d="M20 6 9 17l-5-5" />
      </svg>
    </div>
    <div>
      <div class="req-done-h">Request sent to {{ who }}</div>
      <p class="req-done-p">
        You'll get an email as soon as {{ who }} reopens the link. Nothing more
        to do here.
      </p>
    </div>
  </div>

  <div v-else class="req-box">
    <div class="req-h">Ask {{ who }} to reopen it</div>
    <p class="req-p">
      {{ blockedBy }}
      {{
        recipientsOnly
          ? 'Ask from the address it was sent to'
          : 'Leave an address to reach you at'
      }}, say what you need, and we'll pass the request on to {{ who }}.
    </p>

    <UiInput
      v-model="email"
      type="email"
      autocomplete="email"
      placeholder="your@email.com"
      :disabled="submitting"
      @keydown.enter="submit"
    />
    <UiTextarea
      v-model="message"
      class="req-msg"
      rows="3"
      maxlength="1000"
      placeholder="Optional — what you need, e.g. a few more days or another download"
      :disabled="submitting"
    />

    <div v-if="error" class="verify-error">{{ error }}</div>

    <UiTooltip :label="disabledReason ?? ''">
      <UiButton
        variant="primary"
        size="lg"
        block
        style="margin-top: 12px"
        :disabled="!!disabledReason"
        :loading="submitting"
        loading-label="Sending…"
        @click="submit"
      >
        Send request
      </UiButton>
    </UiTooltip>
    <p class="req-note" :class="{ blocked: disabledReason }">
      {{
        disabledReason ??
        `Only ${who} sees this. It doesn't reopen the link on its own — they decide.`
      }}
    </p>
  </div>
</template>

<style scoped>
/* Colocated from send.css: these classes are used only by this
   component. Shared primitives stay in assets/components.css — several
   components need them, and scoped CSS cannot be shared. */
/* Access request form, shown under a terminal state so a stuck recipient has somewhere to go. Left-aligned inside the otherwise centred terminal card. */
.req-box {
  margin-top: 26px;
  padding-top: 24px;
  border-top: 1px solid var(--color-line);
  text-align: left;
}
.req-h {
  font-size: 15px;
  font-weight: 600;
  color: var(--color-ink);
}
.req-p {
  font-size: 13px;
  color: var(--color-ink-2);
  margin: 7px 0 16px;
  line-height: 1.55;
}
.req-msg {
  margin-top: 8px;
  min-height: 62px;
}
.req-note {
  text-align: center;
  font-size: 11.5px;
  color: var(--color-ink-3);
  margin: 12px 0 0;
  line-height: 1.5;
}
.req-done {
  display: flex;
  gap: 13px;
  align-items: flex-start;
  margin-top: 26px;
  padding-top: 24px;
  border-top: 1px solid var(--color-line);
  text-align: left;
}
.req-done-ic {
  width: 32px;
  height: 32px;
  border-radius: 9px;
  flex-shrink: 0;
  display: grid;
  place-items: center;
  background: color-mix(in oklch, var(--color-ok), transparent 82%);
  border: 1px solid color-mix(in oklch, var(--color-ok), transparent 58%);
  color: oklch(0.82 0.11 160);
}
.req-done-h {
  font-size: 14.5px;
  font-weight: 600;
  color: var(--color-ink);
}
.req-done-p {
  font-size: 13px;
  color: var(--color-ink-2);
  margin: 5px 0 0;
  line-height: 1.55;
}
/* Modifier of the scoped .req-note above — same scope, or it ties and loses. */
.req-note.blocked {
  color: var(--color-ink-2);
}
</style>
