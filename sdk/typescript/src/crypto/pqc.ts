/**
 * Post-Quantum Cryptographic operations for AIM SDK
 * Implements ML-DSA (FIPS 204) using @noble/post-quantum
 *
 * This module provides:
 * - ML-DSA key generation (44/65/87 variants)
 * - ML-DSA signing and verification
 * - Hybrid Ed25519+ML-DSA for defense-in-depth
 */

import * as ed from '@noble/ed25519';
import { toBase64, fromBase64 } from './ed25519';

// Algorithm identifiers matching the Go backend
export type Algorithm =
  | 'Ed25519'
  | 'ML-DSA-44'
  | 'ML-DSA-65'
  | 'ML-DSA-87'
  | 'Ed25519+ML-DSA-44'
  | 'Ed25519+ML-DSA-65'
  | 'Ed25519+ML-DSA-87';

// Key sizes for validation (matching Go backend)
export const KEY_SIZES = {
  'ML-DSA-44': { publicKey: 1312, privateKey: 2560, signature: 2420 },
  'ML-DSA-65': { publicKey: 1952, privateKey: 4032, signature: 3309 },
  'ML-DSA-87': { publicKey: 2592, privateKey: 4896, signature: 4627 },
  Ed25519: { publicKey: 32, privateKey: 32, signature: 64 },
} as const;

/**
 * ML-DSA key pair
 */
export interface MLDSAKeyPair {
  algorithm: Algorithm;
  publicKey: Uint8Array;
  privateKey: Uint8Array;
  createdAt: Date;
}

/**
 * Hybrid key pair containing both Ed25519 and ML-DSA keys
 */
export interface HybridKeyPair {
  algorithm: Algorithm;
  ed25519: {
    publicKey: Uint8Array;
    privateKey: Uint8Array;
  };
  mldsa: {
    algorithm: Algorithm;
    publicKey: Uint8Array;
    privateKey: Uint8Array;
  };
  createdAt: Date;
}

/**
 * Hybrid signature containing both Ed25519 and ML-DSA signatures
 */
export interface HybridSignature {
  algorithm: Algorithm;
  ed25519Sig: Uint8Array;
  mldsaSig: Uint8Array;
  timestamp: number;
}

/**
 * Encoded hybrid signature for transmission
 */
export interface EncodedHybridSignature {
  alg: Algorithm;
  ed25519Sig: string; // base64
  mldsaSig: string; // base64
  ts: number;
}

/**
 * Hybrid public key for verification
 */
export interface HybridPublicKey {
  ed25519Pub: Uint8Array;
  mldsaPub: Uint8Array;
  mldsaVariant: Algorithm;
}

/**
 * Encoded hybrid public key for transmission
 */
export interface EncodedHybridPublicKey {
  algorithm: Algorithm;
  ed25519PublicKey: string; // base64
  mldsaPublicKey: string; // base64
  mldsaVariant: Algorithm;
}

// Lazy-loaded ML-DSA modules
let ml_dsa44: any = null;
let ml_dsa65: any = null;
let ml_dsa87: any = null;

/**
 * Load the ML-DSA module for a specific variant
 */
async function loadMLDSA(variant: 'ML-DSA-44' | 'ML-DSA-65' | 'ML-DSA-87') {
  try {
    const mldsa = await import('@noble/post-quantum/ml-dsa');
    ml_dsa44 = mldsa.ml_dsa44;
    ml_dsa65 = mldsa.ml_dsa65;
    ml_dsa87 = mldsa.ml_dsa87;

    switch (variant) {
      case 'ML-DSA-44':
        return ml_dsa44;
      case 'ML-DSA-65':
        return ml_dsa65;
      case 'ML-DSA-87':
        return ml_dsa87;
    }
  } catch (e) {
    throw new Error(
      'ML-DSA support requires @noble/post-quantum package. ' +
        'Install it with: npm install @noble/post-quantum'
    );
  }
}

/**
 * Get the ML-DSA module for a specific variant (uses cached module if available)
 */
async function getMLDSA(variant: 'ML-DSA-44' | 'ML-DSA-65' | 'ML-DSA-87') {
  switch (variant) {
    case 'ML-DSA-44':
      return ml_dsa44 || (await loadMLDSA(variant));
    case 'ML-DSA-65':
      return ml_dsa65 || (await loadMLDSA(variant));
    case 'ML-DSA-87':
      return ml_dsa87 || (await loadMLDSA(variant));
  }
}

/**
 * Check if ML-DSA is available
 */
export async function isPQCAvailable(): Promise<boolean> {
  try {
    await loadMLDSA('ML-DSA-65');
    return true;
  } catch {
    return false;
  }
}

/**
 * Generate an ML-DSA key pair
 */
