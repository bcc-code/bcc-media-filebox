import { reactive } from 'vue'

export interface Provider {
  id: string
  displayName: string
}

const providers = reactive<Provider[]>([])
let initialized: Promise<void> | null = null

async function fetchProviders() {
  try {
    const res = await fetch('/auth/providers')
    if (!res.ok) return
    const data = (await res.json()) as Provider[]
    providers.splice(0, providers.length, ...data)
  } catch {
    // network failure — leave providers empty (no sign-in UI)
  }
}

export function initProviders(): Promise<void> {
  if (!initialized) initialized = fetchProviders()
  return initialized
}

export function useProviders() {
  return providers
}
