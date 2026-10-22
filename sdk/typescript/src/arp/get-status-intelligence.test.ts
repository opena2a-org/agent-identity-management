/**
 * `AgentRuntimeProtection.getStatus().intelligence` says whether L2 and the
 * runtime twin run (#857).
 *
 * The block is read from the objects the instance built: the L2 decision the
 * coordinator stored as codes when it was built, and the twin's own send check.
 * These tests hold it to what those objects do: `describeL2Status`, which the CLI
 * status line prints, for L2, and the posts a flush makes for the fleet gradient.
 * The marker tests plant values in the config and assert that none of them
 * reaches `JSON.stringify(getStatus())`, with the config holding each marker as
 * the control.
 */
import { describe, it, expect, beforeEach, afterEach, vi, type Mock, type MockInstance } from 'vitest';
import * as fs from 'fs';
import * as os from 'os';
import * as path from 'path';
import { AgentRuntimeProtection } from './index';
import { describeL2Status } from './intelligence/coordinator';
import { writeOptOutMarker } from './telemetry/signature/config';
import type { IntelligenceConfig, LLMAdapterType } from './types';

// The twin reads its baseline and appends its event log under os.homedir().
// As in runtime-twin.gradient-norm.test.ts, the function is replaced so that
// nothing here touches the real home directory.
const homeOverride = vi.hoisted(() => ({ dir: undefined as string | undefined }));
vi.mock('os', async (importOriginal) => {
  const actual = await importOriginal<typeof import('os')>();
  const mocked = { ...actual, homedir: () => homeOverride.dir ?? actual.homedir() };
  return { ...mocked, default: mocked };
});

const GRADIENT_URL = 'https://api.oa2a.org/api/v1/telemetry/behavioral-gradient';
/** The variables isOptedOut() reads; cleared for each test, then put back. */
const OPT_OUT_VARS = ['OPENA2A_TELEMETRY_OPTOUT', 'ARP_TELEMETRY_DISABLED', 'OPENA2A_TELEMETRY'];

let home: string;
let savedEnv: Record<string, string | undefined>;
let fetchSpy: Mock<unknown[], Promise<Response>>;
let warnSpy: MockInstance<Parameters<typeof console.warn>, void>;

beforeEach(() => {
  home = fs.mkdtempSync(path.join(os.tmpdir(), 'arp-status-intelligence-'));
  homeOverride.dir = home;
  savedEnv = { OPENA2A_HOME: process.env.OPENA2A_HOME };
  for (const name of OPT_OUT_VARS) savedEnv[name] = process.env[name];
  process.env.OPENA2A_HOME = path.join(home, '.opena2a');
  for (const name of OPT_OUT_VARS) delete process.env[name];

  fetchSpy = vi.fn<unknown[], Promise<Response>>(async () => new Response('{}', { status: 200 }));
  vi.stubGlobal('fetch', fetchSpy);
  // The coordinator warns once when L2 was switched on and does not run.
  warnSpy = vi.spyOn(console, 'warn').mockImplementation(() => {});
});

afterEach(() => {
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
  for (const [name, value] of Object.entries(savedEnv)) {
    if (value === undefined) delete process.env[name];
    else process.env[name] = value;
  }
  homeOverride.dir = undefined;
  fs.rmSync(home, { recursive: true, force: true });
});

/** An ARP with no monitors, so start() starts none, writing its files under the test home. */
function arpWith(intelligence: IntelligenceConfig | undefined): AgentRuntimeProtection {
  return new AgentRuntimeProtection({
    agentName: 'status-intelligence-test',
    dataDir: path.join(home, 'data'),
    rules: [],
    monitors: { process: { enabled: false }, network: { enabled: false }, filesystem: { enabled: false } },
    intelligence,
  });
}

function gradientPosts(): number {
  return fetchSpy.mock.calls.filter((c) => String(c[0]) === GRADIENT_URL).length;
}

/**
 * Start the ARP, pass five events through its engine, run `beforeFlush`, then
 * make the flush the twin's shutdown() makes, and stop the ARP.
 */
