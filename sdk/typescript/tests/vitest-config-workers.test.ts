/**
 * `npx vitest run --maxWorkers=N <file>` runs without a matching --minWorkers.
 *
 * vitest 1.x starts one worker per CPU but one unless a minimum is configured,
 * and tinypool refuses a maximum below that minimum with "options.minThreads
 * and options.maxThreads must not conflict" before any test runs. On a machine
 * with more than N + 1 CPUs, `--maxWorkers=N` alone stopped every run. The
 * config pins the minimum at 1, so every maximum of 1 or more is accepted.
 */

import { describe, it, expect } from 'vitest';

import config from '../vitest.config';

describe('vitest worker bounds', () => {
  it('configures a minimum of at most one worker, so any --maxWorkers=N of 1 or more is accepted', () => {
    const minWorkers = config.test?.minWorkers;
    expect(minWorkers, 'vitest.config.ts sets no minWorkers; --maxWorkers=N below the CPU count fails').toBeDefined();
    expect(Number(minWorkers)).toBeGreaterThanOrEqual(1);
    expect(Number(minWorkers)).toBeLessThanOrEqual(1);
  });
});
