/**
 * Self-serve sensor enrollment (the producer half of `telemetry register`,
 * internal CLI only — the shipped `aim-arp` bin deliberately does not register it).
 *
 * A sensor already holds an Ed25519 keypair (sensor-identity.ts). Enrollment
 * registers that key with the registry, which creates a PENDING (unverified)
 * enrollment that an admin must approve before the sensor's reports count as
 * verified. Enrollment proves key ownership by signing an enroll canonical with the
 * sensor key, so an attacker cannot pre-register a key it cannot sign for.
 *
 * On success the registry returns the service-account id it assigned. Future reports
 * MUST carry that id as their sensorId (the registry binds the registered key to that
 * id and only marks a report verified when its sensorId matches), so we ADOPT it
 * locally via persistSensorId.
 *
 * Fails OPEN: a network/registry failure NEVER throws. The caller prints a manual
 * retry command so a sensor is never silently left un-enrolled.
 *
 * The canonical MUST byte-match the server's Go EnrollCanonical()
 * (opena2a-registry/internal/domain/telemetry_signature.go):
 *
 *   telemetry-enroll-v1|<publicKey>|<nonce>|<signedAt>
 *
 * publicKey and signature are transmitted as lowercase hex; the registry's
 * normalizeToBase64 hex-decodes them before ed25519.Verify, and binds the publicKey
 * verbatim into the canonical it reconstructs, so the hex form round-trips.
 */

import { existsSync, mkdirSync, readFileSync, writeFileSync } from 'fs';
import {
  loadSensorPrivateKey,
  publicKeyHex,
  signCanonicalHex,
  persistSensorId,
} from './sensor-identity';
import { generateNonce } from './wire';
import { resolveRegistryUrl, type SignatureTelemetryConfig } from './config';
import { sanitizeTerminalText } from './sanitize';
import { opena2aHome, homePath, SENSOR_ENROLLMENT_FILE } from './paths';

/** The enroll canonical schema prefix (distinct from the ingest/purge schemas). */
export const ENROLL_SCHEMA_VERSION = 'telemetry-enroll-v1';

/** The self-serve enrollment endpoint on the registry. */
export const SENSOR_ENROLL_PATH = '/api/v1/telemetry/sensors/enroll';

const ENROLL_TIMEOUT_MS = 10_000;

/** The two enrollment states the registry reports. */
export type EnrollmentState = 'pending' | 'verified';

/** Build the deterministic enroll canonical that is signed and server-verified. */
export function buildEnrollCanonical(r: { publicKey: string; nonce: string; signedAt: number }): string {
  return [ENROLL_SCHEMA_VERSION, r.publicKey, r.nonce, String(r.signedAt)].join('|');
}

/** The signed enrollment body POSTed to the enroll endpoint. */
export interface EnrollRequestBody {
  publicKey: string;
  nonce: string;
  signedAt: number; // unix SECONDS
  signature: string;
}

export interface EnrollResult {
  /** True only when the registry accepted the enrollment (HTTP 2xx). */
  ok: boolean;
  /** Enrollment state the registry reported (present on success). */
  state?: EnrollmentState;
  /** The registry-assigned service-account id for this sensor (present on success). */
  sensorId?: string;
  /** Human-facing next-step guidance from the registry (present on success). */
  message?: string;
  /** HTTP status when a response was received. */
  status?: number;
  /** Human-readable failure reason when ok is false. */
  error?: string;
  /** The exact URL targeted (so the caller can print a manual retry). */
  url: string;
  /** The signed body (so the caller can print a manual curl on failure). */
  body: EnrollRequestBody;
}

/** Locally persisted enrollment record. */
export interface EnrollmentRecord {
  sensorId: string;
  state: EnrollmentState;
  updatedAt: string;
}

/**
 * Build the signed enrollment body for this sensor. Pure except for the persisted
 * sensor key and the supplied clock (defaulted to now). Exposed for testing.
 */
export async function buildEnrollProof(opts: { now?: Date } = {}): Promise<{ url: string; body: EnrollRequestBody }> {
  const now = opts.now ?? new Date();
  const signedAt = Math.floor(now.getTime() / 1000);
  const nonce = generateNonce();

  const priv = loadSensorPrivateKey();
  const publicKey = await publicKeyHex(priv);
  const canonical = buildEnrollCanonical({ publicKey, nonce, signedAt });
  const signature = await signCanonicalHex(priv, canonical);

  const body: EnrollRequestBody = { publicKey, nonce, signedAt, signature };
  return { url: SENSOR_ENROLL_PATH, body };
}

