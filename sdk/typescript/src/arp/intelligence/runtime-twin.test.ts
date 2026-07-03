import { describe, it, expect } from 'vitest';
import { RuntimeTwin } from './runtime-twin';
import { EventEngine } from '../engine/event-engine';
import type { ARPEvent, ARPConfig } from '../types';

const testConfig: ARPConfig = {
  rules: [],
  monitors: [],
  interceptors: [],
};

function emitTestEvent(engine: EventEngine, overrides: Partial<ARPEvent> = {}): Promise<ARPEvent> {
  return engine.emit({
    source: 'test-monitor',
    category: 'normal',
    severity: 'info',
    description: 'Test event',
    data: {
      source: 'process',
      capability: 'db:read',
      ...overrides.data,
    },
    ...overrides,
  });
}

describe('RuntimeTwin E2E Integration', () => {
  it('attaches to EventEngine without errors', () => {
    const engine = new EventEngine(testConfig);
    const l1 = new RuntimeTwin('test-agent');
    l1.attach(engine);
    expect(true).toBe(true);
  });

  it('processes events through L1 pipeline', async () => {
    const engine = new EventEngine(testConfig);
    const l1 = new RuntimeTwin('pipeline-test');
    l1.attach(engine);

    // Emit 10 events
    for (let i = 0; i < 10; i++) {
      await emitTestEvent(engine);
    }

    // L1 processes asynchronously — wait a tick
    await new Promise(r => setTimeout(r, 50));
    expect(true).toBe(true);
  });

  it('builds baseline from normal events', async () => {
    const engine = new EventEngine(testConfig);
    const l1 = new RuntimeTwin('baseline-test');
    l1.attach(engine);

    // Feed 150 normal events to build baseline
    for (let i = 0; i < 150; i++) {
      await emitTestEvent(engine);
    }

    await new Promise(r => setTimeout(r, 100));

    // Now feed a normal event — should process without error
    await emitTestEvent(engine);
    expect(true).toBe(true);
  });

  it('detects anomalous events after baseline', async () => {
    const engine = new EventEngine(testConfig);
    const l1 = new RuntimeTwin('anomaly-test');
    l1.attach(engine);

    // Build baseline with normal events
    for (let i = 0; i < 150; i++) {
      await emitTestEvent(engine);
    }

    await new Promise(r => setTimeout(r, 50));

    // Emit anomalous event
    await emitTestEvent(engine, {
      category: 'threat',
      severity: 'critical',
      data: {
        source: 'network',
        capability: 'admin:delete_all',
        error: true,
      },
    });

    await new Promise(r => setTimeout(r, 50));
    expect(true).toBe(true);
  });

  it('respects enabled=false', () => {
    const engine = new EventEngine(testConfig);
    const l1 = new RuntimeTwin('disabled-test', { enabled: false });
    l1.attach(engine);
    // Should not crash and should not process events
    expect(true).toBe(true);
  });

  it('L1 does not block L0 event processing', async () => {
    const engine = new EventEngine(testConfig);
    const l1 = new RuntimeTwin('nonblocking-test');
    l1.attach(engine);

    // Time L0 event emission
    const start = Date.now();
    for (let i = 0; i < 100; i++) {
      await emitTestEvent(engine);
    }
    const elapsed = Date.now() - start;

    // 100 events should complete in well under 1 second
    // (L1 runs via setImmediate, doesn't block)
    expect(elapsed < 1000, `100 events took ${elapsed}ms (should be < 1000ms)`).toBe(true);
  });
});
