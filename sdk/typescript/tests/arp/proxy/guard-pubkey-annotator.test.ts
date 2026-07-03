/**
 * Integration coverage for the Guard public-key config surface that activates
 * the ARP proxy's classification annotator.
 *
 * These exercise the production wiring end to end: ARPProxy.start() loads a
 * signed capability manifest, resolves the configured Guard public key, builds
 * a NanoMindGuardClassificationProvider + ClassificationAnnotator, and the
 * existing detection wiring enqueues every event for annotation. The daemon
 * client (`sendClassify`) is mocked so we control the signed result without a
 * live Guard socket.
 *
 *   1. Valid key + signed result    → event.data.classification populated.
 *   2. No key configured            → annotator not built, classification null.
 *   3. Malformed key                → log-once + disabled, startup survives.
 *   4. Daemon down (null response)  → classification stays null.
 *   5. Enforce-default-off invariant: a class that WOULD be denied under
 *      enforcement is annotated (label written) but never denied, because the
 *      manifest does not set comply.enforce.
 *
 * Signing reuses the deterministic Ed25519 (0x03) + ML-DSA-44 (0x04) lane the
 * other ARP intelligence tests use, so a verified result round-trips through
 * the real verify-classification path (no crypto is stubbed).
 */

import { describe, it, expect, beforeAll, afterEach, vi } from 'vitest';
import * as crypto from 'crypto';
import * as path from 'path';
import * as ed25519 from '@noble/ed25519';
import { ml_dsa44 } from '@noble/post-quantum/ml-dsa';

// Mock the daemon client the provider wraps. The real socket is never touched.
vi.mock('../../../src/arp/guard/nanomind-guard-client', () => ({
  sendClassify: vi.fn(),
}));

import { sendClassify } from '../../../src/arp/guard/nanomind-guard-client';
import type { ClassifyResponse } from '../../../src/arp/guard/nanomind-guard-client';
import { ARPProxy, type ARPProxyDeps } from '../../../src/arp/proxy/server';
import { EventEngine } from '../../../src/arp/engine/event-engine';
import { IntelligenceCoordinator } from '../../../src/arp/intelligence/coordinator';
import {
  GUARD_SIGNATURE_ALGORITHM,
  canonicalizeGuardResultPayload,
} from '../../../src/arp/intelligence/verify-classification';
import type { ARPConfig, ARPEvent, ProxyConfig } from '../../../src/arp/types';
import type {
  EncodedHybridPublicKey,
  EncodedHybridSignature,
} from '../../../src/arp/crypto/types';

const HAPPY_MANIFEST = path.join(
  __dirname,
  '..',
  '..',
  '..',
  'test',
  'fixtures',
  'capability-manifests',
  'happy.yaml',
);

const ED_PRIV = new Uint8Array(32).fill(0x03);
const MLDSA_SEED = new Uint8Array(32).fill(0x04);

const mockedSendClassify = vi.mocked(sendClassify);

let guardPublicKeyJson: string;
let mldsaSecret: Uint8Array;

function toBase64(bytes: Uint8Array): string {
  return Buffer.from(bytes).toString('base64');
}

function sha256hex(s: string): string {
  return crypto.createHash('sha256').update(s).digest('hex');
}

/**
 * Produce a signed daemon ok-bag bound to `text`. The contentHash is the
 * SHA-256 of the exact text the provider will hand the daemon, so the
 * annotator's content-hash binding check passes. Signs over the canonical
 * payload (sans signature) with the deterministic Guard keys.
 */
async function signedBagForText(
  text: string,
  classification: string,
): Promise<ClassifyResponse> {
  const payload = {
    classification,
    confidence: 0.9,
    modelVersion: 'nanomind-guard-test',
    contentHash: sha256hex(text),
    timestamp: Date.now(),
  };
  const canonical = canonicalizeGuardResultPayload({ ...payload });
  const edSig = await ed25519.signAsync(canonical, ED_PRIV);
  const mldsaSig = ml_dsa44.sign(mldsaSecret, canonical);
  const signature: EncodedHybridSignature = {
    alg: GUARD_SIGNATURE_ALGORITHM as EncodedHybridSignature['alg'],
    ed25519Sig: toBase64(edSig),
    mldsaSig: toBase64(mldsaSig),
    ts: payload.timestamp,
  };
  return { ok: true, ...payload, signature } as unknown as ClassifyResponse;
}

function baseConfig(): ARPConfig {
  // intelligence.enabled false keeps L2 inert; we exercise only the annotator
  // build path and the comply gate. The proxy builds the annotator from the
  // key regardless of this flag (the CLI is what gates on `enabled`).
  return { agentName: 'guard-pubkey-test-agent', intelligence: { enabled: false } };
}

function proxyConfig(withManifest: boolean): ProxyConfig {
  return {
    port: 0,
    upstreams: [],
    ...(withManifest ? { manifestPath: HAPPY_MANIFEST } : {}),
  };
}

interface Harness {
  proxy: ARPProxy;
  engine: EventEngine;
  teed: ARPEvent[];
  /** Resolves the next time an event is teed (annotation has completed). */
  nextTee(): Promise<void>;
}

function buildHarness(deps: Partial<ARPProxyDeps>, withManifest = true): Harness {
  const engine = new EventEngine(baseConfig());
  const teed: ARPEvent[] = [];
  let resolveTee: (() => void) | null = null;
  const proxy = new ARPProxy(proxyConfig(withManifest), {
    engine,
    onInScopeEvent: (e) => {
      teed.push(e);
      resolveTee?.();
      resolveTee = null;
    },
    ...deps,
  });
  return {
    proxy,
    engine,
    teed,
    nextTee: () => new Promise<void>((r) => (resolveTee = r)),
  };
}

