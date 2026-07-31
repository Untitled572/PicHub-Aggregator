import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import AutoImport from 'unplugin-auto-import/vite'
import compression from 'vite-plugin-compression'
import { execSync } from 'child_process'

const version = (() => {
  const explicit = process.env.VITE_APP_VERSION
  if (explicit) return explicit
  try {
    return execSync('git describe --tags --abbrev=0 2>/dev/null || echo "dev"').toString().trim()
  } catch {
    return 'dev'
  }
})()

export default defineConfig({
  plugins: [
    vue(),
    AutoImport({
      imports: ['vue', 'vue-router'],
      dts: 'src/auto-imports.d.ts',
    }),
    compression({
      threshold: 10240,
      algorithm: 'gzip',
      ext: '.gz',
    }),
  ],
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