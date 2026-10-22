import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import * as fs from 'fs';
import * as path from 'path';
import * as os from 'os';
import { BudgetController, type BudgetReservation } from '../../../src/arp/intelligence/budget';

function reserveOrFail(budget: BudgetController, estimatedCostUsd: number): BudgetReservation {
  const reservation = budget.reserve(estimatedCostUsd);
  if (!reservation) throw new Error(`reserve(${estimatedCostUsd}) was refused`);
  return reservation;
}

describe('BudgetController', () => {
  let tmpDir: string;

  beforeEach(() => {
    tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'arp-budget-test-'));
  });

  afterEach(() => {
    vi.useRealTimers();
    fs.rmSync(tmpDir, { recursive: true, force: true });
  });

  it('starts with zero spending', () => {
    const budget = new BudgetController(tmpDir);
    const status = budget.getStatus();
    expect(status.spent).toBe(0);
    expect(status.budget).toBe(5.0);
    expect(status.remaining).toBe(5.0);
    expect(status.percentUsed).toBe(0);
  });

  it('allows calls within budget', () => {
    const budget = new BudgetController(tmpDir);
    expect(budget.reserve(0.001)).not.toBeNull();
  });

  it('rejects calls exceeding budget', () => {
    const budget = new BudgetController(tmpDir);
    expect(budget.reserve(6.0)).toBeNull();
  });

  it('tracks spending', () => {
    const budget = new BudgetController(tmpDir);
    budget.record(reserveOrFail(budget, 0.001), 0.001, 300);
    budget.record(reserveOrFail(budget, 0.002), 0.002, 500);

    const status = budget.getStatus();
    expect(status.spent).toBe(0.003);
    expect(status.totalCalls).toBe(2);
  });

  it('enforces hourly rate limit', () => {
    const budget = new BudgetController(tmpDir, { maxCallsPerHour: 3 });

    budget.record(reserveOrFail(budget, 0.001), 0.001, 100);
    budget.record(reserveOrFail(budget, 0.001), 0.001, 100);
    budget.record(reserveOrFail(budget, 0.001), 0.001, 100);

    expect(budget.reserve(0.001)).toBeNull();
  });

  it('getStatus counts the calls of the current hour once the hour they were counted in is over', () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date(2026, 2, 10, 10, 0));
    const budget = new BudgetController(tmpDir);
    budget.record(reserveOrFail(budget, 0.01), 0.01, 10);
    expect(budget.getStatus().callsThisHour).toBe(1);

    vi.setSystemTime(new Date(2026, 2, 10, 11, 1));

    expect(budget.getStatus().callsThisHour).toBe(0);
    // A process that makes no call, such as a status command, reads the count from budget.json.
    expect(new BudgetController(tmpDir).getStatus().callsThisHour).toBe(0);
    reserveOrFail(budget, 0.01);
    expect(budget.getStatus().callsThisHour).toBe(1);
  });

  it('persists state to disk', () => {
    const budget1 = new BudgetController(tmpDir);
    budget1.record(reserveOrFail(budget1, 0.05), 0.05, 1000);

    const budget2 = new BudgetController(tmpDir);
    const status = budget2.getStatus();
    expect(status.spent).toBe(0.05);
    expect(status.totalCalls).toBe(1);
  });

  it('respects custom budget limit', () => {
    const budget = new BudgetController(tmpDir, { budgetUsd: 1.0 });
    expect(budget.reserve(0.5)).not.toBeNull();
    expect(budget.reserve(1.5)).toBeNull();
  });

  it('resets budget', () => {
    const budget = new BudgetController(tmpDir);
    budget.record(reserveOrFail(budget, 0.5), 0.5, 10000);
    budget.reset();

    const status = budget.getStatus();
    expect(status.spent).toBe(0);
    expect(status.totalCalls).toBe(0);
  });

  it('keeps recent costs capped at 100', () => {
    // 110 calls need an hourly limit of at least 110; the default is 20.
    const budget = new BudgetController(tmpDir, { maxCallsPerHour: 110 });
    for (let i = 0; i < 110; i++) {
      budget.record(reserveOrFail(budget, 0.0001), 0.0001, 10);
    }

    const status = budget.getStatus();
    expect(status.totalCalls).toBe(110);
    // Internal check via persisted file
    const state = JSON.parse(fs.readFileSync(path.join(tmpDir, 'budget.json'), 'utf-8'));
    expect(state.recentCosts.length).toBeLessThanOrEqual(100);
  });

  describe('reservations', () => {
    it('counts a call against the hourly limit when it is reserved, before it is recorded', () => {
      const budget = new BudgetController(tmpDir, { maxCallsPerHour: 2 });

      expect(budget.reserve(0.001)).not.toBeNull();
      expect(budget.reserve(0.001)).not.toBeNull();

      expect(budget.reserve(0.001)).toBeNull();
      expect(budget.getStatus().callsThisHour).toBe(2);
    });

    it('holds the estimate against the budget until the call is recorded', () => {
      const budget = new BudgetController(tmpDir, { budgetUsd: 1.0 });

      expect(budget.reserve(0.6)).not.toBeNull();

      expect(budget.reserve(0.6)).toBeNull();
      expect(budget.getStatus().spent).toBe(0.6);
    });

    it('record replaces the estimate with the actual cost and counts the call once', () => {
      const budget = new BudgetController(tmpDir);
      const reservation = reserveOrFail(budget, 0.5);

      budget.record(reservation, 0.2, 100);

      const status = budget.getStatus();
      expect(status.spent).toBe(0.2);
      expect(status.totalCalls).toBe(1);
      expect(status.callsThisHour).toBe(1);
    });

    it('release gives back the slot and the estimate of a call that failed', () => {
      const budget = new BudgetController(tmpDir, { budgetUsd: 1.0, maxCallsPerHour: 1 });
      const reservation = reserveOrFail(budget, 0.6);
      expect(budget.reserve(0.1)).toBeNull();

      budget.release(reservation);

      const status = budget.getStatus();
      expect(status.spent).toBe(0);
      expect(status.callsThisHour).toBe(0);
      expect(status.totalCalls).toBe(0);
      expect(budget.reserve(0.6)).not.toBeNull();
    });

    it('closes a reservation once', () => {
      const budget = new BudgetController(tmpDir);
      const reservation = reserveOrFail(budget, 0.1);
      budget.record(reservation, 0.1, 10);

      expect(() => budget.record(reservation, 0.1, 10)).toThrow(/already recorded or released/);
      expect(() => budget.release(reservation)).toThrow(/already recorded or released/);

      const status = budget.getStatus();
      expect(status.spent).toBe(0.1);
      expect(status.totalCalls).toBe(1);
      expect(status.callsThisHour).toBe(1);
    });

    it('record and release refuse a value that did not come from reserve()', () => {
      const budget = new BudgetController(tmpDir);
      // The earlier record(costUsd, tokens) form, as a JavaScript caller would still send it.
      const recordAsBefore = budget.record as unknown as (costUsd: number, tokens: number) => void;

      expect(() => recordAsBefore.call(budget, 0.001, 300)).toThrow(/did not come from reserve\(\)/);
      expect(() => budget.record({ estimatedCostUsd: 0 }, 0, 0)).toThrow(/did not come from reserve\(\)/);
      expect(() => budget.release({ estimatedCostUsd: 0 })).toThrow(/did not come from reserve\(\)/);

      const status = budget.getStatus();
      expect(status.spent).toBe(0);
      expect(status.totalCalls).toBe(0);
      expect(status.callsThisHour).toBe(0);
    });

    it('refuses an estimate that is not a number of 0 or more', () => {
      const budget = new BudgetController(tmpDir);

      expect(budget.reserve(Number.NaN)).toBeNull();
      expect(budget.reserve(-1)).toBeNull();
      expect(budget.reserve(Number.NEGATIVE_INFINITY)).toBeNull();
      expect(budget.reserve('0.1' as unknown as number)).toBeNull();

      expect(budget.getStatus().callsThisHour).toBe(0);
      expect(budget.reserve(0)).not.toBeNull();
    });

    it('canReserve answers as reserve would, and reserves nothing', () => {
      const budget = new BudgetController(tmpDir, { budgetUsd: 1.0, maxCallsPerHour: 2 });

      expect(budget.canReserve(0.6)).toBe(true);
      expect(budget.canReserve(0.6)).toBe(true);
      expect(budget.getStatus().spent).toBe(0);
      expect(budget.getStatus().callsThisHour).toBe(0);
      expect(fs.existsSync(path.join(tmpDir, 'budget.json'))).toBe(false);

      reserveOrFail(budget, 0.6);
      // The estimate held for the call in flight leaves 0.4 of the budget.
      expect(budget.canReserve(0.6)).toBe(false);
      expect(budget.reserve(0.6)).toBeNull();
      expect(budget.canReserve(0.4)).toBe(true);

      reserveOrFail(budget, 0.4);
      // Both of the hour's calls are reserved.
      expect(budget.canReserve(0)).toBe(false);
      expect(budget.reserve(0)).toBeNull();
    });

    it('canReserve refuses an estimate that is not a number of 0 or more', () => {
      const budget = new BudgetController(tmpDir);

      expect(budget.canReserve(Number.NaN)).toBe(false);
      expect(budget.canReserve(-1)).toBe(false);
      expect(budget.canReserve('0.1' as unknown as number)).toBe(false);
      expect(budget.canReserve(0)).toBe(true);
    });

    it('canReserve counts the calls of the current hour only', () => {
      vi.useFakeTimers();
      vi.setSystemTime(new Date(2026, 2, 10, 10, 0));
      const budget = new BudgetController(tmpDir, { maxCallsPerHour: 1 });
      reserveOrFail(budget, 0.01);
      expect(budget.canReserve(0.01)).toBe(false);

      vi.setSystemTime(new Date(2026, 2, 10, 11, 1));
      expect(budget.canReserve(0.01)).toBe(true);
    });

    it('charges the estimate for a cost that is not a finite number of 0 or more', () => {
      const budget = new BudgetController(tmpDir);

      budget.record(reserveOrFail(budget, 0.25), Number.NaN, 10);
      budget.record(reserveOrFail(budget, 0.25), -1, 10);
      budget.record(reserveOrFail(budget, 0.25), Number.POSITIVE_INFINITY, 10);

      const status = budget.getStatus();
      expect(status.spent).toBe(0.75);
      expect(status.totalCalls).toBe(3);
    });

    it('saves the hold when it is reserved, and the settled or released state after', () => {
      const budget = new BudgetController(tmpDir);
      const answered = reserveOrFail(budget, 0.3);

      const whileInFlight = new BudgetController(tmpDir).getStatus();
      expect(whileInFlight.spent).toBe(0.3);
      expect(whileInFlight.callsThisHour).toBe(1);
      expect(whileInFlight.totalCalls).toBe(0);

      budget.record(answered, 0.1, 50);
      const afterRecord = new BudgetController(tmpDir).getStatus();
      expect(afterRecord.spent).toBe(0.1);
      expect(afterRecord.callsThisHour).toBe(1);
      expect(afterRecord.totalCalls).toBe(1);

      budget.release(reserveOrFail(budget, 0.3));
      const afterRelease = new BudgetController(tmpDir).getStatus();
      expect(afterRelease.spent).toBe(0.1);
      expect(afterRelease.callsThisHour).toBe(1);
      expect(afterRelease.totalCalls).toBe(1);
    });

    it('holds nothing for a reservation it could not save', () => {
      const notADirectory = path.join(tmpDir, 'not-a-directory');
      fs.writeFileSync(notADirectory, '');
      const budget = new BudgetController(notADirectory);

      expect(() => budget.reserve(0.1)).toThrow();

      const status = budget.getStatus();
      expect(status.spent).toBe(0);
      expect(status.callsThisHour).toBe(0);
    });

    it('a release after the hour starts again does not free a slot in the new hour', () => {
      vi.useFakeTimers();
      vi.setSystemTime(new Date(2026, 2, 10, 10, 0));
      const budget = new BudgetController(tmpDir, { maxCallsPerHour: 2 });
      const fromPreviousHour = reserveOrFail(budget, 0.01);

      vi.setSystemTime(new Date(2026, 2, 10, 11, 1));
      reserveOrFail(budget, 0.01);
      budget.release(fromPreviousHour);

      expect(budget.getStatus().callsThisHour).toBe(1);
      expect(budget.reserve(0.01)).not.toBeNull();
      expect(budget.reserve(0.01)).toBeNull();
    });

    it('a release after getStatus starts the hour again does not take the count below zero', () => {
      vi.useFakeTimers();
      vi.setSystemTime(new Date(2026, 2, 10, 10, 0));
      const budget = new BudgetController(tmpDir);
      const fromPreviousHour = reserveOrFail(budget, 0.01);

      vi.setSystemTime(new Date(2026, 2, 10, 11, 1));
      expect(budget.getStatus().callsThisHour).toBe(0);
      budget.release(fromPreviousHour);

      expect(budget.getStatus().callsThisHour).toBe(0);
    });

    it('a call in flight when the month changes is charged to the month it is recorded in', () => {
      vi.useFakeTimers();
      vi.setSystemTime(new Date(2026, 2, 31, 23, 30));
      const budget = new BudgetController(tmpDir);
      budget.record(reserveOrFail(budget, 0.4), 0.4, 10);
      const answered = reserveOrFail(budget, 0.4);
      const failed = reserveOrFail(budget, 0.4);

      vi.setSystemTime(new Date(2026, 3, 1, 0, 30));
      budget.record(answered, 0.1, 10);
      budget.release(failed);

      const status = budget.getStatus();
      expect(status.spent).toBe(0.1);
      expect(status.totalCalls).toBe(1);
      expect(status.callsThisHour).toBe(0);
    });
  });

  describe('failed calls', () => {
    it('recordFailure counts a call that threw and keeps the latest one, beside totalCalls and in budget.json', () => {
      vi.useFakeTimers();
      vi.setSystemTime(new Date(2026, 9, 18, 10, 0));
      const budget = new BudgetController(tmpDir);
      expect(budget.getStatus().failedCalls).toBe(0);
      expect(budget.getStatus().lastFailure).toBeNull();

      budget.release(reserveOrFail(budget, 0.01));
      budget.recordFailure('ollama', 'ECONNREFUSED');
      vi.setSystemTime(new Date(2026, 9, 18, 10, 5));
      budget.release(reserveOrFail(budget, 0.01));
      budget.recordFailure('ollama', 'HTTP 500');
      budget.record(reserveOrFail(budget, 0.01), 0.01, 10);

      const status = budget.getStatus();
      expect(status.failedCalls).toBe(2);
      expect(status.totalCalls).toBe(1);
      expect(status.lastFailure).toEqual({
        at: new Date(2026, 9, 18, 10, 5).toISOString(),
        adapter: 'ollama',
        message: 'HTTP 500',
      });

      // budget.json keeps them with totalCalls, so a status command in another process reads them.
      const reloaded = new BudgetController(tmpDir).getStatus();
      expect(reloaded.failedCalls).toBe(2);
      expect(reloaded.totalCalls).toBe(1);
      expect(reloaded.lastFailure).toEqual(status.lastFailure);

      // The status is a copy: changing it changes nothing the controller holds.
      if (status.lastFailure) status.lastFailure.message = 'changed';
      expect(budget.getStatus().lastFailure?.message).toBe('HTTP 500');
    });

    it('a budget.json saved before failedCalls existed reads as no failed call', () => {
      const now = new Date().toISOString();
      fs.writeFileSync(
        path.join(tmpDir, 'budget.json'),
        JSON.stringify({ totalSpentUsd: 0.2, periodStart: now, totalCalls: 3, callsThisHour: 0, hourStart: now, recentCosts: [] }),
      );

      const status = new BudgetController(tmpDir).getStatus();

      expect(status.totalCalls).toBe(3);
      expect(status.failedCalls).toBe(0);
      expect(status.lastFailure).toBeNull();
    });

    it('failedCalls and lastFailure start again with the monthly period, as totalCalls does', () => {
      vi.useFakeTimers();
      vi.setSystemTime(new Date(2026, 2, 31, 23, 30));
      const budget = new BudgetController(tmpDir);
      budget.recordFailure('ollama', 'ECONNREFUSED');
      expect(budget.getStatus().failedCalls).toBe(1);

      vi.setSystemTime(new Date(2026, 3, 1, 0, 30));

      const status = budget.getStatus();
      expect(status.failedCalls).toBe(0);
      expect(status.lastFailure).toBeNull();
    });
  });
});
