import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import * as fs from 'fs';
import * as os from 'os';
import * as path from 'path';
import type { ARPConfig, ARPEvent, BudgetState, IntelligenceConfig } from '../../../src/arp/types';

/**
 * The L2 budget is taken before the adapter is awaited, not checked before the call
 * and recorded after it.
 *
 * `analyze()` runs synchronously up to the adapter call, so events analyzed together
 * all reach the budget step before the first call returns. A check recorded only after
 * the call returned did not count the calls in flight: before this change, 50 critical
 * events analyzed together made 50 adapter calls with `maxCallsPerHour: 20`, whether
 * `budgetUsd` held 10 estimates or 20,000. These tests drive the coordinator through a
 * stub adapter and read the result from the adapter's call count and the persisted
 * budget file.
 */

const stub = vi.hoisted(() => {
  const state = {
    calls: 0,
    /** The prompt of each call, in the order the calls were made. */
    prompts: [] as string[],
    fail: false,
    /** When true, pricing an answer (150 tokens in) throws; pricing an estimate does not. */
    pricingFails: false,
    /** When true, estimating a call before it is made (200 tokens in) throws. */
    estimateFails: false,
    /** When set, every call waits on it instead of on the next turn of the event loop. */
    gate: null as Promise<void> | null,
  };
  const adapter = {
    name: 'stub',
    async assess(prompt: string): Promise<{ content: string; inputTokens: number; outputTokens: number; model: string }> {
      state.calls += 1;
      state.prompts.push(prompt);
      await (state.gate ?? new Promise<void>((resolve) => setImmediate(resolve)));
      if (state.fail) throw new Error('stub adapter: the call failed');
      return {
        content: 'CONSISTENT: YES\nCONFIDENCE: 0.9\nREASONING: stub\nACTION: ALLOW',
        inputTokens: 150,
        outputTokens: 50,
        model: 'stub',
      };
    },
    // One cent per thousand tokens: the coordinator's estimate for a single event
    // (200 tokens in, maxTokensPerCall 300 out) is 0.005, a batch estimate (200 in,
    // 600 out) is 0.008, and an answered call (150 in, 50 out) costs 0.002.
    estimateCost(inputTokens: number, outputTokens: number): number {
      if (state.estimateFails && inputTokens === 200) throw new Error('stub adapter: cannot estimate the call');
      if (state.pricingFails && inputTokens === 150) throw new Error('stub adapter: cannot price the answer');
      return (inputTokens + outputTokens) / 100000;
    },
    async healthCheck(): Promise<boolean> {
      return true;
    },
  };
  return { state, adapter };
});

vi.mock('../../../src/arp/intelligence/adapters', () => ({
  createAdapter: () => stub.adapter,
}));

import { IntelligenceCoordinator } from '../../../src/arp/intelligence/coordinator';

const ESTIMATE_USD = 0.005;
const BATCH_ESTIMATE_USD = 0.008;
const ANSWERED_COST_USD = 0.002;

function config(intelligence: IntelligenceConfig): ARPConfig {
  return {
    agentName: 'test-agent',
    intelligence: { enabled: true, adapter: 'anthropic', ...intelligence },
  };
}

function event(i: number, severity: ARPEvent['severity'] = 'critical'): ARPEvent {
  return {
    id: `evt-${i}`,
    timestamp: new Date().toISOString(),
    source: 'network',
    category: 'threat',
    severity,
    description: 'Outbound connection to an unexpected host',
    data: { remoteAddr: '203.0.113.9:443' },
    classifiedBy: 'L0-rules',
  };
}

function criticalEvents(count: number): ARPEvent[] {
  return Array.from({ length: count }, (_, i) => event(i));
}

function readBudgetFile(dir: string): BudgetState {
  return JSON.parse(fs.readFileSync(path.join(dir, 'budget.json'), 'utf-8')) as BudgetState;
}

/**
 * The number of events waiting for the next batch call. The queue is private; these
 * tests read it because what they check is the memory it holds.
 */
function queuedForBatch(coord: IntelligenceCoordinator): number {
  const queue = (coord as unknown as { batchQueue: unknown }).batchQueue;
  if (!Array.isArray(queue)) throw new Error('IntelligenceCoordinator no longer keeps its batch queue in batchQueue');
  return queue.length;
}

