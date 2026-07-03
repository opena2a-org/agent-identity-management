import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import * as os from 'os';
import * as path from 'path';
import * as fs from 'fs';
import { IntelligenceCoordinator } from '../../../src/arp/intelligence/coordinator';
import {
  InProcessBehavioralRiskSource,
  type BehavioralRiskResult,
  type BehavioralRiskScore,
  type BehavioralRiskScoreable,
  type BehavioralRiskSource,
} from '../../../src/arp/intelligence/behavioral-risk';
import type {
  ARPConfig,
  ARPEvent,
  CapabilityManifest,
} from '../../../src/arp/types';

/**
 * Integration coverage for the behavioral risk fusion hook on
 * IntelligenceCoordinator.analyze().
 *
 * The coordinator calls the injected `BehavioralRiskSource` after the
 * L0-comply gate and before L1 statistical. The fusion policy is:
 *   - score >= 0.8 raises category to `threat` and severity to `critical`
 *   - score >= 0.6 raises to `violation` / `high`
 *   - score >= 0.4 raises to `anomaly` / `medium`
 *   - score <  0.4 records only, no mutation
 *   - unavailable results record a code on event.data but never mutate
 *
 * Every test here inspects `event.data.behavioralRisk` rather than
 * scraping logs, so the structured audit record is asserted end-to-end.
 */

function baseConfig(): ARPConfig {
  return {
    agentName: 'test-agent',
    intelligence: { enabled: false, behavioralRiskTimeoutMs: 200 },
  };
}

function makeEvent(): ARPEvent {
  return {
    id: `evt-${Math.random().toString(36).slice(2)}`,
    timestamp: new Date().toISOString(),
    source: 'prompt',
    category: 'normal',
    severity: 'info',
    description: 'fused risk test',
    data: { capability: 'test-capability' },
    classifiedBy: 'L0-rules',
  };
}

function scoreSource(score: BehavioralRiskScore): BehavioralRiskSource {
  const twin: BehavioralRiskScoreable = {
    scoreARPEvent: () => score,
  };
  return new InProcessBehavioralRiskSource(twin, 'test-src');
}

function unavailableSource(
  code:
    | 'NOT_READY'
    | 'TIMEOUT'
    | 'TRANSPORT_ERROR'
    | 'PARSE_ERROR'
    | 'CIRCUIT_OPEN'
    | 'INTERNAL_ERROR',
): BehavioralRiskSource {
  return {
    async getBehavioralRiskSignal(): Promise<BehavioralRiskResult> {
      return { status: 'unavailable', code, reason: `forced ${code}` };
    },
    async close() {},
  };
}

function makeManifest(
  permitted: string[],
  prohibited: string[] = ['credential-access'],
): CapabilityManifest {
  return {
    version: '1.0.0',
    agentId: 'fixture-agent',
    tier: 'execute',
    comply: {
      permitted_classes: permitted,
      prohibited_classes: prohibited,
      on_violation: 'deny',
      // Active enforcement is asserted here; opt in explicitly (default is
      // detection-only, covered by coordinator-detection-mode.test.ts).
      enforce: true,
    },
    issuedAt: '2026-04-14T00:00:00.000Z',
    ed25519PublicKey: 'unused-in-unit-test',
    mldsa65PublicKey: 'unused-in-unit-test',
  };
}

