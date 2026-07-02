/**
 * Hybrid Ed25519 + ML-DSA signing for ARP (AIComply P1).
 *
 * Defense-in-depth signing: every payload is co-signed by a classical curve
 * (Ed25519, FIPS 186-5) and a post-quantum lattice scheme (ML-DSA, FIPS 204).
 * Verification requires BOTH halves to validate. This matches the wire format
 * used by the AIM SDK (`agent-identity-management/sdk/typescript/src/crypto/pqc.ts`)
 * and the Go backend so hybrid signatures can round trip between the two.
 *
 * Design notes:
 * - Deps are imported statically at module top. Previous scaffolding already
 *   proved the CJS resolution works against pinned `@noble/ed25519` v2 and
 *   `@noble/post-quantum` v0.2. Lazy loading is not needed and would hide
 *   install failures until first call.
 * - `hybridVerify()` evaluates BOTH halves unconditionally. It does not
 *   short-circuit on an ed25519 failure. This avoids a side-channel where an
 *   attacker could probe which half of a signature is broken by timing the
 *   verify call, and it matches the structured `HybridVerifyResult` contract.
 * - Failures inside either half (thrown exceptions, wrong lengths) become a
 *   `{valid: false}` result with a `reason`. The caller is expected to fail
 *   closed (parse-to-deny) per CR-001.
 * - `KEY_SIZES` is the FIPS 204 table and must match the AIM SDK and Go backend
 *   byte for byte. Any size drift between the three is a bug somewhere.
 */

import { randomBytes } from 'crypto';

import * as ed25519 from '@noble/ed25519';
import { ml_dsa44, ml_dsa65, ml_dsa87 } from '@noble/post-quantum/ml-dsa';

import type {
  EncodedHybridPublicKey,
  EncodedHybridSignature,
  HybridAlgorithm,
  HybridKeyPair,
  HybridPublicKey,
  HybridSignature,
  HybridVerifyResult,
  MLDsaVariant,
} from './types';

/**
 * FIPS 204 key and signature sizes (in bytes). Used to validate key material
 * and signatures before handing them to the underlying primitives.
 *
 * Source: NIST FIPS 204 (ML-DSA), Table 1. Verified against the current
 * `@noble/post-quantum` v0.2 keygen output.
 */
export const KEY_SIZES = {
  Ed25519: { publicKey: 32, privateKey: 32, signature: 64 },
  'ML-DSA-44': { publicKey: 1312, privateKey: 2560, signature: 2420 },
  'ML-DSA-65': { publicKey: 1952, privateKey: 4032, signature: 3309 },
  'ML-DSA-87': { publicKey: 2592, privateKey: 4896, signature: 4627 },
} as const;

/**
 * Error raised when a hybrid primitive rejects its input before any crypto
 * runs. Catching code should fail closed (parse-to-deny) per CR-001.
 */
export class HybridCryptoError extends Error {
  constructor(message: string) {
    super(`[arp/crypto] ${message}`);
    this.name = 'HybridCryptoError';
  }
}

/**
 * Return the canonical hybrid algorithm identifier for a given ML-DSA variant.
 * Stable helper, safe to use from any layer.
 */
export function hybridAlgorithmFor(variant: MLDsaVariant): HybridAlgorithm {
  switch (variant) {
    case 'ML-DSA-44':
      return 'Ed25519+ML-DSA-44';
    case 'ML-DSA-65':
      return 'Ed25519+ML-DSA-65';
    case 'ML-DSA-87':
      return 'Ed25519+ML-DSA-87';
  }
}

/**
 * Resolve an ML-DSA variant string to the `@noble/post-quantum` handle.
 */
function mldsaFor(variant: MLDsaVariant) {
  switch (variant) {
    case 'ML-DSA-44':
      return ml_dsa44;
    case 'ML-DSA-65':
      return ml_dsa65;
    case 'ML-DSA-87':
      return ml_dsa87;
  }
}

/**
 * Validate a byte array against the expected FIPS 204 / Ed25519 size.
 * Returns true if the size matches, false otherwise.
 */
export function validateKeySize(
  variantOrAlg: MLDsaVariant | HybridAlgorithm | 'Ed25519',
  kind: 'publicKey' | 'privateKey' | 'signature',
  data: Uint8Array,
): boolean {
  const variant: MLDsaVariant | 'Ed25519' =
    variantOrAlg === 'Ed25519'
      ? 'Ed25519'
      : variantOrAlg === 'Ed25519+ML-DSA-44'
        ? 'ML-DSA-44'
        : variantOrAlg === 'Ed25519+ML-DSA-65'
          ? 'ML-DSA-65'
          : variantOrAlg === 'Ed25519+ML-DSA-87'
            ? 'ML-DSA-87'
            : variantOrAlg;
  const sizes = KEY_SIZES[variant];
  if (!sizes) return false;
  return data.length === sizes[kind];
}

