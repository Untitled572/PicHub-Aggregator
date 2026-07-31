import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import { execSync } from 'child_process'

const version = (() => {
  try {
    return execSync('git describe --tags --abbrev=0 2>/dev/null || echo "dev"').toString().trim()
  } catch {
    return 'dev'
  }
})()

export default defineConfig({
  plugins: [vue()],
  define: {
    __APP_VERSION__: JSON.stringify(version),
  },
  server: {
    proxy: {
      '/api': 'http://localhost:5721',
      '/random': 'http://localhost:5721',
    }
  }
})