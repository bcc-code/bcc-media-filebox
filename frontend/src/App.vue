<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { useAuth } from './composables/useAuth'
import LoginGate from './components/LoginGate.vue'

const route = useRoute()
const { mustChoose } = useAuth()
const showLoginGate = computed(() => mustChoose.value && !route.meta.public)
</script>

<template>
  <LoginGate v-if="showLoginGate" />
  <router-view v-else />
</template>
