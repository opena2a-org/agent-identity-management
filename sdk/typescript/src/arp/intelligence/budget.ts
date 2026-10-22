import * as fs from 'fs';
import * as path from 'path';
import type { BudgetState, IntelligenceConfig, L2CallFailure } from '../types';

const BUDGET_FILE = 'budget.json';
const DEFAULT_BUDGET_USD = 5.0;
const DEFAULT_MAX_CALLS_PER_HOUR = 20;

/**
 * One L2 call's hold on the budget: a slot in the hour's call count and the call's
 * estimated cost. `reserve` takes it before the call is made, and it is closed once:
 * by `record` with the actual cost when the call returns, or by `release` when it fails.
 */
export interface BudgetReservation {
  /** The estimated cost held for the call, in USD. */
  readonly estimatedCostUsd: number;
}

/**
 * Tracks LLM usage costs with hard limits to prevent runaway spending.
 * Resets monthly. Persists to disk so restarts don't lose tracking.
 *
 * The limits are enforced by reservation: `reserve` counts a call and holds its
 * estimated cost before the call is made, so the calls still in flight count against
 * `budgetUsd` and `maxCallsPerHour` when the next call asks. A limit that counted a
 * call only after it returned would not see the calls in flight.
 */
export class BudgetController {
  private state: BudgetState;
  private readonly budgetUsd: number;
  private readonly maxCallsPerHour: number;
  private readonly dataDir: string;
  /** Open reservations, each with the hour window its call was counted in. */
  private readonly holds = new Map<BudgetReservation, { hourWindow: number }>();
  /** Advances every time `callsThisHour` starts again from zero. */
  private hourWindow = 0;

  constructor(dataDir: string, config?: IntelligenceConfig) {
    this.dataDir = dataDir;
    this.budgetUsd = config?.budgetUsd ?? DEFAULT_BUDGET_USD;
    this.maxCallsPerHour = config?.maxCallsPerHour ?? DEFAULT_MAX_CALLS_PER_HOUR;
    this.state = this.loadState();
  }

  /**
   * Whether `reserve(estimatedCostUsd)` would be granted now. Reserves nothing.
   *
   * For a caller that decides now whether to make a call later: the coordinator queues
   * a batched event only when the batch's call could be reserved. The answer can change
   * before the call is made, so the call still takes its own reservation.
   */
  canReserve(estimatedCostUsd: number): boolean {
    if (typeof estimatedCostUsd !== 'number' || !(estimatedCostUsd >= 0)) return false;
    this.rolloverIfNeeded();
    this.rolloverHourIfNeeded();

    // Hard budget limit
    if (this.state.totalSpentUsd + this.heldUsd() + estimatedCostUsd > this.budgetUsd) {
      return false;
    }

    // Hourly rate limit
    return this.state.callsThisHour < this.maxCallsPerHour;
  }

  /**
   * Reserve one call and its estimated cost before the call is made.
   *
   * Returns null when the estimate does not fit in what is left of the budget once the
   * estimates held for calls in flight are counted, or when the hour's calls are used
   * up. An estimate that is not a number of 0 or more is refused as well: NaN passes
   * every comparison, and a negative estimate would make room for other calls. The
   * reservation is saved to disk before it is returned.
   */
  reserve(estimatedCostUsd: number): BudgetReservation | null {
    if (!this.canReserve(estimatedCostUsd)) return null;

    const reservation: BudgetReservation = Object.freeze({ estimatedCostUsd });
    this.holds.set(reservation, { hourWindow: this.hourWindow });
    this.state.callsThisHour += 1;
    try {
      this.saveState();
    } catch (error) {
      // The caller never receives this reservation, so nothing stays held for it.
      this.holds.delete(reservation);
      this.state.callsThisHour -= 1;
      throw error;
    }
    return reservation;
  }

  /**
   * Settle a reservation with what the call cost, in place of the estimate held for it.
   * The call stays counted in the hour it was reserved in. A cost that is NaN, infinite
   * or negative is charged at the estimate.
   */
  record(reservation: BudgetReservation, costUsd: number, tokens: number): void {
    this.close(reservation, 'record');
    this.rolloverIfNeeded();
    this.rolloverHourIfNeeded();

    const charged = Number.isFinite(costUsd) && costUsd >= 0 ? costUsd : reservation.estimatedCostUsd;
    this.state.totalSpentUsd += charged;
    this.state.totalCalls += 1;

    this.state.recentCosts.push({
      timestamp: new Date().toISOString(),
      cost: charged,
      tokens,
    });

    // Keep last 100 entries
    if (this.state.recentCosts.length > 100) {
      this.state.recentCosts = this.state.recentCosts.slice(-100);
    }

    this.saveState();
  }

  /**
   * Give back the reservation of a call that failed: its estimate, and its slot in the
   * hour's call count unless that count has started again since the reservation.
   */
  release(reservation: BudgetReservation): void {
    const hold = this.close(reservation, 'release');
    this.rolloverIfNeeded();
    this.rolloverHourIfNeeded();

    if (hold.hourWindow === this.hourWindow) {
      this.state.callsThisHour -= 1;
    }

    this.saveState();
  }

