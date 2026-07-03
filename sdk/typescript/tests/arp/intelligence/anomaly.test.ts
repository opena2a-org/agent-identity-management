import { describe, it, expect } from 'vitest';
import { AnomalyDetector } from '../../../src/arp/intelligence/anomaly';
import type { ARPEvent } from '../../../src/arp/types';

function makeEvent(source: 'process' | 'network' | 'filesystem' = 'process'): ARPEvent {
  return {
    id: 'test-' + Math.random().toString(36).slice(2),
    timestamp: new Date().toISOString(),
    source,
    category: 'normal',
    severity: 'info',
    description: 'Test event',
    data: {},
    classifiedBy: 'L0-rules',
  };
}

describe('AnomalyDetector', () => {
  it('returns 0 score with insufficient data', () => {
    const detector = new AnomalyDetector();
    const score = detector.score(makeEvent());
    expect(score).toBe(0);
  });

  it('builds baseline from recorded events', () => {
    const detector = new AnomalyDetector();

    // Record 50 events to build baseline
    for (let i = 0; i < 50; i++) {
      detector.record(makeEvent());
    }

    const baseline = detector.getBaseline('process');
    expect(baseline).not.toBeNull();
    expect(baseline!.count).toBeGreaterThan(0);
    expect(baseline!.mean).toBeGreaterThan(0);
  });

  it('tracks separate baselines per monitor', () => {
    const detector = new AnomalyDetector();

    for (let i = 0; i < 50; i++) {
      detector.record(makeEvent('process'));
    }
    for (let i = 0; i < 30; i++) {
      detector.record(makeEvent('network'));
    }

    const processBaseline = detector.getBaseline('process');
    const networkBaseline = detector.getBaseline('network');

    expect(processBaseline).not.toBeNull();
    expect(networkBaseline).not.toBeNull();
  });

  it('resets all baselines', () => {
    const detector = new AnomalyDetector();

    for (let i = 0; i < 50; i++) {
      detector.record(makeEvent());
    }

    detector.reset();
    expect(detector.getBaseline('process')).toBeNull();
  });
});
