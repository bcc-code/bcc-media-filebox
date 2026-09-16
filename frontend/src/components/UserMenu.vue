<script setup lang="ts">
import { computed } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useAuth } from '../composables/useAuth'
import { useProviders } from '../composables/useProviders'
import UiMenu, { type UiMenuEntry } from './ui/UiMenu.vue'

const { state, signIn, signOut, changeUser } = useAuth()
const providers = useProviders()
const router = useRouter()
const route = useRoute()

const isAdmin = computed(() => state.authenticated && state.role === 'admin')
const onAdminPage = computed(() => route.path.startsWith('/admin'))

const displayName = computed(() =>
  state.authenticated ? state.name || state.email || 'Signed in' : 'Guest',
)
const initial = computed(() => displayName.value.charAt(0).toUpperCase())

const items = computed<UiMenuEntry[]>(() => {
  const entries: UiMenuEntry[] = []

  // Guests get the sign-in options first; there is nothing else to offer them.
  if (!state.authenticated) {
    for (const p of providers) {
      entries.push({
        label: `Sign in with ${p.displayName}`,
        onSelect: () => signIn(p.id),
      })
    }
    if (providers.length > 0) entries.push({ type: 'separator' })
  }

  if (isAdmin.value && !onAdminPage.value) {
    entries.push({ label: 'Admin', onSelect: () => router.push('/admin') })
  }
  entries.push({ label: 'Change user', onSelect: () => changeUser() })
  if (state.authenticated) {
    entries.push({ label: 'Sign out', onSelect: () => signOut() })
  }

  return entries
})
</script>

<template>
  <UiMenu :items="items" width="240px" placement="bottom-end">
    <template #trigger="{ open }">
      <span class="user-trigger" :class="{ open }">
        <span class="avatar" :class="{ guest: !state.authenticated }">{{
          initial
        }}</span>
        <span class="name">{{ displayName }}</span>
      </span>
    </template>

    <template #header>
      <template v-if="state.authenticated">
        <div class="n">{{ state.name || '—' }}</div>
        <div class="e">{{ state.email }}</div>
        <div class="o">{{ state.provider }}</div>
      </template>
      <template v-else>
        <div class="n">Guest</div>
        <div class="e">No account · uploads stay on this device</div>
      </template>
    </template>
  </UiMenu>
</template>

<style scoped>
/* Hide the name on narrow screens; the avatar carries the affordance. */
@media (max-width: 560px) {
  .user-trigger .name {
    display: none;
  }
}
</style>
