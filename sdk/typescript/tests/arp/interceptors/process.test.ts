import { describe, it, expect, vi, afterEach } from 'vitest';
import { once } from 'events';
import { createRequire } from 'module';
import { ProcessInterceptor } from '../../../src/arp/interceptors/process';
import { ProcessMonitor } from '../../../src/arp/monitors/process';
import type { EventEngine } from '../../../src/arp/engine/event-engine';
import type { ARPEvent } from '../../../src/arp/types';

/**
 * In the CommonJS build, `child_process` is the module object that
 * ProcessInterceptor patches, and a named import such as `spawnSync` is read
 * from that object where it is used. This mock gives the source under test the
 * same binding, so a call that reaches the interceptor's hook in that build
 * reaches it here too.
 */
vi.mock('child_process', async () => {
  const { createRequire: requireFrom } = await import('module');
  return requireFrom(import.meta.url)('child_process');
});

const childProcess: typeof import('child_process') = createRequire(import.meta.url)('child_process');

type EmittedEvent = Omit<ARPEvent, 'id' | 'timestamp' | 'classifiedBy'>;

/** The arguments of the `ps` call the process monitor reads the process table with. */
const LISTING_ARGS = ['-ax', '-o', 'pid=,ppid=,user=,%cpu=,%mem=,command='];
const LISTING_SPAWN = `Intercepted process spawn: ps ${LISTING_ARGS.join(' ')}`;

/** A directory that does not exist: a spawn of a file in it fails with ENOENT and runs nothing. */
const NOWHERE = '/nonexistent-arp-probe-dir';

let events: EmittedEvent[] = [];
let interceptor: ProcessInterceptor | undefined;
let monitor: ProcessMonitor | undefined;

const engine = {
  emit: (event: EmittedEvent) => {
    events.push(event);
    return Promise.resolve(event);
  },
} as unknown as EventEngine;

async function intercept(): Promise<void> {
  interceptor = new ProcessInterceptor(engine);
  await interceptor.start();
}

function summary(): string[][] {
  return events.map((e) => [e.category, e.severity, e.description]);
}

afterEach(async () => {
  await monitor?.stop();
  await interceptor?.stop();
  monitor = undefined;
  interceptor = undefined;
  events = [];
});

describe('ProcessInterceptor matches a listed binary in any letter case and with a version', () => {
  it('reports a spawn of Python, python3.13 and PERL once each as suspicious', async () => {
    await intercept();
    for (const name of ['Python', 'python3.13', 'PERL']) {
      const child = childProcess.spawn(`${NOWHERE}/${name}`, ['-c', '1']);
      await once(child, 'error');
    }
    expect(summary()).toEqual([
      ['violation', 'high', `Intercepted suspicious binary: Python — ${NOWHERE}/Python -c 1`],
      ['violation', 'high', `Intercepted suspicious binary: python3.13 — ${NOWHERE}/python3.13 -c 1`],
      ['violation', 'high', `Intercepted suspicious binary: PERL — ${NOWHERE}/PERL -c 1`],
    ]);
  });

  it('reports an exec of PERL as suspicious', async () => {
    await intercept();
    expect(() => childProcess.execSync(`${NOWHERE}/PERL -e 1`, { stdio: 'ignore' })).toThrow();
    expect(summary()).toEqual([['violation', 'high', `Intercepted suspicious exec: PERL -- ${NOWHERE}/PERL -e 1`]]);
  });

  it('reports a spawn of python3-config as an ordinary spawn', async () => {
    await intercept();
    childProcess.spawnSync(`${NOWHERE}/python3-config`, ['--prefix']);
    expect(summary()).toEqual([['normal', 'info', `Intercepted process spawn: ${NOWHERE}/python3-config --prefix`]]);
  });
});

describe('ProcessInterceptor with the process monitor running', () => {
  it("reports none of the monitor's ps listings, and reports the same ps call made by the agent", async () => {
    await intercept();
    monitor = new ProcessMonitor(engine, 60_000);
    await monitor.start();
    const poll = (monitor as unknown as { poll(): void }).poll.bind(monitor);
    poll();
    poll();
    poll();
    const listingSpawns = () => events.filter((e) => e.description === LISTING_SPAWN);
    expect(listingSpawns()).toEqual([]);

    childProcess.spawnSync('ps', LISTING_ARGS, { encoding: 'utf-8', maxBuffer: 64 * 1024 * 1024 });
    expect(listingSpawns()).toEqual([
      {
        source: 'process',
        category: 'normal',
        severity: 'info',
        description: LISTING_SPAWN,
        data: {
          binary: 'ps',
          command: `ps ${LISTING_ARGS.join(' ')}`,
          args: LISTING_ARGS,
          intercepted: true,
          suspicious: false,
        },
      },
    ]);
  });
});
