import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import tailwindcss from '@tailwindcss/vite'

// The built interface is embedded in the Go binary (internal/webui/ui).
export default defineConfig({
  plugins: [vue(), tailwindcss()],
  build: {
    outDir: '../internal/webui/ui',
    emptyOutDir: false,
    assetsDir: 'assets',
    sourcemap: false,
  },
  server: {
    proxy: { '/api': 'http://127.0.0.1:9090' },
  },
})