/** Persist the enrollment record locally (best effort; never throws). */
export function writeEnrollmentRecord(rec: EnrollmentRecord): void {
  try {
    const home = opena2aHome();
    if (!existsSync(home)) mkdirSync(home, { recursive: true });
    writeFileSync(homePath(SENSOR_ENROLLMENT_FILE), JSON.stringify(rec) + '\n', { mode: 0o600 });
  } catch {
    // best effort — a failed local record never blocks the (successful) enrollment
  }
}

/** Read the locally persisted enrollment record, or null if none / unreadable. */
export function readEnrollmentRecord(): EnrollmentRecord | null {
  const p = homePath(SENSOR_ENROLLMENT_FILE);
  if (!existsSync(p)) return null;
  try {
    const parsed = JSON.parse(readFileSync(p, 'utf8')) as Partial<EnrollmentRecord>;
    if (parsed && typeof parsed.sensorId === 'string' && (parsed.state === 'pending' || parsed.state === 'verified')) {
      return { sensorId: parsed.sensorId, state: parsed.state, updatedAt: parsed.updatedAt ?? '' };
    }
  } catch {
    // unreadable / malformed — treat as no record
  }
  return null;
}

/**
 * Enroll this sensor with the registry. ALWAYS resolves (never throws) — a network
 * error, timeout, or non-2xx is returned in the result so the caller can fail OPEN.
 *
 * On a successful enrollment the registry-assigned sensorId is adopted locally (so
 * future reports carry it) and the enrollment record is persisted.
 *
 * `fetchImpl` is injectable for testing; it defaults to the global fetch.
 */
export async function enrollSensor(
  config?: SignatureTelemetryConfig,
  opts: { now?: Date; fetchImpl?: typeof fetch } = {},
): Promise<EnrollResult> {
  const doFetch = opts.fetchImpl ?? fetch;
  const base = resolveRegistryUrl(config).replace(/\/+$/, '');
  const { url: path, body } = await buildEnrollProof({ now: opts.now });
  const url = `${base}${path}`;

  try {
    const controller = new AbortController();
    const timer = setTimeout(() => controller.abort(), ENROLL_TIMEOUT_MS);
    let res: Response;
    try {
      res = await doFetch(url, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body),
        signal: controller.signal,
      });
    } finally {
      clearTimeout(timer);
    }

    if (!res.ok) {
      let error = `HTTP ${res.status}`;
      try {
        const json = (await res.json()) as { error?: string };
        // Sanitised where the server text enters the result (#384 class).
        if (json?.error) error = `HTTP ${res.status}: ${sanitizeTerminalText(json.error)}`;
      } catch {
        // non-JSON error body; keep the bare status
      }
      return { ok: false, status: res.status, error, url, body };
    }

    let state: EnrollmentState | undefined;
    let sensorId: string | undefined;
    let message: string | undefined;
    try {
      const json = (await res.json()) as { sensorId?: string; state?: string; message?: string };
      if (json?.state === 'pending' || json?.state === 'verified') state = json.state;
      if (typeof json?.sensorId === 'string') sensorId = json.sensorId;
      if (typeof json?.message === 'string') message = sanitizeTerminalText(json.message);
    } catch {
      // A 2xx with an unparseable body still counts as success; details unknown.
    }

    // Adopt the registry-assigned id locally so future reports verify, and record state.
    if (sensorId) {
      persistSensorId(sensorId);
      writeEnrollmentRecord({
        sensorId,
        state: state ?? 'pending',
        updatedAt: (opts.now ?? new Date()).toISOString(),
      });
    }

    return { ok: true, status: res.status, state, sensorId, message, url, body };
  } catch (err) {
    const error = sanitizeTerminalText(err instanceof Error ? err.message : String(err));
    return { ok: false, error, url, body };
  }
}

/** Escape a value for safe inclusion inside a single-quoted shell string. */
function shellSingleQuote(value: string): string {
  return `'${value.replace(/'/g, `'\\''`)}'`;
}

/** A copy-pasteable curl the user can run to retry a failed enrollment by hand. */
export function manualEnrollCurl(result: EnrollResult): string {
  const json = JSON.stringify(result.body);
  return `curl -X POST ${shellSingleQuote(result.url)} -H 'Content-Type: application/json' -d ${shellSingleQuote(json)}`;
}
