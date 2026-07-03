/**
 * Unit coverage for the classification annotator.
 *
 *   1. annotate writes event.data.classification ONLY on a verified result.
 *   2. A null provider (outage / unavailable) leaves the field untouched.
 *   3. A result that fails verification (tampered signature) leaves it untouched.
 *   4. A throwing provider degrades to null, never escapes.
 *   5. The buffered enqueue path processes off the caller's stack and drains.
 *   6. toSignedResult only forwards a fully signed daemon bag.
 *   7. The NanoMind-Guard provider returns null when the daemon socket is absent.
 *
 * Signing reuses the deterministic Ed25519 (0x03) + ML-DSA-44 (0x04) lane from
 * verify-classification.test.ts so a verified result round-trips.
 */

import * as ed25519 from '@noble/ed25519';
import { ml_dsa44 } from '@noble/post-quantum/ml-dsa';
import { describe, it, expect, beforeAll } from 'vitest';

import {
  ClassificationAnnotator,
  NanoMindGuardClassificationProvider,
  eventToClassifyText,
  classifyTextHash,
  toSignedResult,
  type ClassificationProvider,
} from '../../../src/arp/intelligence/classification-annotator';
import {
  GUARD_SIGNATURE_ALGORITHM,
  canonicalizeGuardResultPayload,
} from '../../../src/arp/intelligence/verify-classification';
import type {
  ARPEvent,
  CapabilityManifest,
  NanoMindGuardResult,
  NanoMindGuardVerifyOptions,
} from '../../../src/arp/types';
import type {
  EncodedHybridPublicKey,
  EncodedHybridSignature,
} from '../../../src/arp/crypto/types';

const ED_PRIV = new Uint8Array(32).fill(0x03);
const MLDSA_SEED = new Uint8Array(32).fill(0x04);
const FIXED_NOW = 1_745_000_000_000;

function toBase64(bytes: Uint8Array): string {
  return Buffer.from(bytes).toString('base64');
}

let guardPublicKey: EncodedHybridPublicKey;
let mldsaSecret: Uint8Array;

async function signResult(
  payload: Omit<NanoMindGuardResult, 'signature'>,
): Promise<NanoMindGuardResult> {
  const canonical = canonicalizeGuardResultPayload(
    payload as unknown as Record<string, unknown>,
  );
  const edSig = await ed25519.signAsync(canonical, ED_PRIV);
  const mldsaSig = ml_dsa44.sign(mldsaSecret, canonical);
  const signature: EncodedHybridSignature = {
    alg: GUARD_SIGNATURE_ALGORITHM as EncodedHybridSignature['alg'],
    ed25519Sig: toBase64(edSig),
    mldsaSig: toBase64(mldsaSig),
    ts: payload.timestamp,
  };
  return { ...payload, signature };
}

function basePayload(): Omit<NanoMindGuardResult, 'signature'> {
  return {
    classification: 'code-generation',
    confidence: 0.87,
    modelVersion: 'nanomind-guard-0.1.0',
    // Bound to the event the tests annotate: the annotator requires
    // result.contentHash === classifyTextHash(event) before applying.
    contentHash: classifyTextHash(makeEvent()),
    timestamp: FIXED_NOW,
  };
}

function makeManifest(): CapabilityManifest {
  return {
    version: '1.0.0',
    agentId: 'annotator-test-agent',
    tier: 'execute',
    comply: {
      permitted_classes: ['code-generation'],
      prohibited_classes: ['credential-access'],
      on_violation: 'deny',
    },
    issuedAt: '2026-04-14T00:00:00.000Z',
    ed25519PublicKey: 'unused',
    mldsa65PublicKey: 'unused',
  };
}

function makeOptions(): NanoMindGuardVerifyOptions {
  return {
    guardPublicKey,
    manifest: makeManifest(),
    now: () => FIXED_NOW,
  };
}

function makeEvent(): ARPEvent {
  return {
    id: 'evt-1',
    timestamp: new Date(FIXED_NOW).toISOString(),
    source: 'prompt',
    category: 'normal',
    severity: 'info',
    description: 'agent wrote code',
    data: {},
    classifiedBy: 'L0-rules',
  };
}

/** Provider that always returns a fixed pre-signed result. */
class FixedProvider implements ClassificationProvider {
  constructor(private readonly result: NanoMindGuardResult | null) {}
  async classify(): Promise<NanoMindGuardResult | null> {
    return this.result;
  }
}

class ThrowingProvider implements ClassificationProvider {
  async classify(): Promise<NanoMindGuardResult | null> {
    throw new Error('provider blew up');
  }
}

beforeAll(async () => {
  const edPub = await ed25519.getPublicKeyAsync(ED_PRIV);
  const mldsaKeys = ml_dsa44.keygen(MLDSA_SEED);
  mldsaSecret = mldsaKeys.secretKey;
  guardPublicKey = {
    algorithm: GUARD_SIGNATURE_ALGORITHM,
    ed25519PublicKey: toBase64(edPub),
    mldsaPublicKey: toBase64(mldsaKeys.publicKey),
    mldsaVariant: 'ML-DSA-44',
  };
});

