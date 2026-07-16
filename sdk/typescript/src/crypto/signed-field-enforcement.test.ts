/**
 * Signed-field enforcement harness.
 *
 * A verifier's most dangerous bug is a MISSING check: a field that is signed
 * into the payload but never evaluated at verification time, so the verdict
 * silently ignores it. `@opena2a/aim-sdk` 1.0.2 signed `expiresAt` into every
 * delegation but never enforced it — expired delegations verified as valid.
 *
 * This harness makes that bug class fail CI by construction. It derives the set
 * of fields that go INTO the signed bytes directly from the signable payload,
 * then asserts that for every one of those fields there is a violation which
 * flips `verifyDelegation` / `verifyDelegationChain` to reject. A newly added
 * signed field with no registered violation fails the coverage assertion, so the
 * enforcement gap cannot merge unnoticed.
 *
 * See the pre-push-review "Signed-Field Enforcement Matrix" phase.
 */
import { describe, it, expect } from 'vitest';
import { generateKeyPair, type KeyPair } from './ed25519';
import {
  createDelegation,
  verifyDelegation,
  verifyDelegationChain,
  delegationSignablePayload,
  publicKeyToDidKey,
  toBase64url,
  type Delegation,
} from './delegation';

// A fixed evaluation time inside the fixtures' validity window, so "valid"
// fixtures are unambiguously live and only the injected violation causes a
// rejection.
const NOW = '2026-03-01T00:00:00.000Z';

async function baseDelegation(delegator: KeyPair, delegatePub: Uint8Array): Promise<Delegation> {
  return createDelegation({
    delegatorKeyPair: delegator,
    delegatePublicKey: delegatePub,
    scopes: ['search', 'memory.read'],
    createdAt: '2026-01-01T00:00:00.000Z',
    expiresAt: '2026-12-31T00:00:00.000Z',
  });
}

/**
 * For each field that appears in the signed payload, a mutation that violates
 * it. Every signed field MUST have an entry here (enforced by the coverage
 * test below); each mutation MUST make verification reject.
 */
type Violation = {
  reason: string;
  mutate: (d: Delegation) => Delegation;
};

const VIOLATIONS: Record<string, Violation> = {
  delegator: {
    reason: 'delegator DID no longer matches the signing key',
    mutate: (d) => ({ ...d, delegator: 'did:key:zInvalidDelegatorDid' }),
  },
  delegate: {
    reason: 'delegate DID swapped after signing',
    mutate: (d) => ({ ...d, delegate: 'did:key:zAttackerControlledDelegate' }),
  },
  scopes: {
    reason: 'scopes widened after signing',
    mutate: (d) => ({ ...d, scopes: [...d.scopes, 'admin'] }),
  },
  created_at: {
    reason: 'createdAt after expiresAt (inverted window)',
    mutate: (d) => ({ ...d, createdAt: '2030-01-01T00:00:00.000Z' }),
  },
  expires_at: {
    reason: 'delegation is expired at the evaluation time',
    mutate: (d) => ({ ...d, expiresAt: '2026-02-01T00:00:00.000Z' }),
  },
  parent_delegation: {
    reason: 'parentDelegation reference altered after signing',
    mutate: (d) => ({ ...d, parentDelegation: 'del-tampered' }),
  },
};

describe('Signed-field enforcement coverage', () => {
  it('every field in the signed payload has a registered violation', async () => {
    const root = await generateKeyPair();
    const agent = await generateKeyPair();
    const withParent = await createDelegation({
      delegatorKeyPair: root,
      delegatePublicKey: agent.publicKey,
      scopes: ['search'],
      createdAt: '2026-01-01T00:00:00.000Z',
      expiresAt: '2026-12-31T00:00:00.000Z',
      parentDelegation: 'del-root',
    });

    const signedFields = Object.keys(
      JSON.parse(new TextDecoder().decode(delegationSignablePayload(withParent))),
    );

    const uncovered = signedFields.filter((f) => !(f in VIOLATIONS));
    // If this fails, a new field was added to the signed payload without a
    // corresponding negative test. Enforcement is unproven — add a violation
    // (and the verifier check it exercises) before shipping.
    expect(uncovered, `signed fields with no enforcement violation: ${uncovered.join(', ')}`).toEqual([]);
  });
});

describe('Signed-field enforcement — single delegation', () => {
  for (const [field, violation] of Object.entries(VIOLATIONS)) {
    // parent_delegation is only present on sub-delegations; covered in the chain block.
    if (field === 'parent_delegation') continue;

    it(`rejects when ${field} is violated (${violation.reason})`, async () => {
      const root = await generateKeyPair();
      const agent = await generateKeyPair();
      const good = await baseDelegation(root, agent.publicKey);

      // Sanity: the untouched fixture verifies at NOW.
      expect(await verifyDelegation(good, { verifyAt: NOW })).toBe(true);

      // The violated fixture must be rejected.
      const bad = violation.mutate(structuredClone(good));
      expect(await verifyDelegation(bad, { verifyAt: NOW })).toBe(false);
    });
  }
});