async function flushTwin(arp: AgentRuntimeProtection, beforeFlush?: () => void): Promise<void> {
  await arp.start();
  for (let i = 0; i < 5; i++) {
    await arp.getEngine().emit({
      source: 'process',
      category: 'normal',
      severity: 'info',
      description: 'status test event',
      data: { source: 'process', capability: 'db:read' },
    });
  }
  // The twin takes each event in a setImmediate callback.
  await new Promise((r) => setTimeout(r, 50));
  beforeFlush?.();
  await arp.getRuntimeTwin()?.shutdown();
  await arp.stop();
}

describe('getStatus().intelligence reports L2 as codes', () => {
  it('reports L2 and the twin off when intelligence.enabled is false', () => {
    expect(arpWith({ enabled: false }).getStatus().intelligence).toEqual({
      l2: { running: false, reason: 'not-enabled' },
      twin: { running: false, fleetGradient: false },
    });
  });

  it('reports why L2 is not running as not-enabled, no-adapter or adapter-failed', () => {
    expect(arpWith(undefined).getStatus().intelligence).toEqual({
      l2: { running: false, reason: 'not-enabled' },
      twin: { running: true, fleetGradient: false },
    });
    expect(arpWith({ adapter: 'ollama' }).getStatus().intelligence.l2).toEqual({
      running: false,
      reason: 'not-enabled',
    });
    expect(arpWith({ enabled: true }).getStatus().intelligence.l2).toEqual({
      running: false,
      reason: 'no-adapter',
    });
    for (const adapter of ['agent-proxy', 'custom'] as const) {
      expect(arpWith({ enabled: true, adapter }).getStatus().intelligence.l2, adapter).toEqual({
        running: false,
        adapter: 'other',
        reason: 'adapter-failed',
      });
    }
  });

  it('reports the kind of the adapter L2 built', () => {
    for (const kind of ['ollama', 'anthropic', 'openai'] as const) {
      expect(arpWith({ enabled: true, adapter: kind }).getStatus().intelligence, kind).toEqual({
        l2: { running: true, adapter: kind },
        twin: { running: true, fleetGradient: false },
      });
    }
  });

  it('agrees with describeL2Status, which the CLI status line prints, on whether L2 runs', () => {
    const configs: Array<IntelligenceConfig | undefined> = [
      undefined,
      {},
      { enabled: false },
      { adapter: 'ollama' },
      { enabled: false, adapter: 'ollama' },
      { enabled: true },
      { enabled: true, adapter: 'agent-proxy' },
      { enabled: true, adapter: 'custom' },
      { enabled: true, adapter: 'ollama' },
      { enabled: true, adapter: 'anthropic' },
      { enabled: true, adapter: 'openai' },
    ];
    const seen = new Set<boolean>();
    for (const intelligence of configs) {
      const running = arpWith(intelligence).getStatus().intelligence.l2.running;
      expect(running, JSON.stringify(intelligence)).toBe(describeL2Status(intelligence).running);
      seen.add(running);
    }
    // CONTROL: the table holds configs on both sides, so the agreement above is
    // not two answers that are always false.
    expect([...seen].sort()).toEqual([false, true]);
  });

  it('says the same before and after start(), and hands the caller a copy', async () => {
    const arp = arpWith({ enabled: true, adapter: 'ollama' });
    const before = arp.getStatus();
    expect(before.running).toBe(false);

    await arp.start();
    expect(arp.getStatus().running).toBe(true);
    expect(arp.getStatus().intelligence).toEqual(before.intelligence);
    await arp.stop();

    // Changing the block a caller was given does not change the next one.
    (before.intelligence.l2 as { running: boolean }).running = false;
    before.intelligence.twin.running = false;
    expect(arp.getStatus().intelligence).toEqual({
      l2: { running: true, adapter: 'ollama' },
      twin: { running: true, fleetGradient: false },
    });
  });
});

