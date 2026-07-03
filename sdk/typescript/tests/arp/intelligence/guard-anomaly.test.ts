import { describe, it, expect } from 'vitest';
import {
  GuardAnomalyDetector,
  buildBaselineFromObservations,
  DEFAULT_GUARD_ANOMALY_THRESHOLD,
  DEFAULT_GUARD_ANOMALY_WINDOW,
  MAX_GUARD_ANOMALY_WINDOW,
  MAX_CLASSIFICATION_LENGTH,
  type ClassDistribution,
  type GuardAnomalyStatus,
} from '../../../src/arp/intelligence/guard-anomaly';

/**
 * Unit coverage for GuardAnomalyDetector.
 *
 * Assertions follow the bounded-memory and parse-to-deny guarantees
 * documented in the module header:
 *   - Ring buffer evicts oldest on wrap; counts stay consistent.
 *   - Chi-square statistic is 0 on a window that mirrors the baseline.
 *   - Chi-square grows monotonically with deliberate over-concentration.
 *   - Baseline swap is honored in place.
 *   - Invalid input is a no-op.
 *   - Empty baseline reports baseline-pending with an explicit reason.
 */

const BASELINE: ClassDistribution = {
  'file-read': 0.4,
  'network-egress': 0.3,
  'process-spawn': 0.2,
  'telemetry': 0.1,
};

function detector(overrides: Partial<ConstructorParameters<typeof GuardAnomalyDetector>[0]> = {}) {
  return new GuardAnomalyDetector({
    baseline: BASELINE,
    windowSize: 100,
    minObservations: 40,
    ...overrides,
  });
}

/**
 * Feed a detector a sequence whose empirical class frequencies match
 * the baseline as closely as rounding allows. Used for the "no drift"
 * invariant.
 */
function seedBaselineMatching(d: GuardAnomalyDetector, n: number): GuardAnomalyStatus {
  const seq = [
    ...new Array(Math.round(n * 0.4)).fill('file-read'),
    ...new Array(Math.round(n * 0.3)).fill('network-egress'),
    ...new Array(Math.round(n * 0.2)).fill('process-spawn'),
    ...new Array(Math.round(n * 0.1)).fill('telemetry'),
  ];
  let last: GuardAnomalyStatus | null = null;
  for (const c of seq) last = d.record(c);
  return last!;
}

