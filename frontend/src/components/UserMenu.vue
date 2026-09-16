<script setup lang="ts">
import { computed, ref, onMounted, onBeforeUnmount } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useAuth } from '../composables/useAuth'
import { useProviders } from '../composables/useProviders'

const { state, signIn, signOut, changeUser } = useAuth()
const providers = useProviders()
const router = useRouter()
const route = useRoute()

const isAdmin = computed(() => state.authenticated && state.role === 'admin')
const onAdminPage = computed(() => route.path.startsWith('/admin'))

const open = ref(false)
const menuRef = ref<HTMLDivElement | null>(null)

const displayName = computed(() =>
  state.authenticated ? state.name || state.email || 'Signed in' : 'Guest',
)
const initial = computed(() => displayName.value.charAt(0).toUpperCase())

function toggle() {
  open.value = !open.value
}

function close() {
  open.value = false
}

function onDocClick(e: MouseEvent) {
  if (menuRef.value && !menuRef.value.contains(e.target as Node)) close()
}

onMounted(() => document.addEventListener('click', onDocClick))
onBeforeUnmount(() => document.removeEventListener('click', onDocClick))
</script>

<template>
  <div ref="menuRef" class="user-menu-root">
    <button type="button" class="user-trigger" :class="{ open }" @click.stop="toggle">
      <div class="avatar" :class="{ guest: !state.authenticated }">{{ initial }}</div>
      <span class="name">{{ displayName }}</span>
    </button>

    <div v-if="open" class="user-menu fb-pop" @click.stop>
      <div class="who">
        <template v-if="state.authenticated">
          <div class="n">{{ state.name || '—' }}</div>
          <div class="e">{{ state.email }}</div>
          <div class="o">{{ state.provider }}</div>
        </template>
        <template v-else>
          <div class="n">Guest</div>
          <div class="e">No account · uploads stay on this device</div>
        </template>
      </div>

      <template v-if="!state.authenticated">
        <button
          v-for="p in providers"
          :key="p.id"
          type="button"
          @click="close(); signIn(p.id)"
        >
          Sign in with {{ p.displayName }}
        </button>
        <div v-if="providers.length > 0" class="sep"></div>
      </template>

      <button v-if="isAdmin && !onAdminPage" type="button" @click="close(); router.push('/admin')">
        Admin
      </button>
      <button type="button" @click="close(); changeUser()">Change user</button>
      <button v-if="state.authenticated" type="button" @click="close(); signOut()">Sign out</button>
    </div>
  </div>
</template>

<style scoped>
.user-menu-root { position: relative; }
/* Hide the name on narrow screens; the avatar carries the affordance. */
@media (max-width: 560px) {
  .user-trigger .name { display: none; }
}
</style>
