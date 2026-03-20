import { describe, it, expect } from 'vitest';
import { generateKeyPair } from './ed25519';
import {
  publicKeyToDidKey,
  didKeyToPublicKey,
  canonicalJSONDeep,
  createDelegation,
  verifyDelegation,
  verifyDelegatorIdentity,
  verifyScopeNarrowing,
  verifyDelegationChain,
  exportDelegationChain,
  delegationSignablePayload,
  toBase64url,
  fromBase64url,
  base58btcEncode,
  base58btcDecode,
} from './delegation';

describe('Base58btc', () => {
  it('should roundtrip encode/decode', () => {
    const data = new Uint8Array([0xed, 0x01, 1, 2, 3, 4, 5]);
    const encoded = base58btcEncode(data);
    const decoded = base58btcDecode(encoded);
    expect(decoded).toEqual(data);
  });

  it('should handle leading zeros', () => {
    const data = new Uint8Array([0, 0, 0, 1, 2, 3]);
    const encoded = base58btcEncode(data);
    expect(encoded.startsWith('111')).toBe(true);
    const decoded = base58btcDecode(encoded);
    expect(decoded).toEqual(data);
  });
});

describe('Base64url', () => {
  it('should roundtrip encode/decode', () => {
    const data = new Uint8Array([255, 254, 253, 0, 1, 2]);
    expect(fromBase64url(toBase64url(data))).toEqual(data);
  });

  it('should not contain +, /, or = characters', () => {
    const data = new Uint8Array(64);
    for (let i = 0; i < 64; i++) data[i] = i * 4;
    const encoded = toBase64url(data);
    expect(encoded).not.toMatch(/[+/=]/);
  });
});

describe('did:key', () => {
  it('should create a valid did:key from Ed25519 public key', async () => {
    const kp = await generateKeyPair();
    const did = publicKeyToDidKey(kp.publicKey);
    expect(did).toMatch(/^did:key:z[1-9A-HJ-NP-Za-km-z]+$/);
  });

  it('should roundtrip publicKey -> did:key -> publicKey', async () => {
    const kp = await generateKeyPair();
    const did = publicKeyToDidKey(kp.publicKey);
    const recovered = didKeyToPublicKey(did);
    expect(recovered).toEqual(kp.publicKey);
  });

  it('should reject non-32-byte keys', () => {
    expect(() => publicKeyToDidKey(new Uint8Array(16))).toThrow('32 bytes');
  });

  it('should reject invalid did:key format', () => {
    expect(() => didKeyToPublicKey('did:web:example.com')).toThrow('Invalid did:key');
  });

  it('should produce deterministic DIDs for the same key', async () => {
    const kp = await generateKeyPair();
    expect(publicKeyToDidKey(kp.publicKey)).toBe(publicKeyToDidKey(kp.publicKey));
  });
});

describe('Canonical JSON', () => {
  it('should sort keys alphabetically', () => {
    expect(canonicalJSONDeep({ z: 1, a: 2, m: 3 })).toBe('{"a":2,"m":3,"z":1}');
  });

  it('should sort nested object keys', () => {
    expect(canonicalJSONDeep({ b: { z: 1, a: 2 }, a: 1 })).toBe('{"a":1,"b":{"a":2,"z":1}}');
  });

  it('should preserve array order', () => {
    expect(canonicalJSONDeep({ scopes: ['write', 'read'] })).toBe('{"scopes":["write","read"]}');
  });

  it('should produce compact JSON (no spaces)', () => {
    const result = canonicalJSONDeep({ key: 'value', arr: [1, 2] });
    expect(result).not.toMatch(/ /);
  });
});

describe('Delegation signing', () => {
  it('should create and verify a delegation', async () => {
    const root = await generateKeyPair();
    const coordinator = await generateKeyPair();

    const delegation = await createDelegation({
      delegatorKeyPair: root,
      delegatePublicKey: coordinator.publicKey,
      scopes: ['search', 'memory.read', 'memory.write'],
    });

    expect(delegation.delegator).toMatch(/^did:key:z/);
    expect(delegation.delegate).toMatch(/^did:key:z/);
    expect(delegation.signature).toBeDefined();
    expect(delegation.publicKey).toBeDefined();

    const valid = await verifyDelegation(delegation);
    expect(valid).toBe(true);
  });

  it('should fail verification with tampered scopes', async () => {
    const root = await generateKeyPair();
    const coordinator = await generateKeyPair();

    const delegation = await createDelegation({
      delegatorKeyPair: root,
      delegatePublicKey: coordinator.publicKey,
      scopes: ['search', 'memory.read'],
    });

    // Tamper with scopes
    delegation.scopes = ['search', 'memory.read', 'memory.write'];
    expect(await verifyDelegation(delegation)).toBe(false);
  });

  it('should fail verification with tampered delegate', async () => {
    const root = await generateKeyPair();
    const coordinator = await generateKeyPair();
    const attacker = await generateKeyPair();

    const delegation = await createDelegation({
      delegatorKeyPair: root,
      delegatePublicKey: coordinator.publicKey,
      scopes: ['search'],
    });

    delegation.delegate = publicKeyToDidKey(attacker.publicKey);
    expect(await verifyDelegation(delegation)).toBe(false);
  });

  it('should fail verification with wrong public key', async () => {
    const root = await generateKeyPair();
    const coordinator = await generateKeyPair();
    const other = await generateKeyPair();

    const delegation = await createDelegation({
      delegatorKeyPair: root,
      delegatePublicKey: coordinator.publicKey,
      scopes: ['search'],
    });

    delegation.publicKey = toBase64url(other.publicKey);
    expect(await verifyDelegation(delegation)).toBe(false);
  });
});