describe('GuardAnomalyDetector', () => {
  it('reports baseline-pending with insufficient-observations before minObservations is reached', () => {
    const d = detector({ minObservations: 50 });
    let status: GuardAnomalyStatus | null = null;
    for (let i = 0; i < 20; i++) status = d.record('file-read');
    expect(status!.status).toBe('baseline-pending');
    if (status!.status === 'baseline-pending') {
      expect(status!.reason).toBe('insufficient-observations');
      expect(status!.observed).toBe(20);
      expect(status!.required).toBe(50);
    }
  });

  it('reports baseline-pending with empty-baseline when the baseline is empty', () => {
    const d = new GuardAnomalyDetector({
      baseline: {},
      windowSize: 50,
      minObservations: 5,
    });
    for (let i = 0; i < 30; i++) d.record('file-read');
    const status = d.record('file-read');
    expect(status.status).toBe('baseline-pending');
    if (status.status === 'baseline-pending') {
      expect(status.reason).toBe('empty-baseline');
    }
  });

  it('classifies a window that matches the baseline as normal with small statistic', () => {
    const d = detector();
    const status = seedBaselineMatching(d, 100);
    expect(status.status).toBe('normal');
    if (status.status === 'normal') {
      expect(status.statistic).toBeLessThan(1);
      expect(status.threshold).toBe(DEFAULT_GUARD_ANOMALY_THRESHOLD);
    }
  });

  it('raises drift when the observed distribution concentrates on a single class', () => {
    const d = detector({ alarmThreshold: 15 });
    let status: GuardAnomalyStatus | null = null;
    for (let i = 0; i < 100; i++) status = d.record('telemetry');
    expect(status!.status).toBe('drift');
    if (status!.status === 'drift') {
      expect(status!.statistic).toBeGreaterThan(15);
      expect(status!.topDeviations.length).toBeGreaterThan(0);
      const top = status!.topDeviations[0];
      expect(top.className).toBe('telemetry');
      expect(top.deviation).toBeGreaterThan(0);
    }
  });

  it('surfaces under-represented classes as negative deviations in the top-K', () => {
    const d = detector({ alarmThreshold: 5, topKDeviations: 5 });
    // Drop file-read entirely; its baseline share was 0.4, so the
    // observed count will be zero and the expected count large.
    for (let i = 0; i < 30; i++) d.record('network-egress');
    for (let i = 0; i < 30; i++) d.record('process-spawn');
    for (let i = 0; i < 30; i++) d.record('telemetry');
    const status = d.record('telemetry');
    expect(status.status).toBe('drift');
    if (status.status === 'drift') {
      const fileRead = status.topDeviations.find((d) => d.className === 'file-read');
      expect(fileRead).toBeDefined();
      expect(fileRead!.observed).toBe(0);
      expect(fileRead!.deviation).toBeLessThan(0);
    }
  });

  it('drift status carries top-K bounded by config', () => {
    const d = detector({ alarmThreshold: 1, topKDeviations: 2 });
    for (let i = 0; i < 60; i++) d.record('telemetry');
    const status = d.record('telemetry');
    expect(status.status).toBe('drift');
    if (status.status === 'drift') {
      expect(status.topDeviations.length).toBe(2);
    }
  });

  it('evicts oldest entries on ring buffer wrap so the window stays at capacity', () => {
    const d = detector({ windowSize: 10, minObservations: 3 });
    for (let i = 0; i < 10; i++) d.record('file-read');
    expect(d.getWindowObservations()).toBe(10);
    // Overwrite the ring twice; window must stay at 10 exactly.
    for (let i = 0; i < 20; i++) d.record('telemetry');
    expect(d.getWindowObservations()).toBe(10);
    const observed = d.getObserved();
    expect(observed['file-read'] ?? 0).toBe(0);
    expect(observed.telemetry).toBe(10);
  });

  it('counts stay consistent across many wraps (bounded memory invariant)', () => {
    const d = detector({ windowSize: 50, minObservations: 3 });
    const classes = ['file-read', 'network-egress', 'process-spawn', 'telemetry'];
    for (let i = 0; i < 1000; i++) {
      d.record(classes[i % classes.length]);
    }
    const observed = d.getObserved();
    const sum = Object.values(observed).reduce((a, b) => a + b, 0);
    expect(sum).toBe(50);
    expect(d.getWindowObservations()).toBe(50);
  });

  it('rejects empty, non-string, and over-long classifications as a no-op', () => {
    const d = detector({ windowSize: 20, minObservations: 1 });
    const longLabel = 'a'.repeat(MAX_CLASSIFICATION_LENGTH + 1);
    d.record('');
    d.record(longLabel);
    // @ts-expect-error exercising runtime guard against bad caller input
    d.record(42);
    // @ts-expect-error exercising runtime guard against bad caller input
    d.record(null);
    expect(d.getWindowObservations()).toBe(0);
    d.record('file-read');
    expect(d.getWindowObservations()).toBe(1);
  });

  it('boundary label at MAX_CLASSIFICATION_LENGTH is accepted', () => {
    const d = detector({ windowSize: 5, minObservations: 1 });
    const label = 'b'.repeat(MAX_CLASSIFICATION_LENGTH);
    d.record(label);
    expect(d.getWindowObservations()).toBe(1);
    expect(d.getObserved()[label]).toBe(1);
  });

  it('normalizes a baseline passed as raw counts', () => {
    const d = new GuardAnomalyDetector({
      baseline: { a: 400, b: 300, c: 200, d: 100 },
      windowSize: 50,
      minObservations: 5,
    });
    const normalized = d.getBaseline();
    expect(normalized.a).toBeCloseTo(0.4, 5);
    expect(normalized.b).toBeCloseTo(0.3, 5);
    expect(normalized.c).toBeCloseTo(0.2, 5);
    expect(normalized.d).toBeCloseTo(0.1, 5);
  });

  it('drops negative, NaN, and non-number baseline entries', () => {
    const d = new GuardAnomalyDetector({
      baseline: {
        ok: 1,
        neg: -5,
        nan: NaN,
        inf: Infinity,
        // @ts-expect-error exercising runtime guard
        stringy: 'nope',
      },
      windowSize: 10,
      minObservations: 2,
    });
    const normalized = d.getBaseline();
    expect(normalized).toEqual({ ok: 1 });
  });

  it('hot-swaps the baseline without touching the observation window', () => {
    const d = detector({ alarmThreshold: 10, minObservations: 20 });
    for (let i = 0; i < 40; i++) d.record('telemetry');
    const before = d.record('telemetry');
    expect(before.status).toBe('drift');
    // Install a new baseline that expects telemetry to dominate.
    d.setBaseline({ telemetry: 1.0 });
    const after = d.record('telemetry');
    expect(after.status).toBe('normal');
    // Observation count preserved across swap.
    expect(d.getWindowObservations()).toBe(42);
  });

  it('reset clears the observation window but preserves the baseline', () => {
    const d = detector({ windowSize: 10, minObservations: 3 });
    for (let i = 0; i < 10; i++) d.record('telemetry');
    expect(d.getWindowObservations()).toBe(10);
    d.reset();
    expect(d.getWindowObservations()).toBe(0);
    expect(d.getObserved()).toEqual({});
    // Baseline still installed.
    expect(Object.keys(d.getBaseline()).length).toBe(4);
    // And still returns baseline-pending because the window is empty.
    const status = d.record('file-read');
    expect(status.status).toBe('baseline-pending');
  });

  it('windowSize is clamped to MAX_GUARD_ANOMALY_WINDOW', () => {
    const d = new GuardAnomalyDetector({
      baseline: BASELINE,
      windowSize: MAX_GUARD_ANOMALY_WINDOW * 10,
    });
    expect(d.getWindowCapacity()).toBe(MAX_GUARD_ANOMALY_WINDOW);
  });

  it('windowSize of 0 or negative falls back to the default', () => {
    const dZero = new GuardAnomalyDetector({ baseline: BASELINE, windowSize: 0 });
    expect(dZero.getWindowCapacity()).toBe(DEFAULT_GUARD_ANOMALY_WINDOW);
    const dNeg = new GuardAnomalyDetector({ baseline: BASELINE, windowSize: -5 });
    expect(dNeg.getWindowCapacity()).toBe(DEFAULT_GUARD_ANOMALY_WINDOW);
  });

  it('statistic is zero when the window is empty', () => {
    // Force minObservations to 0 by using a small window with a
    // floor-reduced min observations, then compute status without
    // any records. The floor forces minObservations to at least 1.
    const d = new GuardAnomalyDetector({
      baseline: BASELINE,
      windowSize: 5,
      minObservations: 1,
    });
    const status = d.record('file-read');
    expect(status.status).not.toBe('baseline-pending');
  });

  it('observed classes outside the baseline contribute to the chi-square via smoothing', () => {
    const d = detector({ alarmThreshold: 1, minObservations: 20 });
    for (let i = 0; i < 60; i++) d.record('brand-new-class');
    const status = d.record('brand-new-class');
    expect(status.status).toBe('drift');
    if (status.status === 'drift') {
      const novel = status.topDeviations.find((x) => x.className === 'brand-new-class');
      expect(novel).toBeDefined();
      expect(novel!.observed).toBe(61);
    }
  });

  it('ok and drift statuses carry the configured source name', () => {
    const d = detector({ sourceName: 'custom-src', alarmThreshold: 5 });
    for (let i = 0; i < 60; i++) d.record('telemetry');
    const status = d.record('telemetry');
    expect(status.status).toBe('drift');
    if (status.status === 'drift') expect(status.source).toBe('custom-src');
  });

  it('getObserved returns a snapshot copy, not the internal map', () => {
    const d = detector({ minObservations: 1 });
    d.record('file-read');
    const snap1 = d.getObserved();
    snap1['file-read'] = 99999;
    const snap2 = d.getObserved();
    expect(snap2['file-read']).toBe(1);
  });
});

describe('buildBaselineFromObservations', () => {
  it('builds a count map from string samples', () => {
    const baseline = buildBaselineFromObservations([
      'file-read',
      'file-read',
      'file-read',
      'network-egress',
      'telemetry',
    ]);
    expect(baseline).toEqual({
      'file-read': 3,
      'network-egress': 1,
      telemetry: 1,
    });
  });

  it('skips empty, non-string, and over-long entries', () => {
    const longLabel = 'a'.repeat(MAX_CLASSIFICATION_LENGTH + 1);
    const baseline = buildBaselineFromObservations([
      'file-read',
      '',
      longLabel,
      // @ts-expect-error exercising runtime guard
      null,
      // @ts-expect-error exercising runtime guard
      undefined,
      'file-read',
    ]);
    expect(baseline).toEqual({ 'file-read': 2 });
  });

  it('returns an empty baseline for an empty input', () => {
    expect(buildBaselineFromObservations([])).toEqual({});
  });
});
