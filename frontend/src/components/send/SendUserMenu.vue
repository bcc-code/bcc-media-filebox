<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useAuth } from '../../composables/useAuth'

const { state, signOut, changeUser } = useAuth()
const router = useRouter()

const isAdmin = computed(() => state.authenticated && state.role === 'admin')
const displayName = computed(() => state.name || state.email || 'Signed in')
const initial = computed(() => displayName.value.charAt(0).toUpperCase())

const open = ref(false)
const menuRef = ref<HTMLDivElement | null>(null)

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
  <div ref="menuRef" style="position: relative">
    <button type="button" class="me-trigger" :class="{ open }" @click.stop="toggle">
      <div class="avatar">{{ initial }}</div>
      <span>{{ displayName }}</span>
    </button>
    <div v-if="open" class="me-menu fb-pop" @click.stop>
      <div class="who">
        <div class="n">{{ displayName }}</div>
        <div class="e">{{ state.email }}</div>
        <div class="o">{{ state.provider }}</div>
      </div>
      <button v-if="isAdmin" type="button" @click="close(); router.push('/admin')">Admin</button>
      <button type="button" @click="close(); changeUser()">Change user</button>
      <button type="button" @click="close(); signOut()">Sign out</button>
    </div>
  </div>
</template>
