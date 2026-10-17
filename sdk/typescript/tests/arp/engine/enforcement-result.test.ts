import { describe, it, expect, afterEach, vi } from 'vitest';
import { spawn, type ChildProcess } from 'child_process';
import { mkdirSync, mkdtempSync, readFileSync, rmSync } from 'fs';
import { tmpdir } from 'os';
import { join } from 'path';
import { AgentRuntimeProtection } from '../../../src/arp';
import { EventEngine } from '../../../src/arp/engine/event-engine';
import type { AlertRule, ARPConfig, ARPEvent, EnforcementResult } from '../../../src/arp/types';

/**
 * What an enforcement handler is told about an action: the result of the
 * action after it ran, the same result the ARP writes to enforcement.jsonl.
 *
 * The signalling tests send signals only to a child process this file spawns,
 * or to PID 2147483646, which is above every platform's PID limit. The kill
 * path arms a 5 s SIGKILL follow-up timer; every test that reaches it holds
 * that timer in a fake clock and drops it, so no signal is sent after the test.
 * The PID-guard test runs with process.kill replaced by a stub.
 */

/** A PID no process can hold: above the PID limit of every supported platform. */
const UNUSED_PID = 2147483646;

interface Decision {
  action: EnforcementResult['action'];
  success: boolean;
  outcome: EnforcementResult['outcome'];
  targetPid: number | undefined;
}

interface LoggedEnforcement extends Decision {
  reason: string;
  eventId: string;
}

function decision(r: Pick<EnforcementResult, 'action' | 'success' | 'outcome' | 'targetPid'>): Decision {
  return { action: r.action, success: r.success, outcome: r.outcome, targetPid: r.targetPid };
}

const scratchDirs: string[] = [];
const children: ChildProcess[] = [];

afterEach(() => {
  for (const child of children.splice(0)) {
    // Signal only a child node still tracks as running, so a PID the system
    // has already handed to another process is never signalled.
    if (child.pid && child.exitCode === null && child.signalCode === null) {
      try { child.kill('SIGKILL'); } catch { /* already gone */ }
    }
  }
  for (const dir of scratchDirs.splice(0)) {
    rmSync(dir, { recursive: true, force: true });
  }
});

interface Harness {
  arp: AgentRuntimeProtection;
  dataDir: string;
  /** Results seen by a handler registered with arp.onEnforcement */
  viaArp: EnforcementResult[];
  /** Results seen by a handler registered with arp.getEngine().onEnforcement */
  viaEngine: EnforcementResult[];
}

function makeArp(rules?: AlertRule[]): Harness {
  const dataDir = mkdtempSync(join(tmpdir(), 'arp-enforcement-result-'));
  scratchDirs.push(dataDir);
  const config: ARPConfig = {
    agentName: 'enforcement-result-test',
    dataDir,
    monitors: {
      process: { enabled: false },
      network: { enabled: false },
      filesystem: { enabled: false },
    },
    intelligence: { enabled: false },
    ...(rules ? { rules } : {}),
  };
  const arp = new AgentRuntimeProtection(config);
  const viaArp: EnforcementResult[] = [];
  const viaEngine: EnforcementResult[] = [];
  arp.onEnforcement((r) => { viaArp.push(r); });
  arp.getEngine().onEnforcement((r) => { viaEngine.push(r); });
  return { arp, dataDir, viaArp, viaEngine };
}

function readEnforcementLog(dataDir: string): LoggedEnforcement[] {
  let text: string;
  try {
    text = readFileSync(join(dataDir, 'enforcement.jsonl'), 'utf-8');
  } catch {
    return [];
  }
  return text.split('\n').filter((l) => l.trim()).map((l) => JSON.parse(l) as LoggedEnforcement);
}

function emitCriticalThreat(engine: EventEngine, data: Record<string, unknown>): Promise<ARPEvent> {
  return engine.emit({
    source: 'process',
    category: 'threat',
    severity: 'critical',
    description: 'Test critical threat',
    data,
  });
}

/** Runs fn with setTimeout faked, then drops every timer it armed. */
async function withHeldTimers<T>(fn: () => Promise<T>): Promise<T> {
  vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout'] });
  try {
    return await fn();
  } finally {
    vi.clearAllTimers();
    vi.useRealTimers();
  }
}