async function emitOne(engine: EventEngine, description = 'agent did a thing'): Promise<ARPEvent> {
  return engine.emit({
    source: 'prompt',
    category: 'normal',
    severity: 'info',
    description,
    data: {},
  });
}

beforeAll(async () => {
  const edPub = await ed25519.getPublicKeyAsync(ED_PRIV);
  const mldsaKeys = ml_dsa44.keygen(MLDSA_SEED);
  mldsaSecret = mldsaKeys.secretKey;
  const guardPublicKey: EncodedHybridPublicKey = {
    algorithm: GUARD_SIGNATURE_ALGORITHM,
    ed25519PublicKey: toBase64(edPub),
    mldsaPublicKey: toBase64(mldsaKeys.publicKey),
    mldsaVariant: 'ML-DSA-44',
  };
  guardPublicKeyJson = JSON.stringify(guardPublicKey);
});

afterEach(() => {
  mockedSendClassify.mockReset();
});

describe('ARP proxy Guard public-key annotator', () => {
  it('populates event.data.classification with a valid key and signed result', async () => {
    mockedSendClassify.mockImplementation(
      async (text: string) => signedBagForText(text, 'code-generation'),
    );

    const h = buildHarness({ guardPublicKey: guardPublicKeyJson });
    await h.proxy.start();
    try {
      const tee = h.nextTee();
      const event = await emitOne(h.engine);
      await tee;

      expect(event.data.classification).toBe('code-generation');
      expect(h.teed[0].data.classification).toBe('code-generation');
    } finally {
      await h.proxy.stop();
    }
  });

  it('does not build the annotator when no key is configured (classification stays null)', async () => {
    mockedSendClassify.mockImplementation(
      async (text: string) => signedBagForText(text, 'code-generation'),
    );

    const h = buildHarness({ /* no guardPublicKey */ });
    await h.proxy.start();
    try {
      const tee = h.nextTee();
      const event = await emitOne(h.engine);
      await tee;

      // No annotator was built, so the daemon was never consulted.
      expect(mockedSendClassify).not.toHaveBeenCalled();
      expect(event.data.classification).toBeUndefined();
    } finally {
      await h.proxy.stop();
    }
  });

  it('disables annotation and logs once on a malformed key without crashing startup', async () => {
    const errorSpy = vi.spyOn(console, 'error').mockImplementation(() => {});
    mockedSendClassify.mockImplementation(
      async (text: string) => signedBagForText(text, 'code-generation'),
    );

    const h = buildHarness({ guardPublicKey: '}{ not json at all' });
    try {
      // start() must resolve — a bad key must never be a startup-time DoS.
      await expect(h.proxy.start()).resolves.toBeUndefined();

      const tee = h.nextTee();
      const event = await emitOne(h.engine);
      await tee;

      expect(event.data.classification).toBeUndefined();
      expect(mockedSendClassify).not.toHaveBeenCalled();
      expect(errorSpy).toHaveBeenCalledTimes(1);
      expect(errorSpy.mock.calls[0][0]).toContain('guard public key is malformed');
    } finally {
      await h.proxy.stop();
      errorSpy.mockRestore();
    }
  });

  it('leaves classification null when the daemon is down (null response)', async () => {
    mockedSendClassify.mockResolvedValue(null);

    const h = buildHarness({ guardPublicKey: guardPublicKeyJson });
    await h.proxy.start();
    try {
      const tee = h.nextTee();
      const event = await emitOne(h.engine);
      await tee;

      // The annotator WAS built (the daemon was consulted) but the outage
      // degrades to no label rather than planting an unverified one.
      expect(mockedSendClassify).toHaveBeenCalledTimes(1);
      expect(event.data.classification).toBeUndefined();
    } finally {
      await h.proxy.stop();
    }
  });

  it('annotates a would-be-denied class but does NOT deny it (enforce default off)', async () => {
    // 'network-egress' clears the tier matrix at the manifest's `execute` tier
    // but is NOT on happy.yaml's permitted list — so under comply.enforce=true
    // it would be an `unknown` violation and denied. happy.yaml omits enforce,
    // so the live annotator writes the label for DETECTION and the comply gate
    // takes no action. Proves a populated classification cannot, by itself,
    // become a hot-path deny.
    mockedSendClassify.mockImplementation(
      async (text: string) => signedBagForText(text, 'network-egress'),
    );

    const engine = new EventEngine(baseConfig());
    const coordinator = new IntelligenceCoordinator(baseConfig(), '.', null);
    const teed: ARPEvent[] = [];
    let resolveTee: (() => void) | null = null;
    const proxy = new ARPProxy(proxyConfig(true), {
      engine,
      coordinator,
      guardPublicKey: guardPublicKeyJson,
      onInScopeEvent: (e) => {
        teed.push(e);
        resolveTee?.();
        resolveTee = null;
      },
    });

    await proxy.start();
    try {
      // The proxy handed the verified manifest to the coordinator.
      expect(coordinator.getCapabilityManifest()?.agentId).toBe('fixture-agent');
      expect(coordinator.getCapabilityManifest()?.comply.enforce).toBeUndefined();

      const tee = new Promise<void>((r) => (resolveTee = r));
      const event = await emitOne(engine);
      await tee;

      // Label written (detection) ...
      expect(event.data.classification).toBe('network-egress');
      // ... but the comply gate never fired: no deny, no decision record.
      expect(event.classifiedBy).not.toBe('L0-comply');
      expect(event.data.comply).toBeUndefined();
      expect(event.category).toBe('normal');
      expect(event.severity).toBe('info');
    } finally {
      await proxy.stop();
    }
  });
});
