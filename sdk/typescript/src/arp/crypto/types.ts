/**
 * ARP crypto types (scaffolding, AIComply P1).
 *
 * Shared shapes for hybrid Ed25519 + ML-DSA signing. The first commit lands only
 * these types and stub functions. The follow-up commit wires real signing against
 * `@noble/ed25519` and `@noble/post-quantum`, mirroring
 * `agent-identity-management/sdk/typescript/src/crypto/pqc.ts`.
 */

/** Supported ML-DSA parameter sets (FIPS 204). */
export type MLDsaVariant = 'ML-DSA-44' | 'ML-DSA-65' | 'ML-DSA-87';

/** Hybrid algorithm identifier, matching the AIM SDK and Go backend wire format. */
export type HybridAlgorithm =
  | 'Ed25519+ML-DSA-44'
  | 'Ed25519+ML-DSA-65'
  | 'Ed25519+ML-DSA-87';

/**
 * Hybrid key pair: Ed25519 (classical) plus ML-DSA (post-quantum).
 * Both halves co-sign every payload; verification requires both to pass.
 */
export interface HybridKeyPair {
  algorithm: HybridAlgorithm;
  ed25519: {
    publicKey: Uint8Array;
    privateKey: Uint8Array;
  };
  mldsa: {
    variant: MLDsaVariant;
    publicKey: Uint8Array;
    privateKey: Uint8Array;
  };
  createdAt: Date;
}

/** Hybrid public key in raw (non-encoded) form, used for verification. */
export interface HybridPublicKey {
  algorithm: HybridAlgorithm;
  ed25519PublicKey: Uint8Array;
  mldsaPublicKey: Uint8Array;
  mldsaVariant: MLDsaVariant;
}

/** Hybrid signature: both halves must validate for the signature to be accepted. */
export interface HybridSignature {
  algorithm: HybridAlgorithm;
  ed25519Sig: Uint8Array;
  mldsaSig: Uint8Array;
  /** Unix ms timestamp when the signature was produced */
  timestamp: number;
}

/** Base64-encoded form of a hybrid signature (for YAML/JSON transport). */
export interface EncodedHybridSignature {
  alg: HybridAlgorithm;
  ed25519Sig: string;
  mldsaSig: string;
  ts: number;
}

/** Base64-encoded form of a hybrid public key (for YAML/JSON transport). */
export interface EncodedHybridPublicKey {
  algorithm: HybridAlgorithm;
  ed25519PublicKey: string;
  mldsaPublicKey: string;
  mldsaVariant: MLDsaVariant;
}

/** Result of a hybrid signature verification. */
export interface HybridVerifyResult {
  /** True only if BOTH ed25519 and mldsa halves verify successfully. */
  valid: boolean;
  /** Per-half breakdown, for diagnostics and structured logging. */
  ed25519Valid: boolean;
  mldsaValid: boolean;
  /** Human-readable reason when `valid` is false. */
  reason?: string;
}
