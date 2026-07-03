/**
 * Behavioral hash (G4) — SHA-256 of the structural SHAPE, never the payload.
 *
 * See opena2a-registry/docs/telemetry-behavioral-hash-spec.md. The hash binds
 * ONLY allowlisted enum tokens, so its preimage contains no attacker- or
 * customer-controlled BYTES and cannot fingerprint an environment. The single
 * residual, low-bandwidth channel is the ORDER of the sequencePattern tokens
 * (a bounded alphabet of <=8 actionClasses, <=16 steps); content is never
 * attacker-controlled, only the ordering of structural categories. See
 * buildSequencePattern in redaction.ts.
 *
 *   behavioralHash = SHA-256( "v1|tactic|technique|action|target|sequence|outcome" )
 *
 * Canonicalization (spec): lowercase every token, fixed pipe-separated order,
 * version-prefixed so a future shape change occupies a disjoint hash space.
 */

import { createHash } from 'crypto';
import type { RedactedSignal } from './redaction';

/** Spec version prefix. Bumping this forks the hash space (no silent collisions). */
export const HASH_SHAPE_VERSION = 'v1';

/**
 * Build the canonical, lowercase, pipe-separated structural string that is
 * hashed. Exported for golden-vector tests and audit transparency.
 */
export function canonicalShape(s: RedactedSignal): string {
  return [
    HASH_SHAPE_VERSION,
    s.tacticId,
    s.techniqueId,
    s.actionClass,
    s.targetClass,
    s.sequencePattern,
    s.outcomeClass,
  ]
    .join('|')
    .toLowerCase();
}

/** SHA-256 of the canonical shape, lowercase hex (64 chars). */
export function behavioralHash(s: RedactedSignal): string {
  return createHash('sha256').update(canonicalShape(s), 'utf8').digest('hex');
}
