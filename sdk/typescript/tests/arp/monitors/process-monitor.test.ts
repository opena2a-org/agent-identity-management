import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { execSync, spawnSync } from 'child_process';
import { ProcessMonitor } from '../../../src/arp/monitors/process';
import type { EventEngine } from '../../../src/arp/engine/event-engine';
import type { ARPEvent } from '../../../src/arp/types';

/**
 * The process monitor reads the process table with `ps` and reports the
 * agent's descendants. The `ps` it runs is itself a child of the agent while it
 * prints, so its own row is in every listing it reads; that row is never a
 * child process. Each scripted listing below holds the listing process's own
 * row, under the pid the mocked `spawnSync` reports for it.
 */

vi.mock('child_process', () => ({ execSync: vi.fn(), spawnSync: vi.fn() }));
// The root-user check compares a child's user with the user the agent runs as.
vi.mock('os', async (importOriginal) => ({
  ...(await importOriginal<typeof import('os')>()),
  userInfo: () => ({ username: 'agent' }),
}));

type EmittedEvent = Omit<ARPEvent, 'id' | 'timestamp' | 'classifiedBy'>;

interface Row {
  pid: number;
  ppid: number;
  user?: string;
  cpu?: number;
  mem?: number;
  command: string;
}

/** One read of the process table: its rows, or a `ps` that failed. */
type Listing = { ownPid: number; rows: Row[] } | { fail: true };

const AGENT = process.pid;
const LISTING_COMMAND = 'ps -ax -o pid=,ppid=,user=,%cpu=,%mem=,command=';

/** Processes that are not the agent's descendants; present in every listing. */
const OTHERS: Row[] = [
  { pid: 1, ppid: 0, user: 'root', command: '/sbin/launchd' },
  { pid: 501, ppid: 1, command: 'sleep 30' },
  { pid: 502, ppid: 1, command: 'curl --version' },
];

/** The listing process's own row, as `ps` prints it: a child of the agent. */
function own(pid: number): Row {
  return { pid, ppid: AGENT, user: 'root', command: LISTING_COMMAND };
}

function line(r: Row): string {
  const user = (r.user ?? 'agent').padEnd(16);
  const cpu = (r.cpu ?? 0).toFixed(1).padStart(4);
  const mem = (r.mem ?? 0).toFixed(1).padStart(4);
  return `${String(r.pid).padStart(5)} ${String(r.ppid).padStart(5)} ${user} ${cpu} ${mem} ${r.command}`;
}

function table(l: { ownPid: number; rows: Row[] }): string {
  return [...OTHERS, own(l.ownPid), ...l.rows].map(line).join('\n') + '\n';
}

let listings: Listing[] = [];
let events: EmittedEvent[] = [];
let monitor: ProcessMonitor;

/** The next scripted listing; a read past the end of the script fails the test. */
function nextListing(): Listing {
  const l = listings.shift();
  if (!l) throw new Error('process table read past the end of the script');
  return l;
}

/** Every call that reads the process table, whichever function made it. */
function reads(): number {
  return vi.mocked(spawnSync).mock.calls.length + vi.mocked(execSync).mock.calls.length;
}

function poll(): void {
  (monitor as unknown as { poll(): void }).poll();
}

beforeEach(() => {
  listings = [];
  events = [];
  vi.mocked(spawnSync).mockReset();
  vi.mocked(execSync).mockReset();
  vi.mocked(spawnSync).mockImplementation(((file: string, args?: readonly string[]) => {
    if (file !== 'ps' || !args?.includes('-ax')) {
      return { pid: 0, output: [], stdout: '', stderr: '', status: 1, signal: null };
    }
    const l = nextListing();
    if ('fail' in l) {
      return { pid: 0, output: [], stdout: '', stderr: '', status: null, signal: null, error: new Error('spawnSync ps ETIMEDOUT') };
    }
    return { pid: l.ownPid, output: [], stdout: table(l), stderr: '', status: 0, signal: null };
  }) as unknown as typeof spawnSync);
  vi.mocked(execSync).mockImplementation(((command: string) => {
    // A query for one pid finds nothing: only the whole table is scripted.
    if (!command.includes('-ax')) return '';
    const l = nextListing();
    if ('fail' in l) throw new Error('Command failed: ps');
    return table(l);
  }) as unknown as typeof execSync);

  const engine = {
    emit: (event: EmittedEvent) => {
      events.push(event);
      return Promise.resolve(event);
    },
  } as unknown as EventEngine;
  monitor = new ProcessMonitor(engine, 60_000);
});

