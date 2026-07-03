import { describe, it, expect } from 'vitest';
import { canonicalShape, behavioralHash, HASH_SHAPE_VERSION } from './behavioral-hash';
import type { RedactedSignal } from './redaction';

const signal: RedactedSignal = {
  tacticId: 'exfiltration',
  techniqueId: 'ATM-T8002',
  actionClass: 'network_egress',
  targetClass: 'network',
  sequencePattern: '',
  outcomeClass: 'blocked',
  severity: 'high',
};

describe('behavioral hash', () => {
  it('canonical is version-prefixed, lowercased, pipe-separated, fixed order', () => {
    expect(canonicalShape(signal)).toBe(
      `${HASH_SHAPE_VERSION}|exfiltration|atm-t8002|network_egress|network||blocked`,
    );
  });

  it('is 64 lowercase hex characters (SHA-256) — matches the registry floor', () => {
    expect(behavioralHash(signal)).toMatch(/^[a-f0-9]{64}$/);
  });

  it('is deterministic for identical shapes', () => {
    expect(behavioralHash({ ...signal })).toBe(behavioralHash({ ...signal }));
  });

  it('differs when any structural token differs', () => {
    expect(behavioralHash({ ...signal, outcomeClass: 'detected' })).not.toBe(behavioralHash(signal));
    expect(behavioralHash({ ...signal, techniqueId: 'ATM-T8003' })).not.toBe(behavioralHash(signal));
  });

  it('does not depend on severity (severity is not a hash input)', () => {
    expect(behavioralHash({ ...signal, severity: 'low' })).toBe(behavioralHash(signal));
  });
});