/**
 * Generate a hybrid Ed25519 + ML-DSA key pair. Both halves sign the same
 * payloads; the returned pair carries enough material to produce and verify
 * hybrid signatures.
 *
 * @param variant ML-DSA parameter set. 44 for high-throughput identity signals,
 *                65 for manifests (recommended default), 87 for root keys.
 */
export async function generateHybridKeyPair(
  variant: MLDsaVariant,
): Promise<HybridKeyPair> {
  const ed25519PrivateKey = ed25519.utils.randomPrivateKey();
  const ed25519PublicKey = await ed25519.getPublicKeyAsync(ed25519PrivateKey);

  // ML-DSA keygen takes a 32-byte seed. Source entropy from Node's
  // `crypto.randomBytes` so the seed is unique per call and does not reuse
  // any secret material from the Ed25519 private key.
  const mldsaSeed = new Uint8Array(randomBytes(32));
  const mldsa = mldsaFor(variant);
  const mldsaKeys = mldsa.keygen(mldsaSeed);

  // Defense in depth: make sure noble handed us the FIPS 204 sizes we expect
  // before anyone can sign with these keys. A size mismatch here means the
  // dep drifted and every downstream assumption is suspect.
  if (!validateKeySize('Ed25519', 'publicKey', ed25519PublicKey)) {
    throw new HybridCryptoError(
      `Ed25519 public key size mismatch: got ${ed25519PublicKey.length}, expected ${KEY_SIZES.Ed25519.publicKey}`,
    );
  }
  if (!validateKeySize(variant, 'publicKey', mldsaKeys.publicKey)) {
    throw new HybridCryptoError(
      `${variant} public key size mismatch: got ${mldsaKeys.publicKey.length}, expected ${KEY_SIZES[variant].publicKey}`,
    );
  }
  if (!validateKeySize(variant, 'privateKey', mldsaKeys.secretKey)) {
    throw new HybridCryptoError(
      `${variant} private key size mismatch: got ${mldsaKeys.secretKey.length}, expected ${KEY_SIZES[variant].privateKey}`,
    );
  }

  return {
    algorithm: hybridAlgorithmFor(variant),
    ed25519: {
      publicKey: ed25519PublicKey,
      privateKey: ed25519PrivateKey,
    },
    mldsa: {
      variant,
      publicKey: mldsaKeys.publicKey,
      privateKey: mldsaKeys.secretKey,
    },
    createdAt: new Date(),
  };
}

/**
 * Sign a payload with a hybrid key pair. Both halves (Ed25519 and ML-DSA)
 * sign the SAME payload bytes; the returned signature carries both.
 */
export async function hybridSign(
  payload: Uint8Array,
  keyPair: HybridKeyPair,
): Promise<HybridSignature> {
  if (!(payload instanceof Uint8Array)) {
    throw new HybridCryptoError('hybridSign: payload must be Uint8Array');
  }

  const ed25519Sig = await ed25519.signAsync(payload, keyPair.ed25519.privateKey);
  const mldsa = mldsaFor(keyPair.mldsa.variant);
  const mldsaSig = mldsa.sign(keyPair.mldsa.privateKey, payload);

  return {
    algorithm: keyPair.algorithm,
    ed25519Sig,
    mldsaSig,
    timestamp: Date.now(),
  };
}

/**
 * Verify a hybrid signature. BOTH halves must validate for the overall result
 * to be valid. The function intentionally evaluates both halves on every call,
 * even when the first half fails, so that timing cannot reveal which half of
 * a signature is broken. All thrown errors from the underlying primitives are
 * trapped and reported as a structured failure (`valid: false`).
 */
