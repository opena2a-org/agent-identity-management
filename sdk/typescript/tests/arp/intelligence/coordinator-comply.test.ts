import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import * as os from 'os';
import * as path from 'path';
import * as fs from 'fs';
import { IntelligenceCoordinator } from '../../../src/arp/intelligence/coordinator';
import type {
  ARPConfig,
  ARPEvent,
  CapabilityManifest,
  ComplyOnViolation,
} from '../../../src/arp/types';

/**
 * Unit coverage for the L0-comply gate on IntelligenceCoordinator.
 *
 * The gate fires before the L1 anomaly detector on every classified event.
 * When a classification lands in `prohibited_classes` it denies with reason
 * `prohibited`. When it is not in `permitted_classes` and not in
 * `prohibited_classes` it denies as `unknown` (parse-to-deny, CR-001).
 * Unclassified events are passed through unchanged. A coordinator with no
 * manifest behaves identically to the pre-wiring implementation.
 */

function baseConfig(): ARPConfig {
  return {
    agentName: 'test-agent',
    intelligence: {
      enabled: false, // keeps L2 inert; we only care about L0-comply mutations
    },
  };
}

function makeClassifiedEvent(classification: string | null): ARPEvent {
  return {
    id: `evt-${Math.random().toString(36).slice(2)}`,
    timestamp: new Date().toISOString(),
    source: 'prompt',
    category: 'normal',
    severity: 'info',
    description: 'test event',
    data: classification === null ? {} : { classification },
    classifiedBy: 'L0-rules',
  };
}

function makeManifest(
  overrides: Partial<CapabilityManifest['comply']> = {},
  on_violation: ComplyOnViolation = 'deny',
): CapabilityManifest {
  return {
    version: '1.0.0',
    agentId: 'fixture-agent',
    tier: 'execute',
    comply: {
      permitted_classes: ['code-generation', 'documentation'],
      prohibited_classes: ['credential-access'],
      on_violation,
      // These cases assert active enforcement, so they opt into it explicitly.
      // The default (enforce omitted/false) is detection-only and is covered
      // by coordinator-detection-mode.test.ts.
      enforce: true,
      ...overrides,
    },
    issuedAt: '2026-04-13T00:00:00.000Z',
    ed25519PublicKey: 'unused-in-unit-test',
    mldsa65PublicKey: 'unused-in-unit-test',
  };
}

