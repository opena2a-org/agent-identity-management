import { describe, it, expect, beforeAll, afterAll, afterEach, vi, type MockInstance } from 'vitest';
import { mkdtempSync, rmSync, writeFileSync } from 'fs';
import { tmpdir } from 'os';
import { join } from 'path';
import { AgentRuntimeProtection, defaultConfig } from '../../../src/arp';
import type { ARPConfig } from '../../../src/arp/types';

/**
 * The AgentRuntimeProtection constructor checks the shape of the `monitors`,
 * `interceptors` and `aiLayer` blocks before it builds anything, whichever form
 * the config came in.
 *
 * Before this check, a guessed shape was read as "not set":
 * `monitors: { filesystem: false }` and `monitors: { fileSystem: { enabled:
 * false } }` left every monitor running, `aiLayer: { prompt: true }` built no
 * prompt interceptor, and `intervalMs: 'abc'` or `-5` reached setInterval, which
 * polls every 1 ms for both.
 *
 * Lowest accepted `intervalMs`: 1000 ms. Measured on macOS (Apple M4 Max) by
 * calling each monitor's poll() back to back from the built dist/arp/index.js
 * under `/usr/bin/time -p`, which counts the `ps` and `lsof` child processes:
 * 100 ProcessMonitor polls took 2.87 s wall and 2.66 s user+sys, 40
 * NetworkMonitor polls 2.52 s wall and 2.52 s user+sys, against 0.04 s wall and
 * 0.03 s user+sys for the same script with no polls. That is about 28 ms of
 * wall and 26 ms of CPU per process poll and about 62 ms of each per network
 * poll. At 1000 ms a poll takes about 3% (process) and 6% (network) of one core;
 * an interval under one poll's duration runs the polls back to back.
 */
const MIN_INTERVAL_MS = 1000;

let scratch: string;

beforeAll(() => {
  scratch = mkdtempSync(join(tmpdir(), 'arp-config-shape-'));
});

afterAll(() => {
  rmSync(scratch, { recursive: true, force: true });
});

afterEach(() => {
  vi.restoreAllMocks();
});

function spyWarn(): MockInstance<Parameters<typeof console.warn>, void> {
  return vi.spyOn(console, 'warn').mockImplementation(() => undefined);
}

/** A config with intelligence off and its data directory under the scratch directory. */
function cfg(extra: Record<string, unknown>): ARPConfig {
  return {
    agentName: 'config-shape-test',
    dataDir: join(scratch, 'data'),
    intelligence: { enabled: false },
    ...extra,
  } as ARPConfig;
}

/**
 * Construct, and return the error, or a description of what was built when
 * nothing was thrown, so a failure shows the monitors the config left running.
 */
function construct(config: ARPConfig | string): unknown {
  try {
    const arp = new AgentRuntimeProtection(config);
    return { constructed: true, monitors: arp.getStatus().monitors.map((m) => m.type) };
  } catch (err) {
    return err;
  }
}

function expectRefused(config: ARPConfig | string, keyPath: string): string {
  const outcome = construct(config);
  expect(outcome, `constructed instead: ${JSON.stringify(outcome)}`).toBeInstanceOf(TypeError);
  const message = (outcome as TypeError).message;
  expect(message).toContain(keyPath);
  return message;
}

function expectAccepted(config: ARPConfig): string[] {
  const warn = spyWarn();
  const outcome = construct(config);
  expect(outcome).toMatchObject({ constructed: true });
  expect(warn).not.toHaveBeenCalled();
  return (outcome as { monitors: string[] }).monitors;
}

