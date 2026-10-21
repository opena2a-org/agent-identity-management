/**
 * The fleet gradient the runtime twin posts has an L2 norm of 1.0 or less (#835).
 *
 * `addPrivacyNoise` clips the gradient to an L2 norm of 1.0 and then adds
 * Gaussian noise to each of its nine coordinates, which takes the norm well
 * above 1.0. #835 reports that the behavioral-gradient endpoint answers HTTP 400
 * to a gradient whose L2 norm exceeds 1.0 by more than 1e-9, so the twin divides
 * the noised vector by its norm when that norm is above 1.0. Every test reads
 * the vector from the request body the twin passes to fetch().
 */
import { describe, it, expect, beforeEach, afterEach, vi, type Mock } from 'vitest';
import * as fs from 'fs';
import * as os from 'os';
import * as path from 'path';
import { RuntimeTwin } from './runtime-twin';
import { EventEngine } from '../engine/event-engine';
import type { ARPConfig } from '../types';

// The twin reads its baseline and appends its event log under os.homedir().
// As in runtime-twin.log-rotation.test.ts, the function is replaced so that
// nothing here touches the real home directory.
const homeOverride = vi.hoisted(() => ({ dir: undefined as string | undefined }));
vi.mock('os', async (importOriginal) => {
  const actual = await importOriginal<typeof import('os')>();
  const mocked = { ...actual, homedir: () => homeOverride.dir ?? actual.homedir() };
  return { ...mocked, default: mocked };
});

type BehavioralEvent = Parameters<RuntimeTwin['processEvent']>[0];

const GRADIENT_URL = 'https://api.oa2a.org/api/v1/telemetry/behavioral-gradient';
const GRADIENT_DIMS = 9;
/** An L2 norm above 1.0 by more than this is refused, as #835 reports. */
const NORM_TOLERANCE = 1e-9;
/** The variables isOptedOut() reads; cleared for each test, then put back. */
const OPT_OUT_VARS = ['OPENA2A_TELEMETRY_OPTOUT', 'ARP_TELEMETRY_DISABLED', 'OPENA2A_TELEMETRY'];

const engineConfig: ARPConfig = {
  agentName: 'runtime-twin-gradient-norm-test',
  rules: [],
  monitors: {},
  interceptors: {},
};

let home: string;
let savedEnv: Record<string, string | undefined>;
let fetchSpy: Mock<unknown[], Promise<Response>>;
let sequenceNum = 0;

