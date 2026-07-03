import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { spawn, type ChildProcess } from 'child_process';
import { EnforcementEngine } from './kill-switch';
import type { ARPEvent } from '../types';

function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

function isAlive(pid: number): boolean {
  try { process.kill(pid, 0); return true; } catch { return false; }
}

function mockEvent(pid: number): ARPEvent {
  return {
    id: 'test',
    timestamp: new Date().toISOString(),
    source: 'process',
    category: 'threat',
    severity: 'critical',
    description: 'test',
    data: { pid },
    classifiedBy: 'L0-rules',
  };
}

describe('EnforcementEngine.kill — paused process regression', () => {
  let engine: EnforcementEngine;
  let child: ChildProcess | null = null;

  beforeEach(() => { engine = new EnforcementEngine(); });

  afterEach(() => {
    if (child?.pid) {
      try { process.kill(child.pid, 'SIGKILL'); } catch { /* dead */ }
      child = null;
    }
  });

  it('kill terminates a paused (SIGSTOPped) process by resuming it first', async () => {
    child = spawn('node', ['-e', 'setTimeout(()=>{},30000)'], { stdio: 'ignore' });
    const pid = child.pid!;

    await engine.execute('pause', mockEvent(pid), pid);
    expect(engine.getPausedPids()).toContain(pid);
    expect(isAlive(pid)).toBe(true);

    const result = await engine.execute('kill', mockEvent(pid), pid);
    expect(result.success).toBe(true);
    expect(engine.getPausedPids()).not.toContain(pid);

    await sleep(1000);
    expect(isAlive(pid)).toBe(false);
  }, 10000);
});
