import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import * as os from 'os';
import * as path from 'path';
import * as fs from 'fs';
import { IntelligenceCoordinator } from '../../../src/arp/intelligence/coordinator';
import {
  GuardAnomalyDetector,
  type GuardAnomalySource,
  type GuardAnomalyStatus,
} from '../../../src/arp/intelligence/guard-anomaly';
import type {
  ARPConfig,
  ARPEvent,
  CapabilityManifest,
} from '../../../src/arp/types';

/**
 * Integration coverage for the guard anomaly fusion hook on
 * IntelligenceCoordinator.analyze().
 *
 * The coordinator runs the guard anomaly detector FIRST (before the
 * L0-comply gate), so the drift window observes every classified
 * event including those the comply gate will reject. Fusion policy:
 *   - drift      : raise category to `anomaly`, severity to `medium`
 *   - normal     : record only, no mutation
 *   - baseline-pending : record only, no mutation
 *
 * Classification-less events are passed through without touching the
 * detector so unclassified traffic cannot dilute the drift window.
 */

function baseConfig(): ARPConfig {
  return {
    agentName: 'test-agent',
    intelligence: { enabled: false },
  };
}

function makeEvent(overrides: Partial<ARPEvent> = {}): ARPEvent {
  return {
    id: `evt-${Math.random().toString(36).slice(2)}`,
    timestamp: new Date().toISOString(),
    source: 'prompt',
    category: 'normal',
    severity: 'info',
    description: 'guard anomaly fixture',
    data: { classification: 'file-read' },
    classifiedBy: 'L0-rules',
    ...overrides,
  };
}

function makeManifest(
  permitted: string[],
  prohibited: string[] = [],
): CapabilityManifest {
  return {
    version: '1.0.0',
    agentId: 'fixture-agent',
    tier: 'execute',
    comply: {
      permitted_classes: permitted,
      prohibited_classes: prohibited,
      on_violation: 'deny',
      // These cases assert active comply enforcement; opt in explicitly (the
      // default is detection-only, covered by coordinator-detection-mode.test.ts).
      enforce: true,
    },
    issuedAt: '2026-04-14T00:00:00.000Z',
    ed25519PublicKey: 'unused-in-unit-test',
    mldsa65PublicKey: 'unused-in-unit-test',
  };
}

/**
 * Stub source that produces a fixed status for every record call.
 * Used to drive coordinator fusion without building a whole
 * detector instance for every test case.
 */
function fixedSource(status: GuardAnomalyStatus): GuardAnomalySource & { calls: number } {
  const s = {
    calls: 0,
    record(_classification: string): GuardAnomalyStatus {
      s.calls++;
      return status;
    },
  };
  return s;
}