  /**
   * Count an L2 call that threw instead of answering, and keep it as `lastFailure`.
   *
   * `release` gives back the failed call's slot and estimate; this records that the call
   * was made and failed, so that a status read after any number of failed calls does not
   * read as one where L2 never ran. `message` is the error's code or message, never the
   * prompt or the answer. Saved to disk with the rest of the state.
   */
  recordFailure(adapter: string, message: string): void {
    this.rolloverIfNeeded();
    this.rolloverHourIfNeeded();

    this.state.failedCalls += 1;
    this.state.lastFailure = { at: new Date().toISOString(), adapter, message };

    this.saveState();
  }

  /**
   * Get current budget status. `spent` counts each call in flight at its estimate, and
   * `callsThisHour` starts again from zero once the hour is over, as it does in `reserve`.
   * `totalCalls` counts the answered calls and `failedCalls` the calls that threw.
   */
  getStatus(): {
    spent: number;
    budget: number;
    remaining: number;
    percentUsed: number;
    callsThisHour: number;
    maxCallsPerHour: number;
    totalCalls: number;
    failedCalls: number;
    lastFailure: L2CallFailure | null;
  } {
    this.rolloverIfNeeded();
    this.rolloverHourIfNeeded();
    const spent = this.state.totalSpentUsd + this.heldUsd();
    return {
      spent: Math.round(spent * 10000) / 10000,
      budget: this.budgetUsd,
      remaining: Math.round((this.budgetUsd - spent) * 10000) / 10000,
      percentUsed: Math.round((spent / this.budgetUsd) * 100),
      callsThisHour: this.state.callsThisHour,
      maxCallsPerHour: this.maxCallsPerHour,
      totalCalls: this.state.totalCalls,
      failedCalls: this.state.failedCalls,
      lastFailure: this.state.lastFailure ? { ...this.state.lastFailure } : null,
    };
  }

  /** Reset budget for new period. A call still in flight stays held until it settles. */
  reset(): void {
    this.state = freshState();
    this.hourWindow += 1;
    this.saveState();
  }

  /** The estimates held for calls still in flight, summed afresh so none is left over. */
  private heldUsd(): number {
    let held = 0;
    for (const reservation of this.holds.keys()) {
      held += reservation.estimatedCostUsd;
    }
    return held;
  }

  /** Close an open reservation, so that each one is recorded or released once. */
  private close(reservation: BudgetReservation, method: 'record' | 'release'): { hourWindow: number } {
    const hold = this.holds.get(reservation);
    if (!hold) {
      throw new Error(
        `BudgetController.${method}() needs an open reservation from reserve() on this controller; ` +
          'this one was already recorded or released, or did not come from reserve()',
      );
    }
    this.holds.delete(reservation);
    return hold;
  }

  private rolloverIfNeeded(): void {
    const periodStart = new Date(this.state.periodStart);
    const now = new Date();

    // Reset monthly
    if (now.getMonth() !== periodStart.getMonth() || now.getFullYear() !== periodStart.getFullYear()) {
      this.state = freshState();
      this.hourWindow += 1;
      this.saveState();
    }
  }

  private rolloverHourIfNeeded(): void {
    const hourStart = new Date(this.state.hourStart);
    const now = new Date();

    if (now.getTime() - hourStart.getTime() > 3600000) {
      this.state.callsThisHour = 0;
      this.state.hourStart = now.toISOString();
      this.hourWindow += 1;
    }
  }

  private loadState(): BudgetState {
    const filePath = path.join(this.dataDir, BUDGET_FILE);
    try {
      if (fs.existsSync(filePath)) {
        const saved: BudgetState = JSON.parse(fs.readFileSync(filePath, 'utf-8'));
        // A budget.json saved before failed calls were counted holds neither field.
        return { ...saved, failedCalls: saved.failedCalls ?? 0, lastFailure: saved.lastFailure ?? null };
      }
    } catch {
      // Corrupted file — start fresh
    }
    return freshState();
  }

  private saveState(): void {
    fs.mkdirSync(this.dataDir, { recursive: true });
    const filePath = path.join(this.dataDir, BUDGET_FILE);
    // The file counts the estimates still held as spent, so a process that stops while
    // a call is in flight leaves that call's estimate charged instead of forgotten.
    const persisted: BudgetState = { ...this.state, totalSpentUsd: this.state.totalSpentUsd + this.heldUsd() };
    fs.writeFileSync(filePath, JSON.stringify(persisted, null, 2), 'utf-8');
  }
}

function freshState(): BudgetState {
  const now = new Date().toISOString();
  return {
    totalSpentUsd: 0,
    periodStart: now,
    totalCalls: 0,
    failedCalls: 0,
    lastFailure: null,
    callsThisHour: 0,
    hourStart: now,
    recentCosts: [],
  };
}