describe('Delegator identity verification', () => {
  it('should pass when publicKey matches delegator DID', async () => {
    const root = await generateKeyPair();
    const coord = await generateKeyPair();

    const delegation = await createDelegation({
      delegatorKeyPair: root,
      delegatePublicKey: coord.publicKey,
      scopes: ['search'],
    });

    expect(verifyDelegatorIdentity(delegation)).toBe(true);
  });

  it('should fail when publicKey does not match delegator DID', async () => {
    const root = await generateKeyPair();
    const coord = await generateKeyPair();
    const other = await generateKeyPair();

    const delegation = await createDelegation({
      delegatorKeyPair: root,
      delegatePublicKey: coord.publicKey,
      scopes: ['search'],
    });

    // Swap in a different public key
    delegation.publicKey = toBase64url(other.publicKey);
    expect(verifyDelegatorIdentity(delegation)).toBe(false);
  });
});

describe('Scope narrowing', () => {
  it('should accept strict subset', async () => {
    const root = await generateKeyPair();
    const coord = await generateKeyPair();
    const researcher = await generateKeyPair();

    const d1 = await createDelegation({
      delegatorKeyPair: root,
      delegatePublicKey: coord.publicKey,
      scopes: ['search', 'memory.read', 'memory.write'],
    });

    const d2 = await createDelegation({
      delegatorKeyPair: coord,
      delegatePublicKey: researcher.publicKey,
      scopes: ['search', 'memory.read'],
      parentDelegation: 'del-1',
    });

    expect(verifyScopeNarrowing(d1, d2)).toBe(true);
  });

  it('should reject scope escalation', async () => {
    const root = await generateKeyPair();
    const coord = await generateKeyPair();
    const researcher = await generateKeyPair();

    const d1 = await createDelegation({
      delegatorKeyPair: root,
      delegatePublicKey: coord.publicKey,
      scopes: ['search', 'memory.read'],
    });

    const d2 = await createDelegation({
      delegatorKeyPair: coord,
      delegatePublicKey: researcher.publicKey,
      scopes: ['search', 'memory.read', 'memory.write'],
    });

    expect(verifyScopeNarrowing(d1, d2)).toBe(false);
  });
});

describe('Delegation chain verification', () => {
  it('should reject an empty chain', async () => {
    const result = await verifyDelegationChain([]);
    expect(result.valid).toBe(false);
    expect(result.results[0].error).toMatch(/Empty/i);
  });

  it('should verify a valid 3-hop chain', async () => {
    const human = await generateKeyPair();
    const coordinator = await generateKeyPair();
    const researcher = await generateKeyPair();

    const d1 = await createDelegation({
      delegatorKeyPair: human,
      delegatePublicKey: coordinator.publicKey,
      scopes: ['search', 'memory.read', 'memory.write', 'delegate'],
    });

    const d2 = await createDelegation({
      delegatorKeyPair: coordinator,
      delegatePublicKey: researcher.publicKey,
      scopes: ['search', 'memory.read'],
      parentDelegation: 'del-1',
    });

    const result = await verifyDelegationChain([d1, d2]);
    expect(result.valid).toBe(true);
    expect(result.results).toHaveLength(2);
    expect(result.results[0].signatureValid).toBe(true);
    expect(result.results[1].signatureValid).toBe(true);
    expect(result.results[1].scopeValid).toBe(true);
  });

  it('should reject broken chain linkage', async () => {
    const human = await generateKeyPair();
    const coordinator = await generateKeyPair();
    const researcher = await generateKeyPair();
    const unrelated = await generateKeyPair();

    const d1 = await createDelegation({
      delegatorKeyPair: human,
      delegatePublicKey: coordinator.publicKey,
      scopes: ['search', 'memory.read'],
    });

    // d2 is signed by unrelated key, not coordinator
    const d2 = await createDelegation({
      delegatorKeyPair: unrelated,
      delegatePublicKey: researcher.publicKey,
      scopes: ['search'],
    });

    const result = await verifyDelegationChain([d1, d2]);
    expect(result.valid).toBe(false);
    expect(result.results[1].error).toMatch(/Chain broken/);
  });

  it('should reject scope escalation in chain', async () => {
    const human = await generateKeyPair();
    const coordinator = await generateKeyPair();
    const researcher = await generateKeyPair();

    const d1 = await createDelegation({
      delegatorKeyPair: human,
      delegatePublicKey: coordinator.publicKey,
      scopes: ['search'],
    });

    const d2 = await createDelegation({
      delegatorKeyPair: coordinator,
      delegatePublicKey: researcher.publicKey,
      scopes: ['search', 'memory.write'],
    });

    const result = await verifyDelegationChain([d1, d2]);
    expect(result.valid).toBe(false);
    expect(result.results[1].scopeValid).toBe(false);
  });
});

