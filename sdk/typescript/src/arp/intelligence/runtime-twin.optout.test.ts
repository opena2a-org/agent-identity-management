import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { mkdtempSync, rmSync } from 'fs';
import { tmpdir } from 'os';
import { join } from 'path';
import { RuntimeTwin } from './runtime-twin';
import { EventEngine } from '../engine/event-engine';
import { writeOptOutMarker } from '../telemetry/signature/config';
import type { ARPEvent, ARPConfig } from '../types';

/**
 * The fleet-gradient channel and the master opt-out.
 *
 * `arp telemetry opt-out`, the first-run disclosure and the TypeScript README all
 * tell the operator that opting out disables every telemetry channel this module
 * produces. The fleet gradient is one of those channels: it POSTs to
 * api.oa2a.org from inside the customer's process. Before this file, nothing
 * asserted the opt-out reached it, and it did not.
 *
 * The first test is the positive control and it matters more than the rest: a
 * suite that only proves "nothing was sent" passes just as well against a
 * channel that never sends at all, which would make every other assertion here
 * vacuous.
 */

const testConfig: ARPConfig = { rules: [], monitors: [], interceptors: [] };

const GRADIENT_URL = 'https://api.oa2a.org/api/v1/telemetry/behavioral-gradient';

let home: string;
let fetchSpy: ReturnType<typeof vi.fn>;

beforeEach(() => {
  home = mkdtempSync(join(tmpdir(), 'arp-twin-optout-'));
  process.env.OPENA2A_HOME = home;
  delete process.env.OPENA2A_TELEMETRY_OPTOUT;
  delete process.env.ARP_TELEMETRY_DISABLED;

  fetchSpy = vi.fn(async () => new Response('{}', { status: 200 }));
  vi.stubGlobal('fetch', fetchSpy);
});

afterEach(() => {
  vi.unstubAllGlobals();
  delete process.env.OPENA2A_HOME;
  delete process.env.OPENA2A_TELEMETRY_OPTOUT;
  delete process.env.ARP_TELEMETRY_DISABLED;
  rmSync(home, { recursive: true, force: true });
});

function emitTestEvent(engine: EventEngine): Promise<ARPEvent> {
  return engine.emit({
    source: 'test-monitor',
    category: 'normal',
    severity: 'info',
    description: 'Test event',
    data: { source: 'process', capability: 'db:read' },
  });
}

/** Feed the twin real events, then force the flush that shutdown() performs. */
async function runAndFlush(twin: RuntimeTwin, engine: EventEngine, events = 5) {
  twin.attach(engine);
  for (let i = 0; i < events; i++) await emitTestEvent(engine);
  await new Promise(r => setTimeout(r, 50));
  await twin.shutdown();
  await new Promise(r => setTimeout(r, 20));
}

function gradientPosts() {
  return fetchSpy.mock.calls.filter(c => String(c[0]) === GRADIENT_URL);
}

describe('fleet gradients honor the master opt-out', () => {
  it('POSITIVE CONTROL: transmits when fleet is on and nobody has opted out', async () => {
    const engine = new EventEngine(testConfig);
    const twin = new RuntimeTwin('control-agent', { fleetEnabled: true });

    await runAndFlush(twin, engine);

    // If this ever goes to 0, every assertion below is proving nothing.
    expect(gradientPosts().length).toBeGreaterThan(0);
  });

  it('sends nothing when OPENA2A_TELEMETRY_OPTOUT is set', async () => {
    process.env.OPENA2A_TELEMETRY_OPTOUT = '1';
    const engine = new EventEngine(testConfig);
    const twin = new RuntimeTwin('env-optout-agent', { fleetEnabled: true });

    await runAndFlush(twin, engine);

    expect(gradientPosts()).toHaveLength(0);
  });

  it('sends nothing when ARP_TELEMETRY_DISABLED is set', async () => {
    process.env.ARP_TELEMETRY_DISABLED = 'true';
    const engine = new EventEngine(testConfig);
    const twin = new RuntimeTwin('env2-optout-agent', { fleetEnabled: true });

    await runAndFlush(twin, engine);

    expect(gradientPosts()).toHaveLength(0);
  });

  it('sends nothing when the opt-out marker file exists', async () => {
    writeOptOutMarker();
    const engine = new EventEngine(testConfig);
    const twin = new RuntimeTwin('marker-optout-agent', { fleetEnabled: true });

    await runAndFlush(twin, engine);

    expect(gradientPosts()).toHaveLength(0);
  });

  it('honors an opt-out written AFTER the twin was constructed', async () => {
    // Pins the pre-send re-check specifically. The constructor gate cannot
    // catch this case: fleet was legitimately enabled when the process started
    // and the refusal arrived while it was running.
    const engine = new EventEngine(testConfig);
    const twin = new RuntimeTwin('late-optout-agent', { fleetEnabled: true });

    twin.attach(engine);
    for (let i = 0; i < 5; i++) await emitTestEvent(engine);
    await new Promise(r => setTimeout(r, 50));

    writeOptOutMarker();

    await twin.shutdown();
    await new Promise(r => setTimeout(r, 20));

    expect(gradientPosts()).toHaveLength(0);
  });

  it('stays off when fleet was never enabled, opt-out or not', async () => {
    const engine = new EventEngine(testConfig);
    const twin = new RuntimeTwin('default-agent');

    await runAndFlush(twin, engine);

    expect(gradientPosts()).toHaveLength(0);
  });
});