describe('IntelligenceCoordinator comply gate', () => {
  let tmpDir: string;

  beforeEach(() => {
    tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'coord-comply-'));
  });

  afterEach(() => {
    fs.rmSync(tmpDir, { recursive: true, force: true });
  });

  it('passes a permitted classification through unchanged', async () => {
    const coord = new IntelligenceCoordinator(
      baseConfig(),
      tmpDir,
      makeManifest(),
    );

    const event = makeClassifiedEvent('code-generation');
    const before = { ...event };

    const assessment = await coord.analyze(event);

    expect(assessment).toBeNull();
    expect(event.category).toBe(before.category);
    expect(event.severity).toBe(before.severity);
    expect(event.classifiedBy).not.toBe('L0-comply');
    expect(event.data.comply).toBeUndefined();
  });

  it('denies a prohibited classification with reason=prohibited', async () => {
    const coord = new IntelligenceCoordinator(
      baseConfig(),
      tmpDir,
      makeManifest({}, 'deny'),
    );

    const event = makeClassifiedEvent('credential-access');
    await coord.analyze(event);

    expect(event.classifiedBy).toBe('L0-comply');
    expect(event.category).toBe('threat');
    expect(event.severity).toBe('critical');
    const decision = event.data.comply as {
      classification: string;
      reason: string;
      action: string;
    };
    expect(decision).toMatchObject({
      classification: 'credential-access',
      reason: 'prohibited',
      action: 'deny',
    });
  });

  it('denies an unknown classification as parse-to-deny', async () => {
    const coord = new IntelligenceCoordinator(
      baseConfig(),
      tmpDir,
      makeManifest({}, 'kill'),
    );

    const event = makeClassifiedEvent('unexpected-class');
    await coord.analyze(event);

    expect(event.classifiedBy).toBe('L0-comply');
    const decision = event.data.comply as {
      classification: string;
      reason: string;
      action: string;
    };
    expect(decision.reason).toBe('unknown');
    expect(decision.action).toBe('kill');
    expect(event.category).toBe('threat');
    expect(event.severity).toBe('critical');
  });

  it('prohibited takes precedence over permitted when a class is on both lists', async () => {
    const coord = new IntelligenceCoordinator(
      baseConfig(),
      tmpDir,
      makeManifest({
        permitted_classes: ['shared-class'],
        prohibited_classes: ['shared-class'],
      }),
    );

    const event = makeClassifiedEvent('shared-class');
    await coord.analyze(event);

    const decision = event.data.comply as { reason: string };
    expect(decision.reason).toBe('prohibited');
  });

  it('log action does not mutate category or severity', async () => {
    const coord = new IntelligenceCoordinator(
      baseConfig(),
      tmpDir,
      makeManifest({}, 'log'),
    );

    const event = makeClassifiedEvent('credential-access');
    await coord.analyze(event);

    expect(event.classifiedBy).toBe('L0-comply');
    expect(event.category).toBe('normal');
    expect(event.severity).toBe('info');
    const decision = event.data.comply as { action: string };
    expect(decision.action).toBe('log');
  });

  it('alert action raises to violation/medium but does not lower', async () => {
    const coord = new IntelligenceCoordinator(
      baseConfig(),
      tmpDir,
      makeManifest({}, 'alert'),
    );

    const event = makeClassifiedEvent('credential-access');
    event.category = 'threat';
    event.severity = 'critical';

    await coord.analyze(event);

    expect(event.category).toBe('threat');
    expect(event.severity).toBe('critical');
  });

  it('pause action raises severity to high', async () => {
    const coord = new IntelligenceCoordinator(
      baseConfig(),
      tmpDir,
      makeManifest({}, 'pause'),
    );

    const event = makeClassifiedEvent('credential-access');
    await coord.analyze(event);

    expect(event.category).toBe('violation');
    expect(event.severity).toBe('high');
  });

  it('passes unclassified events straight through the gate', async () => {
    const coord = new IntelligenceCoordinator(
      baseConfig(),
      tmpDir,
      makeManifest(),
    );

    const event = makeClassifiedEvent(null);
    const assessment = await coord.analyze(event);

    expect(assessment).toBeNull();
    expect(event.classifiedBy).not.toBe('L0-comply');
    expect(event.data.comply).toBeUndefined();
  });

  it('passes all events through unchanged when no manifest is installed', async () => {
    const coord = new IntelligenceCoordinator(baseConfig(), tmpDir);

    const allowed = makeClassifiedEvent('code-generation');
    const prohibited = makeClassifiedEvent('credential-access');
    const unknown = makeClassifiedEvent('totally-unknown');

    await coord.analyze(allowed);
    await coord.analyze(prohibited);
    await coord.analyze(unknown);

    for (const e of [allowed, prohibited, unknown]) {
      expect(e.classifiedBy).not.toBe('L0-comply');
      expect(e.data.comply).toBeUndefined();
    }
  });

  it('setCapabilityManifest hot-swaps the installed manifest', async () => {
    const coord = new IntelligenceCoordinator(baseConfig(), tmpDir);
    expect(coord.getCapabilityManifest()).toBeNull();

    coord.setCapabilityManifest(makeManifest());
    const denied = makeClassifiedEvent('credential-access');
    await coord.analyze(denied);
    expect(denied.classifiedBy).toBe('L0-comply');

    coord.setCapabilityManifest(null);
    const passthrough = makeClassifiedEvent('credential-access');
    await coord.analyze(passthrough);
    expect(passthrough.classifiedBy).not.toBe('L0-comply');
    expect(passthrough.data.comply).toBeUndefined();
  });
});
