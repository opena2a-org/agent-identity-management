import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import type { ARPEvent } from '../../../src/arp/types';
import {
  isAnomalousEvent,
  buildGTINPayload,
  mapEventType,
  generateSensorToken,
} from '../../../src/arp/telemetry/gtin';
import { GTINForwarder } from '../../../src/arp/telemetry/forwarder';

// --- Helpers ---

function makeEvent(overrides: Partial<ARPEvent> = {}): ARPEvent {
  return {
    id: 'test-event-001',
    timestamp: new Date().toISOString(),
    source: 'network',
    category: 'anomaly',
    severity: 'medium',
    description: 'Test anomalous event',
    data: {},
    classifiedBy: 'L0-rules',
    ...overrides,
  };
}

// --- isAnomalousEvent ---

describe('isAnomalousEvent', () => {
  it('returns true for anomaly category', () => {
    expect(isAnomalousEvent(makeEvent({ category: 'anomaly' }))).toBe(true);
  });

  it('returns true for violation category', () => {
    expect(isAnomalousEvent(makeEvent({ category: 'violation' }))).toBe(true);
  });

  it('returns true for threat category', () => {
    expect(isAnomalousEvent(makeEvent({ category: 'threat' }))).toBe(true);
  });

  it('returns false for normal category', () => {
    expect(isAnomalousEvent(makeEvent({ category: 'normal' }))).toBe(false);
  });
});

// --- mapEventType ---

describe('mapEventType', () => {
  it('maps network source to unexpected_network', () => {
    const event = makeEvent({ source: 'network', data: {} });
    expect(mapEventType(event)).toBe('unexpected_network');
  });

  it('maps network source with dns data to unexpected_dns', () => {
    const event = makeEvent({ source: 'network', data: { dns: true } });
    expect(mapEventType(event)).toBe('unexpected_dns');
  });

  it('maps network source with dnsLookup to unexpected_dns', () => {
    const event = makeEvent({ source: 'network', data: { dnsLookup: 'evil.com' } });
    expect(mapEventType(event)).toBe('unexpected_dns');
  });

  it('maps network source with type=dns to unexpected_dns', () => {
    const event = makeEvent({ source: 'network', data: { type: 'dns' } });
    expect(mapEventType(event)).toBe('unexpected_dns');
  });

  it('maps process source with eval in data to eval_detected', () => {
    const event = makeEvent({ source: 'process', data: { eval: true } });
    expect(mapEventType(event)).toBe('eval_detected');
  });

  it('maps process source with eval in command to eval_detected', () => {
    const event = makeEvent({ source: 'process', data: { command: 'node -e "eval(code)"' } });
    expect(mapEventType(event)).toBe('eval_detected');
  });

  it('maps mcp-protocol to capability_escalation', () => {
    const event = makeEvent({ source: 'mcp-protocol', data: {} });
    expect(mapEventType(event)).toBe('capability_escalation');
  });

  it('maps a2a-protocol to capability_escalation', () => {
    const event = makeEvent({ source: 'a2a-protocol', data: {} });
    expect(mapEventType(event)).toBe('capability_escalation');
  });

  it('maps sequence detection to suspicious_sequence', () => {
    const event = makeEvent({ source: 'network', data: { sequence: true } });
    expect(mapEventType(event)).toBe('suspicious_sequence');
  });

  it('maps idle activation to idle_activation', () => {
    const event = makeEvent({ source: 'process', data: { idle: true } });
    expect(mapEventType(event)).toBe('idle_activation');
  });

  it('maps filesystem with .env path to env_probe', () => {
    const event = makeEvent({ source: 'filesystem', data: { path: '/app/.env' } });
    expect(mapEventType(event)).toBe('env_probe');
  });

  it('maps filesystem source to env_probe', () => {
    const event = makeEvent({ source: 'filesystem', data: {} });
    expect(mapEventType(event)).toBe('env_probe');
  });

  it('maps prompt source to capability_escalation', () => {
    const event = makeEvent({ source: 'prompt', data: {} });
    expect(mapEventType(event)).toBe('capability_escalation');
  });
});

// --- buildGTINPayload ---

