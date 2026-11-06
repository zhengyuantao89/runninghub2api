import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import path from 'path'
import { fileURLToPath } from 'url'

const __dirname = path.dirname(fileURLToPath(import.meta.url))

export default defineConfig({
  plugins: [vue()],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
    },
  },
  server: {
    port: 5178,
    host: '127.0.0.1',
    proxy: {
      '/v1': 'http://127.0.0.1:8088',
      '/admin': 'http://127.0.0.1:8088',
      '/health': 'http://127.0.0.1:8088',
    },
  },
  build: {
    outDir: 'dist',
    emptyOutDir: true,
  },
})