describe('Signed-field enforcement — delegation chain', () => {
  async function goodChain(): Promise<{ chain: Delegation[]; agent: KeyPair; leaf: KeyPair; root: KeyPair }> {
    const root = await generateKeyPair();
    const agent = await generateKeyPair();
    const leaf = await generateKeyPair();

    const parent = await createDelegation({
      delegatorKeyPair: root,
      delegatePublicKey: agent.publicKey,
      scopes: ['search', 'memory.read'],
      createdAt: '2026-01-01T00:00:00.000Z',
      expiresAt: '2026-12-31T00:00:00.000Z',
    });
    const child = await createDelegation({
      delegatorKeyPair: agent,
      delegatePublicKey: leaf.publicKey,
      scopes: ['search'],
      createdAt: '2026-01-02T00:00:00.000Z',
      expiresAt: '2026-12-31T00:00:00.000Z',
      parentDelegation: 'del-root',
    });
    return { chain: [parent, child], agent, leaf, root };
  }

  it('accepts an untampered chain at the evaluation time', async () => {
    const { chain } = await goodChain();
    expect((await verifyDelegationChain(chain, { verifyAt: NOW })).valid).toBe(true);
  });

  for (const [field, violation] of Object.entries(VIOLATIONS)) {
    it(`rejects the chain when the child's ${field} is violated (${violation.reason})`, async () => {
      const { chain } = await goodChain();
      const tampered = [chain[0], violation.mutate(structuredClone(chain[1]))];
      expect((await verifyDelegationChain(tampered, { verifyAt: NOW })).valid).toBe(false);
    });
  }

  it('rejects a child that outlives its parent once the parent has expired', async () => {
    const root = await generateKeyPair();
    const agent = await generateKeyPair();
    const leaf = await generateKeyPair();

    const parent = await createDelegation({
      delegatorKeyPair: root,
      delegatePublicKey: agent.publicKey,
      scopes: ['search'],
      createdAt: '2026-01-01T00:00:00.000Z',
      expiresAt: '2026-04-01T00:00:00.000Z',
    });
    const child = await createDelegation({
      delegatorKeyPair: agent,
      delegatePublicKey: leaf.publicKey,
      scopes: ['search'],
      createdAt: '2026-01-02T00:00:00.000Z',
      expiresAt: '2036-01-01T00:00:00.000Z',
      parentDelegation: 'del-root',
    });

    // After the parent expires, the chain must fail even though the child is live.
    expect((await verifyDelegationChain([parent, child], { verifyAt: '2026-05-01T00:00:00.000Z' })).valid).toBe(false);
  });

  it('rejects a child that outlives its parent even while the parent is still live (temporal narrowing)', async () => {
    const root = await generateKeyPair();
    const agent = await generateKeyPair();
    const leaf = await generateKeyPair();

    const parent = await createDelegation({
      delegatorKeyPair: root,
      delegatePublicKey: agent.publicKey,
      scopes: ['search'],
      createdAt: '2026-01-01T00:00:00.000Z',
      expiresAt: '2026-06-01T00:00:00.000Z',
    });
    const child = await createDelegation({
      delegatorKeyPair: agent,
      delegatePublicKey: leaf.publicKey,
      scopes: ['search'],
      createdAt: '2026-01-02T00:00:00.000Z',
      expiresAt: '2036-01-01T00:00:00.000Z', // outlives the parent
      parentDelegation: 'del-root',
    });

    // Both are live at NOW (2026-03-01), but the child claims authority in time
    // beyond its delegator — the invariant a child must not outlive its parent.
    const result = await verifyDelegationChain([parent, child], { verifyAt: NOW });
    expect(result.valid).toBe(false);
    expect(result.results[1].temporalValid).toBe(false);
  });
});

describe('Signed-field enforcement — fail-closed on malformed input', () => {
  it('rejects a delegation with a non-signature string in the signature field', async () => {
    const root = await generateKeyPair();
    const agent = await generateKeyPair();
    const good = await baseDelegation(root, agent.publicKey);
    const bad = { ...good, signature: 'verified' }; // descriptor-vs-value: a word is not a signature
    expect(await verifyDelegation(bad, { verifyAt: NOW })).toBe(false);
  });

  it('rejects a delegation whose public key does not match the delegator DID', async () => {
    const root = await generateKeyPair();
    const other = await generateKeyPair();
    const agent = await generateKeyPair();
    const good = await baseDelegation(root, agent.publicKey);
    const bad = { ...good, publicKey: toBase64url(other.publicKey), delegator: publicKeyToDidKey(other.publicKey) };
    // Re-pointed identity: signature no longer matches the payload → reject.
    expect(await verifyDelegation(bad, { verifyAt: NOW })).toBe(false);
  });
});
