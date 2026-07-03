/**
 * Local state paths for the signature telemetry producer.
 *
 * All producer state lives under the OpenA2A home (~/.opena2a by default,
 * overridable with OPENA2A_HOME). Secrets (org root secret, sensor private key)
 * are written 0600 and NEVER transmitted.
 */

import { homedir } from 'os';
import { join } from 'path';

/** Resolve the OpenA2A home directory (OPENA2A_HOME or ~/.opena2a). */
export function opena2aHome(): string {
  return process.env.OPENA2A_HOME || join(homedir(), '.opena2a');
}

/** 0600 secret: per-install org root secret for pseudonym derivation. */
export const ORG_ROOT_SECRET_FILE = 'telemetry-org-root-secret';
/** Per-install org identity (opaque local uuid; never transmitted directly). */
export const ORG_ID_FILE = 'telemetry-org-id';
/** 0600 secret: Ed25519 sensor private key (hex). */
export const SENSOR_KEY_FILE = 'telemetry-sensor-key';
/** Registered/local sensor id (uuid). */
export const SENSOR_ID_FILE = 'telemetry-sensor-id';
/** Append-only local audit log (JSONL) of every payload that left the device. */
export const AUDIT_LOG_FILE = 'telemetry-audit.log';
/** One-time marker that the install-time disclosure has been shown. */
export const DISCLOSURE_MARKER_FILE = 'telemetry-disclosure-shown';
/** Opt-out marker file (presence = opted out, in addition to env/config). */
export const OPTOUT_MARKER_FILE = 'telemetry-optout';
/** Local record of the sensor's enrollment state (JSON: sensorId, state, updatedAt). */
export const SENSOR_ENROLLMENT_FILE = 'telemetry-enrollment';

export function homePath(file: string): string {
  return join(opena2aHome(), file);
}
