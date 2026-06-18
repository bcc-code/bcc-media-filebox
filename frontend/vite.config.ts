import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import tailwindcss from '@tailwindcss/vite'

export default defineConfig({
  plugins: [vue(), tailwindcss()],
  server: {
    // Pin the dev port so it always matches BASE_URL (the origin tusd advertises
    // for upload PATCH/HEAD URLs). If the two drift apart, uploads break: the
    // create succeeds but the follow-up PATCH is sent to the BASE_URL origin and
    // 404s. Keep this in sync with BASE_URL in the project .env.
    port: 8091,
    strictPort: true,
    proxy: {
      // xfwd: true sets X-Forwarded-{For,Host,Proto} so the Go backend can
      // build OAuth redirect URLs that point back at :5173 instead of :8080.
      '/files': { target: 'http://localhost:8080', xfwd: true },
      '/api': { target: 'http://localhost:8080', xfwd: true },
      '/auth': { target: 'http://localhost:8080', xfwd: true },
    },
  },
})
