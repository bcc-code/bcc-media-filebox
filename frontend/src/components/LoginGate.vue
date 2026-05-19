<script setup lang="ts">
import { computed, ref } from 'vue'
import { useAuth } from '../composables/useAuth'
import { useProviders } from '../composables/useProviders'
import bccLogoUrl from '../assets/bcc-logo.svg'
import AppLogo from './AppLogo.vue'

const { signIn, continueAsGuest } = useAuth()
const providers = useProviders()

type Mode = 'choices' | 'guest'
const mode = ref<Mode>('choices')
const guestName = ref('')
const guestEmail = ref('')
const submitting = ref(false)
const error = ref<string | null>(null)

const providerCopy: Record<string, { title: string; subtitle: string; primary?: boolean }> = {
  bcc: {
    title: 'Continue with BCC Login',
    subtitle: 'Recommended for BCC members',
    primary: true,
  },
  azure: {
    title: 'Continue with Microsoft',
    subtitle: 'BCC Media employees',
  },
}

const hasBcc = computed(() => providers.some(p => p.id === 'bcc'))
const orderedProviders = computed(() => {
  const order = ['bcc', 'azure']
  const known = order
    .map(id => providers.find(p => p.id === id))
    .filter((p): p is { id: string; displayName: string } => !!p)
  const extras = providers.filter(p => !order.includes(p.id))
  return [...known, ...extras]
})

const emailLooksValid = computed(() => /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(guestEmail.value.trim()))
const canSubmitGuest = computed(
  () => guestName.value.trim().length > 0 && emailLooksValid.value && !submitting.value,
)

function copyFor(id: string, displayName: string) {
  return providerCopy[id] ?? { title: `Continue with ${displayName}`, subtitle: displayName }
}

function openGuestForm() {
  error.value = null
  mode.value = 'guest'
}

function backToChoices() {
  error.value = null
  mode.value = 'choices'
}

async function submitGuest() {
  if (!canSubmitGuest.value) return
  submitting.value = true
  error.value = null
  const message = await continueAsGuest(guestName.value.trim(), guestEmail.value.trim())
  if (message) {
    submitting.value = false
    error.value = message
  }
  // success path navigates via window.location.reload() inside continueAsGuest
}
</script>

<template>
  <div class="lg-root">
    <div class="lg-bg"></div>
    <div class="lg-card">
      <div class="lg-brand">
        <AppLogo style="width: 22px; height: 22px" />
        <span class="lg-brand-name">FileBox</span>
      </div>

      <div class="lg-header">
        <h1>Sign in to upload</h1>
        <p>Choose a method to continue. Files you upload stay tied to the identity you sign in with.</p>
      </div>

      <div v-if="mode === 'choices'" class="lg-stack">
        <button
          v-for="(p, i) in orderedProviders"
          :key="p.id"
          class="provider-btn"
          :data-tier="copyFor(p.id, p.displayName).primary ? 'primary' : (i === 0 && !hasBcc ? 'primary' : undefined)"
          :data-provider="p.id"
          @click="signIn(p.id)"
        >
          <span class="icon">
            <img v-if="p.id === 'bcc'" :src="bccLogoUrl" alt="" width="22" height="22" />
            <svg v-else-if="p.id === 'azure'" width="18" height="18" viewBox="0 0 24 24">
              <rect x="2" y="2" width="9" height="9" fill="#F25022" />
              <rect x="13" y="2" width="9" height="9" fill="#7FBA00" />
              <rect x="2" y="13" width="9" height="9" fill="#00A4EF" />
              <rect x="13" y="13" width="9" height="9" fill="#FFB900" />
            </svg>
            <svg v-else width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
              <circle cx="12" cy="8" r="3.5" />
              <path d="M4 20c1.6-4 4.5-5.5 8-5.5s6.4 1.5 8 5.5" />
            </svg>
          </span>
          <span class="label">
            <span class="t">{{ copyFor(p.id, p.displayName).title }}</span>
            <span class="s">{{ copyFor(p.id, p.displayName).subtitle }}</span>
          </span>
          <span class="chev">
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <path d="M9 6l6 6-6 6" />
            </svg>
          </span>
        </button>

        <div v-if="orderedProviders.length > 0" class="divider-or">or</div>

        <button class="provider-btn" data-tier="ghost" @click="openGuestForm">
          <span class="icon">
            <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round">
              <circle cx="12" cy="9" r="3.2" />
              <path d="M5 20c1.5-3.6 4-5 7-5s5.5 1.4 7 5" />
            </svg>
          </span>
          <span class="label">
            <span class="t">Continue as guest</span>
            <span class="s">No account · name + email required</span>
          </span>
          <span class="chev">
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <path d="M9 6l6 6-6 6" />
            </svg>
          </span>
        </button>
      </div>

      <form v-else class="lg-form" @submit.prevent="submitGuest">
        <div class="lg-field">
          <label for="lg-guest-name">Your name</label>
          <input
            id="lg-guest-name"
            v-model="guestName"
            type="text"
            placeholder="e.g. Anne Solberg"
            autocomplete="name"
            required
            autofocus
          />
        </div>

        <div class="lg-field">
          <label for="lg-guest-email">Email</label>
          <input
            id="lg-guest-email"
            v-model="guestEmail"
            type="email"
            placeholder="anne@example.com"
            autocomplete="email"
            required
          />
        </div>

        <div v-if="error" class="lg-error">{{ error }}</div>

        <div class="lg-row">
          <button type="button" class="lg-btn ghost" :disabled="submitting" @click="backToChoices">Back</button>
          <button type="submit" class="lg-btn primary" :disabled="!canSubmitGuest">
            {{ submitting ? 'Continuing…' : 'Continue as guest' }}
          </button>
        </div>

        <div class="lg-fine">
          Guests can upload and review their own history. Your name and email are stored with each upload so the team can reach you about the files you send.
        </div>
      </form>
    </div>
  </div>
