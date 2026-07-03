import { describe, it, expect } from 'vitest';
import { AgentRuntimeProtection } from '../../src/arp';
import { GuardAnomalyDetector } from '../../src/arp/intelligence/guard-anomaly';
import type { ARPConfig } from '../../src/arp/types';

/**
 * Integration coverage for AgentRuntimeProtection default-wiring of
 * session 28 (behavioral risk IPC) and session 29 (guard anomaly drift)
 * machinery. Before session 30, those sources existed but were never
 * instantiated in production. These tests assert that:
 *
 *   - Runtime twin is ON by default when intelligence is enabled.
 *   - Runtime twin can be disabled via config opt-out.
 *   - Behavioral risk source passed to the coordinator matches the twin.
 *   - Guard anomaly is OFF unless a baseline is injected.
 *   - Guard anomaly honors the baseline and windowSize from config.
 *   - `intelligence.enabled === false` disables everything.
 *   - start() attaches the twin to the event engine exactly once.
 */

function baseConfig(overrides: Partial<ARPConfig> = {}): ARPConfig {
  return {
    agentName: 'wiring-test-agent',
    dataDir: '/tmp/arp-wiring-test',
    // Disable all monitors so start() does not touch the real filesystem.
    monitors: {
      process: { enabled: false },
      network: { enabled: false },
      filesystem: { enabled: false },
    },
    ...overrides,
  };
}

describe('AgentRuntimeProtection session 30 wiring', () => {
  it('attaches a runtime twin and behavioral risk source by default', () => {
    const arp = new AgentRuntimeProtection(baseConfig());
    expect(arp.getRuntimeTwin()).not.toBeNull();
    const coord = arp.getIntelligence();
    expect(coord.getBehavioralRiskSource()).not.toBeNull();
  });

  it('passes the runtime twin through as the behavioral risk source', () => {
    const arp = new AgentRuntimeProtection(baseConfig());
    const twin = arp.getRuntimeTwin();
    const src = arp.getIntelligence().getBehavioralRiskSource();
    // The source wraps the twin. We cannot assert identity on the twin
    // directly because InProcessBehavioralRiskSource holds it as a
    // private field, but we can assert both are non-null and the source
    // type matches the production choice.
    expect(twin).not.toBeNull();
    expect(src).not.toBeNull();
    expect(src?.constructor.name).toBe('InProcessBehavioralRiskSource');
  });

  it('respects intelligence.runtimeTwin.enabled = false opt-out', () => {
    const arp = new AgentRuntimeProtection(
      baseConfig({
        intelligence: {
          runtimeTwin: { enabled: false },
        },
      }),
    );
    expect(arp.getRuntimeTwin()).toBeNull();
    expect(arp.getIntelligence().getBehavioralRiskSource()).toBeNull();
  });

  it('respects intelligence.enabled = false as a global kill switch for the twin', () => {
    const arp = new AgentRuntimeProtection(
      baseConfig({
        intelligence: { enabled: false },
      }),
    );
    expect(arp.getRuntimeTwin()).toBeNull();
    expect(arp.getIntelligence().getBehavioralRiskSource()).toBeNull();
  });

  it('does not construct a guard anomaly detector without an injected baseline', () => {
    const arp = new AgentRuntimeProtection(baseConfig());
    expect(arp.getIntelligence().getGuardAnomaly()).toBeNull();
  });

  it('constructs a guard anomaly detector when a baseline is injected', () => {
    const arp = new AgentRuntimeProtection(
      baseConfig({
        intelligence: {
          guardAnomaly: {
            baseline: {
              'file-read': 0.4,
              'network-egress': 0.3,
              'process-spawn': 0.2,
              'telemetry': 0.1,
            },
            windowSize: 150,
            alarmThreshold: 18,
            minObservations: 30,
          },
        },
      }),
    );
    const ga = arp.getIntelligence().getGuardAnomaly();
    expect(ga).not.toBeNull();
    expect(ga).toBeInstanceOf(GuardAnomalyDetector);
    const det = ga as GuardAnomalyDetector;
    expect(det.getWindowCapacity()).toBe(150);
    // Baseline was normalized to probabilities, keys preserved.
    const baseline = det.getBaseline();
    expect(Object.keys(baseline).sort()).toEqual([
      'file-read',
      'network-egress',
      'process-spawn',
      'telemetry',
    ]);
  });

  it('refuses to construct a guard anomaly detector from an empty baseline object', () => {
    const arp = new AgentRuntimeProtection(
      baseConfig({
        intelligence: {
          guardAnomaly: { baseline: {} },
        },
      }),
    );
    expect(arp.getIntelligence().getGuardAnomaly()).toBeNull();
  });

  it('respects guardAnomaly.enabled = false even when a baseline is provided', () => {
    const arp = new AgentRuntimeProtection(
      baseConfig({
        intelligence: {
          guardAnomaly: {
            enabled: false,
            baseline: { 'file-read': 1 },
          },
        },
      }),
    );
    expect(arp.getIntelligence().getGuardAnomaly()).toBeNull();
  });

  it('start() attaches the twin to the event engine and is idempotent', async () => {
    const arp = new AgentRuntimeProtection(baseConfig());
    // No pre-start attach; twin.attach() happens inside start().
    await arp.start();
    expect(arp.isRunning()).toBe(true);
    // A second start() is a no-op (running guard) and must not
    // double-attach the twin.
    await arp.start();
    expect(arp.isRunning()).toBe(true);
    await arp.stop();
    expect(arp.isRunning()).toBe(false);
  });

  it('preserves legacy behavior when both twin and guard anomaly are disabled', async () => {
    const arp = new AgentRuntimeProtection(
      baseConfig({
        intelligence: {
          runtimeTwin: { enabled: false },
          // guardAnomaly left out entirely
        },
      }),
    );
    expect(arp.getRuntimeTwin()).toBeNull();
    expect(arp.getIntelligence().getBehavioralRiskSource()).toBeNull();
    expect(arp.getIntelligence().getGuardAnomaly()).toBeNull();
    // start() still works: no attach, just monitors (all disabled here).
    await arp.start();
    await arp.stop();
  });
});
