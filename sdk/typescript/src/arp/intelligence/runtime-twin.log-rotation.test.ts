/**
 * The runtime twin's home-directory logs are capped (#756).
 *
 * `RuntimeTwin` appends one line per processed event to
 * `~/.opena2a/arp/events.jsonl` and one line per escalation to
 * `~/.opena2a/arp/l1-escalations.jsonl`. Every agent process on the machine
 * shares those two files, so without a cap they grow for as long as any agent
 * with the twin on runs. Each test points the home directory at a fresh
 * scratch directory so nothing here touches the real one.
 */
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import * as fs from 'fs';
import * as os from 'os';
import * as path from 'path';
import { RuntimeTwin } from './runtime-twin';
import { EventEngine } from '../engine/event-engine';
import type { ARPConfig } from '../types';

// The twin resolves its log paths through os.homedir(). A test worker is a
// thread, and assigning process.env.HOME there changes the thread's copy of
// the environment, not the one os.homedir() reads, so the function itself is
// replaced for every module this file loads. Everything else on `os` is real.
const homeOverride = vi.hoisted(() => ({ dir: undefined as string | undefined }));
vi.mock('os', async (importOriginal) => {
  const actual = await importOriginal<typeof import('os')>();
  const mocked = { ...actual, homedir: () => homeOverride.dir ?? actual.homedir() };
  return { ...mocked, default: mocked };
});

/** The cap the changelog states; a change to it is a change to that statement. */
const FIFTY_MIB = 50 * 1024 * 1024;
const ROTATED_COPIES_KEPT = 5;

const engineConfig: ARPConfig = {
  agentName: 'runtime-twin-log-rotation-test',
  rules: [],
  monitors: {},
  interceptors: {},
};

let home: string;

beforeEach(() => {
  home = fs.mkdtempSync(path.join(os.tmpdir(), 'arp-twin-log-'));
  homeOverride.dir = home;
  expect(os.homedir()).toBe(home);
});

afterEach(() => {
  homeOverride.dir = undefined;
  fs.rmSync(home, { recursive: true, force: true });
});

function arpDir(): string {
  return path.join(home, '.opena2a', 'arp');
}

function eventsPath(): string {
  return path.join(arpDir(), 'events.jsonl');
}

function escalationsPath(): string {
  return path.join(arpDir(), 'l1-escalations.jsonl');
}

/**
 * Creates `filePath` holding `bytes` zero bytes. The file is extended with
 * truncate, which the file system can store sparsely, so a 50 MiB file costs
 * no 50 MiB write.
 */
function createFileOfSize(filePath: string, bytes: number): void {
  fs.mkdirSync(path.dirname(filePath), { recursive: true });
  fs.writeFileSync(filePath, '');
  fs.truncateSync(filePath, bytes);
}

/** Names of the rotated copies beside `filePath`, oldest first. */
function rotatedCopies(filePath: string): string[] {
  const prefix = path.basename(filePath) + '.';
  return fs
    .readdirSync(path.dirname(filePath))
    .filter((name) => name.startsWith(prefix))
    .sort();
}

function lines(filePath: string): string[] {
  return fs.readFileSync(filePath, 'utf-8').split('\n').filter(Boolean);
}

function behavioralEvent(agentId: string) {
  return {
    agentId,
    sessionId: 'session',
    sequenceNum: 1,
    eventType: 'TOOL_CALL' as const,
    capability: 'db:read',
    toolName: null,
    argHash: '0000000000000000',
    timestampDelta: 0,
    wallClock: Date.now(),
    responseSize: 0,
    responseCode: 0,
    l0Decision: 'allow' as const,
  };
}

async function waitUntil(condition: () => boolean, timeoutMs = 2000): Promise<void> {
  const deadline = Date.now() + timeoutMs;
  while (!condition()) {
    if (Date.now() > deadline) return;
    await new Promise((resolve) => setTimeout(resolve, 10));
  }
}

