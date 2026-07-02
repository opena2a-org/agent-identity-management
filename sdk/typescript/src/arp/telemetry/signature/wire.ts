/**
 * Wire builder — assembles the exact TelemetrySignatureRequest the registry
 * ingestion endpoint accepts, and the canonical string it reconstructs to verify.
 *
 * The canonical MUST byte-match the server's Go Canonical()
 * (opena2a-registry/internal/domain/telemetry_signature.go):
 *
 *   schemaVersion|behavioralHash|techniqueId|severity|outcome|sensorId|orgPseudonym|count|nonce|signedAt
 *
 * Any drift here means the signature fails server-side verification and the whole
 * submission is rejected.
 */

import { randomBytes } from 'crypto';
import type { RedactedSignal } from './redaction';
import { behavioralHash } from './behavioral-hash';
import { loadSensorPrivateKey, loadSensorId, publicKeyHex, signCanonicalHex } from './sensor-identity';
import { currentOrgPseudonym } from './org-pseudonym';

/** The only schema version the registry accepts for v1 ingestion. */
export const SCHEMA_VERSION = 'telemetry-sig-v1';

/** The ingestion path on the registry. */
export const SIGNATURE_INGEST_PATH = '/api/v1/telemetry/signature';

/**
 * The wire request. Field names and JSON shape mirror the registry's
 * TelemetrySignatureRequest exactly (an unknown field is rejected by the
 * handler's DisallowUnknownFields).
 */
export interface TelemetrySignatureRequest {
  schemaVersion: string;
  behavioralHash: string;
  techniqueId: string;
  severity: string;
  outcome: string;
  count: number;
  sensorId: string;
  orgPseudonym: string;
  signature: string;
  publicKey: string;
  signedAt: number; // unix SECONDS
  nonce: string;
}

/** Build the deterministic canonical message that is signed and verified. */
export function buildCanonical(r: {
  schemaVersion: string;
  behavioralHash: string;
  techniqueId: string;
  severity: string;
  outcome: string;
  sensorId: string;
  orgPseudonym: string;
  count: number;
  nonce: string;
  signedAt: number;
}): string {
  const count = r.count > 0 ? r.count : 1; // mirror Go normalizedCount()
  return [
    r.schemaVersion,
    r.behavioralHash,
    r.techniqueId,
    r.severity,
    r.outcome,
    r.sensorId,
    r.orgPseudonym,
    String(count),
    r.nonce,
    String(r.signedAt),
  ].join('|');
}

/** Generate an anti-replay nonce matching ^[A-Za-z0-9_-]{8,128}$ (32 url-safe chars). */
export function generateNonce(): string {
  return randomBytes(24).toString('base64url'); // 24 bytes -> 32 url-safe chars
}

export interface BuiltSubmission {
  request: TelemetrySignatureRequest;
  /** The exact JSON bytes that will be POSTed — what the audit log records. */
  body: string;
  canonical: string;
}

/**
 * Build a fully-signed submission from a redacted signal. Pure inputs except the
 * persisted sensor key / org secret and the supplied clock (defaulted to now).
 */
export async function buildSignedSubmission(
  signal: RedactedSignal,
  opts: { count?: number; now?: Date } = {},
): Promise<BuiltSubmission> {
  const now = opts.now ?? new Date();
  const signedAt = Math.floor(now.getTime() / 1000);
  const count = opts.count && opts.count > 0 ? Math.min(opts.count, 1_000_000) : 1;
  const nonce = generateNonce();

  const priv = loadSensorPrivateKey();
  const sensorId = loadSensorId();
  const publicKey = await publicKeyHex(priv);
  const orgPseudonym = currentOrgPseudonym(now);
  const hash = behavioralHash(signal);

  const canonical = buildCanonical({
    schemaVersion: SCHEMA_VERSION,
    behavioralHash: hash,
    techniqueId: signal.techniqueId,
    severity: signal.severity,
    outcome: signal.outcomeClass,
    sensorId,
    orgPseudonym,
    count,
    nonce,
    signedAt,
  });
  const signature = await signCanonicalHex(priv, canonical);

  const request: TelemetrySignatureRequest = {
    schemaVersion: SCHEMA_VERSION,
    behavioralHash: hash,
    techniqueId: signal.techniqueId,
    severity: signal.severity,
    outcome: signal.outcomeClass,
    count,
    sensorId,
    orgPseudonym,
    signature,
    publicKey,
    signedAt,
    nonce,
  };

  return { request, body: JSON.stringify(request), canonical };
}
