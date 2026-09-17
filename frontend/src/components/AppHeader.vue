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

<style scoped>
/* Colocated from components.css: used only by this component. The @layer
   wrapper is kept so precedence against Tailwind utilities is unchanged.
   Shared primitives stay in assets/components.css. */
@layer components {
  .app-header {
    display: flex;
    align-items: center;
    gap: 22px;
    margin-bottom: 30px;
    position: relative;
  }
  .app-brand {
    display: inline-flex;
    align-items: center;
    gap: 12px;
    color: var(--color-ink);
  }
  .app-nav {
    display: flex;
    /* No background to separate the items, so the gap has to. */
    gap: 20px;
  }
  .app-nav a {
    padding: 3px 1px 7px;
    border-bottom: 2px solid transparent;
    font-size: 14px;
    font-weight: 500;
    /* ink-3 was 4.18:1 here — under the 4.5:1 AA floor — which read as
  disabled rather than clickable. ink-2 is 9.23:1. */
    color: var(--color-ink-2);
    transition:
      color 0.15s,
      border-color 0.15s;
  }
  .app-nav a:hover {
    color: var(--color-ink);
  }

  /* The rest of the header family. Left in components.css these tied with the
     scoped rules above — `.app-nav a.active` is (0,2,1) and so is the scoped
     `.app-nav a`, so which one won came down to bundle order, and the active
     item lost its colour and underline. */
  .app-brand .mark {
    width: 30px;
    height: 30px;
    color: var(--color-ink);
  }
  .app-brand .name {
    font-size: 21px;
    font-weight: 600;
    letter-spacing: -0.3px;
  }
  .app-nav a.active {
    color: var(--color-ink);
    border-bottom-color: var(--color-accent);
  }
}
</style>