describe('RuntimeTwin caps its home-directory logs (#756)', () => {
  it('appends to events.jsonl without rotating it while it is at most 50 MiB', () => {
    createFileOfSize(eventsPath(), FIFTY_MIB);

    new RuntimeTwin('at-the-cap').processEvent(behavioralEvent('at-the-cap'));

    expect(rotatedCopies(eventsPath())).toEqual([]);
    expect(fs.statSync(eventsPath()).size).toBeGreaterThan(FIFTY_MIB);
  });

  it('renames events.jsonl aside and starts a new one once it is above 50 MiB', () => {
    createFileOfSize(eventsPath(), FIFTY_MIB + 1);

    new RuntimeTwin('above-the-cap').processEvent(behavioralEvent('above-the-cap'));

    // The oversized file was renamed aside, not appended to: the live file
    // holds only the new line.
    expect(fs.statSync(eventsPath()).size).toBeLessThan(FIFTY_MIB);
    const copies = rotatedCopies(eventsPath());
    expect(copies).toHaveLength(1);
    expect(fs.statSync(path.join(arpDir(), copies[0])).size).toBe(FIFTY_MIB + 1);
    const live = lines(eventsPath());
    expect(live).toHaveLength(1);
    expect(JSON.parse(live[0]).agentId).toBe('above-the-cap');
  });

  it('keeps the five newest rotated copies of events.jsonl and deletes the older ones', () => {
    // Six copies rotated earlier, named the way the twin names them:
    // <name>.<millisecond timestamp>.<pid>.<8 hex digits>.
    const earlier = [1, 2, 3, 4, 5, 6].map((i) => `events.jsonl.${1700000000000 + i}.1.aaaaaaaa`);
    fs.mkdirSync(arpDir(), { recursive: true });
    for (const name of earlier) fs.writeFileSync(path.join(arpDir(), name), 'old\n');
    createFileOfSize(eventsPath(), FIFTY_MIB + 1);

    new RuntimeTwin('prune').processEvent(behavioralEvent('prune'));

    const copies = rotatedCopies(eventsPath());
    expect(copies).toHaveLength(ROTATED_COPIES_KEPT);
    // The two oldest are gone, the four newest earlier copies stay in order,
    // and the copy just rotated sorts after all of them.
    expect(copies.slice(0, 4)).toEqual(earlier.slice(2));
    expect(copies[4] > earlier[5]).toBe(true);
    expect(fs.statSync(path.join(arpDir(), copies[4])).size).toBe(FIFTY_MIB + 1);
  });

  it('rotates l1-escalations.jsonl the same way, reached through the attached event pipeline', async () => {
    createFileOfSize(escalationsPath(), FIFTY_MIB + 1);
    const engine = new EventEngine(engineConfig);
    const twin = new RuntimeTwin('escalation');
    // A trained baseline that has never seen the capability below: an unknown
    // capability (+0.3) and an L0 block (+0.3) score above the 0.4 line at
    // which the twin logs an escalation.
    twin._setBaselineForTest({
      eventTypeCounts: { TOOL_CALL: 100 },
      avgTimingDelta: 0,
      stdTimingDelta: 0,
      capabilitySet: new Set(['db:read']),
      avgResponseSize: 0,
      totalEvents: 100,
      errorRate: 0,
    });
    twin.attach(engine);

    await engine.emit({
      source: 'process',
      category: 'normal',
      severity: 'info',
      description: 'never seen before',
      data: { source: 'process', capability: 'never-seen', _initialAction: 'kill' },
    });
    // L1 runs on setImmediate after the engine returns.
    await waitUntil(() => rotatedCopies(escalationsPath()).length > 0);

    const copies = rotatedCopies(escalationsPath());
    expect(copies).toHaveLength(1);
    expect(fs.statSync(path.join(arpDir(), copies[0])).size).toBe(FIFTY_MIB + 1);
    const live = lines(escalationsPath());
    expect(live).toHaveLength(1);
    expect(JSON.parse(live[0]).agentId).toBe('escalation');
  });
});
