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
    // Explicit per-test and per-hook bound in place of vitest's 5000 ms and
    // 10000 ms defaults. Measured on 2026-11-13 on a 16-core macOS host with
    // `npx vitest run --maxWorkers=4 --reporter=default --reporter=json`, five
    // runs, 4140 test durations: per-test p99 221.9 ms (nearest rank), slowest
    // single test 939.9 ms. The bound is that p99 times 90, rounded up to a
    // whole second, to leave room for tests that run slower on a busy host. A
    // test's duration includes its beforeEach and afterEach hooks, so hooks get
    // the same bound; beforeAll and afterAll are not in those durations, and
    // the bound stays above their 10000 ms default. A test that needs longer
    // passes its own timeout to it().
    testTimeout: 20_000,
    hookTimeout: 20_000,
    // passWithNoTests stays unset: a run that collects no test file fails.
    // vitest-config.test.ts holds both settings.
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