</template>

<style scoped>
.lg-root {
  position: fixed;
  inset: 0;
  z-index: 50;
  display: grid;
  place-items: center;
  padding: 40px 24px;
  overflow: hidden;
  background: #0a1426;
  color: #e7ecf5;
  font-family: 'Inter', system-ui, -apple-system, 'Segoe UI', sans-serif;
  -webkit-font-smoothing: antialiased;
}

.lg-bg {
  position: absolute;
  inset: 0;
  pointer-events: none;
  background:
    radial-gradient(800px 400px at 50% -10%, color-mix(in oklch, oklch(0.72 0.10 250), transparent 88%), transparent),
    radial-gradient(600px 300px at 50% 110%, color-mix(in oklch, oklch(0.72 0.10 250), transparent 92%), transparent);
}

.lg-card {
  position: relative;
  width: 100%;
  max-width: 420px;
  display: flex;
  flex-direction: column;
  gap: 28px;
}

.lg-brand {
  display: inline-flex;
  align-items: center;
  gap: 10px;
  color: #e7ecf5;
}
.lg-brand-name {
  font-size: 22px;
  font-weight: 600;
  letter-spacing: -0.3px;
}

.lg-header { display: flex; flex-direction: column; gap: 10px; align-items: flex-start; }
.lg-header h1 {
  margin: 0;
  font-size: 28px;
  font-weight: 600;
  letter-spacing: -0.6px;
  line-height: 1.15;
  color: #e7ecf5;
}
.lg-header p {
  margin: 0;
  color: #aeb8cc;
  font-size: 15px;
  line-height: 1.5;
}

.lg-stack { display: flex; flex-direction: column; gap: 10px; }