interface SpawnedChild {
  child: ChildProcess;
  pid: number;
  /** Resolves to the signal that ended the child, or 'timeout' after waitMs */
  exitSignal: (waitMs: number) => Promise<NodeJS.Signals | null | 'timeout'>;
}

async function spawnIdleChild(): Promise<SpawnedChild> {
  const child = spawn(process.execPath, ['-e', 'setTimeout(()=>{},30000)'], { stdio: 'ignore' });
  children.push(child);
  const exited = new Promise<NodeJS.Signals | null>((resolve) => {
    child.once('exit', (_code, signal) => resolve(signal));
  });
  await new Promise<void>((resolve, reject) => {
    child.once('spawn', () => resolve());
    child.once('error', reject);
  });
  const pid = child.pid as number;
  return {
    child,
    pid,
    exitSignal: (waitMs) => {
      let timer: NodeJS.Timeout | undefined;
      const timeout = new Promise<'timeout'>((resolve) => {
        timer = setTimeout(() => resolve('timeout'), waitMs);
      });
      return Promise.race([exited, timeout]).finally(() => clearTimeout(timer));
    },
  };
}

type Case = 'no-pid' | 'live-pid' | 'unused-pid';

interface CaseRun {
  emitted: ARPEvent;
  viaArp: EnforcementResult[];
  viaEngine: EnforcementResult[];
  logged: LoggedEnforcement[];
  childPid?: number;
  childSignal?: NodeJS.Signals | null | 'timeout';
  sigtermCalls?: number;
}

/** One critical threat through a fresh ARP under the default rules (critical-threat: kill). */
async function runCase(kind: Case): Promise<CaseRun> {
  const h = makeArp();
  if (kind === 'no-pid') {
    const emitted = await withHeldTimers(() => emitCriticalThreat(h.arp.getEngine(), {}));
    return { emitted, viaArp: h.viaArp, viaEngine: h.viaEngine, logged: readEnforcementLog(h.dataDir) };
  }
  if (kind === 'unused-pid') {
    const emitted = await withHeldTimers(() => emitCriticalThreat(h.arp.getEngine(), { pid: UNUSED_PID }));
    return { emitted, viaArp: h.viaArp, viaEngine: h.viaEngine, logged: readEnforcementLog(h.dataDir) };
  }
  const spawned = await spawnIdleChild();
  const killSpy = vi.spyOn(process, 'kill');
  let emitted: ARPEvent;
  let sigtermCalls: number;
  try {
    emitted = await withHeldTimers(() => emitCriticalThreat(h.arp.getEngine(), { pid: spawned.pid }));
    sigtermCalls = killSpy.mock.calls.filter(([pid, sig]) => pid === spawned.pid && sig === 'SIGTERM').length;
  } finally {
    killSpy.mockRestore();
  }
  const childSignal = await spawned.exitSignal(5000);
  return {
    emitted,
    viaArp: h.viaArp,
    viaEngine: h.viaEngine,
    logged: readEnforcementLog(h.dataDir),
    childPid: spawned.pid,
    childSignal,
    sigtermCalls,
  };
}

function expectedDecision(kind: Case, childPid?: number): Decision {
  switch (kind) {
    case 'no-pid':
      return { action: 'kill', success: false, outcome: 'no-target', targetPid: undefined };
    case 'live-pid':
      return { action: 'kill', success: true, outcome: 'executed', targetPid: childPid };
    case 'unused-pid':
      return { action: 'kill', success: false, outcome: 'failed', targetPid: UNUSED_PID };
  }
}

const CASES: Case[] = ['no-pid', 'live-pid', 'unused-pid'];