describe('Delegation chain export', () => {
  it('should produce valid interop format', async () => {
    const human = await generateKeyPair();
    const coordinator = await generateKeyPair();
    const researcher = await generateKeyPair();

    const d1 = await createDelegation({
      delegatorKeyPair: human,
      delegatePublicKey: coordinator.publicKey,
      scopes: ['search', 'memory.read', 'memory.write', 'delegate'],
    });

    const d2 = await createDelegation({
      delegatorKeyPair: coordinator,
      delegatePublicKey: researcher.publicKey,
      scopes: ['search', 'memory.read'],
      parentDelegation: 'del-1',
    });

    const exported = exportDelegationChain(
      [d1, d2],
      ['root_to_coordinator', 'coordinator_to_researcher']
    );

    expect(exported.engine).toBe('AIM');
    expect(exported.didMethod).toBe('did:key (Ed25519)');
    expect(exported.verification.algorithm).toBe('Ed25519');
    expect(exported.verification.encoding).toBe('base64url');
    expect(exported.chain).toHaveLength(2);
    expect(exported.chain[0].label).toBe('root_to_coordinator');
    expect(exported.chain[1].label).toBe('coordinator_to_researcher');
  });
});

describe('Signable payload', () => {
  it('should exclude signature and publicKey fields', async () => {
    const root = await generateKeyPair();
    const coord = await generateKeyPair();

    const delegation = await createDelegation({
      delegatorKeyPair: root,
      delegatePublicKey: coord.publicKey,
      scopes: ['search'],
    });

    const payload = new TextDecoder().decode(delegationSignablePayload(delegation));
    const parsed = JSON.parse(payload);

    expect(parsed).not.toHaveProperty('signature');
    expect(parsed).not.toHaveProperty('public_key');
    expect(parsed).not.toHaveProperty('publicKey');
    // Should have snake_case keys
    expect(parsed).toHaveProperty('created_at');
    expect(parsed).toHaveProperty('expires_at');
    expect(parsed).toHaveProperty('delegator');
    expect(parsed).toHaveProperty('delegate');
    expect(parsed).toHaveProperty('scopes');
  });

  it('should use snake_case field names', async () => {
    const root = await generateKeyPair();
    const coord = await generateKeyPair();

    const delegation = await createDelegation({
      delegatorKeyPair: root,
      delegatePublicKey: coord.publicKey,
      scopes: ['search'],
      parentDelegation: 'del-1',
    });

    const payload = new TextDecoder().decode(delegationSignablePayload(delegation));
    expect(payload).toContain('"parent_delegation"');
    expect(payload).not.toContain('"parentDelegation"');
  });

  it('should produce sorted, compact JSON', async () => {
    const root = await generateKeyPair();
    const coord = await generateKeyPair();

    const delegation = await createDelegation({
      delegatorKeyPair: root,
      delegatePublicKey: coord.publicKey,
      scopes: ['search'],
    });

    const payload = new TextDecoder().decode(delegationSignablePayload(delegation));
    // No spaces
    expect(payload).not.toMatch(/: /);
    expect(payload).not.toMatch(/, /);
    // Keys should be sorted
    const keys = Object.keys(JSON.parse(payload));
    expect(keys).toEqual([...keys].sort());
  });
});

describe('Cross-engine verification compatibility', () => {
  it('should produce verifiable chains that roundtrip through JSON serialization', async () => {
    const human = await generateKeyPair();
    const coordinator = await generateKeyPair();

    const delegation = await createDelegation({
      delegatorKeyPair: human,
      delegatePublicKey: coordinator.publicKey,
      scopes: ['search', 'memory.read'],
    });

    // Simulate what another engine would do: parse from JSON
    const json = JSON.stringify(delegation);
    const parsed = JSON.parse(json);

    expect(await verifyDelegation(parsed)).toBe(true);
  });

  it('did:key should resolve to correct public key bytes', async () => {
    const kp = await generateKeyPair();
    const did = publicKeyToDidKey(kp.publicKey);
    const recovered = didKeyToPublicKey(did);

    // Another engine using these bytes with Ed25519 should be able to verify
    expect(recovered.length).toBe(32);
    expect(recovered).toEqual(kp.publicKey);
  });
});