.provider-btn {
  width: 100%;
  display: flex;
  align-items: center;
  gap: 14px;
  padding: 14px 16px;
  background: #0f1c33;
  border: 1px solid #2a3d61;
  border-radius: 10px;
  color: #e7ecf5;
  font-size: 15px;
  font-weight: 500;
  font-family: inherit;
  cursor: pointer;
  transition: border-color 0.15s ease, background 0.15s ease, transform 0.15s ease;
  text-align: left;
}
.provider-btn:hover { border-color: oklch(0.72 0.10 250); background: #15243f; }
.provider-btn:active { transform: translateY(1px); }

.provider-btn .icon {
  width: 36px;
  height: 36px;
  border-radius: 8px;
  display: grid;
  place-items: center;
  background: #15243f;
  border: 1px solid #2a3d61;
  flex-shrink: 0;
  color: #e7ecf5;
}
.provider-btn .label { flex: 1; }
.provider-btn .label .t { display: block; }
.provider-btn .label .s {
  display: block;
  font-size: 12px;
  color: #6c7896;
  font-weight: 400;
  margin-top: 2px;
}
.provider-btn .chev { color: #6c7896; display: inline-flex; }
.provider-btn:hover .chev { color: oklch(0.72 0.10 250); }

.provider-btn[data-tier='primary'] {
  background: oklch(0.72 0.10 250);
  color: #0a1426;
  border-color: oklch(0.72 0.10 250);
}
.provider-btn[data-tier='primary']:hover { background: oklch(0.78 0.10 250); }
.provider-btn[data-tier='primary'] .label .s { color: color-mix(in oklch, #0a1426, transparent 50%); }
.provider-btn[data-tier='primary'] .chev { color: #0a1426; }
.provider-btn[data-tier='primary'] .icon {
  background: rgba(10, 20, 38, 0.18);
  border-color: rgba(10, 20, 38, 0.3);
  color: #0a1426;
}

.provider-btn[data-tier='ghost'] { background: transparent; border-style: dashed; }
.provider-btn[data-tier='ghost']:hover { background: #0f1c33; border-style: solid; }

/* BCC brand override — pulled from the public BCC component library
   (components.bcc.no): --color-bcc-800 / --color-bcc-700 / --color-bcc-100. */
.provider-btn[data-provider='bcc'] {
  background: #014d49;
  border-color: #014d49;
  color: #ffffff;
}
.provider-btn[data-provider='bcc']:hover { background: #0c625c; border-color: #0c625c; }
.provider-btn[data-provider='bcc'] .icon {
  background: rgba(255, 255, 255, 0.10);
  border-color: rgba(255, 255, 255, 0.18);
  color: #ffffff;
}
.provider-btn[data-provider='bcc'] .label .s { color: color-mix(in oklch, #ffffff, transparent 35%); }
.provider-btn[data-provider='bcc'] .chev { color: #ffffff; }
.provider-btn[data-provider='bcc']:hover .chev { color: #f0fcfa; }

.divider-or {
  display: flex;
  align-items: center;
  gap: 10px;
  color: #6c7896;
  font-size: 12px;
  text-transform: uppercase;
  letter-spacing: 0.14em;
  margin: 8px 0;
}
.divider-or::before,
.divider-or::after {
  content: '';
  flex: 1;
  height: 1px;
  background: #1f2f4d;
}

.lg-form { display: flex; flex-direction: column; gap: 14px; }
.lg-field { display: flex; flex-direction: column; gap: 6px; }
.lg-field label {
  font-size: 12px;
  color: #aeb8cc;
  text-transform: uppercase;
  letter-spacing: 0.08em;
}
.lg-field input {
  width: 100%;
  padding: 12px 14px;
  background: #0f1c33;
  border: 1px solid #2a3d61;
  border-radius: 8px;
  color: #e7ecf5;
  font-size: 14px;
  font-family: inherit;
  outline: none;
  transition: border-color 0.15s ease;
}
.lg-field input:focus { border-color: oklch(0.72 0.10 250); }
.lg-field input::placeholder { color: #6c7896; }

.lg-error {
  color: #f08a8a;
  font-size: 13px;
  background: rgba(240, 138, 138, 0.08);
  border: 1px solid rgba(240, 138, 138, 0.25);
  padding: 8px 10px;
  border-radius: 6px;
}

.lg-row { display: flex; gap: 8px; }
.lg-btn {
  flex: 1;
  padding: 12px;
  font-size: 14px;
  font-weight: 500;
  border-radius: 8px;
  cursor: pointer;
  font-family: inherit;
  border: 1px solid #2a3d61;
  transition: border-color 0.15s ease, background 0.15s ease, color 0.15s ease;
}
.lg-btn.ghost { background: transparent; color: #aeb8cc; }
.lg-btn.ghost:hover:not(:disabled) { color: #e7ecf5; border-color: #6c7896; }
.lg-btn.primary {
  background: oklch(0.72 0.10 250);
  color: #0a1426;
  border-color: oklch(0.72 0.10 250);
}
.lg-btn.primary:hover:not(:disabled) { background: oklch(0.78 0.10 250); }
.lg-btn:disabled { opacity: 0.4; cursor: not-allowed; }

.lg-fine {
  color: #6c7896;
  font-size: 12px;
  line-height: 1.55;
  margin-top: 2px;
}
</style>