afterEach(async () => {
  await monitor.stop();
});

describe('ProcessMonitor never reports the ps listing it runs', () => {
  it('emits no event when two listings differ only in the listing process', async () => {
    listings = [
      { ownPid: 1001, rows: [] },
      { ownPid: 1002, rows: [] },
    ];
    await monitor.start();
    poll();
    expect(events).toEqual([]);
  });

  it('reports a process that copies the listing command line under another pid', async () => {
    listings = [
      { ownPid: 1001, rows: [] },
      { ownPid: 1002, rows: [{ pid: 2001, ppid: AGENT, command: LISTING_COMMAND }] },
    ];
    await monitor.start();
    poll();
    expect(events.map((e) => e.description)).toEqual([`New child process: PID 2001 — ${LISTING_COMMAND}`]);
  });
});

describe('ProcessMonitor spawn and termination events', () => {
  it('emits one spawn event for a child that appears, with its command, user, cpu and mem', async () => {
    listings = [
      { ownPid: 1001, rows: [] },
      { ownPid: 1002, rows: [{ pid: 2001, ppid: AGENT, cpu: 1.5, mem: 0.2, command: 'sleep 30' }] },
    ];
    await monitor.start();
    poll();
    expect(events).toEqual([
      {
        source: 'process',
        category: 'normal',
        severity: 'info',
        description: 'New child process: PID 2001 — sleep 30',
        data: { pid: 2001, command: 'sleep 30', user: 'agent', cpu: 1.5, mem: 0.2 },
      },
    ]);
  });

  it('emits one termination event for an announced child that is gone at the next poll', async () => {
    listings = [
      { ownPid: 1001, rows: [] },
      { ownPid: 1002, rows: [{ pid: 2001, ppid: AGENT, command: 'sleep 30' }] },
      { ownPid: 1003, rows: [] },
    ];
    await monitor.start();
    poll();
    events = [];
    poll();
    expect(events).toEqual([
      {
        source: 'process',
        category: 'normal',
        severity: 'info',
        description: 'Child process terminated: PID 2001',
        data: { pid: 2001, action: 'terminated' },
      },
    ]);
  });

  it('emits one termination and no spawn for a child present at start that is gone at the next poll', async () => {
    listings = [
      { ownPid: 1001, rows: [{ pid: 2001, ppid: AGENT, command: 'sleep 30' }] },
      { ownPid: 1002, rows: [] },
    ];
    await monitor.start();
    poll();
    expect(events.map((e) => e.description)).toEqual(['Child process terminated: PID 2001']);
  });

  it('emits no termination when a read of the process table fails', async () => {
    listings = [
      { ownPid: 1001, rows: [{ pid: 2001, ppid: AGENT, command: 'sleep 30' }] },
      { fail: true },
      { ownPid: 1003, rows: [{ pid: 2001, ppid: AGENT, command: 'sleep 30' }] },
    ];
    await monitor.start();
    poll();
    poll();
    expect(events).toEqual([]);
  });

  it('reads the process table once at start and once per poll', async () => {
    const children: Row[] = [
      { pid: 2001, ppid: AGENT, command: 'sleep 30' },
      { pid: 2002, ppid: AGENT, command: 'sh -c sleep 30' },
      { pid: 2003, ppid: 2002, command: 'sleep 30' },
    ];
    listings = [
      { ownPid: 1001, rows: [] },
      { ownPid: 1002, rows: children },
      { ownPid: 1003, rows: children },
    ];
    await monitor.start();
    poll();
    poll();
    expect(reads()).toBe(3);
    expect(events.map((e) => e.description)).toEqual([
      'New child process: PID 2001 — sleep 30',
      'New child process: PID 2002 — sh -c sleep 30',
      'New child process: PID 2003 — sleep 30',
    ]);
  });
});

