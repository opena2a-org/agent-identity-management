/**
 * Right-to-delete / opt-out purge (G6 producer half).
 *
 * When a customer opts out, they can ALSO ask the registry to delete every
 * signature this sensor has already reported. The purge keys on the local
 * sensor_id. For a registered (verified) sensor the registry REQUIRES proof of
 * key ownership, so we sign a purge canonical with the sensor's Ed25519 key and
 * send the public key; for an unverified sensor the registry treats the random
 * sensor_id as the capability and the proof is simply accepted.
 *
 * Fails OPEN: a purge network/registry failure NEVER blocks the local opt-out.
 * The opt-out MARKER (written separately, before this runs) is what actually
 * stops transmission; this remote purge is best-effort cleanup of already-sent
 * data. The caller prints a manual retry command when the purge could not reach
 * the registry, so a customer is never silently left with un-purged history.
 *
 * The canonical MUST byte-match the server's Go PurgeCanonical()
 * (opena2a-registry/internal/domain/telemetry_signature.go):
 *
 *   telemetry-purge-v1|<sensorId>|<nonce>|<signedAt>
 *
 * The server normalizes the path sensor_id to its canonical UUID string and
 * reconstructs the canonical from that, so we sign/send the canonical form too
 * (a locally-generated sensor_id already is one; an operator-pinned
 * OPENA2A_SENSOR_ID must be the canonical lowercase-hyphenated service-account id).
 */

import { loadSensorPrivateKey, loadSensorId, publicKeyHex, signCanonicalHex } from './sensor-identity';
import { generateNonce } from './wire';
import { resolveRegistryUrl, type SignatureTelemetryConfig } from './config';
import { sanitizeTerminalText } from './sanitize';

/** The purge canonical schema prefix (distinct from the ingest schema). */
export const PURGE_SCHEMA_VERSION = 'telemetry-purge-v1';

/** Path template for the purge endpoint; :sensor_id is filled in per request. */
export const SIGNATURE_PURGE_PATH = '/api/v1/telemetry/signature/sensor';

const PURGE_TIMEOUT_MS = 10_000;

/** Build the deterministic purge canonical that is signed and server-verified. */
export function buildPurgeCanonical(r: { sensorId: string; nonce: string; signedAt: number }): string {
  return [PURGE_SCHEMA_VERSION, r.sensorId, r.nonce, String(r.signedAt)].join('|');
}

/** The signed proof body POSTed in the DELETE request. */
export interface PurgeProofBody {
  sensorId: string;
  signature: string;
  publicKey: string;
  signedAt: number; // unix SECONDS
  nonce: string;
}

export interface PurgeResult {
  /** True only when the registry confirmed the delete (HTTP 200). */
  ok: boolean;
  /** Rows the registry reported deleted (present on success). */
  deleted?: number;
  /** HTTP status when a response was received. */
  status?: number;
  /** Human-readable failure reason when ok is false. */
  error?: string;
  /** The exact URL targeted (so the caller can print a manual retry). */
  url: string;
  /** The signed proof body (so the caller can print a manual curl on failure). */
  body: PurgeProofBody;
}

/** Normalize a UUID to the canonical form the server signs (lowercase). */
function canonicalSensorId(id: string): string {
  return id.trim().toLowerCase();
}

/**
 * Build the signed purge proof for this sensor. Pure except for the persisted
 * sensor key and the supplied clock (defaulted to now). Exposed for testing.
 */
export async function buildPurgeProof(opts: { now?: Date } = {}): Promise<{ url: string; body: PurgeProofBody }> {
  const now = opts.now ?? new Date();
  const signedAt = Math.floor(now.getTime() / 1000);
  const nonce = generateNonce();
  const sensorId = canonicalSensorId(loadSensorId());

  const priv = loadSensorPrivateKey();
  const publicKey = await publicKeyHex(priv);
  const canonical = buildPurgeCanonical({ sensorId, nonce, signedAt });
  const signature = await signCanonicalHex(priv, canonical);

  const body: PurgeProofBody = { sensorId, signature, publicKey, signedAt, nonce };
  const url = `${SIGNATURE_PURGE_PATH}/${encodeURIComponent(sensorId)}`;
  return { url, body };
}

/**
 * Request the registry purge every signature reported by this sensor. ALWAYS
 * resolves (never throws) — a network error, timeout, or non-200 is returned in
 * the result so the caller can fail OPEN and continue the local opt-out.
 *
 * `fetchImpl` is injectable for testing; it defaults to the global fetch.
 */
export async function purgeRemoteSignatures(
  config?: SignatureTelemetryConfig,
  opts: { now?: Date; fetchImpl?: typeof fetch } = {},
): Promise<PurgeResult> {
  const doFetch = opts.fetchImpl ?? fetch;
  const base = resolveRegistryUrl(config).replace(/\/+$/, '');
  const { url: path, body } = await buildPurgeProof({ now: opts.now });
  const url = `${base}${path}`;

  try {
    const controller = new AbortController();
    const timer = setTimeout(() => controller.abort(), PURGE_TIMEOUT_MS);
    let res: Response;
    try {
      res = await doFetch(url, {
        method: 'DELETE',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body),
        signal: controller.signal,
      });
    } finally {
      clearTimeout(timer);
    }

    if (!res.ok) {
      return { ok: false, status: res.status, error: `HTTP ${res.status}`, url, body };
    }
    let deleted: number | undefined;
    try {
      const json = (await res.json()) as { deleted?: number };
      if (typeof json?.deleted === 'number') deleted = json.deleted;
    } catch {
      // A 200 with an unparseable body still counts as success; deleted unknown.
    }
    return { ok: true, status: res.status, deleted, url, body };
  } catch (err) {
    // Sanitised where it enters the result, not at the print site (#384 class):
    // fetch error messages can embed response/URL text.
    const error = sanitizeTerminalText(err instanceof Error ? err.message : String(err));
    return { ok: false, error, url, body };
  }
}

/** Escape a value for safe inclusion inside a single-quoted shell string. */
function shellSingleQuote(value: string): string {
  return `'${value.replace(/'/g, `'\\''`)}'`;
}

/** A copy-pasteable curl the user can run to retry a failed purge by hand. */
export function manualPurgeCurl(result: PurgeResult): string {
  const json = JSON.stringify(result.body);
  return `curl -X DELETE ${shellSingleQuote(result.url)} -H 'Content-Type: application/json' -d ${shellSingleQuote(json)}`;
}