describe('enforcement handlers receive the result of the action after it ran', () => {
  it('a kill with no PID reaches the handler as no-target, with the event intact', async () => {
    const run = await runCase('no-pid');
    expect(run.viaArp).toHaveLength(1);
    const [result] = run.viaArp;
    expect(decision(result)).toEqual(expectedDecision('no-pid'));
    expect(result.reason).toBe('No PID to kill');
    expect(result.event).toBe(run.emitted);
    expect(result.event.description).toBe('Test critical threat');
  });

  it('a kill of a live child reaches the handler as executed, with the child PID, and the child exits by SIGTERM', async () => {
    const run = await runCase('live-pid');
    expect(run.viaArp).toHaveLength(1);
    expect(decision(run.viaArp[0])).toEqual(expectedDecision('live-pid', run.childPid));
    expect(run.childSignal).toBe('SIGTERM');
  }, 15000);

  it('a kill of a PID no process holds reaches the handler as failed, with that PID', async () => {
    const run = await runCase('unused-pid');
    expect(run.viaArp).toHaveLength(1);
    const [result] = run.viaArp;
    expect(decision(result)).toEqual(expectedDecision('unused-pid'));
    expect(result.reason).toMatch(new RegExp(`^Failed to kill PID ${UNUSED_PID}: `));
  });

  it('the handler result equals the enforcement.jsonl line for the same decision', async () => {
    for (const kind of CASES) {
      const run = await runCase(kind);
      expect(run.logged, kind).toHaveLength(1);
      const [line] = run.logged;
      expect(line.eventId, kind).toBe(run.emitted.id);
      expect(decision(line), kind).toEqual(expectedDecision(kind, run.childPid));
      expect(decision(run.viaArp[0]), kind).toEqual(decision(line));
      expect(run.viaArp[0].reason, kind).toBe(line.reason);
    }
  }, 20000);

  it('a handler registered on arp.getEngine() receives the same result', async () => {
    for (const kind of CASES) {
      const run = await runCase(kind);
      expect(run.viaEngine, kind).toHaveLength(1);
      expect(decision(run.viaEngine[0]), kind).toEqual(expectedDecision(kind, run.childPid));
      expect(run.viaEngine[0], kind).toBe(run.viaArp[0]);
    }
  }, 20000);

  it('the kill runs once per decision: one SIGTERM, one log line, one result per handler', async () => {
    const run = await runCase('live-pid');
    expect(run.sigtermCalls).toBe(1);
    expect(run.logged).toHaveLength(1);
    expect(run.viaArp).toHaveLength(1);
    expect(run.viaEngine).toHaveLength(1);
    expect(decision(run.viaArp[0])).toEqual(expectedDecision('live-pid', run.childPid));
  }, 15000);
});

describe('an EventEngine with no executor attached', () => {
  it('reports a matched action as not-executed, never as success', async () => {
    const engine = new EventEngine({ agentName: 'enforcement-result-test' });
    const results: EnforcementResult[] = [];
    engine.onEnforcement((r) => { results.push(r); });

    const emitted = await emitCriticalThreat(engine, { pid: UNUSED_PID });

    expect(results).toHaveLength(1);
    expect(decision(results[0])).toEqual({
      action: 'kill',
      success: false,
      outcome: 'not-executed',
      targetPid: undefined,
    });
    expect(results[0].event).toBe(emitted);
  });
});

describe('an EventEngine with a custom executor', () => {
  async function handlerResult(returned: Omit<EnforcementResult, 'action' | 'event'>): Promise<EnforcementResult> {
    const engine = new EventEngine(
      { agentName: 'enforcement-result-test' },
      { execute: async (action, event) => ({ action, event, ...returned }) },
    );
    const results: EnforcementResult[] = [];
    engine.onEnforcement((r) => { results.push(r); });
    const emitted = await emitCriticalThreat(engine, {});
    expect(results).toHaveLength(1);
    expect(results[0].event).toBe(emitted);
    return results[0];
  }

  it('a result with success true and outcome failed reaches the handler as success false', async () => {
    const r = await handlerResult({ success: true, outcome: 'failed', reason: 'x' });
    expect(decision(r)).toEqual({ action: 'kill', success: false, outcome: 'failed', targetPid: undefined });
    expect(r.reason).toBe('x');
  });

  it.each(['no-target', 'not-executed'] as const)(
    'a result with success true and outcome %s reaches the handler as success false',
    async (outcome) => {
      const r = await handlerResult({ success: true, outcome, reason: 'x' });
      expect(decision(r)).toEqual({ action: 'kill', success: false, outcome, targetPid: undefined });
    },
  );

  it('a result with success false and outcome executed reaches the handler as success true', async () => {
    const r = await handlerResult({ success: false, outcome: 'executed', reason: 'x', targetPid: UNUSED_PID });
    expect(decision(r)).toEqual({ action: 'kill', success: true, outcome: 'executed', targetPid: UNUSED_PID });
  });

  it.each([
    [true, 'executed'],
    [false, 'failed'],
  ] as const)('a result with success %s and no outcome reaches the handler as outcome %s', async (success, outcome) => {
    const r = await handlerResult({ success, reason: 'x' });
    expect(decision(r)).toEqual({ action: 'kill', success, outcome, targetPid: undefined });
  });
});