describe('ClassificationAnnotator.annotate', () => {
  it('writes event.data.classification on a verified result', async () => {
    const result = await signResult(basePayload());
    const annotator = new ClassificationAnnotator(new FixedProvider(result), makeOptions());
    const event = makeEvent();

    const verify = await annotator.annotate(event);

    expect(verify?.valid).toBe(true);
    expect(event.data.classification).toBe('code-generation');
  });

  it('leaves classification untouched when the provider returns null', async () => {
    const annotator = new ClassificationAnnotator(new FixedProvider(null), makeOptions());
    const event = makeEvent();

    const verify = await annotator.annotate(event);

    expect(verify).toBeNull();
    expect(event.data.classification).toBeUndefined();
  });

  it('leaves classification untouched when the signature fails to verify', async () => {
    const result = await signResult(basePayload());
    // Tamper the ed25519 half so hybridVerify rejects.
    const tampered: NanoMindGuardResult = {
      ...result,
      signature: { ...result.signature, ed25519Sig: toBase64(new Uint8Array(64).fill(0x09)) },
    };
    const annotator = new ClassificationAnnotator(new FixedProvider(tampered), makeOptions());
    const event = makeEvent();

    const verify = await annotator.annotate(event);

    expect(verify?.valid).toBe(false);
    expect(event.data.classification).toBeUndefined();
  });

  it('degrades to null when the provider throws', async () => {
    const annotator = new ClassificationAnnotator(new ThrowingProvider(), makeOptions());
    const event = makeEvent();

    const verify = await annotator.annotate(event);

    expect(verify).toBeNull();
    expect(event.data.classification).toBeUndefined();
  });

  it('rejects a valid result whose contentHash is not bound to this event', async () => {
    // A signed result for SOME OTHER event content: contentHash does not match
    // classifyTextHash(event), so the label must not be applied even though the
    // signature itself verifies.
    const result = await signResult({ ...basePayload(), contentHash: 'b'.repeat(64) });
    const annotator = new ClassificationAnnotator(new FixedProvider(result), makeOptions());
    const event = makeEvent();

    const verify = await annotator.annotate(event);

    expect(verify).toBeNull();
    expect(event.data.classification).toBeUndefined();
  });
});

describe('ClassificationAnnotator.enqueue (buffered)', () => {
  it('processes queued events off the caller stack and drains', async () => {
    const result = await signResult(basePayload());
    const annotator = new ClassificationAnnotator(new FixedProvider(result), makeOptions(), {
      concurrency: 2,
    });

    const events = Array.from({ length: 5 }, () => makeEvent());
    const teed: ARPEvent[] = [];
    for (const e of events) annotator.enqueue(e, () => teed.push(e));

    // Nothing is guaranteed processed synchronously.
    await annotator.drain();

    expect(teed).toHaveLength(5);
    for (const e of events) expect(e.data.classification).toBe('code-generation');
  });

  it('drops enqueues past maxQueue and reports the drop via onDone(null)', async () => {
    // A provider that never resolves until released keeps the queue full.
    let release!: () => void;
    const gate = new Promise<void>((r) => (release = r));
    const slowProvider: ClassificationProvider = {
      classify: async () => {
        await gate;
        return null;
      },
    };
    const annotator = new ClassificationAnnotator(slowProvider, makeOptions(), {
      concurrency: 1,
      maxQueue: 1,
    });

    // First enqueue starts processing (active), second fills the queue,
    // third is dropped.
    const outcomes: Array<'done' | 'dropped'> = [];
    annotator.enqueue(makeEvent(), () => outcomes.push('done'));
    annotator.enqueue(makeEvent(), () => outcomes.push('done'));
    annotator.enqueue(makeEvent(), (r) => outcomes.push(r === null ? 'dropped' : 'done'));

    expect(outcomes).toContain('dropped');
    release();
    await annotator.drain();
  });
});

describe('toSignedResult', () => {
  it('forwards a fully signed daemon bag', () => {
    const bag = {
      ...basePayload(),
      signature: { alg: GUARD_SIGNATURE_ALGORITHM, ed25519Sig: 'x', mldsaSig: 'y', ts: FIXED_NOW },
      // daemon-only noise fields are ignored
      gateLatencyMs: 3,
    };
    const result = toSignedResult(bag);
    expect(result).not.toBeNull();
    expect(result?.classification).toBe('code-generation');
  });

  it('returns null when the signature block is missing', () => {
    expect(toSignedResult({ ...basePayload() })).toBeNull();
  });

  it('returns null on a non-hex contentHash', () => {
    const bag = {
      ...basePayload(),
      contentHash: 'not-hex',
      signature: { alg: GUARD_SIGNATURE_ALGORITHM, ed25519Sig: 'x', mldsaSig: 'y', ts: FIXED_NOW },
    };
    expect(toSignedResult(bag)).toBeNull();
  });
});

describe('eventToClassifyText', () => {
  it('is deterministic and bounded', () => {
    const event = makeEvent();
    event.data = { a: 1, b: 'two' };
    const t1 = eventToClassifyText(event);
    const t2 = eventToClassifyText(event);
    expect(t1).toBe(t2);
    expect(t1.length).toBeLessThanOrEqual(4096);
    expect(t1).toContain('agent wrote code');
  });
});

describe('NanoMindGuardClassificationProvider', () => {
  it('returns null when the daemon socket is absent', async () => {
    // Point at a socket path that does not exist; sendClassify returns null.
    const provider = new NanoMindGuardClassificationProvider({
      socketPath: '/tmp/nonexistent-nanomind-guard-annotator-test.sock',
      timeoutMs: 500,
    });
    const result = await provider.classify(makeEvent());
    expect(result).toBeNull();
  });
});
