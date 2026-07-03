import { describe, it, expect, vi, beforeEach } from 'vitest';
import { EventEngine } from '../../../src/arp/engine/event-engine';
import type { ARPEvent, ARPConfig } from '../../../src/arp/types';

/**
 * Test connection burst detection in NetworkMonitor.
 *
 * Since NetworkMonitor.poll() calls getConnections() which runs lsof/ss (system calls),
 * we test the burst detection logic by directly invoking the private checkConnectionBurst
 * method through the class. We use a subclass trick to expose the private method for testing.
 */

// Import and extend NetworkMonitor to expose burst detection for testing
import { NetworkMonitor } from '../../../src/arp/monitors/network';

class TestableNetworkMonitor extends NetworkMonitor {
  /** Expose checkConnectionBurst for testing */
  triggerBurstCheck(connectionCount: number): void {
    // Access private method via bracket notation
    (this as any).checkConnectionBurst(connectionCount);
  }
}

function makeConfig(): ARPConfig {
  return { agentName: 'test-agent', rules: [] };
}

describe('NetworkMonitor burst detection', () => {
  let engine: EventEngine;
  let monitor: TestableNetworkMonitor;
  let emittedEvents: ARPEvent[];

  beforeEach(() => {
    engine = new EventEngine(makeConfig());
    emittedEvents = [];
    engine.onEvent((e) => { emittedEvents.push(e); });
    // Use a low burst threshold (2x) for testing
    monitor = new TestableNetworkMonitor(engine, 10000, [], 2);
  });

  it('does not emit burst event on first window (no baseline)', () => {
    monitor.triggerBurstCheck(10);
    // First window — sets baseline, no burst
    expect(emittedEvents).toHaveLength(0);
  });

  it('does not emit burst event when below threshold', () => {
    // With burstThreshold=2, we need currentWindowCount > previousAvg * 2
    // Since all calls happen in the same minute, timestamps accumulate.
    // Call 1: count=2, sets baseline to 2, windowCount=1 (no check, first window)
    // Call 2: count=4, previousAvg=2, threshold=4, 4 > 4 is false (not strictly greater)
    // Call 3: count=6, previousAvg=2.4, threshold=4.8, 6 > 4.8 triggers!
    // Solution: use small increments that never exceed 2x the rolling average
    monitor.triggerBurstCheck(2);  // window 1: baseline=2
    monitor.triggerBurstCheck(1);  // window 2: count=3, prev=2, threshold=4, 3<4 OK
    monitor.triggerBurstCheck(1);  // window 3: count=4, prev=1.8, threshold=3.6, 4>3.6 could trigger

    // With same-minute accumulation, counts grow. Use a fresh monitor with high threshold.
    const safeEngine = new EventEngine(makeConfig());
    const safeEvents: ARPEvent[] = [];
    safeEngine.onEvent((e) => { safeEvents.push(e); });
    // burstThreshold = 10 — very hard to trigger
    const safeMonitor = new TestableNetworkMonitor(safeEngine, 10000, [], 10);

    safeMonitor.triggerBurstCheck(5);
    safeMonitor.triggerBurstCheck(5);
    safeMonitor.triggerBurstCheck(5);
    safeMonitor.triggerBurstCheck(5);

    const burstEvents = safeEvents.filter((e) => e.description.includes('burst'));
    expect(burstEvents.length).toBe(0);
  });

  it('emits burst event when connections spike above threshold', () => {
    // Build a low baseline
    monitor.triggerBurstCheck(2);  // window 1: sets baseline
    monitor.triggerBurstCheck(0);  // window 2: low
    monitor.triggerBurstCheck(0);  // window 3: low — now burst detection activates

    // Clear any prior events
    emittedEvents.length = 0;

    // Spike: add many connections at once (these all land in the same minute window)
    monitor.triggerBurstCheck(50);

    const burstEvents = emittedEvents.filter((e) => e.description.includes('burst'));
    expect(burstEvents.length).toBeGreaterThan(0);
    expect(burstEvents[0].category).toBe('violation');
    expect(burstEvents[0].severity).toBe('high');
    expect(burstEvents[0].source).toBe('network');
    expect(burstEvents[0].data.currentCount).toBeDefined();
    expect(burstEvents[0].data.rollingAverage).toBeDefined();
    expect(burstEvents[0].data.threshold).toBeDefined();
  });

  it('includes burst multiplier in event data', () => {
    monitor.triggerBurstCheck(2);
    monitor.triggerBurstCheck(0);
    monitor.triggerBurstCheck(0);
    emittedEvents.length = 0;
    monitor.triggerBurstCheck(50);

    const burstEvents = emittedEvents.filter((e) => e.description.includes('burst'));
    expect(burstEvents.length).toBeGreaterThan(0);
    expect(burstEvents[0].data.burstMultiplier).toBe(2);
  });

  it('configurable burst threshold works', () => {
    // Create with very high threshold (100x) — should never trigger
    const strictEngine = new EventEngine(makeConfig());
    const strictEvents: ARPEvent[] = [];
    strictEngine.onEvent((e) => { strictEvents.push(e); });
    const strictMonitor = new TestableNetworkMonitor(strictEngine, 10000, [], 100);

    strictMonitor.triggerBurstCheck(2);
    strictMonitor.triggerBurstCheck(0);
    strictMonitor.triggerBurstCheck(0);
    strictMonitor.triggerBurstCheck(50);

    const burstEvents = strictEvents.filter((e) => e.description.includes('burst'));
    expect(burstEvents).toHaveLength(0);
  });
});