export async function generateMLDSAKeyPair(
  variant: 'ML-DSA-44' | 'ML-DSA-65' | 'ML-DSA-87' = 'ML-DSA-65'
): Promise<MLDSAKeyPair> {
  const mldsa = await getMLDSA(variant);
  const keys = mldsa.keygen();

  return {
    algorithm: variant,
    publicKey: keys.publicKey,
    privateKey: keys.secretKey,
    createdAt: new Date(),
  };
}

/**
 * Sign a message using ML-DSA
 */
export async function signMLDSA(
  variant: 'ML-DSA-44' | 'ML-DSA-65' | 'ML-DSA-87',
  privateKey: Uint8Array,
  message: Uint8Array
): Promise<Uint8Array> {
  const mldsa = await getMLDSA(variant);
  return mldsa.sign(privateKey, message);
}

/**
 * Verify an ML-DSA signature
 */
export async function verifyMLDSA(
  variant: 'ML-DSA-44' | 'ML-DSA-65' | 'ML-DSA-87',
  publicKey: Uint8Array,
  message: Uint8Array,
  signature: Uint8Array
): Promise<boolean> {
  try {
    const mldsa = await getMLDSA(variant);
    return mldsa.verify(publicKey, message, signature);
  } catch {
    return false;
  }
}

/**
 * Get the ML-DSA variant from a hybrid algorithm
 */
export function getMLDSAVariant(algorithm: Algorithm): 'ML-DSA-44' | 'ML-DSA-65' | 'ML-DSA-87' {
  switch (algorithm) {
    case 'Ed25519+ML-DSA-44':
    case 'ML-DSA-44':
      return 'ML-DSA-44';
    case 'Ed25519+ML-DSA-65':
    case 'ML-DSA-65':
      return 'ML-DSA-65';
    case 'Ed25519+ML-DSA-87':
    case 'ML-DSA-87':
      return 'ML-DSA-87';
    default:
      return 'ML-DSA-65'; // Default to recommended
  }
}

/**
 * Check if an algorithm is a hybrid algorithm
 */
export function isHybridAlgorithm(algorithm: Algorithm): boolean {
  return (
    algorithm === 'Ed25519+ML-DSA-44' ||
    algorithm === 'Ed25519+ML-DSA-65' ||
    algorithm === 'Ed25519+ML-DSA-87'
  );
}

/**
 * Check if an algorithm is post-quantum
 */
export function isPQCAlgorithm(algorithm: Algorithm): boolean {
  return (
    algorithm === 'ML-DSA-44' ||
    algorithm === 'ML-DSA-65' ||
    algorithm === 'ML-DSA-87' ||
    isHybridAlgorithm(algorithm)
  );
}

/**
 * Generate a hybrid Ed25519+ML-DSA key pair
 */
export async function generateHybridKeyPair(
  variant: 'ML-DSA-44' | 'ML-DSA-65' | 'ML-DSA-87' = 'ML-DSA-65'
): Promise<HybridKeyPair> {
  // Generate Ed25519 key pair
  const ed25519PrivateKey = ed.utils.randomPrivateKey();
  const ed25519PublicKey = await ed.getPublicKeyAsync(ed25519PrivateKey);

  // Generate ML-DSA key pair
  const mldsaKeyPair = await generateMLDSAKeyPair(variant);

  // Determine hybrid algorithm
  let algorithm: Algorithm;
  switch (variant) {
    case 'ML-DSA-44':
      algorithm = 'Ed25519+ML-DSA-44';
      break;
    case 'ML-DSA-65':
      algorithm = 'Ed25519+ML-DSA-65';
      break;
    case 'ML-DSA-87':
      algorithm = 'Ed25519+ML-DSA-87';
      break;
  }

  return {
    algorithm,
    ed25519: {
      publicKey: ed25519PublicKey,
      privateKey: ed25519PrivateKey,
    },
    mldsa: {
      algorithm: variant,
      publicKey: mldsaKeyPair.publicKey,
      privateKey: mldsaKeyPair.privateKey,
    },
    createdAt: new Date(),
  };
}

/**
 * Sign a message using hybrid Ed25519+ML-DSA
 * Creates both signatures for defense-in-depth
 */
export async function hybridSign(
  keys: HybridKeyPair,
  message: Uint8Array
): Promise<HybridSignature> {
  // Sign with Ed25519
  const ed25519Sig = await ed.signAsync(message, keys.ed25519.privateKey);

  // Sign with ML-DSA
  const mldsaVariant = getMLDSAVariant(keys.mldsa.algorithm);
  const mldsaSig = await signMLDSA(mldsaVariant, keys.mldsa.privateKey, message);

  return {
    algorithm: keys.algorithm,
    ed25519Sig,
    mldsaSig,
    timestamp: Date.now(),
  };
}

/**
 * Verify a hybrid signature
 * BOTH signatures must be valid for verification to succeed
 */