describe('ARP config shape: the reproduced configs', () => {
  it('refuses monitors.filesystem: false, naming the shape it takes', () => {
    const message = expectRefused(cfg({ monitors: { filesystem: false, network: false } }), 'monitors.filesystem');
    expect(message).toContain("boolean 'enabled'");
    expect(message).toContain('{ enabled: false }');
    expect(message).toContain('got false');
  });

  it('refuses monitors.network: false on its own', () => {
    expectRefused(cfg({ monitors: { network: false } }), 'monitors.network');
  });

  it('refuses monitors.fileSystem by name and lists the monitors entries', () => {
    const message = expectRefused(cfg({ monitors: { fileSystem: { enabled: false } } }), 'monitors.fileSystem');
    for (const name of ['process', 'network', 'filesystem', 'skill', 'heartbeat']) {
      expect(message).toContain(name);
    }
  });

  it("refuses monitors.process.intervalMs: 'abc', as the report wrote it (no enabled)", () => {
    const message = expectRefused(cfg({ monitors: { process: { intervalMs: 'abc' } } }), 'monitors.process.intervalMs');
    expect(message).toContain(String(MIN_INTERVAL_MS));
  });

  it('refuses monitors.process.intervalMs: -5, as the report wrote it (no enabled)', () => {
    expectRefused(cfg({ monitors: { process: { intervalMs: -5 } } }), 'monitors.process.intervalMs');
  });

  it("refuses monitors.process.intervalMs: 'abc' beside enabled: true", () => {
    expectRefused(cfg({ monitors: { process: { enabled: true, intervalMs: 'abc' } } }), 'monitors.process.intervalMs');
  });

  it('refuses monitors.process with a valid intervalMs and no enabled', () => {
    const message = expectRefused(cfg({ monitors: { process: { intervalMs: 2000 } } }), 'monitors.process');
    expect(message).toContain("boolean 'enabled'");
  });

  it('refuses aiLayer.prompt: true', () => {
    const message = expectRefused(cfg({ aiLayer: { prompt: true } }), 'aiLayer.prompt');
    expect(message).toContain('{ enabled: false }');
  });

  it('warns once about an unknown top-level key, naming it, and constructs', () => {
    const warn = spyWarn();
    const outcome = construct(cfg({ frobnicate: 1, monitors: { process: { enabled: false } } }));
    expect(outcome).toMatchObject({ constructed: true });
    expect(warn).toHaveBeenCalledTimes(1);
    expect(String(warn.mock.calls[0][0])).toContain('frobnicate');
  });

  it('refuses the file form: a path to an arp.json holding monitors.filesystem: false', () => {
    const file = join(scratch, 'arp.json');
    writeFileSync(
      file,
      JSON.stringify({
        monitors: { filesystem: false },
        dataDir: join(scratch, 'file-data'),
        intelligence: { enabled: false },
      }),
    );
    expectRefused(file, 'monitors.filesystem');
  });
});

describe('ARP config shape: the rest of what is refused', () => {
  it.each([
    ['monitors.process.intervalMs', { monitors: { process: { enabled: true, intervalMs: Number.NaN } } }],
    ['monitors.process.intervalMs', { monitors: { process: { enabled: true, intervalMs: Infinity } } }],
    ['monitors.process.intervalMs', { monitors: { process: { enabled: true, intervalMs: MIN_INTERVAL_MS - 1 } } }],
    ['monitors.network.intervalMs', { monitors: { network: { enabled: true, intervalMs: '10000' } } }],
    ['monitors.network.intervalMs', { monitors: { network: { enabled: true, intervalMs: 1 } } }],
    ['monitors.network.allowedHosts', { monitors: { network: { enabled: true, allowedHosts: 'api.example.com' } } }],
    ['monitors.network.allowedHosts', { monitors: { network: { enabled: true, allowedHosts: ['a.example', 7] } } }],
    ['monitors.filesystem.watchPaths', { monitors: { filesystem: { enabled: true, watchPaths: '/tmp' } } }],
    ['monitors.filesystem.allowedPaths', { monitors: { filesystem: { enabled: true, allowedPaths: [null] } } }],
    ['monitors.heartbeat.maxStaleMs', { monitors: { heartbeat: { enabled: true, maxStaleMs: 0 } } }],
    ['monitors.heartbeat.maxStaleMs', { monitors: { heartbeat: { enabled: true, maxStaleMs: -1 } } }],
    ['monitors.heartbeat.maxStaleMs', { monitors: { heartbeat: { enabled: true, maxStaleMs: '60000' } } }],
    ['monitors.heartbeat.maxStaleMs', { monitors: { heartbeat: { enabled: true, maxStaleMs: Infinity } } }],
    ['monitors.skill', { monitors: { skill: { enabled: 'yes' } } }],
    ['monitors.process', { monitors: { process: {} } }],
    ['monitors.process', { monitors: { process: null } }],
    ['monitors.process', { monitors: { process: [] } }],
    ['monitors', { monitors: false }],
    ['monitors', { monitors: [] }],
    ['interceptors.process', { interceptors: { process: true } }],
    ['interceptors.network.allowedHosts', { interceptors: { network: { enabled: true, allowedHosts: 'x' } } }],
    ['interceptors.filesystem.allowedPaths', { interceptors: { filesystem: { enabled: true, allowedPaths: [1] } } }],
    ['interceptors.prompt', { interceptors: { prompt: { enabled: true } } }],
    ['interceptors', { interceptors: 'all' }],
    ['aiLayer.mcp.allowedTools', { aiLayer: { mcp: { enabled: true, allowedTools: 'read_file' } } }],
    ['aiLayer.a2a.trustedAgents', { aiLayer: { a2a: { enabled: true, trustedAgents: [{ id: 'a' }] } } }],
    ['aiLayer.mcp', { aiLayer: { mcp: { enabled: 1 } } }],
    ['aiLayer.Prompt', { aiLayer: { Prompt: { enabled: true } } }],
    ['aiLayer', { aiLayer: true }],
  ])('refuses %s in %o', (keyPath, extra) => {
    expectRefused(cfg(extra), keyPath);
  });

  it('lists the interceptors entries when refusing an unknown one', () => {
    const message = expectRefused(cfg({ interceptors: { prompt: { enabled: true } } }), 'interceptors.prompt');
    for (const name of ['process', 'network', 'filesystem']) expect(message).toContain(name);
  });

  it('lists the aiLayer entries when refusing an unknown one', () => {
    const message = expectRefused(cfg({ aiLayer: { Prompt: { enabled: true } } }), 'aiLayer.Prompt');
    for (const name of ['prompt', 'mcp', 'a2a']) expect(message).toContain(name);
  });
});

