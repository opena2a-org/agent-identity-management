/**
 * Two follow-ups from a new-user pass of the ARP engine
 * (opena2a-org/arp-guard#178):
 *
 *   - getStatus() tells a monitor from an interceptor of the same type: with
 *     the filesystem interceptor on, the list held two entries
 *     `{ type: 'filesystem' }` that nothing told apart.
 *   - Constructing AgentRuntimeProtection does not create the data
 *     directory; start() creates it, and LocalLogger creates it on its first
 *     write when it is missing.
 */
import { describe, it, expect, afterEach, vi } from 'vitest';
import * as fs from 'fs';
import * as os from 'os';
import * as path from 'path';
import { AgentRuntimeProtection, LocalLogger } from '../../src/arp';
import type { ARPEvent } from '../../src/arp/types';

// The runtime twin reads under os.homedir(). A test worker is a thread, where
// assigning process.env.HOME does not change what os.homedir() reads, so the
// function itself is replaced for every module this file loads.
const homeOverride = vi.hoisted(() => ({ dir: undefined as string | undefined }));
vi.mock('os', async (importOriginal) => {
  const actual = await importOriginal<typeof import('os')>();
  const mocked = { ...actual, homedir: () => homeOverride.dir ?? actual.homedir() };
  return { ...mocked, default: mocked };
});

const scratchDirs: string[] = [];

/** Returns `dir`, which is removed after the test. */
function scratch(dir: string): string {
  scratchDirs.push(dir);
  return dir;
}

afterEach(() => {
  vi.restoreAllMocks();
  homeOverride.dir = undefined;
  for (const dir of scratchDirs.splice(0)) fs.rmSync(dir, { recursive: true, force: true });
});

describe('getStatus() names the kind of each monitor', () => {
  it('tells the filesystem monitor from the filesystem interceptor', () => {
    const dir = scratch(fs.mkdtempSync(path.join(os.tmpdir(), 'arp-status-kind-')));
    const arp = new AgentRuntimeProtection({
      agentName: 'status-kind',
      dataDir: path.join(dir, 'arp'),
      intelligence: { enabled: false },
      monitors: { process: { enabled: false }, network: { enabled: false }, filesystem: { enabled: true, watchPaths: [dir] } },
      interceptors: { filesystem: { enabled: true } },
    });

    expect(arp.getStatus().monitors).toEqual([
      { type: 'filesystem', kind: 'monitor', running: false },
      { type: 'filesystem', kind: 'interceptor', running: false },
    ]);
  });

  it('labels every monitor and interceptor the config can turn on', () => {
    const dir = scratch(fs.mkdtempSync(path.join(os.tmpdir(), 'arp-status-kind-')));
    const arp = new AgentRuntimeProtection({
      agentName: 'status-kind',
      dataDir: path.join(dir, 'arp'),
      intelligence: { enabled: false },
      monitors: { process: { enabled: true }, network: { enabled: true }, filesystem: { enabled: true, watchPaths: [dir] } },
      interceptors: { process: { enabled: true }, network: { enabled: true }, filesystem: { enabled: true } },
      aiLayer: { prompt: { enabled: true }, mcp: { enabled: true }, a2a: { enabled: true } },
    });

    expect(arp.getStatus().monitors.map((m) => `${m.kind} ${m.type}`)).toEqual([
      'monitor process',
      'monitor network',
      'monitor filesystem',
      'interceptor process',
      'interceptor network',
      'interceptor filesystem',
      'interceptor prompt',
      'interceptor mcp-protocol',
      'interceptor a2a-protocol',
    ]);
  });
});

describe('the data directory is created by start(), not by the constructor', () => {
  it('constructing creates nothing in the working directory or the home directory', () => {
    const cwd = scratch(fs.mkdtempSync(path.join(os.tmpdir(), 'arp-data-dir-cwd-')));
    const home = scratch(fs.mkdtempSync(path.join(os.tmpdir(), 'arp-data-dir-home-')));
    vi.spyOn(process, 'cwd').mockReturnValue(cwd);
    homeOverride.dir = home;

    // The default monitors and data directory, with the runtime twin off and on.
    new AgentRuntimeProtection({ agentName: 'data-dir', intelligence: { enabled: false } });
    const withTwin = new AgentRuntimeProtection({ agentName: 'data-dir' });
    expect(withTwin.getRuntimeTwin()).not.toBeNull();

    expect(fs.readdirSync(cwd)).toEqual([]);
    expect(fs.readdirSync(home)).toEqual([]);
  });

  it('start() creates the default data directory under the working directory', async () => {
    const cwd = scratch(fs.mkdtempSync(path.join(os.tmpdir(), 'arp-data-dir-cwd-')));
    vi.spyOn(process, 'cwd').mockReturnValue(cwd);
    const arp = new AgentRuntimeProtection({
      agentName: 'data-dir',
      intelligence: { enabled: false },
      monitors: { process: { enabled: false }, network: { enabled: false }, filesystem: { enabled: false } },
    });
    expect(fs.existsSync(path.join(cwd, '.opena2a'))).toBe(false);

    await arp.start();
    try {
      expect(fs.statSync(path.join(cwd, '.opena2a', 'arp')).isDirectory()).toBe(true);
    } finally {
      await arp.stop();
    }
  });

  it('LocalLogger creates its directory, and any missing parent, on its first write', () => {
    const base = scratch(fs.mkdtempSync(path.join(os.tmpdir(), 'arp-data-dir-')));
    const parent = path.join(base, 'nested');
    const dir = path.join(parent, 'arp');
    const logger = new LocalLogger(dir);
    expect(fs.existsSync(parent)).toBe(false);
    expect(logger.readEvents()).toEqual([]);

    const event: ARPEvent = {
      id: 'event-1',
      timestamp: new Date().toISOString(),
      source: 'prompt',
      category: 'threat',
      severity: 'high',
      description: 'an event written by the test',
      data: {},
      classifiedBy: 'L0-rules',
    };
    logger.logEvent(event);

    expect(logger.readEvents()).toEqual([event]);
  });
});
