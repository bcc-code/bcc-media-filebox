import { createApp } from 'vue'
import './style.css'
import App from './App.vue'
import { initAuth } from './composables/useAuth'
import { initProviders } from './composables/useProviders'

// Resolve auth + provider state before the first render so getUserId() and
// the history fetch see the authenticated identity (if any) on first paint.
Promise.allSettled([initAuth(), initProviders()]).finally(() => {
  createApp(App).mount('#app')
})