export async function hybridVerify(
  payload: Uint8Array,
  signature: HybridSignature,
  publicKey: HybridPublicKey,
): Promise<HybridVerifyResult> {
  const reasons: string[] = [];

  // Structural checks first. These do not leak side-channel information because
  // they do not depend on the signature bytes being compared against a secret.
  if (signature.algorithm !== publicKey.algorithm) {
    return {
      valid: false,
      ed25519Valid: false,
      mldsaValid: false,
      reason: `algorithm mismatch: signature=${signature.algorithm} publicKey=${publicKey.algorithm}`,
    };
  }
  if (!validateKeySize('Ed25519', 'signature', signature.ed25519Sig)) {
    reasons.push(
      `ed25519 signature size invalid: ${signature.ed25519Sig.length}`,
    );
  }
  if (!validateKeySize(publicKey.mldsaVariant, 'signature', signature.mldsaSig)) {
    reasons.push(
      `${publicKey.mldsaVariant} signature size invalid: ${signature.mldsaSig.length}`,
    );
  }
  if (!validateKeySize('Ed25519', 'publicKey', publicKey.ed25519PublicKey)) {
    reasons.push(
      `ed25519 public key size invalid: ${publicKey.ed25519PublicKey.length}`,
    );
  }
  if (!validateKeySize(publicKey.mldsaVariant, 'publicKey', publicKey.mldsaPublicKey)) {
    reasons.push(
      `${publicKey.mldsaVariant} public key size invalid: ${publicKey.mldsaPublicKey.length}`,
    );
  }

  // Evaluate both halves unconditionally. The non-short-circuit property is a
  // security requirement, not a convenience. Do not refactor this to bail
  // early on ed25519 failure.
  let ed25519Valid = false;
  try {
    ed25519Valid = await ed25519.verifyAsync(
      signature.ed25519Sig,
      payload,
      publicKey.ed25519PublicKey,
    );
  } catch (err) {
    reasons.push(`ed25519 verify threw: ${(err as Error).message}`);
  }

  let mldsaValid = false;
  try {
    const mldsa = mldsaFor(publicKey.mldsaVariant);
    mldsaValid = mldsa.verify(
      publicKey.mldsaPublicKey,
      payload,
      signature.mldsaSig,
    );
  } catch (err) {
    reasons.push(`${publicKey.mldsaVariant} verify threw: ${(err as Error).message}`);
  }

  const valid = ed25519Valid && mldsaValid;
  if (!valid && !ed25519Valid) reasons.push('ed25519 half rejected');
  if (!valid && !mldsaValid) reasons.push(`${publicKey.mldsaVariant} half rejected`);

  return {
    valid,
    ed25519Valid,
    mldsaValid,
    ...(valid ? {} : { reason: reasons.join('; ') }),
  };
}

/**
 * Project a `HybridKeyPair` to its `HybridPublicKey` shape, suitable for
 * distributing to verifiers.
 */
export function getHybridPublicKey(keyPair: HybridKeyPair): HybridPublicKey {
  return {
    algorithm: keyPair.algorithm,
    ed25519PublicKey: keyPair.ed25519.publicKey,
    mldsaPublicKey: keyPair.mldsa.publicKey,
    mldsaVariant: keyPair.mldsa.variant,
  };
}

// --- Encoding helpers for YAML/JSON transport ------------------------------

function toBase64(data: Uint8Array): string {
  return Buffer.from(data).toString('base64');
}

function fromBase64(encoded: string): Uint8Array {
  return new Uint8Array(Buffer.from(encoded, 'base64'));
}

/** Encode a hybrid signature for on-disk or on-wire transport. */
export function encodeHybridSignature(
  signature: HybridSignature,
): EncodedHybridSignature {
  return {
    alg: signature.algorithm,
    ed25519Sig: toBase64(signature.ed25519Sig),
    mldsaSig: toBase64(signature.mldsaSig),
    ts: signature.timestamp,
  };
}

/** Decode a hybrid signature from its transport form. */
export function decodeHybridSignature(
  encoded: EncodedHybridSignature,
): HybridSignature {
  return {
    algorithm: encoded.alg,
    ed25519Sig: fromBase64(encoded.ed25519Sig),
    mldsaSig: fromBase64(encoded.mldsaSig),
    timestamp: encoded.ts,
  };
}

/** Encode a hybrid public key for on-disk or on-wire transport. */
export function encodeHybridPublicKey(
  publicKey: HybridPublicKey,
): EncodedHybridPublicKey {
  return {
    algorithm: publicKey.algorithm,
    ed25519PublicKey: toBase64(publicKey.ed25519PublicKey),
    mldsaPublicKey: toBase64(publicKey.mldsaPublicKey),
    mldsaVariant: publicKey.mldsaVariant,
  };
}

/** Decode a hybrid public key from its transport form. */
export function decodeHybridPublicKey(
  encoded: EncodedHybridPublicKey,
): HybridPublicKey {
  return {
    algorithm: encoded.algorithm,
    ed25519PublicKey: fromBase64(encoded.ed25519PublicKey),
    mldsaPublicKey: fromBase64(encoded.mldsaPublicKey),
    mldsaVariant: encoded.mldsaVariant,
  };
}