beforeEach(() => {
  home = fs.mkdtempSync(path.join(os.tmpdir(), 'arp-twin-gradient-'));
  homeOverride.dir = home;
  savedEnv = { OPENA2A_HOME: process.env.OPENA2A_HOME };
  for (const name of OPT_OUT_VARS) savedEnv[name] = process.env[name];
  process.env.OPENA2A_HOME = path.join(home, '.opena2a');
  for (const name of OPT_OUT_VARS) delete process.env[name];

  fetchSpy = vi.fn<unknown[], Promise<Response>>(async () => new Response('{}', { status: 200 }));
  vi.stubGlobal('fetch', fetchSpy);
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

/**
 * An event with the same timing, capability and response code as every other
 * one, so the timing, error and novelty dimensions (6 to 8) stay 0 and the
 * gradient before the noise is known: each event type's share of the events.
 */
function behavioralEvent(eventType: BehavioralEvent['eventType'] = 'TOOL_CALL'): BehavioralEvent {
  sequenceNum++;
  return {
    agentId: 'gradient-norm-agent',
    sessionId: 'gradient-norm-session',
    sequenceNum,
    eventType,
    capability: 'db:read',
    toolName: null,
    argHash: 'test-arg-hash',
    timestampDelta: 10,
    wallClock: sequenceNum * 10,
    responseSize: 0,
    responseCode: 0,
    l0Decision: 'allow',
  };
}

/** The `gradientVector` of each POST to the behavioral-gradient endpoint, in order. */
function postedGradients(): unknown[] {
  return fetchSpy.mock.calls
    .filter(call => String(call[0]) === GRADIENT_URL)
    .map(call => JSON.parse(String((call[1] as RequestInit).body)).gradientVector);
}

function l2Norm(vector: number[]): number {
  return Math.sqrt(vector.reduce((sum, g) => sum + g * g, 0));
}

/** Asserts `posted` is nine finite numbers whose L2 norm the endpoint takes, and returns it. */
function expectInsideBound(posted: unknown): number[] {
  expect(Array.isArray(posted)).toBe(true);
  const vector = posted as unknown[];
  expect(vector).toHaveLength(GRADIENT_DIMS);
  expect(vector.filter(g => typeof g !== 'number' || !Number.isFinite(g))).toEqual([]);
  expect(l2Norm(vector as number[])).toBeLessThanOrEqual(1 + NORM_TOLERANCE);
  return vector as number[];
}

describe('the runtime twin posts its fleet gradient at an L2 norm of 1.0 or less (#835)', () => {
  it('after events recorded through the ARP event engine and flushed by shutdown()', async () => {
    const engine = new EventEngine(engineConfig);
    const twin = new RuntimeTwin('engine-flush-agent', { fleetEnabled: true });
    twin.attach(engine);
    for (let i = 0; i < 5; i++) {
      await engine.emit({
        source: 'process',
        category: 'normal',
        severity: 'info',
        description: 'Test event',
        data: { source: 'process', capability: 'db:read' },
      });
    }
    await new Promise(resolve => setTimeout(resolve, 50));
    await twin.shutdown();

    const posted = postedGradients();
    expect(posted).toHaveLength(1);
    expectInsideBound(posted[0]);
  });

  it('from the flush that the 300th event starts', async () => {
    const twin = new RuntimeTwin('interval-flush-agent', { fleetEnabled: true });
    for (let i = 0; i < 300; i++) twin.processEvent(behavioralEvent());
    await new Promise(resolve => setTimeout(resolve, 0));

    const posted = postedGradients();
    expect(posted).toHaveLength(1);
    expectInsideBound(posted[0]);
  });

  it('for each of 200 flushes, each with its own noise', async () => {
    for (let i = 0; i < 200; i++) {
      const twin = new RuntimeTwin(`many-flush-agent-${i}`, { fleetEnabled: true });
      for (let j = 0; j < 3; j++) twin.processEvent(behavioralEvent());
      await twin.shutdown();
    }

    const posted = postedGradients() as number[][];
    expect(posted).toHaveLength(200);
    expect(posted.filter(v => v.length !== GRADIENT_DIMS || !v.every(g => Number.isFinite(g)))).toEqual([]);
    const norms = posted.map(v => l2Norm(v));
    const normsAboveBound = norms.filter(norm => !(norm <= 1 + NORM_TOLERANCE));
    expect(
      normsAboveBound,
      `${normsAboveBound.length} of ${norms.length} posted gradients have an L2 norm above the bound; ` +
        `the norms range from ${Math.min(...norms)} to ${Math.max(...norms)}`,
    ).toEqual([]);
  });

  it('by dividing the noised vector by its norm, which keeps its direction', async () => {
    // Every uniform draw is 0.1, so every coordinate gets the same noise c,
    // and c is positive. Before the noise the gradient is [1, 0, ..., 0], so
    // the noised vector is [1 + c, c, ..., c], of norm above 1.0; divided by
    // that norm it keeps eight equal positive coordinates and a larger first one.
    vi.spyOn(Math, 'random').mockReturnValue(0.1);
    const twin = new RuntimeTwin('direction-agent', { fleetEnabled: true });
    for (let i = 0; i < 3; i++) twin.processEvent(behavioralEvent());
    await twin.shutdown();

    const posted = postedGradients();
    expect(posted).toHaveLength(1);
    const vector = expectInsideBound(posted[0]);
    expect(l2Norm(vector)).toBeCloseTo(1, 12);
    const noiseOnly = vector.slice(1);
    expect(new Set(noiseOnly).size).toBe(1);
    expect(noiseOnly[0]).toBeGreaterThan(0);
    expect(vector[0]).toBeGreaterThan(noiseOnly[0]);
  });

  it('and posts a noised vector already inside the bound as it is', async () => {
    // A draw of 0.25 puts the cosine of the Box-Muller transform at
    // cos(pi / 2), within 1e-16 of 0, so the noise is below 1e-15. Half tool
    // calls and half MCP calls give [0.5, 0, 0.5, 0, ..., 0], of norm 0.707,
    // which must not be scaled up to 1.0.
    vi.spyOn(Math, 'random').mockReturnValue(0.25);
    const twin = new RuntimeTwin('inside-bound-agent', { fleetEnabled: true });
    twin.processEvent(behavioralEvent('TOOL_CALL'));
    twin.processEvent(behavioralEvent('MCP_CALL'));
    await twin.shutdown();

    const posted = postedGradients();
    expect(posted).toHaveLength(1);
    const vector = expectInsideBound(posted[0]);
    const expected = [0.5, 0, 0.5, 0, 0, 0, 0, 0, 0];
    vector.forEach((g, i) => expect(g).toBeCloseTo(expected[i], 12));
    expect(l2Norm(vector)).toBeCloseTo(Math.SQRT1_2, 12);
  });

  it('with finite coordinates when Math.random() returns 0', async () => {
    // The Box-Muller transform takes the logarithm of a uniform draw; a draw
    // of 0 there made the noise infinite.
    vi.spyOn(Math, 'random').mockReturnValue(0);
    const twin = new RuntimeTwin('zero-draw-agent', { fleetEnabled: true });
    for (let i = 0; i < 3; i++) twin.processEvent(behavioralEvent());
    await twin.shutdown();

    const posted = postedGradients();
    expect(posted).toHaveLength(1);
    expectInsideBound(posted[0]);
  });
});
