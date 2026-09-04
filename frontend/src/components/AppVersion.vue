<script setup lang="ts">
import { useAuth } from '../composables/useAuth'

const { state } = useAuth()
</script>

<template>
  <!-- Fixed above LoginGate's overlay (z-50); pointer-events off so it never
       intercepts clicks on whatever sits in the corner. -->
  <span
    v-if="state.commit"
    class="fixed bottom-2 left-2 z-[60] pointer-events-none select-none font-mono text-[10px] leading-none text-gray-400/70 dark:text-gray-500/70"
    :title="`Build ${state.commit}`"
  >
    {{ state.commit }}
  </span>
</template>
