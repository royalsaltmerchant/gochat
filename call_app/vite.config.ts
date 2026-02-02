import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  base: '/call/',
  build: {
    outDir: '../relay_server/static/call',
    emptyDirBeforeWrite: true,
  },
  server: {
    port: 5173,
  },
})
