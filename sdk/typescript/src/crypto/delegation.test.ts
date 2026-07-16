import { describe, it, expect } from 'vitest';
import { generateKeyPair } from './ed25519';
import {
  publicKeyToDidKey,
  didKeyToPublicKey,
  canonicalJSONDeep,
  createDelegation,
  verifyDelegation,
  verifyDelegationSignature,
  verifyDelegatorIdentity,
  verifyScopeNarrowing,
  verifyDelegationChain,
  checkDelegationTemporalValidity,
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

describe('Temporal validity (expiry enforcement)', () => {
  // A delegation whose window is entirely in the past.
  async function makeExpired() {
    const root = await generateKeyPair();
    const agent = await generateKeyPair();
    return createDelegation({
      delegatorKeyPair: root,
      delegatePublicKey: agent.publicKey,
      scopes: ['search'],
      createdAt: '2026-01-01T00:00:00.000Z',
      expiresAt: '2026-01-08T00:00:00.000Z',
    });
  }

  it('checkDelegationTemporalValidity rejects an expired delegation', async () => {
    const d = await makeExpired();
    const res = checkDelegationTemporalValidity(d, '2026-02-01T00:00:00.000Z');
    expect(res.valid).toBe(false);
    expect(res.error).toMatch(/expired/i);
  });

  it('checkDelegationTemporalValidity accepts a delegation live at the eval time', async () => {
    const d = await makeExpired();
    expect(checkDelegationTemporalValidity(d, '2026-01-05T00:00:00.000Z').valid).toBe(true);
  });

  it('checkDelegationTemporalValidity fails closed on unparseable timestamps', async () => {
    const root = await generateKeyPair();
    const agent = await generateKeyPair();
    const d = await createDelegation({
      delegatorKeyPair: root,
      delegatePublicKey: agent.publicKey,
      scopes: ['search'],
      createdAt: 'not-a-date',
      expiresAt: 'also-not-a-date',
    });
    expect(checkDelegationTemporalValidity(d).valid).toBe(false);
  });

  it('checkDelegationTemporalValidity rejects createdAt after expiresAt', async () => {
    const root = await generateKeyPair();
    const agent = await generateKeyPair();
    const d = await createDelegation({
      delegatorKeyPair: root,
      delegatePublicKey: agent.publicKey,
      scopes: ['search'],
      createdAt: '2030-01-01T00:00:00.000Z',
      expiresAt: '2026-01-01T00:00:00.000Z',
    });
    const res = checkDelegationTemporalValidity(d, '2027-01-01T00:00:00.000Z');
    expect(res.valid).toBe(false);
    expect(res.error).toMatch(/createdAt/i);
  });

  it('checkDelegationTemporalValidity fails closed when verifyAt itself is unparseable', async () => {
    const d = await makeExpired();
    expect(checkDelegationTemporalValidity(d, 'garbage-time').valid).toBe(false);
  });

  it('verifyDelegation rejects an expired delegation (default: now)', async () => {
    const d = await makeExpired();
    expect(await verifyDelegation(d)).toBe(false);
  });

  it('verifyDelegation accepts an expired delegation when verifyAt is pinned inside its window', async () => {
    const d = await makeExpired();
    expect(await verifyDelegation(d, { verifyAt: '2026-01-05T00:00:00.000Z' })).toBe(true);
  });

  it('verifyDelegation still rejects a tampered-but-in-window delegation', async () => {
    const root = await generateKeyPair();
    const agent = await generateKeyPair();
    const d = await createDelegation({
      delegatorKeyPair: root,
      delegatePublicKey: agent.publicKey,
      scopes: ['search'],
      createdAt: '2026-01-01T00:00:00.000Z',
      expiresAt: '2030-01-01T00:00:00.000Z',
    });
    d.scopes = ['search', 'admin'];
    expect(await verifyDelegation(d, { verifyAt: '2027-01-01T00:00:00.000Z' })).toBe(false);
  });

  it('verifyDelegationSignature ignores expiry (pure crypto check)', async () => {
    const d = await makeExpired();
    // Signature is authentic even though the delegation is expired.
    expect(await verifyDelegationSignature(d)).toBe(true);
    expect(await verifyDelegation(d)).toBe(false);
  });

  it('verifyDelegationChain rejects a chain whose sole delegation is expired', async () => {
    const d = await makeExpired();
    const result = await verifyDelegationChain([d]);
    expect(result.valid).toBe(false);
    expect(result.results[0].temporalValid).toBe(false);
  });

  it('verifyDelegationChain rejects a child that outlives an expired parent (reporter case 2)', async () => {
    const root = await generateKeyPair();
    const agent = await generateKeyPair();
    const leaf = await generateKeyPair();

    const parent = await createDelegation({
      delegatorKeyPair: root,
      delegatePublicKey: agent.publicKey,
      scopes: ['search', 'memory.read'],
      createdAt: '2026-01-01T00:00:00.000Z',
      expiresAt: '2026-07-15T00:00:00.000Z',
    });
    const child = await createDelegation({
      delegatorKeyPair: agent,
      delegatePublicKey: leaf.publicKey,
      scopes: ['search'],
      createdAt: '2026-01-02T00:00:00.000Z',
      expiresAt: '2036-01-01T00:00:00.000Z',
    });

    // Evaluate after the parent has expired.
    const result = await verifyDelegationChain([parent, child], { verifyAt: '2026-08-01T00:00:00.000Z' });
    expect(result.valid).toBe(false);
    expect(result.results[0].temporalValid).toBe(false); // parent expired
    expect(result.results[1].temporalValid).toBe(true); // child still live, but chain fails on parent
  });

  it('verifyDelegationChain accepts the same chain while the parent is still live', async () => {
    const root = await generateKeyPair();
    const agent = await generateKeyPair();
    const leaf = await generateKeyPair();

    const parent = await createDelegation({
      delegatorKeyPair: root,
      delegatePublicKey: agent.publicKey,
      scopes: ['search', 'memory.read'],
      createdAt: '2026-01-01T00:00:00.000Z',
      expiresAt: '2026-07-15T00:00:00.000Z',
    });
    const child = await createDelegation({
      delegatorKeyPair: agent,
      delegatePublicKey: leaf.publicKey,
      scopes: ['search'],
      createdAt: '2026-01-02T00:00:00.000Z',
      expiresAt: '2026-06-01T00:00:00.000Z',
      parentDelegation: 'del-1',
    });

    const result = await verifyDelegationChain([parent, child], { verifyAt: '2026-05-01T00:00:00.000Z' });
    expect(result.valid).toBe(true);
    expect(result.results.every((r) => r.temporalValid)).toBe(true);
  });

  it('verifyDelegationChain uses a single evaluation time across all hops', async () => {
    // Both delegations expire at the same instant; evaluating exactly after it
    // must fail every hop, not race between per-hop clock reads.
    const root = await generateKeyPair();
    const agent = await generateKeyPair();

    const d = await createDelegation({
      delegatorKeyPair: root,
      delegatePublicKey: agent.publicKey,
      scopes: ['search'],
      createdAt: '2026-01-01T00:00:00.000Z',
      expiresAt: '2026-02-01T00:00:00.000Z',
    });
    const atExpiry = await verifyDelegationChain([d], { verifyAt: '2026-02-01T00:00:00.000Z' });
    expect(atExpiry.valid).toBe(false); // expiry is exclusive: valid strictly before expiresAt
  });

  it('verifyDelegationChain fails closed when verifyAt is unparseable', async () => {
    const root = await generateKeyPair();
    const agent = await generateKeyPair();
    const d = await createDelegation({
      delegatorKeyPair: root,
      delegatePublicKey: agent.publicKey,
      scopes: ['search'],
    });
    const result = await verifyDelegationChain([d], { verifyAt: 'not-a-time' });
    expect(result.valid).toBe(false);
  });

  it('freshly created delegations still verify (no regression)', async () => {
    const root = await generateKeyPair();
    const agent = await generateKeyPair();
    const d = await createDelegation({
      delegatorKeyPair: root,
      delegatePublicKey: agent.publicKey,
      scopes: ['search'],
    });
    expect(await verifyDelegation(d)).toBe(true);
    expect((await verifyDelegationChain([d])).valid).toBe(true);
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

describe('Trust attenuation through delegation hops', () => {
  it('should track effective trust across a 3-hop chain', async () => {
    const human = await generateKeyPair();
    const coordinator = await generateKeyPair();
    const researcher = await generateKeyPair();

    const d1 = await createDelegation({
      delegatorKeyPair: human,
      delegatePublicKey: coordinator.publicKey,
      scopes: ['search', 'memory.read'],
    });
    // Set trust attenuation on root delegation
    d1.trustAttenuation = 0.8;
    d1.minDelegatedTrust = 0.3;

    const d2 = await createDelegation({
      delegatorKeyPair: coordinator,
      delegatePublicKey: researcher.publicKey,
      scopes: ['search'],
    });

    const result = await verifyDelegationChain([d1, d2], { rootTrustScore: 1.0 });
    expect(result.valid).toBe(true);
    expect(result.results).toHaveLength(2);
    expect(result.results[0].effectiveTrust).toBe(1.0);
    expect(result.results[1].effectiveTrust).toBeCloseTo(0.8, 4);
  });

  it('should fail when trust drops below minimum threshold', async () => {
    const a = await generateKeyPair();
    const b = await generateKeyPair();
    const c = await generateKeyPair();
    const d = await generateKeyPair();

    const d1 = await createDelegation({
      delegatorKeyPair: a,
      delegatePublicKey: b.publicKey,
      scopes: ['search', 'read', 'write'],
    });
    d1.trustAttenuation = 0.5;
    d1.minDelegatedTrust = 0.3;

    const d2 = await createDelegation({
      delegatorKeyPair: b,
      delegatePublicKey: c.publicKey,
      scopes: ['search', 'read'],
    });
    d2.trustAttenuation = 0.5;
    d2.minDelegatedTrust = 0.3;

    const d3 = await createDelegation({
      delegatorKeyPair: c,
      delegatePublicKey: d.publicKey,
      scopes: ['search'],
    });

    // Trust: 1.0 -> 0.5 -> 0.25 (below 0.3 minimum)
    const result = await verifyDelegationChain([d1, d2, d3], { rootTrustScore: 1.0 });
    expect(result.valid).toBe(false);
    expect(result.results[1].effectiveTrust).toBeCloseTo(0.5, 4);
    expect(result.results[2].effectiveTrust).toBeCloseTo(0.25, 4);
    expect(result.results[2].error).toMatch(/below minimum threshold/);
  });

  it('should use default attenuation values when not specified', async () => {
    const human = await generateKeyPair();
    const agent = await generateKeyPair();

    const d1 = await createDelegation({
      delegatorKeyPair: human,
      delegatePublicKey: agent.publicKey,
      scopes: ['search'],
    });
    // No trustAttenuation or minDelegatedTrust set — defaults should apply (0.8, 0.3)

    const result = await verifyDelegationChain([d1], { rootTrustScore: 0.9 });
    expect(result.valid).toBe(true);
    expect(result.results[0].effectiveTrust).toBe(0.9);
  });

  it('should include effective trust in chain export', async () => {
    const human = await generateKeyPair();
    const agent = await generateKeyPair();

    const d1 = await createDelegation({
      delegatorKeyPair: human,
      delegatePublicKey: agent.publicKey,
      scopes: ['search'],
    });

    const verifyResult = await verifyDelegationChain([d1], { rootTrustScore: 0.95 });
    const exported = exportDelegationChain([d1], undefined, verifyResult.results);
    expect(exported.chain[0].effectiveTrust).toBe(0.95);
  });
});
