/**
 * Tests for the async joiner. Deterministic: a mutable clock is injected and
 * records are captured via a sink array, so no timers or filesystem are used.
 */
import { describe, it, expect } from 'vitest';
import { CorrelationJoiner, isHighConfidence, type EnforcementEvent } from './joiner';
import type { CorrelatedRecord } from './correlated-record';

const CID = 'cde_000000000_aabbccddeeff';

const enforcement = (correlationId = CID): EnforcementEvent => ({
  correlationId,
  agentId: 'agent-1',
  enforcement: {
    decision: 'deny',
    outcome: 'DENY_INTENT',
    capability: 'fs:write',
    resource: '/etc/x',
    occurredAt: '2026-06-06T00:00:00.000Z',
    source: 'aim-pdp',
  },
});

const intent = (correlationId = CID) => ({
  correlationId,
  intent: { intentClass: 'exfil', confidence: 0.7, blocked: true, source: 'nanomind-intent' },
});

const detection = (correlationId = CID) => ({
  correlationId,
  detection: {
    injectionDetected: true,
    techniqueId: 'T-2002',
    techniqueSource: 'interim-mapping' as const,
    confidence: 0.84,
    detector: 'nanomind-guard',
    detectedAt: '2026-06-06T00:00:00.000Z',
  },
});

function harness(windowMs = 5000) {
  let clock = 1000;
  const records: CorrelatedRecord[] = [];
  const joiner = new CorrelationJoiner({
    windowMs,
    now: () => clock,
    onRecord: (r) => records.push(r),
  });
  return {
    joiner,
    records,
    advance: (ms: number) => {
      clock += ms;
    },
    setClock: (v: number) => {
      clock = v;
    },
  };
}

describe('CorrelationJoiner', () => {
  it('emits a full record immediately when all parts arrive (any order)', () => {
    const h = harness();
    h.joiner.ingestDetection(detection());
    h.joiner.ingestEnforcement(enforcement());
    h.joiner.ingestIntent(intent());

    expect(h.records).toHaveLength(1);
    expect(h.records[0].assembly.completeness).toBe('full');
    expect(h.records[0].assembly.joinedBy).toBe('correlationId');
    expect(h.joiner.pending).toBe(0);
    expect(isHighConfidence(h.records[0])).toBe(true);
  });

  it('does not emit before the window when parts are missing', () => {
    const h = harness();
    h.joiner.ingestEnforcement(enforcement());
    h.joiner.ingestIntent(intent());
    h.advance(4999);
    h.joiner.flushExpired();
    expect(h.records).toHaveLength(0);
    expect(h.joiner.pending).toBe(1);
  });

  it('emits a partial record after the window elapses', () => {
    const h = harness();
    h.joiner.ingestEnforcement(enforcement());
    h.joiner.ingestIntent(intent());
    h.advance(5000);
    h.joiner.flushExpired();

    expect(h.records).toHaveLength(1);
    expect(h.records[0].assembly.completeness).toBe('partial-missing-detection');
    expect(h.records[0].assembly.joinedBy).toBe('fallback:window');
    expect(isHighConfidence(h.records[0])).toBe(false);
    expect(h.joiner.pending).toBe(0);
  });

  it('emits enforcement-only as partial-missing-both', () => {
    const h = harness();
    h.joiner.ingestEnforcement(enforcement());
    h.advance(5000);
    h.joiner.flushExpired();
    expect(h.records).toHaveLength(1);
    expect(h.records[0].assembly.completeness).toBe('partial-missing-both');
  });

  it('drops orphans that never receive an enforcement fact', () => {
    const h = harness();
    h.joiner.ingestIntent(intent());
    h.joiner.ingestDetection(detection());
    h.advance(5000);
    h.joiner.flushExpired();
    expect(h.records).toHaveLength(0);
    expect(h.joiner.droppedOrphans).toBe(1);
    expect(h.joiner.pending).toBe(0);
  });

  it('does not double-emit after a full join', () => {
    const h = harness();
    h.joiner.ingestEnforcement(enforcement());
    h.joiner.ingestIntent(intent());
    h.joiner.ingestDetection(detection());
    expect(h.records).toHaveLength(1);
    h.advance(10000);
    h.joiner.flushExpired();
    expect(h.records).toHaveLength(1);
  });

  it('keeps separate correlation IDs independent', () => {
    const h = harness();
    h.joiner.ingestEnforcement(enforcement('cde_000000000_aaaaaaaaaaaa'));
    h.joiner.ingestEnforcement(enforcement('cde_000000000_bbbbbbbbbbbb'));
    h.joiner.ingestIntent(intent('cde_000000000_aaaaaaaaaaaa'));
    h.joiner.ingestDetection(detection('cde_000000000_aaaaaaaaaaaa'));
    expect(h.records).toHaveLength(1); // first id completed
    expect(h.joiner.pending).toBe(1); // second id still waiting
  });
});
