import { describe, it, expect } from 'vitest';
import { CorrelationEngine } from '../../../src/arp/engine/correlation';
import type { ARPEvent } from '../../../src/arp/types';

function makeEvent(
  source: ARPEvent['source'],
  severity: ARPEvent['severity'] = 'medium',
  category: ARPEvent['category'] = 'anomaly',
): ARPEvent {
  return {
    id: 'test-' + Math.random().toString(36).slice(2),
    timestamp: new Date().toISOString(),
    source,
    category,
    severity,
    description: `Test ${source} event`,
    data: {},
    classifiedBy: 'L0-rules',
  };
}

describe('CorrelationEngine', () => {
  it('returns null when only one source is present', () => {
    const engine = new CorrelationEngine();

    const result1 = engine.correlate(makeEvent('process'));
    expect(result1).toBeNull();

    const result2 = engine.correlate(makeEvent('process'));
    expect(result2).toBeNull();
  });

  it('emits correlation event when events from 2+ sources appear', () => {
    const engine = new CorrelationEngine();

    engine.correlate(makeEvent('process'));
    const result = engine.correlate(makeEvent('network'));

    expect(result).not.toBeNull();
    expect(result!.category).toBe('threat');
    expect(result!.description).toContain('Cross-monitor correlation');
    expect(result!.description).toContain('network');
    expect(result!.description).toContain('process');
    expect(result!.data.correlatedSources).toContain('process');
    expect(result!.data.correlatedSources).toContain('network');
    expect(result!.classifiedBy).toBe('L1-statistical');
  });

  it('escalates severity by one level', () => {
    const engine = new CorrelationEngine();

    engine.correlate(makeEvent('process', 'medium'));
    const result = engine.correlate(makeEvent('network', 'high'));

    expect(result).not.toBeNull();
    // Highest component is 'high', escalated to 'critical'
    expect(result!.severity).toBe('critical');
  });

  it('caps escalation at critical', () => {
    const engine = new CorrelationEngine();

    engine.correlate(makeEvent('process', 'critical'));
    const result = engine.correlate(makeEvent('network', 'critical'));

    expect(result).not.toBeNull();
    expect(result!.severity).toBe('critical');
  });

  it('does not emit duplicate correlation for same source combination', () => {
    const engine = new CorrelationEngine();

    engine.correlate(makeEvent('process'));
    const first = engine.correlate(makeEvent('network'));
    expect(first).not.toBeNull();

    // Same two sources again — should not re-emit
    engine.correlate(makeEvent('process'));
    const duplicate = engine.correlate(makeEvent('network'));
    expect(duplicate).toBeNull();
  });

  it('emits new correlation when a third source appears', () => {
    const engine = new CorrelationEngine();

    engine.correlate(makeEvent('process'));
    const twoSource = engine.correlate(makeEvent('network'));
    expect(twoSource).not.toBeNull();

    // Third source appears — new combination
    const threeSource = engine.correlate(makeEvent('filesystem'));
    expect(threeSource).not.toBeNull();
    expect(threeSource!.data.correlatedSources).toContain('filesystem');
  });

  it('reset() clears state', () => {
    const engine = new CorrelationEngine();

    engine.correlate(makeEvent('process'));
    engine.correlate(makeEvent('network'));
    engine.reset();

    expect(engine.getWindowSize()).toBe(0);

    // After reset, same combination should trigger again
    engine.correlate(makeEvent('process'));
    const result = engine.correlate(makeEvent('network'));
    expect(result).not.toBeNull();
  });

  it('getWindowSize() reflects current event count', () => {
    const engine = new CorrelationEngine();

    expect(engine.getWindowSize()).toBe(0);
    engine.correlate(makeEvent('process'));
    expect(engine.getWindowSize()).toBe(1);
    engine.correlate(makeEvent('network'));
    expect(engine.getWindowSize()).toBe(2);
  });
});

describe('CorrelationEngine integrated with EventEngine', () => {
  it('EventEngine emits correlation events automatically', async () => {
    // Import EventEngine here to test integration
    const { EventEngine } = await import('../../../src/arp/engine/event-engine');

    const engine = new EventEngine({ agentName: 'test-agent', rules: [] });
    const events: ARPEvent[] = [];
    engine.onEvent((e) => { events.push(e); });

    // Emit from two different sources
    await engine.emit({
      source: 'process',
      category: 'anomaly',
      severity: 'medium',
      description: 'Suspicious process',
      data: {},
    });

    await engine.emit({
      source: 'network',
      category: 'anomaly',
      severity: 'high',
      description: 'Suspicious connection',
      data: {},
    });

    // Should have: process event, network event, and a correlation event
    const correlationEvents = events.filter((e) =>
      e.description.includes('Cross-monitor correlation')
    );
    expect(correlationEvents.length).toBe(1);
    expect(correlationEvents[0].category).toBe('threat');
    expect(correlationEvents[0].data.correlatedSources).toContain('process');
    expect(correlationEvents[0].data.correlatedSources).toContain('network');
  });
});
