<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import AppLogo from './AppLogo.vue'
import UserMenu from './UserMenu.vue'
import { useAuth } from '../composables/useAuth'

const route = useRoute()
const { state: authState } = useAuth()

// Send is unavailable to guests, so the link is hidden rather than dead.
const canSend = computed(() => authState.provider !== 'guest')
</script>

<template>
  <div class="app-header">
    <router-link to="/" class="app-brand">
      <AppLogo class="mark" />
      <span class="name">FileBox</span>
    </router-link>
    <nav class="app-nav">
      <!-- The active link is derived from the route rather than passed in by
           each page. Home and Send each hand-wrote their own `class="active"`,
           which is a pair of copies that can disagree. -->
      <router-link to="/" :class="{ active: route.path === '/' }">
        Upload
      </router-link>
      <router-link
        v-if="canSend"
        to="/send"
        :class="{ active: route.path.startsWith('/send') }"
      >
        Send
      </router-link>
    </nav>
    <span class="spacer"></span>
    <UserMenu />
  </div>
</template>