describe('IntelligenceCoordinator guard anomaly fusion', () => {
  let tmpDir: string;

  beforeEach(() => {
    tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'coord-guard-anomaly-'));
  });

  afterEach(() => {
    fs.rmSync(tmpDir, { recursive: true, force: true });
  });

  it('records a baseline-pending status without mutating category or severity', async () => {
    const src = fixedSource({
      status: 'baseline-pending',
      observed: 5,
      required: 30,
      reason: 'insufficient-observations',
    });
    const coord = new IntelligenceCoordinator(
      baseConfig(),
      tmpDir,
      null,
      null,
      src,
    );
    const event = makeEvent();
    await coord.analyze(event);
    expect(event.category).toBe('normal');
    expect(event.severity).toBe('info');
    expect(event.classifiedBy).toBe('L0-rules');
    expect(src.calls).toBe(1);
    const rec = event.data.guardAnomaly as GuardAnomalyStatus;
    expect(rec.status).toBe('baseline-pending');
  });

  it('records a normal status without mutating category or severity', async () => {
    const src = fixedSource({
      status: 'normal',
      statistic: 1.2,
      threshold: 21.666,
      windowSize: 100,
      source: 'guard-anomaly',
    });
    const coord = new IntelligenceCoordinator(
      baseConfig(),
      tmpDir,
      null,
      null,
      src,
    );
    const event = makeEvent();
    await coord.analyze(event);
    expect(event.category).toBe('normal');
    expect(event.severity).toBe('info');
    expect(event.classifiedBy).toBe('L0-rules');
    const rec = event.data.guardAnomaly as GuardAnomalyStatus;
    expect(rec.status).toBe('normal');
    if (rec.status === 'normal') {
      expect(rec.statistic).toBeCloseTo(1.2);
    }
  });

  it('raises severity to anomaly/medium on a drift status', async () => {
    const src = fixedSource({
      status: 'drift',
      statistic: 42,
      threshold: 21.666,
      topDeviations: [
        { className: 'credential-access', observed: 30, expected: 5, deviation: 25 },
      ],
      windowSize: 100,
      source: 'guard-anomaly',
    });
    const coord = new IntelligenceCoordinator(
      baseConfig(),
      tmpDir,
      null,
      null,
      src,
    );
    const event = makeEvent();
    await coord.analyze(event);
    expect(event.category).toBe('anomaly');
    expect(event.severity).toBe('medium');
    expect(event.classifiedBy).toBe('L1-guard-anomaly');
    const rec = event.data.guardAnomaly as GuardAnomalyStatus;
    expect(rec.status).toBe('drift');
  });

  it('skips events that carry no classification without touching the detector', async () => {
    const src = fixedSource({
      status: 'drift',
      statistic: 99,
      threshold: 21.666,
      topDeviations: [],
      windowSize: 100,
      source: 'guard-anomaly',
    });
    const coord = new IntelligenceCoordinator(
      baseConfig(),
      tmpDir,
      null,
      null,
      src,
    );
    const event = makeEvent({ data: {} });
    await coord.analyze(event);
    expect(src.calls).toBe(0);
    expect(event.data.guardAnomaly).toBeUndefined();
    expect(event.category).toBe('normal');
  });

  it('records drift BEFORE the comply gate so denied events still feed the detector', async () => {
    // Drift source that ALSO asserts it was called exactly once even
    // though the comply gate denies the event below.
    const src = fixedSource({
      status: 'drift',
      statistic: 50,
      threshold: 21.666,
      topDeviations: [
        { className: 'credential-access', observed: 30, expected: 2, deviation: 28 },
      ],
      windowSize: 100,
      source: 'guard-anomaly',
    });
    const manifest = makeManifest(['file-read'], ['credential-access']);
    const coord = new IntelligenceCoordinator(
      baseConfig(),
      tmpDir,
      manifest,
      null,
      src,
    );
    const event = makeEvent({
      data: { classification: 'credential-access' },
    });
    await coord.analyze(event);
    // Drift detector saw the event:
    expect(src.calls).toBe(1);
    const rec = event.data.guardAnomaly as GuardAnomalyStatus;
    expect(rec.status).toBe('drift');
    // Comply gate ran after and denied:
    expect(event.classifiedBy).toBe('L0-comply');
    expect(event.category).toBe('threat');
    expect(event.severity).toBe('critical');
    // Comply decision is also recorded:
    expect(event.data.comply).toBeDefined();
  });

  it('preserves pre-existing classifiedBy that was already stricter than L0-rules', async () => {
    const src = fixedSource({
      status: 'drift',
      statistic: 30,
      threshold: 21.666,
      topDeviations: [],
      windowSize: 100,
      source: 'guard-anomaly',
    });
    const coord = new IntelligenceCoordinator(
      baseConfig(),
      tmpDir,
      null,
      null,
      src,
    );
    const event = makeEvent({
      classifiedBy: 'L1-statistical',
      category: 'anomaly',
      severity: 'medium',
    });
    await coord.analyze(event);
    // classifiedBy must NOT have been rewritten because the prior
    // layer was not L0-rules.
    expect(event.classifiedBy).toBe('L1-statistical');
  });

  it('never lowers a pre-existing severity higher than the drift floor', async () => {
    const src = fixedSource({
      status: 'drift',
      statistic: 30,
      threshold: 21.666,
      topDeviations: [],
      windowSize: 100,
      source: 'guard-anomaly',
    });
    const coord = new IntelligenceCoordinator(
      baseConfig(),
      tmpDir,
      null,
      null,
      src,
    );
    const event = makeEvent({
      category: 'threat',
      severity: 'critical',
    });
    await coord.analyze(event);
    expect(event.category).toBe('threat');
    expect(event.severity).toBe('critical');
  });

  it('hot-swap from null to a drift source takes effect on the next call', async () => {
    const coord = new IntelligenceCoordinator(
      baseConfig(),
      tmpDir,
      null,
      null,
      null,
    );
    const e1 = makeEvent();
    await coord.analyze(e1);
    expect(e1.data.guardAnomaly).toBeUndefined();

    coord.setGuardAnomaly(
      fixedSource({
        status: 'drift',
        statistic: 30,
        threshold: 21.666,
        topDeviations: [],
        windowSize: 100,
        source: 'swapped',
      }),
    );
    const e2 = makeEvent();
    await coord.analyze(e2);
    expect(e2.classifiedBy).toBe('L1-guard-anomaly');
    expect(e2.category).toBe('anomaly');
    expect(e2.severity).toBe('medium');

    coord.setGuardAnomaly(null);
    const e3 = makeEvent();
    await coord.analyze(e3);
    expect(e3.data.guardAnomaly).toBeUndefined();
    expect(coord.getGuardAnomaly()).toBeNull();
  });

  it('end-to-end: a real GuardAnomalyDetector surfaces drift after a class spike', async () => {
    const detector = new GuardAnomalyDetector({
      baseline: {
        'file-read': 0.4,
        'network-egress': 0.3,
        'process-spawn': 0.2,
        'telemetry': 0.1,
      },
      windowSize: 100,
      minObservations: 40,
      alarmThreshold: 15,
    });
    const coord = new IntelligenceCoordinator(
      baseConfig(),
      tmpDir,
      null,
      null,
      detector,
    );

    // Warm up with 50 baseline-matching events. All should land
    // normal or baseline-pending, never drift.
    const mix = [
      ...new Array(20).fill('file-read'),
      ...new Array(15).fill('network-egress'),
      ...new Array(10).fill('process-spawn'),
      ...new Array(5).fill('telemetry'),
    ];
    for (const c of mix) {
      const e = makeEvent({ data: { classification: c } });
      await coord.analyze(e);
      const rec = e.data.guardAnomaly as GuardAnomalyStatus;
      expect(rec.status === 'normal' || rec.status === 'baseline-pending').toBe(true);
    }

    // Now spike telemetry so it dominates the window.
    let sawDrift = false;
    for (let i = 0; i < 80; i++) {
      const e = makeEvent({ data: { classification: 'telemetry' } });
      await coord.analyze(e);
      const rec = e.data.guardAnomaly as GuardAnomalyStatus;
      if (rec.status === 'drift') {
        sawDrift = true;
        expect(e.category).toBe('anomaly');
        expect(e.severity).toBe('medium');
        break;
      }
    }
    expect(sawDrift).toBe(true);
  });

  it('no detector attached preserves legacy coordinator behavior', async () => {
    const coord = new IntelligenceCoordinator(
      baseConfig(),
      tmpDir,
    );
    const event = makeEvent();
    await coord.analyze(event);
    expect(event.data.guardAnomaly).toBeUndefined();
    expect(event.category).toBe('normal');
    expect(event.classifiedBy).toBe('L0-rules');
  });
});
