import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import * as os from 'os';
import * as path from 'path';
import * as fs from 'fs';
import { IntelligenceCoordinator } from '../../../src/arp/intelligence/coordinator';
import { EventEngine } from '../../../src/arp/engine/event-engine';
import { ARPProxy } from '../../../src/arp/proxy/server';
import type { ClassificationAnnotator } from '../../../src/arp/intelligence/classification-annotator';
import type {
  ARPConfig,
  ARPEvent,
  CapabilityManifest,
} from '../../../src/arp/types';

/**
 * The load-bearing constraint: populating
 * `event.data.classification` must NOT, by itself, turn the comply gate into a
 * hot-path deny control. Enforcement is OPT-IN behind `comply.enforce`, default
 * OFF. These tests prove that a classified event that WOULD be denied under
 * enforcement passes straight through on the default path, and that the proxy
 * wires a detection coordinator without enabling the deny path.
 */

function baseConfig(): ARPConfig {
  return {
    agentName: 'detection-mode-test-agent',
    intelligence: { enabled: false }, // L2 inert; isolate the comply gate
  };
}

function classifiedEvent(classification: string): ARPEvent {
  return {
    id: `evt-${Math.random().toString(36).slice(2)}`,
    timestamp: new Date().toISOString(),
    source: 'prompt',
    category: 'normal',
    severity: 'info',
    description: 'classified event',
    data: { classification },
    classifiedBy: 'L0-rules',
  };
}

function manifest(enforce?: boolean): CapabilityManifest {
  return {
    version: '1.0.0',
    agentId: 'fixture-agent',
    tier: 'execute',
    comply: {
      permitted_classes: ['code-generation'],
      prohibited_classes: ['credential-access'],
      on_violation: 'deny',
      ...(enforce === undefined ? {} : { enforce }),
    },
    issuedAt: '2026-04-13T00:00:00.000Z',
    ed25519PublicKey: 'unused',
    mldsa65PublicKey: 'unused',
  };
}

describe('comply gate is detection-only by default', () => {
  let tmpDir: string;
  beforeEach(() => {
    tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'coord-detect-'));
  });
  afterEach(() => {
    fs.rmSync(tmpDir, { recursive: true, force: true });
  });

  it('does NOT deny a prohibited class when enforce is omitted (default off)', async () => {
    const coord = new IntelligenceCoordinator(baseConfig(), tmpDir, manifest(/* enforce omitted */));
    const event = classifiedEvent('credential-access');

    await coord.analyze(event);

    // Classification is present (detection), but the gate took no action.
    expect(event.data.classification).toBe('credential-access');
    expect(event.classifiedBy).not.toBe('L0-comply');
    expect(event.data.comply).toBeUndefined();
    expect(event.category).toBe('normal');
    expect(event.severity).toBe('info');
  });

  it('does NOT deny an unknown class when enforce is explicitly false', async () => {
    const coord = new IntelligenceCoordinator(baseConfig(), tmpDir, manifest(false));
    const event = classifiedEvent('totally-unknown-class');

    await coord.analyze(event);

    expect(event.classifiedBy).not.toBe('L0-comply');
    expect(event.data.comply).toBeUndefined();
  });

  it('DOES deny when the operator opts in with enforce=true (sanity)', async () => {
    const coord = new IntelligenceCoordinator(baseConfig(), tmpDir, manifest(true));
    const event = classifiedEvent('credential-access');

    await coord.analyze(event);

    expect(event.classifiedBy).toBe('L0-comply');
    expect(event.category).toBe('threat');
    expect(event.severity).toBe('critical');
  });
});

describe('ARPProxy detection-mode wiring', () => {
  it('hands its manifest to the coordinator and tees events without denying', async () => {
    const config: ARPConfig = baseConfig();
    const engine = new EventEngine(config);
    const coordinator = new IntelligenceCoordinator(config, '.', null);

    const teed: ARPEvent[] = [];
    const proxy = new ARPProxy(
      { port: 0, upstreams: [] },
      {
        engine,
        coordinator,
        onInScopeEvent: (e) => teed.push(e),
      },
    );

    await proxy.start();
    try {
      // No manifestPath configured → the proxy hands null to the coordinator
      // (it owns the lifecycle). Detection still runs.
      expect(coordinator.getCapabilityManifest()).toBeNull();

      const emitted = await engine.emit({
        source: 'prompt',
        category: 'normal',
        severity: 'info',
        description: 'classified event',
        data: { classification: 'credential-access' },
      });
      // Let the detached analyze() settle.
      await new Promise((r) => setImmediate(r));

      // The sequence tee fired, and nothing denied the event.
      expect(teed).toHaveLength(1);
      expect(teed[0].data.classification).toBe('credential-access');
      expect(emitted.classifiedBy).not.toBe('L0-comply');
      expect(emitted.data.comply).toBeUndefined();
    } finally {
      await proxy.stop();
    }
  });

  it('runs the coordinator AFTER annotation so the comply gate sees the label (no fail-open)', async () => {
    // Regression guard: if analyze() ran before the async annotator populated
    // the classification, an enforce=true operator path would read a null
    // label and silently fail open. The proxy must annotate first, then analyze.
    const config: ARPConfig = baseConfig();
    const engine = new EventEngine(config);

    // Stub annotator: writes the classification (as a real annotator would on a
    // verified result), THEN fires onDone — exactly the buffered contract.
    const annotatorStub = {
      enqueue(event: ARPEvent, onDone?: () => void) {
        event.data.classification = 'credential-access';
        onDone?.();
      },
    } as unknown as ClassificationAnnotator;

    // Spy coordinator: record the classification value AT analyze() time.
    const seenAtAnalyze: Array<unknown> = [];
    const coordinatorSpy = {
      setCapabilityManifest() {},
      analyze(event: ARPEvent) {
        seenAtAnalyze.push(event.data.classification);
        return Promise.resolve(null);
      },
    } as unknown as IntelligenceCoordinator;

    const proxy = new ARPProxy(
      { port: 0, upstreams: [] },
      { engine, coordinator: coordinatorSpy, annotator: annotatorStub, onInScopeEvent: () => {} },
    );

    await proxy.start();
    try {
      await engine.emit({
        source: 'prompt',
        category: 'normal',
        severity: 'info',
        description: 'classified event',
        data: {},
      });
      await new Promise((r) => setImmediate(r));

      // analyze() observed the label the annotator wrote — not null.
      expect(seenAtAnalyze).toEqual(['credential-access']);
    } finally {
      await proxy.stop();
    }
  });
});
