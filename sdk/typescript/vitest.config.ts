import { defineConfig } from 'vitest/config';

export default defineConfig({
  test: {
    globals: true,
    environment: 'node',
    include: ['src/**/*.test.ts', 'tests/**/*.test.ts'],
    // Without a minimum, vitest starts one worker per CPU but one, and a
    // `--maxWorkers=N` below that count stops the run before any test with
    // "options.minThreads and options.maxThreads must not conflict". With 1,
    // `--maxWorkers=N` works on its own for every N of 1 or more.
    minWorkers: 1,
    coverage: {
      provider: 'v8',
      reporter: ['text', 'json', 'html'],
    },
  },
});
