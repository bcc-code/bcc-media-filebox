<script setup lang="ts">
defineProps<{ heading: string; message?: string }>()
</script>

<template>
  <div style="text-align: center">
    <div class="terminal-icon" style="margin: 0 auto 20px">
      <svg
        width="24"
        height="24"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        stroke-width="1.7"
        stroke-linecap="round"
        stroke-linejoin="round"
      >
        <circle cx="12" cy="12" r="9" />
        <path d="M12 8v5M12 16h.01" />
      </svg>
    </div>
    <h2 class="verify-h">{{ heading }}</h2>
    <p v-if="message" class="verify-p">{{ message }}</p>
  </div>
</template>

<style scoped>
/* Colocated from send.css: these classes are used only by this
   component. Shared primitives stay in assets/components.css — several
   components need them, and scoped CSS cannot be shared. */
/* Terminal states (expired / revoked / not found) on the real recipient page */
.terminal-icon {
  width: 52px;
  height: 52px;
  border-radius: 13px;
  background: var(--color-surface-3);
  border: 1px solid var(--color-line-2);
  color: var(--color-ink-3);
  display: grid;
  place-items: center;
  margin-bottom: 20px;
}
</style>
