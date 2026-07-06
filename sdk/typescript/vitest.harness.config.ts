import { defineConfig } from 'vitest/config';

/**
 * Manual ARP live-fire harnesses (tests/arp/harness/*.ts).
 *
 * These are NOT part of the default `npm test` suite: they are named `*.ts`
 * (not `*.test.ts`) so the main vitest `include` glob skips them, and several
 * require an external target (a running DVAA fleet). Run them explicitly with
 * `npm run test:harness` when validating the engine against a live target.
 *
 * Prerequisites for the live-DVAA harness (live-dvaa.ts):
 *   docker run -p 9000:9000 -p 7001-7021:7001-7021 opena2a/dvaa:0.9.2
 * The self-contained harnesses (dvaa-integration.ts, proxy-e2e.ts,
 * benchmark.ts) need no external services.
 */
export default defineConfig({
  test: {
    globals: true,
    environment: 'node',
    include: ['tests/arp/harness/**/*.ts'],
    // Harnesses drive a real proxy / live target — keep them serial and give
    // the live-DVAA round-trips room over the default 5s timeout.
    fileParallelism: false,
    testTimeout: 20000,
    hookTimeout: 20000,
  },
});
