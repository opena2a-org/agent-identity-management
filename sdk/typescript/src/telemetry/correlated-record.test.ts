/**
 * Tests for the correlated-record schema, builder, and shared-indicator
 * reduction. The central guarantee under test: only the enforcement fact is
 * observed; detection/intent are inferences; and the shared indicator never
 * carries identifying or sensitive fields.
 */
import { describe, it, expect } from 'vitest';
import {
  buildCorrelatedRecord,
  assertObservedInvariant,
  toSharedIndicator,
  TELEMETRY_SCHEMA_VERSION,
  type BuildCorrelatedRecordInput,
} from './correlated-record';

const baseInput = (): BuildCorrelatedRecordInput => ({
  correlationId: 'cde_000000000_aabbccddeeff',
  agentId: 'agent-123',
  enforcement: {
    decision: 'deny',
    outcome: 'DENY_INTENT',
    deniedBy: 'capability_check',
    deniedReason: 'capability not granted',
    capability: 'fs:write',
    resource: '/etc/secret.conf',
    credentialRef: 'cred:abc',
    occurredAt: '2026-06-05T00:00:00.000Z',
    source: 'aim-pdp',
  },
  intent: {
    intentClass: 'data_exfiltration',
    confidence: 0.72,
    blocked: true,
    source: 'nanomind-intent',
  },
  detection: {
    injectionDetected: true,
    attackClass: 'indirect',
    techniqueId: 'T-2002',
    techniqueSource: 'interim-mapping',
    confidence: 0.84,
    detector: 'nanomind-guard',
    inputRef: 'sha256:deadbeef',
    detectedAt: '2026-06-05T00:00:00.000Z',
  },
});

describe('buildCorrelatedRecord', () => {
  it('marks only enforcement as observed', () => {
    const r = buildCorrelatedRecord(baseInput());
    expect(r.enforcement.observed).toBe(true);
    expect(r.intent?.observed).toBe(false);
    expect(r.detection?.observed).toBe(false);
    expect(r.detection?.attribution).toBe('inferred');
    expect(() => assertObservedInvariant(r)).not.toThrow();
  });

  it('stamps schema version, record id, and createdAt', () => {
    const r = buildCorrelatedRecord(baseInput());
    expect(r.schemaVersion).toBe(TELEMETRY_SCHEMA_VERSION);
    expect(r.recordId).toBeTruthy();
    expect(Date.parse(r.createdAt)).not.toBeNaN();
  });

  it('preserves the technique source for the APIA-agnostic path', () => {
    const r = buildCorrelatedRecord(baseInput());
    expect(r.detection?.techniqueSource).toBe('interim-mapping');
    expect(r.detection?.techniqueId).toBe('T-2002');
  });

  it('derives completeness from the present parts', () => {
    expect(buildCorrelatedRecord(baseInput()).assembly.completeness).toBe('full');

    const noDetection = { ...baseInput(), detection: undefined };
    expect(buildCorrelatedRecord(noDetection).assembly.completeness).toBe(
      'partial-missing-detection'
    );

    const noIntent = { ...baseInput(), intent: undefined };
    expect(buildCorrelatedRecord(noIntent).assembly.completeness).toBe('partial-missing-intent');

    const enforcementOnly = { ...baseInput(), intent: undefined, detection: undefined };
    expect(buildCorrelatedRecord(enforcementOnly).assembly.completeness).toBe(
      'partial-missing-both'
    );
  });

  it('defaults the join key to correlationId', () => {
    expect(buildCorrelatedRecord(baseInput()).assembly.joinedBy).toBe('correlationId');
    const fallback = { ...baseInput(), joinedBy: 'fallback:agentId+capability+window' };
    expect(buildCorrelatedRecord(fallback).assembly.joinedBy).toBe(
      'fallback:agentId+capability+window'
    );
  });
});

describe('toSharedIndicator', () => {
  const ctx = {
    sensorToken: 'sha256:sensor',
    agentCategory: 'coding-assistant',
    daySinceInstall: 12,
    runtimeEnv: 'node',
  };

  it('never carries identifying or sensitive fields', () => {
    const record = buildCorrelatedRecord(baseInput());
    const indicator = toSharedIndicator(record, ctx);
    const keys = Object.keys(indicator);
    for (const forbidden of [
      'correlationId',
      'recordId',
      'agentId',
      'resource',
      'capability',
      'credentialRef',
      'inputRef',
      'deniedReason',
      'traceId',
    ]) {
      expect(keys).not.toContain(forbidden);
    }
    // And nothing nested leaks the raw values either.
    const serialized = JSON.stringify(indicator);
    expect(serialized).not.toContain('/etc/secret.conf');
    expect(serialized).not.toContain('cred:abc');
    expect(serialized).not.toContain('agent-123');
    expect(serialized).not.toContain('cde_');
  });

  it('carries the shareable signal', () => {
    const indicator = toSharedIndicator(buildCorrelatedRecord(baseInput()), ctx);
    expect(indicator.techniqueId).toBe('T-2002');
    expect(indicator.techniqueSource).toBe('interim-mapping');
    expect(indicator.detectionConfidence).toBe(0.84);
    expect(indicator.enforcementOutcome).toBe('deny');
    expect(indicator.sensorToken).toBe('sha256:sensor');
  });

  it('derives event type from injection + decision', () => {
    const denied = toSharedIndicator(buildCorrelatedRecord(baseInput()), ctx);
    expect(denied.eventType).toBe('denied_injection_attempt');

    const allowInput = baseInput();
    allowInput.enforcement.decision = 'allow';
    allowInput.enforcement.outcome = 'ALLOW';
    const allowed = toSharedIndicator(buildCorrelatedRecord(allowInput), ctx);
    expect(allowed.eventType).toBe('allow_action');
  });
});