describe('ProcessMonitor checks on descendant rows', () => {
  it('reports a suspicious binary, high CPU and a root user on a grandchild', async () => {
    const children: Row[] = [
      { pid: 2001, ppid: AGENT, command: 'sh -c curl --version' },
      { pid: 2002, ppid: 2001, user: 'root', cpu: 95.5, command: 'curl --version' },
    ];
    listings = [
      { ownPid: 1001, rows: [] },
      { ownPid: 1002, rows: children },
    ];
    await monitor.start();
    poll();
    expect(events).toEqual([
      {
        source: 'process',
        category: 'normal',
        severity: 'info',
        description: 'New child process: PID 2001 — sh -c curl --version',
        data: { pid: 2001, command: 'sh -c curl --version', user: 'agent', cpu: 0, mem: 0 },
      },
      {
        source: 'process',
        category: 'normal',
        severity: 'info',
        description: 'New child process: PID 2002 — curl --version',
        data: { pid: 2002, command: 'curl --version', user: 'root', cpu: 95.5, mem: 0 },
      },
      {
        source: 'process',
        category: 'violation',
        severity: 'high',
        description: 'Suspicious binary executed: curl (PID 2002)',
        data: { pid: 2002, binary: 'curl', command: 'curl --version', user: 'root' },
      },
      {
        source: 'process',
        category: 'anomaly',
        severity: 'medium',
        description: 'High CPU usage: PID 2002 at 95.5% — curl --version',
        data: { pid: 2002, cpu: 95.5, command: 'curl --version' },
      },
      {
        source: 'process',
        category: 'violation',
        severity: 'high',
        description: 'Child process running as root: PID 2002 — curl --version',
        data: { pid: 2002, user: 'root', command: 'curl --version' },
      },
    ]);
  });
});