describe('IntelligenceCoordinator L2 budget reservation', () => {
  let dataDir: string;

  beforeEach(() => {
    dataDir = fs.mkdtempSync(path.join(os.tmpdir(), 'coord-budget-'));
    stub.state.calls = 0;
    stub.state.prompts = [];
    stub.state.fail = false;
    stub.state.pricingFails = false;
    stub.state.estimateFails = false;
    stub.state.gate = null;
  });

  afterEach(() => {
    vi.useRealTimers();
    fs.rmSync(dataDir, { recursive: true, force: true });
  });

  it('50 concurrent critical events make at most maxCallsPerHour calls and spend at most budgetUsd', async () => {
    // The budget holds 10 estimates (10 x 0.005 = 0.05 <= 0.0525, 11 would be 0.055),
    // fewer than the 20 calls the hourly limit allows, so the budget is the limit that
    // binds here and the spend assertion below is not met by the call cap alone.
    const budgetUsd = 0.0525;
    const maxCallsPerHour = 20;
    const coord = new IntelligenceCoordinator(config({ budgetUsd, maxCallsPerHour }), dataDir);

    await Promise.all(criticalEvents(50).map((e) => coord.analyze(e)));

    expect(stub.state.calls).toBeLessThanOrEqual(maxCallsPerHour);
    expect(stub.state.calls).toBe(10);
    const state = readBudgetFile(dataDir);
    expect(state.totalSpentUsd).toBeLessThanOrEqual(budgetUsd);
    expect(state.totalSpentUsd).toBeCloseTo(10 * ANSWERED_COST_USD, 10);
    expect(state.totalCalls).toBe(10);
  });

  it('the hourly limit holds under 50 concurrent critical events when the budget has room', async () => {
    const coord = new IntelligenceCoordinator(config({ budgetUsd: 100, maxCallsPerHour: 20 }), dataDir);

    await Promise.all(criticalEvents(50).map((e) => coord.analyze(e)));

    expect(stub.state.calls).toBe(20);
    expect(coord.getBudgetStatus().callsThisHour).toBe(20);
    expect(readBudgetFile(dataDir).callsThisHour).toBe(20);
  });

  it('a failed call gives back its slot and its estimate', async () => {
    const coord = new IntelligenceCoordinator(config({ budgetUsd: 100, maxCallsPerHour: 20 }), dataDir);
    stub.state.fail = true;

    const failed = await Promise.all(criticalEvents(50).map((e) => coord.analyze(e)));

    // The 20 reserved calls were made and failed; the other 30 never got a slot.
    expect(stub.state.calls).toBe(20);
    expect(failed.every((assessment) => assessment === null)).toBe(true);
    const released = readBudgetFile(dataDir);
    expect(released.callsThisHour).toBe(0);
    expect(released.totalCalls).toBe(0);
    expect(released.totalSpentUsd).toBe(0);

    stub.state.fail = false;
    const answered = await coord.analyze(event(99));

    expect(answered).not.toBeNull();
    expect(stub.state.calls).toBe(21);
    const settled = readBudgetFile(dataDir);
    expect(settled.callsThisHour).toBe(1);
    expect(settled.totalCalls).toBe(1);
    expect(settled.totalSpentUsd).toBe(ANSWERED_COST_USD);
  });

  it('the hold is on disk while the call is in flight, and the actual cost replaces it', async () => {
    let open!: () => void;
    stub.state.gate = new Promise<void>((resolve) => {
      open = resolve;
    });
    const coord = new IntelligenceCoordinator(config({ budgetUsd: 100, maxCallsPerHour: 20 }), dataDir);

    const pending = coord.analyze(event(1));

    expect(stub.state.calls).toBe(1);
    const held = readBudgetFile(dataDir);
    expect(held.callsThisHour).toBe(1);
    expect(held.totalSpentUsd).toBe(ESTIMATE_USD);

    open();
    await pending;

    const settled = readBudgetFile(dataDir);
    expect(settled.callsThisHour).toBe(1);
    expect(settled.totalCalls).toBe(1);
    expect(settled.totalSpentUsd).toBe(ANSWERED_COST_USD);
  });

  it('an answered call whose cost cannot be worked out is recorded at its estimate', async () => {
    stub.state.pricingFails = true;
    const coord = new IntelligenceCoordinator(config({ budgetUsd: 100, maxCallsPerHour: 20 }), dataDir);

    expect(await coord.analyze(event(1))).toBeNull();

    expect(stub.state.calls).toBe(1);
    const state = readBudgetFile(dataDir);
    // totalCalls and recentCosts change only when the reservation is recorded; a
    // reservation left open would show the same spend with neither of them.
    expect(state.totalCalls).toBe(1);
    expect(state.recentCosts.map((entry) => entry.cost)).toEqual([ESTIMATE_USD]);
    expect(state.totalSpentUsd).toBe(ESTIMATE_USD);
    expect(state.callsThisHour).toBe(1);
  });

  it('a batch flush takes its own reservation and makes no call once the hourly limit is reached', async () => {
    const coord = new IntelligenceCoordinator(
      config({ budgetUsd: 100, maxCallsPerHour: 2, enableBatching: true, batchWindowMs: 3600000 }),
      dataDir,
    );

    // A non-critical event is queued for the batch while the hour still has room.
    expect(await coord.analyze(event(1, 'high'))).toBeNull();
    expect(stub.state.calls).toBe(0);

    // Two critical events bypass the batch and use both of the hour's calls.
    await Promise.all([event(2), event(3)].map((e) => coord.analyze(e)));
    expect(stub.state.calls).toBe(2);

    // stop() flushes the queued event; the hour has no call left for it.
    await coord.stop();

    expect(stub.state.calls).toBe(2);
    expect(readBudgetFile(dataDir).callsThisHour).toBe(2);
  });

  it('a batch flush with room in the hour makes one call and records what it cost', async () => {
    const coord = new IntelligenceCoordinator(
      config({ budgetUsd: 100, maxCallsPerHour: 20, enableBatching: true, batchWindowMs: 3600000 }),
      dataDir,
    );

    await coord.analyze(event(1, 'high'));
    await coord.analyze(event(2, 'high'));
    await coord.stop();

    expect(stub.state.calls).toBe(1);
    const state = readBudgetFile(dataDir);
    expect(state.callsThisHour).toBe(1);
    expect(state.totalCalls).toBe(1);
    expect(state.totalSpentUsd).toBe(ANSWERED_COST_USD);
  });

  describe('a batched event is queued only when the budget can pay for its batch', () => {
    it('2000 batched events are not queued while the budget is below one batch estimate', async () => {
      vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout'] });
      const coord = new IntelligenceCoordinator(config({ budgetUsd: 0.001, enableBatching: true }), dataDir);

      const assessments = await Promise.all(
        Array.from({ length: 2000 }, (_, i) => coord.analyze(event(i, 'high'))),
      );

      expect(assessments.every((assessment) => assessment === null)).toBe(true);
      expect(queuedForBatch(coord)).toBe(0);
      // No batch is waiting, so no timer is set to send one.
      expect(vi.getTimerCount()).toBe(0);

      await coord.stop();
      expect(stub.state.calls).toBe(0);
    });

    it('an event that arrives once the calls of the hour are used up is not queued, and one in the next hour is', async () => {
      vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout', 'Date'] });
      vi.setSystemTime(new Date(2026, 2, 10, 10, 0));
      const coord = new IntelligenceCoordinator(
        config({ budgetUsd: 100, maxCallsPerHour: 2, enableBatching: true, batchWindowMs: 3600000 }),
        dataDir,
      );

      // Two critical events bypass the batch and use both of the hour's calls.
      await Promise.all([event(1), event(2)].map((e) => coord.analyze(e)));
      expect(stub.state.calls).toBe(2);

      expect(await coord.analyze(event(3, 'high'))).toBeNull();
      expect(queuedForBatch(coord)).toBe(0);
      expect(vi.getTimerCount()).toBe(0);

      // The next hour has calls again: an event is queued, and the batch sent holds it alone.
      vi.setSystemTime(new Date(2026, 2, 10, 11, 1));
      expect(await coord.analyze(event(4, 'high'))).toBeNull();
      expect(queuedForBatch(coord)).toBe(1);

      await coord.stop();
      expect(stub.state.calls).toBe(3);
      expect(stub.state.prompts[2]).toMatch(/^BATCH SECURITY ASSESSMENT — 1 anomalies\./);
    });

    it('an event is not queued when the budget holds a single-event estimate and not a batch estimate', async () => {
      // 0.006 is more than a single-event estimate (0.005) and less than a batch estimate (0.008).
      const coord = new IntelligenceCoordinator(config({ budgetUsd: 0.006, enableBatching: true }), dataDir);

      expect(await coord.analyze(event(1, 'high'))).toBeNull();
      expect(queuedForBatch(coord)).toBe(0);

      await coord.stop();
      expect(stub.state.calls).toBe(0);
    });

    it('an event is queued, and its batch sent, when the budget holds exactly one batch estimate', async () => {
      const coord = new IntelligenceCoordinator(config({ budgetUsd: BATCH_ESTIMATE_USD, enableBatching: true }), dataDir);

      expect(await coord.analyze(event(1, 'high'))).toBeNull();
      expect(queuedForBatch(coord)).toBe(1);

      await coord.stop();
      expect(stub.state.calls).toBe(1);
      expect(readBudgetFile(dataDir).totalSpentUsd).toBe(ANSWERED_COST_USD);
    });

    it('an event is not queued when the adapter cannot estimate the batch call, and analyze() resolves', async () => {
      stub.state.estimateFails = true;
      const coord = new IntelligenceCoordinator(config({ budgetUsd: 100, enableBatching: true }), dataDir);

      await expect(coord.analyze(event(1, 'high'))).resolves.toBeNull();
      expect(queuedForBatch(coord)).toBe(0);

      await coord.stop();
      expect(stub.state.calls).toBe(0);
    });
  });
});
