import { fileURLToPath, URL } from 'node:url'
import { defineConfig } from 'vitest/config'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      '@shared': fileURLToPath(new URL('../frontend/shared/src', import.meta.url)),
    },
  },
  test: {
    environment: 'jsdom',
    pool: 'vmThreads',
    setupFiles: ['./src/test/setup.ts'],
    include: ['src/**/*.test.{ts,tsx}'],
    coverage: {
      provider: 'v8',
      include: ['src/**/*.{ts,tsx}'],
      exclude: ['**/*.test.{ts,tsx}', 'src/test/**', 'src/main.tsx', '**/*.css'],
      thresholds: { lines: 80, statements: 80, functions: 75, branches: 70 },
    },
  },
})
