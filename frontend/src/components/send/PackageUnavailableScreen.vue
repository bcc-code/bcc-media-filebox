<script setup lang="ts">
defineProps<{ heading: string; message?: string }>()
</script>

<template>
  <div style="text-align: center">
    <div class="terminal-icon" style="margin: 0 auto 20px">
      <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="9"/><path d="M12 8v5M12 16h.01"/></svg>
    </div>
    <h2 class="verify-h">{{ heading }}</h2>
    <p v-if="message" class="verify-p">{{ message }}</p>
  </div>
</template>