export async function hybridVerify(
  publicKey: HybridPublicKey,
  message: Uint8Array,
  signature: HybridSignature
): Promise<boolean> {
  // Verify Ed25519 signature first (faster)
  let ed25519Valid: boolean;
  try {
    ed25519Valid = await ed.verifyAsync(signature.ed25519Sig, message, publicKey.ed25519Pub);
  } catch {
    ed25519Valid = false;
  }

  if (!ed25519Valid) {
    return false;
  }

  // Verify ML-DSA signature
  const mldsaVariant = getMLDSAVariant(publicKey.mldsaVariant);
  const mldsaValid = await verifyMLDSA(mldsaVariant, publicKey.mldsaPub, message, signature.mldsaSig);

  // BOTH must be valid
  return mldsaValid;
}

/**
 * Get hybrid public key from key pair
 */
export function getHybridPublicKey(keys: HybridKeyPair): HybridPublicKey {
  return {
    ed25519Pub: keys.ed25519.publicKey,
    mldsaPub: keys.mldsa.publicKey,
    mldsaVariant: keys.mldsa.algorithm,
  };
}

/**
 * Encode a hybrid signature for transmission
 */
export function encodeHybridSignature(signature: HybridSignature): EncodedHybridSignature {
  return {
    alg: signature.algorithm,
    ed25519Sig: toBase64(signature.ed25519Sig),
    mldsaSig: toBase64(signature.mldsaSig),
    ts: signature.timestamp,
  };
}

/**
 * Decode a hybrid signature from transmission format
 */
export function decodeHybridSignature(encoded: EncodedHybridSignature): HybridSignature {
  return {
    algorithm: encoded.alg,
    ed25519Sig: fromBase64(encoded.ed25519Sig),
    mldsaSig: fromBase64(encoded.mldsaSig),
    timestamp: encoded.ts,
  };
}

/**
 * Encode a hybrid public key for transmission
 */
export function encodeHybridPublicKey(publicKey: HybridPublicKey): EncodedHybridPublicKey {
  return {
    algorithm: `Ed25519+${publicKey.mldsaVariant}` as Algorithm,
    ed25519PublicKey: toBase64(publicKey.ed25519Pub),
    mldsaPublicKey: toBase64(publicKey.mldsaPub),
    mldsaVariant: publicKey.mldsaVariant,
  };
}

/**
 * Decode a hybrid public key from transmission format
 */
export function decodeHybridPublicKey(encoded: EncodedHybridPublicKey): HybridPublicKey {
  return {
    ed25519Pub: fromBase64(encoded.ed25519PublicKey),
    mldsaPub: fromBase64(encoded.mldsaPublicKey),
    mldsaVariant: encoded.mldsaVariant,
  };
}

/**
 * Validate key sizes match expected values
 */
export function validateKeySize(algorithm: Algorithm, keyType: 'publicKey' | 'privateKey' | 'signature', data: Uint8Array): boolean {
  const variant = getMLDSAVariant(algorithm);
  const sizes = KEY_SIZES[variant];
  if (!sizes) return false;
  return data.length === sizes[keyType];
}

/**
 * Create request signature headers for hybrid mode
 * Returns headers compatible with the Go backend
 */
export async function createHybridRequestHeaders(
  keys: HybridKeyPair,
  method: string,
  path: string,
  body?: string | object,
  timestamp?: number
): Promise<Record<string, string>> {
  const ts = timestamp ?? Date.now();

  // Create body hash
  let bodyHash = '';
  if (body) {
    const bodyStr = typeof body === 'string' ? body : JSON.stringify(body);
    const encoder = new TextEncoder();
    const data = encoder.encode(bodyStr);
    bodyHash = await hashToHex(data);
  }

  // Create message to sign
  const message = `${ts}:${method.toUpperCase()}:${path}:${bodyHash}`;
  const encoder = new TextEncoder();
  const messageBytes = encoder.encode(message);

  // Sign with hybrid
  const signature = await hybridSign(keys, messageBytes);
  const encodedSig = encodeHybridSignature(signature);

  return {
    'X-Agent-ID': '', // To be filled by caller
    'X-Algorithm': keys.algorithm,
    'X-Signature-Ed25519': encodedSig.ed25519Sig,
    'X-Signature-MLDSA': encodedSig.mldsaSig,
    'X-Timestamp': ts.toString(),
  };
}

/**
 * Hash data to hex string using SHA-256
 */
async function hashToHex(data: Uint8Array): Promise<string> {
  let hashBuffer: ArrayBuffer;
  if (typeof crypto !== 'undefined' && crypto.subtle) {
    hashBuffer = await crypto.subtle.digest('SHA-256', data);
  } else {
    // Node.js fallback
    const nodeCrypto = await import('crypto');
    const hash = nodeCrypto.createHash('sha256');
    hash.update(data);
    hashBuffer = hash.digest().buffer;
  }
  return Array.from(new Uint8Array(hashBuffer))
    .map((b) => b.toString(16).padStart(2, '0'))
    .join('');
}