describe('ARP config shape: what is accepted', () => {
  it('accepts defaultConfig() without a throw or a warning', () => {
    const monitors = expectAccepted({ ...defaultConfig(), dataDir: join(scratch, 'default-data'), intelligence: { enabled: false } });
    expect(monitors).toEqual(['process', 'network', 'filesystem']);
  });

  it("accepts the wiring test's baseConfig() without a throw or a warning", () => {
    const monitors = expectAccepted({
      agentName: 'wiring-test-agent',
      dataDir: join(scratch, 'wiring-data'),
      monitors: {
        process: { enabled: false },
        network: { enabled: false },
        filesystem: { enabled: false },
      },
      intelligence: { enabled: false },
    });
    expect(monitors).toEqual([]);
  });

  it("accepts { agentName: 'x', intelligence: { enabled: false } } without a throw or a warning", () => {
    // dataDir is set so the run writes under the scratch directory, not the working directory.
    expectAccepted({ agentName: 'x', dataDir: join(scratch, 'x-data'), intelligence: { enabled: false } });
  });

  it('accepts monitors.filesystem: { enabled: false } and builds no filesystem monitor', () => {
    const monitors = expectAccepted(cfg({ monitors: { filesystem: { enabled: false } } }));
    expect(monitors).toEqual(['process', 'network']);
  });

  it('accepts an intervalMs at the minimum and every documented field in its documented shape', () => {
    expectAccepted(
      cfg({
        monitors: {
          process: { enabled: false, intervalMs: MIN_INTERVAL_MS },
          network: { enabled: false, intervalMs: MIN_INTERVAL_MS, allowedHosts: ['api.example.com'] },
          filesystem: { enabled: false, watchPaths: ['/tmp'], allowedPaths: [] },
          skill: { enabled: false },
          heartbeat: { enabled: false, expectedUrl: 'http://127.0.0.1:1/health', maxStaleMs: 60000 },
        },
        interceptors: {
          process: { enabled: false },
          network: { enabled: false, allowedHosts: [] },
          filesystem: { enabled: false, allowedPaths: ['/tmp'] },
        },
        aiLayer: {
          prompt: { enabled: false },
          mcp: { enabled: false, allowedTools: ['read_file'] },
          a2a: { enabled: false, trustedAgents: ['agent-a'] },
        },
      }),
    );
  });

  it('accepts empty monitors, interceptors and aiLayer blocks', () => {
    expectAccepted(cfg({ monitors: { process: { enabled: false }, network: { enabled: false }, filesystem: { enabled: false } }, interceptors: {}, aiLayer: {} }));
  });
});
