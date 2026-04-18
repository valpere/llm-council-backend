import { defineConfig, loadEnv } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig(({ mode }) => {
  // Read PORT from the root .env so the dev proxy targets the correct backend port.
  const rootEnv = loadEnv(mode, new URL('..', import.meta.url).pathname, '')
  const backendPort = rootEnv.PORT || '8001'

  return {
    plugins: [react()],
    server: {
      proxy: {
        '/api': {
          target: `http://localhost:${backendPort}`,
          changeOrigin: true,
        },
      },
    },
  }
})