describe('ProcessMonitor reports each check once per process', () => {
  /** How `ps` on macOS lists a child started as Homebrew's python3. */
  const HOMEBREW_PYTHON =
    '/opt/homebrew/Cellar/python@3.14/3.14.3_1/Frameworks/Python.framework/Versions/3.14/Resources/Python.app/Contents/MacOS/Python -c import time; time.sleep(30)';

  function child(command: string, extra: Partial<Row> = {}): Row {
    return { pid: 2001, ppid: AGENT, command, ...extra };
  }

  /** Scripts a start() read with no child and one read per entry of `polls`, then starts the monitor. */
  async function script(polls: Array<Row[] | { fail: true }>): Promise<void> {
    listings = [{ ownPid: 1001, rows: [] }, ...polls.map((p, i) => (Array.isArray(p) ? { ownPid: 1002 + i, rows: p } : p))];
    await monitor.start();
  }

  /** As script(), then polls once for each entry of `polls`. */
  async function pollEach(polls: Array<Row[] | { fail: true }>): Promise<void> {
    await script(polls);
    for (let i = 0; i < polls.length; i++) poll();
  }

  /** The descriptions of the events emitted so far that begin with `prefix`. */
  function described(prefix: string): string[] {
    return events.filter((e) => e.description.startsWith(prefix)).map((e) => e.description);
  }

  it('reports a python child that macOS lists as Python once over three polls', async () => {
    const row = child(HOMEBREW_PYTHON);
    await pollEach([[row], [row], [row]]);
    expect(events.filter((e) => e.category !== 'normal')).toEqual([
      {
        source: 'process',
        category: 'violation',
        severity: 'high',
        description: 'Suspicious binary executed: Python (PID 2001)',
        data: { pid: 2001, binary: 'Python', command: HOMEBREW_PYTHON, user: 'agent' },
      },
    ]);
  });

  it('reports a child run as /usr/bin/python3.12 once over three polls', async () => {
    const row = child('/usr/bin/python3.12 x.py');
    await pollEach([[row], [row], [row]]);
    expect(described('Suspicious binary executed')).toEqual(['Suspicious binary executed: python3.12 (PID 2001)']);
  });

  it('reports a perl child once over three polls, not at each poll', async () => {
    const row = child('/usr/bin/perl -e sleep 30');
    await pollEach([[row], [row], [row]]);
    expect(described('Suspicious binary executed')).toEqual(['Suspicious binary executed: perl (PID 2001)']);
  });

  it('reports a pid whose command changes from sh to curl once, for curl', async () => {
    await pollEach([[child('sh -c curl --version')], [child('curl --version')], [child('curl --version')]]);
    expect(described('Suspicious binary executed')).toEqual(['Suspicious binary executed: curl (PID 2001)']);
  });

  it('reports a pid again when it execs into another listed binary', async () => {
    await pollEach([[child('perl -e 1')], [child('curl --version')], [child('curl --version')]]);
    expect(described('Suspicious binary executed')).toEqual([
      'Suspicious binary executed: perl (PID 2001)',
      'Suspicious binary executed: curl (PID 2001)',
    ]);
  });

  it('reports a reused pid again on each check after the pid left the listing', async () => {
    const row = child('perl -e 1', { user: 'root', cpu: 95 });
    await pollEach([[row], [row], [], [row]]);
    expect(events.filter((e) => e.category !== 'normal').map((e) => e.description)).toEqual([
      'Suspicious binary executed: perl (PID 2001)',
      'High CPU usage: PID 2001 at 95% — perl -e 1',
      'Child process running as root: PID 2001 — perl -e 1',
      'Suspicious binary executed: perl (PID 2001)',
      'High CPU usage: PID 2001 at 95% — perl -e 1',
      'Child process running as root: PID 2001 — perl -e 1',
    ]);
  });

  it('does not report a pid again after a read of the process table fails', async () => {
    const row = child('perl -e 1');
    await pollEach([[row], { fail: true }, [row]]);
    expect(described('Suspicious binary executed')).toEqual(['Suspicious binary executed: perl (PID 2001)']);
  });

  it('reports a child running as root once over three polls', async () => {
    const row = child('sleep 30', { user: 'root' });
    await pollEach([[row], [row], [row]]);
    expect(described('Child process running as root')).toEqual([
      'Child process running as root: PID 2001 — sleep 30',
    ]);
  });

  it('reports high CPU once over three polls above 90%, and again after a poll at 90% or below', async () => {
    await script([95, 97.5, 99, 90, 96].map((cpu) => [child('sleep 30', { cpu })]));
    poll();
    poll();
    poll();
    expect(described('High CPU usage')).toEqual(['High CPU usage: PID 2001 at 95% — sleep 30']);
    poll();
    poll();
    expect(described('High CPU usage')).toEqual([
      'High CPU usage: PID 2001 at 95% — sleep 30',
      'High CPU usage: PID 2001 at 96% — sleep 30',
    ]);
  });

  it('reports nothing but new children for sleep, python3-config, ncdu and sshd', async () => {
    const rows = ['/bin/sleep 5', '/usr/bin/python3-config --prefix', 'ncdu /', '/usr/sbin/sshd -D'].map(
      (command, i): Row => ({ pid: 2001 + i, ppid: AGENT, command }),
    );
    await pollEach([rows, rows, rows]);
    expect(events.filter((e) => e.category !== 'normal')).toEqual([]);
    expect(events).toHaveLength(4);
  });
});
