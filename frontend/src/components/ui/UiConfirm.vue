<script setup lang="ts">
import UiDialog from './UiDialog.vue'
import { pending, settle } from '../../composables/useConfirm'
</script>

<template>
  <!--
    Mounted once, near the router view. Escape is re-enabled even though this is
    an alertdialog: dismissing resolves to "cancel", which is the safe
    direction, and it matches the native confirm() this replaces. Clicking
    outside stays disabled so a destructive prompt needs an explicit answer.
  -->
  <UiDialog
    v-if="pending"
    role="alertdialog"
    :title="pending.title"
    :description="pending.body"
    width="440px"
    close-on-escape
    @close="settle(false)"
  >
    <template #actions>
      <!-- Cancel first, so the focus trap lands on it and Enter is harmless. -->
      <button class="btn btn-ghost" @click="settle(false)">
        {{ pending.cancelLabel ?? 'Cancel' }}
      </button>
      <button
        class="btn"
        :class="pending.danger ? 'btn-danger' : 'btn-primary'"
        @click="settle(true)"
      >
        {{ pending.confirmLabel ?? 'Confirm' }}
      </button>
    </template>
  </UiDialog>
</template>