describe('getStatus() carries no value from adapterConfig or the adapter name', () => {
  const HOST_MARKER = 'status-marker-host-857';
  const MODEL_MARKER = 'status-marker-model-857';
  const NAME_MARKER = 'status-marker-adapter-857';
  const adapterConfig = { host: `http://${HOST_MARKER}.invalid:11434`, model: MODEL_MARKER };

  it('L2 running: the host and the model in adapterConfig do not appear', () => {
    const intelligence: IntelligenceConfig = { enabled: true, adapter: 'ollama', adapterConfig };
    const arp = arpWith(intelligence);

    // CONTROL: the markers are in the config the instance was built from, and
    // the adapter built from it is the one L2 runs.
    expect(JSON.stringify(intelligence)).toContain(HOST_MARKER);
    expect(JSON.stringify(intelligence)).toContain(MODEL_MARKER);
    expect(arp.getStatus().intelligence.l2).toEqual({ running: true, adapter: 'ollama' });

    const status = JSON.stringify(arp.getStatus());
    expect(status).not.toContain(HOST_MARKER);
    expect(status).not.toContain(MODEL_MARKER);
  });

  it('an unknown adapter name: the name, the error that quotes it and adapterConfig do not appear', () => {
    const intelligence: IntelligenceConfig = {
      enabled: true,
      adapter: NAME_MARKER as LLMAdapterType,
      adapterConfig,
    };
    const arp = arpWith(intelligence);

    // CONTROL: the markers are in the config, and the error the adapter could not
    // be built with quotes the name: the coordinator's startup warning prints it.
    for (const marker of [HOST_MARKER, MODEL_MARKER, NAME_MARKER]) {
      expect(JSON.stringify(intelligence)).toContain(marker);
    }
    expect(warnSpy.mock.calls.flat().join('\n')).toContain(`Unknown LLM adapter type: ${NAME_MARKER}`);
    expect(arp.getStatus().intelligence.l2).toEqual({
      running: false,
      adapter: 'other',
      reason: 'adapter-failed',
    });

    const status = JSON.stringify(arp.getStatus());
    for (const marker of [HOST_MARKER, MODEL_MARKER, NAME_MARKER]) {
      expect(status, marker).not.toContain(marker);
    }
  });
});

describe('getStatus().intelligence.twin.fleetGradient is the check the send path makes', () => {
  it('POSITIVE CONTROL: true with fleetEnabled and no opt-out, and the flush posts the gradient', async () => {
    const arp = arpWith({ runtimeTwin: { fleetEnabled: true } });
    expect(arp.getStatus().intelligence.twin).toEqual({ running: true, fleetGradient: true });

    await flushTwin(arp);

    // If this is 0, the posts the tests below do not see prove nothing.
    expect(gradientPosts()).toBe(1);
  });

  it('false once the opt-out marker is written after the twin was built, and the flush posts nothing', async () => {
    const arp = arpWith({ runtimeTwin: { fleetEnabled: true } });
    expect(arp.getStatus().intelligence.twin.fleetGradient).toBe(true);

    await flushTwin(arp, () => {
      writeOptOutMarker();
      expect(arp.getStatus().intelligence.twin.fleetGradient).toBe(false);
    });

    expect(gradientPosts()).toBe(0);
  });

  it('false when OPENA2A_TELEMETRY_OPTOUT is set as the twin is built, still false once it is cleared, and the flush posts nothing', async () => {
    process.env.OPENA2A_TELEMETRY_OPTOUT = '1';
    const arp = arpWith({ runtimeTwin: { fleetEnabled: true } });
    expect(arp.getStatus().intelligence.twin).toEqual({ running: true, fleetGradient: false });

    // The twin settled fleet gradients off when it was built, so clearing the
    // variable does not turn the channel on. A status worked out from the config
    // and the opt-out check alone would say true here.
    delete process.env.OPENA2A_TELEMETRY_OPTOUT;
    expect(arp.getStatus().intelligence.twin.fleetGradient).toBe(false);

    await flushTwin(arp);
    expect(gradientPosts()).toBe(0);
  });

  it('false when fleetEnabled is not set, and the flush posts nothing', async () => {
    const arp = arpWith(undefined);
    expect(arp.getStatus().intelligence.twin).toEqual({ running: true, fleetGradient: false });

    await flushTwin(arp);
    expect(gradientPosts()).toBe(0);
  });
});
