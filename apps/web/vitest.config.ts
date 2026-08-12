import { defineConfig } from 'vitest/config'
import path from 'path'

export default defineConfig({
  test: {
    environment: 'jsdom',
    // jsdom only exposes localStorage/sessionStorage on a real origin; the default
    // about:blank document has no storage area, so anything touching localStorage
    // fails with "Cannot read properties of undefined". Pin an origin.
    environmentOptions: {
      jsdom: { url: 'http://localhost:3000' },
    },
    // Installs a Storage shim when the environment does not provide one. See
    // vitest.setup.ts — jsdom 27 under this Node version exposes no localStorage.
    setupFiles: ['./vitest.setup.ts'],
    passWithNoTests: true,
    exclude: [
      '**/node_modules/**',
      '**/e2e/**',
      '**/tests/e2e/**',
      '**/*.e2e.*',
      '**/*.spec.ts',
    ],
  },
  resolve: {
    alias: {
      '@': path.resolve(__dirname, '.'),
    },
  },
})
