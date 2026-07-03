/**
 * ARP crypto module barrel.
 *
 * Public entry point for hybrid Ed25519 + ML-DSA signing primitives used by:
 * - Capability manifest loader (verifies Ed25519+ML-DSA-65 signature before load)
 * - `verifyClassification()` (verifies Ed25519+ML-DSA-44 on NanoMind-Guard output)
 * - AIComply P1 integration (AC-001, AC-002, AC-010, AC-014, AC-015, AC-016)
 */

export type {
  EncodedHybridPublicKey,
  EncodedHybridSignature,
  HybridAlgorithm,
  HybridKeyPair,
  HybridPublicKey,
  HybridSignature,
  HybridVerifyResult,
  MLDsaVariant,
} from './types';

export {
  HybridCryptoError,
  KEY_SIZES,
  decodeHybridPublicKey,
  decodeHybridSignature,
  encodeHybridPublicKey,
  encodeHybridSignature,
  generateHybridKeyPair,
  getHybridPublicKey,
  hybridAlgorithmFor,
  hybridSign,
  hybridVerify,
  validateKeySize,
} from './hybrid-signing';

export {
  CapabilityManifestError,
  MANIFEST_SIGNATURE_ALGORITHM,
  MANIFEST_VERSION,
  MAX_MANIFEST_SIZE_BYTES,
  canonicalizeManifestPayload,
  loadCapabilityManifest,
  parseCapabilityManifest,
} from './manifest-loader';

export type { CapabilityManifestErrorCode } from './manifest-loader';
