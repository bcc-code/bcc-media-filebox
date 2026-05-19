import { computed, reactive } from 'vue'

export interface AuthState {
  authenticated: boolean
  userId: string
  provider: string
  email: string
  name: string
  role: string
}

const state = reactive<AuthState>({
  authenticated: false,
  userId: '',
  provider: '',
  email: '',
  name: '',
  role: '',
})

// mustChoose is the gate: anyone without a backend session — including
// returning visitors whose session expired — sees the login picker.
const mustChoose = computed(() => !state.authenticated)

let initialized: Promise<void> | null = null

async function fetchMe() {
  try {
    const res = await fetch('/api/me', { credentials: 'same-origin' })
    if (!res.ok) return
    const data = await res.json()
    state.authenticated = !!data.authenticated
    state.userId = data.userId ?? ''
    state.provider = data.provider ?? ''
    state.email = data.email ?? ''
    state.name = data.name ?? ''
    state.role = data.role ?? ''
  } catch {
    // network failure — fall through as unauthenticated
  }
}

export function initAuth(): Promise<void> {
  if (!initialized) initialized = fetchMe()
  return initialized
}

function signIn(providerId: string) {
  window.location.href = `/auth/login/${encodeURIComponent(providerId)}`
}

async function continueAsGuest(name: string, email: string): Promise<string | null> {
  const res = await fetch('/auth/guest', {
    method: 'POST',
    credentials: 'same-origin',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ name, email }),
  })
  if (!res.ok) {
    const text = await res.text()
    return text.trim() || `Request failed (${res.status})`
  }
  // Hard reload so /api/me refetches with the new session cookie and the
  // rest of the app sees the guest as a fully signed-in user.
  window.location.reload()
  return null
}

async function signOut() {
  await fetch('/auth/logout', { method: 'POST', credentials: 'same-origin' })
  window.location.reload()
}

async function changeUser() {
  if (state.authenticated) {
    await fetch('/auth/logout', { method: 'POST', credentials: 'same-origin' })
  }
  window.location.reload()
}

export function useAuth() {
  return { state, mustChoose, signIn, signOut, changeUser, continueAsGuest }
}
