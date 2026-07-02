import * as fs from 'fs';
import * as path from 'path';
import type { BudgetState, IntelligenceConfig } from '../types';

const BUDGET_FILE = 'budget.json';
const DEFAULT_BUDGET_USD = 5.0;
const DEFAULT_MAX_CALLS_PER_HOUR = 20;

/**
 * Tracks LLM usage costs with hard limits to prevent runaway spending.
 * Resets monthly. Persists to disk so restarts don't lose tracking.
 */
export class BudgetController {
  private state: BudgetState;
  private readonly budgetUsd: number;
  private readonly maxCallsPerHour: number;
  private readonly dataDir: string;

  constructor(dataDir: string, config?: IntelligenceConfig) {
    this.dataDir = dataDir;
    this.budgetUsd = config?.budgetUsd ?? DEFAULT_BUDGET_USD;
    this.maxCallsPerHour = config?.maxCallsPerHour ?? DEFAULT_MAX_CALLS_PER_HOUR;
    this.state = this.loadState();
  }

  /** Check if we can afford an LLM call. Returns false if budget exhausted. */
  canAfford(estimatedCostUsd: number): boolean {
    this.rolloverIfNeeded();

    // Hard budget limit
    if (this.state.totalSpentUsd + estimatedCostUsd > this.budgetUsd) {
      return false;
    }

    // Hourly rate limit
    this.rolloverHourIfNeeded();
    if (this.state.callsThisHour >= this.maxCallsPerHour) {
      return false;
    }

    return true;
  }

  /** Record a completed LLM call */
  record(costUsd: number, tokens: number): void {
    this.rolloverIfNeeded();
    this.rolloverHourIfNeeded();

    this.state.totalSpentUsd += costUsd;
    this.state.totalCalls += 1;
    this.state.callsThisHour += 1;

    this.state.recentCosts.push({
      timestamp: new Date().toISOString(),
      cost: costUsd,
      tokens,
    });

    // Keep last 100 entries
    if (this.state.recentCosts.length > 100) {
      this.state.recentCosts = this.state.recentCosts.slice(-100);
    }

    this.saveState();
  }

  /** Get current budget status */
  getStatus(): {
    spent: number;
    budget: number;
    remaining: number;
    percentUsed: number;
    callsThisHour: number;
    maxCallsPerHour: number;
    totalCalls: number;
  } {
    this.rolloverIfNeeded();
    return {
      spent: Math.round(this.state.totalSpentUsd * 10000) / 10000,
      budget: this.budgetUsd,
      remaining: Math.round((this.budgetUsd - this.state.totalSpentUsd) * 10000) / 10000,
      percentUsed: Math.round((this.state.totalSpentUsd / this.budgetUsd) * 100),
      callsThisHour: this.state.callsThisHour,
      maxCallsPerHour: this.maxCallsPerHour,
      totalCalls: this.state.totalCalls,
    };
  }

  /** Reset budget for new period */
  reset(): void {
    this.state = freshState();
    this.saveState();
  }

  private rolloverIfNeeded(): void {
    const periodStart = new Date(this.state.periodStart);
    const now = new Date();

    // Reset monthly
    if (now.getMonth() !== periodStart.getMonth() || now.getFullYear() !== periodStart.getFullYear()) {
      this.state = freshState();
      this.saveState();
    }
  }

  private rolloverHourIfNeeded(): void {
    const hourStart = new Date(this.state.hourStart);
    const now = new Date();

    if (now.getTime() - hourStart.getTime() > 3600000) {
      this.state.callsThisHour = 0;
      this.state.hourStart = now.toISOString();
    }
  }

  private loadState(): BudgetState {
    const filePath = path.join(this.dataDir, BUDGET_FILE);
    try {
      if (fs.existsSync(filePath)) {
        return JSON.parse(fs.readFileSync(filePath, 'utf-8'));
      }
    } catch {
      // Corrupted file — start fresh
    }
    return freshState();
  }

  private saveState(): void {
    fs.mkdirSync(this.dataDir, { recursive: true });
    const filePath = path.join(this.dataDir, BUDGET_FILE);
    fs.writeFileSync(filePath, JSON.stringify(this.state, null, 2), 'utf-8');
  }
}

function freshState(): BudgetState {
  const now = new Date().toISOString();
  return {
    totalSpentUsd: 0,
    periodStart: now,
    totalCalls: 0,
    callsThisHour: 0,
    hourStart: now,
    recentCosts: [],
  };
}
