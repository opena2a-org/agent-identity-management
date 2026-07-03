import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import * as fs from 'fs';
import * as path from 'path';
import * as os from 'os';
import { BudgetController } from '../../../src/arp/intelligence/budget';

describe('BudgetController', () => {
  let tmpDir: string;

  beforeEach(() => {
    tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'arp-budget-test-'));
  });

  afterEach(() => {
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
    expect(budget.canAfford(0.001)).toBe(true);
  });

  it('rejects calls exceeding budget', () => {
    const budget = new BudgetController(tmpDir);
    expect(budget.canAfford(6.0)).toBe(false);
  });

  it('tracks spending', () => {
    const budget = new BudgetController(tmpDir);
    budget.record(0.001, 300);
    budget.record(0.002, 500);

    const status = budget.getStatus();
    expect(status.spent).toBe(0.003);
    expect(status.totalCalls).toBe(2);
  });

  it('enforces hourly rate limit', () => {
    const budget = new BudgetController(tmpDir, { maxCallsPerHour: 3 });

    budget.record(0.001, 100);
    budget.record(0.001, 100);
    budget.record(0.001, 100);

    expect(budget.canAfford(0.001)).toBe(false);
  });

  it('persists state to disk', () => {
    const budget1 = new BudgetController(tmpDir);
    budget1.record(0.05, 1000);

    const budget2 = new BudgetController(tmpDir);
    const status = budget2.getStatus();
    expect(status.spent).toBe(0.05);
    expect(status.totalCalls).toBe(1);
  });

  it('respects custom budget limit', () => {
    const budget = new BudgetController(tmpDir, { budgetUsd: 1.0 });
    expect(budget.canAfford(0.5)).toBe(true);
    expect(budget.canAfford(1.5)).toBe(false);
  });

  it('resets budget', () => {
    const budget = new BudgetController(tmpDir);
    budget.record(0.5, 10000);
    budget.reset();

    const status = budget.getStatus();
    expect(status.spent).toBe(0);
    expect(status.totalCalls).toBe(0);
  });

  it('keeps recent costs capped at 100', () => {
    const budget = new BudgetController(tmpDir);
    for (let i = 0; i < 110; i++) {
      budget.record(0.0001, 10);
    }

    const status = budget.getStatus();
    expect(status.totalCalls).toBe(110);
    // Internal check via persisted file
    const state = JSON.parse(fs.readFileSync(path.join(tmpDir, 'budget.json'), 'utf-8'));
    expect(state.recentCosts.length).toBeLessThanOrEqual(100);
  });
});