describe('buildGTINPayload', () => {
  it('builds payload with required fields', () => {
    const event = makeEvent({ source: 'network', category: 'anomaly' });
    const payload = buildGTINPayload(event, 'test-agent', '1.0.0');

    expect(payload.sensorToken).toBeTruthy();
    expect(payload.eventType).toBe('unexpected_network');
    expect(payload.packageName).toBe('test-agent');
    expect(payload.packageVersion).toBe('1.0.0');
    expect(payload.runtimeEnv).toBe('node');
    expect(payload.triggeredAt).toBe(event.timestamp);
    expect(typeof payload.daySinceInstall).toBe('number');
    expect(payload.daySinceInstall).toBeGreaterThanOrEqual(0);
  });

  it('defaults packageVersion to empty string', () => {
    const event = makeEvent();
    const payload = buildGTINPayload(event, 'test-agent');
    expect(payload.packageVersion).toBe('');
  });

  it('contains only allowed fields (no PII)', () => {
    const event = makeEvent({
      source: 'network',
      data: {
        host: 'evil.com',
        filePath: '/etc/passwd',
        credentials: 'secret123',
        command: 'rm -rf /',
        conversation: 'user said something private',
      },
    });

    const payload = buildGTINPayload(event, 'test-agent', '1.0.0');

    const allowedKeys = new Set([
      'sensorToken',
      'eventType',
      'packageName',
      'packageVersion',
      'daySinceInstall',
      'runtimeEnv',
      'triggeredAt',
    ]);
    const payloadKeys = new Set(Object.keys(payload));
    expect(payloadKeys).toEqual(allowedKeys);

    const payloadStr = JSON.stringify(payload);
    expect(payloadStr).not.toContain('evil.com');
    expect(payloadStr).not.toContain('/etc/passwd');
    expect(payloadStr).not.toContain('secret123');
    expect(payloadStr).not.toContain('rm -rf');
    expect(payloadStr).not.toContain('user said something private');
  });
});

// --- generateSensorToken ---

describe('generateSensorToken', () => {
  it('returns a valid SHA-256 hex string (64 chars)', () => {
    const token = generateSensorToken();
    expect(token).toMatch(/^[a-f0-9]{64}$/);
  });

  it('returns stable token across calls', () => {
    const token1 = generateSensorToken();
    const token2 = generateSensorToken();
    expect(token1).toBe(token2);
  });
});

// --- GTINForwarder ---

describe('GTINForwarder', () => {
  let forwarder: GTINForwarder;

  beforeEach(() => {
    forwarder = new GTINForwarder({
      enabled: true,
      sensorToken: 'a'.repeat(64),
      packageName: 'test-agent',
      packageVersion: '1.0.0',
      registryUrl: 'http://localhost:9999',
    });
  });

  afterEach(async () => {
    await forwarder.shutdown();
  });

  it('only queues anomalous events', () => {
    forwarder.onEvent(makeEvent({ category: 'normal' }));
    expect(forwarder.getQueueLength()).toBe(0);

    forwarder.onEvent(makeEvent({ category: 'anomaly' }));
    expect(forwarder.getQueueLength()).toBe(1);

    forwarder.onEvent(makeEvent({ category: 'violation' }));
    expect(forwarder.getQueueLength()).toBe(2);

    forwarder.onEvent(makeEvent({ category: 'threat' }));
    expect(forwarder.getQueueLength()).toBe(3);
  });

  it('does not queue events when disabled', () => {
    const disabled = new GTINForwarder({
      enabled: false,
      sensorToken: 'a'.repeat(64),
      packageName: 'test-agent',
    });

    disabled.onEvent(makeEvent({ category: 'anomaly' }));
    expect(disabled.getQueueLength()).toBe(0);
  });

  it('drains queue on flush', async () => {
    forwarder.onEvent(makeEvent({ category: 'anomaly' }));
    forwarder.onEvent(makeEvent({ category: 'violation' }));
    expect(forwarder.getQueueLength()).toBe(2);

    await forwarder.flush();
    expect(forwarder.getQueueLength()).toBe(0);
  });

  it('drains queue on shutdown', async () => {
    forwarder.onEvent(makeEvent({ category: 'anomaly' }));
    expect(forwarder.getQueueLength()).toBe(1);

    await forwarder.shutdown();
    expect(forwarder.getQueueLength()).toBe(0);
  });

  it('does not accept events after shutdown', async () => {
    await forwarder.shutdown();

    forwarder.onEvent(makeEvent({ category: 'anomaly' }));
    expect(forwarder.getQueueLength()).toBe(0);
  });

  it('reports running state correctly', async () => {
    expect(forwarder.isRunning()).toBe(true);

    await forwarder.shutdown();
    expect(forwarder.isRunning()).toBe(false);
  });
});
