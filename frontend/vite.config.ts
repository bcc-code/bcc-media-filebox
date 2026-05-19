import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import tailwindcss from '@tailwindcss/vite'

export default defineConfig({
  plugins: [vue(), tailwindcss()],
  server: {
    proxy: {
      // xfwd: true sets X-Forwarded-{For,Host,Proto} so the Go backend can
      // build OAuth redirect URLs that point back at :5173 instead of :8080.
      '/files': { target: 'http://localhost:8080', xfwd: true },
      '/api': { target: 'http://localhost:8080', xfwd: true },
      '/auth': { target: 'http://localhost:8080', xfwd: true },
    },
  },
})