describe('pause and kill send a signal only to a positive integer PID', () => {
  const values: unknown[] = [-1, 0, 1.5, '4242'];

  for (const action of ['kill', 'pause'] as const) {
    it(`${action} with data.pid in ${JSON.stringify(values)} sends no signal and reports no-target`, async () => {
      const h = makeArp([
        { name: `${action}-on-critical`, condition: { category: 'threat', minSeverity: 'critical' }, action },
      ]);
      const killStub = vi.spyOn(process, 'kill').mockImplementation(() => true);
      try {
        await withHeldTimers(async () => {
          for (const pid of values) {
            await emitCriticalThreat(h.arp.getEngine(), { pid });
          }
        });
        expect(killStub.mock.calls).toEqual([]);
      } finally {
        killStub.mockRestore();
      }
      expect(h.viaArp).toHaveLength(values.length);
      for (const result of h.viaArp) {
        expect(decision(result)).toEqual({ action, success: false, outcome: 'no-target', targetPid: undefined });
      }
    });
  }
});

describe('alert results', () => {
  const alertRule: AlertRule = {
    name: 'alert-on-critical',
    condition: { category: 'threat', minSeverity: 'critical' },
    action: 'alert',
  };

  it('an alert callback that throws makes the alert failed, and the reason names the error', async () => {
    const h = makeArp([alertRule]);
    h.arp.setAlertCallback(() => { throw new Error('pager endpoint unreachable'); });

    await emitCriticalThreat(h.arp.getEngine(), {});

    expect(h.viaArp).toHaveLength(1);
    const [result] = h.viaArp;
    expect(decision(result)).toEqual({ action: 'alert', success: false, outcome: 'failed', targetPid: undefined });
    expect(result.reason).toContain('pager endpoint unreachable');
    const [line] = readEnforcementLog(h.dataDir);
    expect(decision(line)).toEqual(decision(result));
  });

  it('an alert whose callback returns is executed', async () => {
    const h = makeArp([alertRule]);
    const alerted: ARPEvent[] = [];
    h.arp.setAlertCallback((event) => { alerted.push(event); });

    await emitCriticalThreat(h.arp.getEngine(), {});

    expect(alerted).toHaveLength(1);
    expect(decision(h.viaArp[0])).toEqual({ action: 'alert', success: true, outcome: 'executed', targetPid: undefined });
  });

  it('a log action is executed once the result is recorded', async () => {
    const h = makeArp([{ ...alertRule, name: 'log-on-critical', action: 'log' }]);

    await emitCriticalThreat(h.arp.getEngine(), {});

    expect(decision(h.viaArp[0])).toEqual({ action: 'log', success: true, outcome: 'executed', targetPid: undefined });
    const [line] = readEnforcementLog(h.dataDir);
    expect(decision(line)).toEqual(decision(h.viaArp[0]));
  });

  it('a log action whose result cannot be written to enforcement.jsonl is failed', async () => {
    const h = makeArp([{ ...alertRule, name: 'log-on-critical', action: 'log' }]);
    // A directory where the log file belongs makes the append fail.
    mkdirSync(join(h.dataDir, 'enforcement.jsonl'));

    await emitCriticalThreat(h.arp.getEngine(), {});

    expect(h.viaArp).toHaveLength(1);
    expect(decision(h.viaArp[0])).toEqual({ action: 'log', success: false, outcome: 'failed', targetPid: undefined });
    expect(h.viaArp[0].reason).toMatch(/^Failed to record the result: /);
    expect(h.viaEngine[0]).toBe(h.viaArp[0]);
  });

  it('a kill whose result cannot be written to enforcement.jsonl keeps the outcome of the kill', async () => {
    const h = makeArp();
    mkdirSync(join(h.dataDir, 'enforcement.jsonl'));

    await emitCriticalThreat(h.arp.getEngine(), {});

    expect(decision(h.viaArp[0])).toEqual(expectedDecision('no-pid'));
  });
});
