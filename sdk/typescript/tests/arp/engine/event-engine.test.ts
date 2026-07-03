import { describe, it, expect } from 'vitest';
import { EventEngine } from '../../../src/arp/engine/event-engine';
import type { ARPConfig, ARPEvent } from '../../../src/arp/types';

function makeConfig(rules?: ARPConfig['rules']): ARPConfig {
  return { agentName: 'test-agent', rules };
}

describe('EventEngine', () => {
  it('emits events and calls handlers', async () => {
    const engine = new EventEngine(makeConfig());
    const events: ARPEvent[] = [];

    engine.onEvent((e) => { events.push(e); });

    await engine.emit({
      source: 'process',
      category: 'normal',
      severity: 'info',
      description: 'Test event',
      data: {},
    });

    expect(events.length).toBe(1);
    expect(events[0].source).toBe('process');
    expect(events[0].id).toBeTruthy();
    expect(events[0].timestamp).toBeTruthy();
  });

  it('evaluates rules and triggers enforcement', async () => {
    const engine = new EventEngine(makeConfig([
      {
        name: 'test-rule',
        condition: { category: 'threat', minSeverity: 'high' },
        action: 'alert',
      },
    ]));

    const enforcements: { action: string }[] = [];
    engine.onEnforcement((r) => { enforcements.push({ action: r.action }); });

    // Normal event — should NOT trigger
    await engine.emit({
      source: 'process',
      category: 'normal',
      severity: 'info',
      description: 'Normal event',
      data: {},
    });
    expect(enforcements.length).toBe(0);

    // Threat event — should trigger
    await engine.emit({
      source: 'network',
      category: 'threat',
      severity: 'critical',
      description: 'Suspicious connection',
      data: {},
    });
    // At least 1 enforcement from the direct rule match; correlation may add more
    expect(enforcements.length).toBeGreaterThanOrEqual(1);
    expect(enforcements[0].action).toBe('alert');
  });

  it('handles threshold-based rules', async () => {
    const engine = new EventEngine(makeConfig([
      {
        name: 'burst-rule',
        condition: {
          source: 'network',
          threshold: { count: 3, windowMs: 60000 },
        },
        action: 'alert',
      },
    ]));

    const enforcements: { action: string }[] = [];
    engine.onEnforcement((r) => { enforcements.push({ action: r.action }); });

    // First 2 events — below threshold
    await engine.emit({ source: 'network', category: 'anomaly', severity: 'low', description: 'Event 1', data: {} });
    await engine.emit({ source: 'network', category: 'anomaly', severity: 'low', description: 'Event 2', data: {} });
    expect(enforcements.length).toBe(0);

    // 3rd event — threshold reached
    await engine.emit({ source: 'network', category: 'anomaly', severity: 'low', description: 'Event 3', data: {} });
    expect(enforcements.length).toBe(1);
  });

  it('returns recent events', async () => {
    const engine = new EventEngine(makeConfig());

    await engine.emit({ source: 'process', category: 'normal', severity: 'info', description: 'E1', data: {} });
    await engine.emit({ source: 'network', category: 'normal', severity: 'info', description: 'E2', data: {} });
    await engine.emit({ source: 'process', category: 'normal', severity: 'info', description: 'E3', data: {} });

    const recent = engine.getRecentEvents(60000);
    // 3 original events + possible correlation events from cross-source detection
    expect(recent.length).toBeGreaterThanOrEqual(3);

    const processOnly = engine.getRecentEvents(60000, 'process');
    expect(processOnly.length).toBeGreaterThanOrEqual(2);
  });

  it('reclassifies events', async () => {
    const engine = new EventEngine(makeConfig());

    const event = await engine.emit({
      source: 'process',
      category: 'normal',
      severity: 'info',
      description: 'Test',
      data: {},
    });

    expect(event.classifiedBy).toBe('L0-rules');

    engine.reclassify(event, 'anomaly', 'medium', 'L1-statistical');
    expect(event.category).toBe('anomaly');
    expect(event.severity).toBe('medium');
    expect(event.classifiedBy).toBe('L1-statistical');
  });
});
