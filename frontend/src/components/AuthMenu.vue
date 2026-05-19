<script setup lang="ts">
import { computed, ref, onMounted, onBeforeUnmount } from 'vue'
import { useAuth } from '../composables/useAuth'
import { useProviders } from '../composables/useProviders'

const { state, signIn, signOut, changeUser } = useAuth()
const providers = useProviders()

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

function onSignIn(providerId: string) {
  close()
  signIn(providerId)
}

function onChangeUser() {
  close()
  changeUser()
}

function onSignOut() {
  close()
  signOut()
}

function onDocClick(e: MouseEvent) {
  if (!menuRef.value) return
  if (!menuRef.value.contains(e.target as Node)) close()
}

onMounted(() => document.addEventListener('click', onDocClick))
onBeforeUnmount(() => document.removeEventListener('click', onDocClick))
</script>

<template>
  <div ref="menuRef" class="relative">
    <button
      type="button"
      @click="toggle"
      class="flex items-center gap-2 px-3 py-1.5 rounded-lg text-sm text-gray-700 dark:text-gray-200 hover:bg-gray-100 dark:hover:bg-gray-800 cursor-pointer"
    >
      <span
        class="w-7 h-7 rounded-full flex items-center justify-center font-semibold text-xs"
        :class="state.authenticated ? 'bg-blue-500 text-white' : 'bg-gray-300 dark:bg-gray-700 text-gray-700 dark:text-gray-200'"
      >
        {{ initial }}
      </span>
      <span class="hidden sm:inline">{{ displayName }}</span>
    </button>

    <div
      v-if="open"
      class="absolute right-0 mt-2 w-60 rounded-lg border border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-800 shadow-lg z-10 py-1"
    >
      <div class="px-3 py-2 text-xs text-gray-500 dark:text-gray-400 border-b border-gray-100 dark:border-gray-700">
        <template v-if="state.authenticated">
          <div class="font-medium text-gray-900 dark:text-gray-100 truncate">{{ state.name || '—' }}</div>
          <div class="truncate">{{ state.email }}</div>
          <div class="mt-1 uppercase tracking-wide text-[10px]">{{ state.provider }}</div>
        </template>
        <template v-else>
          <div class="font-medium text-gray-900 dark:text-gray-100">Guest</div>
          <div class="truncate">No account · uploads stay on this device</div>
        </template>
      </div>

      <template v-if="!state.authenticated">
        <button
          v-for="p in providers"
          :key="p.id"
          type="button"
          @click="onSignIn(p.id)"
          class="w-full text-left px-3 py-2 text-sm text-gray-700 dark:text-gray-200 hover:bg-gray-100 dark:hover:bg-gray-700 cursor-pointer"
        >
          Sign in with {{ p.displayName }}
        </button>
        <div v-if="providers.length > 0" class="my-1 border-t border-gray-100 dark:border-gray-700"></div>
      </template>

      <button
        type="button"
        @click="onChangeUser"
        class="w-full text-left px-3 py-2 text-sm text-gray-700 dark:text-gray-200 hover:bg-gray-100 dark:hover:bg-gray-700 cursor-pointer"
      >
        Change user
      </button>

      <button
        v-if="state.authenticated"
        type="button"
        @click="onSignOut"
        class="w-full text-left px-3 py-2 text-sm text-gray-700 dark:text-gray-200 hover:bg-gray-100 dark:hover:bg-gray-700 cursor-pointer"
      >
        Sign out
      </button>
    </div>
  </div>
</template>