describe('IntelligenceCoordinator behavioral risk fusion', () => {
  let tmpDir: string;

  beforeEach(() => {
    tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'coord-risk-'));
  });

  afterEach(() => {
    fs.rmSync(tmpDir, { recursive: true, force: true });
  });

  it('records a low-score signal without mutating category or severity', async () => {
    const coord = new IntelligenceCoordinator(
      baseConfig(),
      tmpDir,
      null,
      scoreSource({ score: 0.15, action: 'allow', reason: 'normal' }),
    );
    const event = makeEvent();
    await coord.analyze(event);
    expect(event.category).toBe('normal');
    expect(event.severity).toBe('info');
    const rec = event.data.behavioralRisk as {
      status: string;
      score: number;
      band: string;
    };
    expect(rec).toMatchObject({ status: 'ok', band: 'low' });
    expect(rec.score).toBeCloseTo(0.15);
    // classifiedBy should remain L0-rules because the low band does not
    // claim credit for the event.
    expect(event.classifiedBy).toBe('L0-rules');
  });

  it('raises to anomaly/medium on elevated band (0.4 <= score < 0.6)', async () => {
    const coord = new IntelligenceCoordinator(
      baseConfig(),
      tmpDir,
      null,
      scoreSource({ score: 0.45, action: 'alert', reason: 'elevated' }),
    );
    const event = makeEvent();
    await coord.analyze(event);
    expect(event.category).toBe('anomaly');
    expect(event.severity).toBe('medium');
    expect(event.classifiedBy).toBe('L1-behavioral-risk');
    const rec = event.data.behavioralRisk as { band: string };
    expect(rec.band).toBe('elevated');
  });

  it('raises to violation/high on high band (0.6 <= score < 0.8)', async () => {
    const coord = new IntelligenceCoordinator(
      baseConfig(),
      tmpDir,
      null,
      scoreSource({ score: 0.7, action: 'throttle', reason: 'high' }),
    );
    const event = makeEvent();
    await coord.analyze(event);
    expect(event.category).toBe('violation');
    expect(event.severity).toBe('high');
    expect(event.classifiedBy).toBe('L1-behavioral-risk');
    const rec = event.data.behavioralRisk as { band: string };
    expect(rec.band).toBe('high');
  });

  it('raises to threat/critical on critical band (score >= 0.8)', async () => {
    const coord = new IntelligenceCoordinator(
      baseConfig(),
      tmpDir,
      null,
      scoreSource({ score: 0.95, action: 'kill', reason: 'critical' }),
    );
    const event = makeEvent();
    await coord.analyze(event);
    expect(event.category).toBe('threat');
    expect(event.severity).toBe('critical');
    expect(event.classifiedBy).toBe('L1-behavioral-risk');
    const rec = event.data.behavioralRisk as { band: string };
    expect(rec.band).toBe('critical');
  });

  it('records an unavailable signal without mutating severity', async () => {
    const coord = new IntelligenceCoordinator(
      baseConfig(),
      tmpDir,
      null,
      unavailableSource('TIMEOUT'),
    );
    const event = makeEvent();
    await coord.analyze(event);
    expect(event.category).toBe('normal');
    expect(event.severity).toBe('info');
    expect(event.classifiedBy).toBe('L0-rules');
    const rec = event.data.behavioralRisk as { status: string; code: string };
    expect(rec).toEqual({ status: 'unavailable', code: 'TIMEOUT' });
  });

  it('NOT_READY is also recorded and does not mutate the event', async () => {
    const coord = new IntelligenceCoordinator(
      baseConfig(),
      tmpDir,
      null,
      unavailableSource('NOT_READY'),
    );
    const event = makeEvent();
    await coord.analyze(event);
    const rec = event.data.behavioralRisk as { status: string; code: string };
    expect(rec).toEqual({ status: 'unavailable', code: 'NOT_READY' });
    expect(event.severity).toBe('info');
  });

  it('comply deny short-circuits before the risk source is queried', async () => {
    let riskCalled = 0;
    const trackingSource: BehavioralRiskSource = {
      async getBehavioralRiskSignal() {
        riskCalled++;
        return {
          status: 'ok',
          signal: {
            score: 0.95,
            action: 'kill',
            reason: 'should not be reached',
            source: 'tracking',
            computedAtMs: Date.now(),
          },
        };
      },
      async close() {},
    };
    const coord = new IntelligenceCoordinator(
      baseConfig(),
      tmpDir,
      makeManifest(['code-generation']),
      trackingSource,
    );
    const event = makeEvent();
    event.data.classification = 'credential-access'; // prohibited
    await coord.analyze(event);
    expect(riskCalled).toBe(0);
    // Comply decision was written.
    expect(event.classifiedBy).toBe('L0-comply');
    // No behavioralRisk record because the risk source was never called.
    expect(event.data.behavioralRisk).toBeUndefined();
  });

  it('a permitted classification passes the comply gate and still runs risk fusion', async () => {
    const coord = new IntelligenceCoordinator(
      baseConfig(),
      tmpDir,
      makeManifest(['code-generation']),
      scoreSource({ score: 0.5, action: 'alert', reason: 'elevated' }),
    );
    const event = makeEvent();
    event.data.classification = 'code-generation';
    await coord.analyze(event);
    // Risk fusion raised the severity floor on top of the permitted class.
    expect(event.category).toBe('anomaly');
    expect(event.severity).toBe('medium');
    const rec = event.data.behavioralRisk as { band: string };
    expect(rec.band).toBe('elevated');
  });

  it('setBehavioralRiskSource hot-swaps the source', async () => {
    const coord = new IntelligenceCoordinator(
      baseConfig(),
      tmpDir,
      null,
      unavailableSource('NOT_READY'),
    );
    // First call: NOT_READY, no mutation.
    const e1 = makeEvent();
    await coord.analyze(e1);
    expect(e1.severity).toBe('info');
    // Swap in a critical-band source.
    coord.setBehavioralRiskSource(
      scoreSource({ score: 0.9, action: 'kill', reason: 'critical' }),
    );
    const e2 = makeEvent();
    await coord.analyze(e2);
    expect(e2.severity).toBe('critical');
    expect(e2.category).toBe('threat');
    // And unset clears fusion entirely.
    coord.setBehavioralRiskSource(null);
    const e3 = makeEvent();
    await coord.analyze(e3);
    expect(e3.severity).toBe('info');
    expect(e3.data.behavioralRisk).toBeUndefined();
  });

  it('no source attached means the coordinator behaves as before', async () => {
    const coord = new IntelligenceCoordinator(baseConfig(), tmpDir, null, null);
    const event = makeEvent();
    await coord.analyze(event);
    expect(event.data.behavioralRisk).toBeUndefined();
    expect(event.severity).toBe('info');
  });
});
