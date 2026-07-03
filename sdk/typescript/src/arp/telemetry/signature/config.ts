/**
 * Signature telemetry configuration + the single master opt-out.
 *
 * Ratified posture (project_telemetry_consent_design): structural signatures are
 * DEFAULT-ON, opt-out. The opt-out is a SINGLE master switch — when a customer
 * opts out, BOTH this signature channel and the legacy GTIN runtime channel are
 * disabled, so there is never a surprise second channel. Opt-out is honored from
 * three independent sources (any one disables):
 *   1. Environment: OPENA2A_TELEMETRY_OPTOUT / ARP_TELEMETRY_DISABLED truthy.
 *   2. A marker file at ~/.opena2a/telemetry-optout (written by `arp telemetry opt-out`).
 *   3. Config: `signatureTelemetry.enabled === false`.
 */

import { existsSync, mkdirSync, writeFileSync, unlinkSync } from 'fs';
import { opena2aHome, homePath, OPTOUT_MARKER_FILE } from './paths';
import type { SignatureTelemetryConfig } from '../../types';

export type { SignatureTelemetryConfig };

const DEFAULT_REGISTRY_URL = 'https://api.oa2a.org';

function envTruthy(name: string): boolean {
  const v = process.env[name];
  if (!v) return false;
  const s = v.trim().toLowerCase();
  return s === '1' || s === 'true' || s === 'yes' || s === 'on';
}

/** True if the opt-out marker file is present. */
export function optOutMarkerExists(): boolean {
  return existsSync(homePath(OPTOUT_MARKER_FILE));
}

/**
 * The single master opt-out decision. True if ANY opt-out source is active. This
 * is consulted both for the signature channel AND (in index.ts) to gate GTIN.
 */
export function isOptedOut(config?: SignatureTelemetryConfig): boolean {
  if (config?.enabled === false) return true;
  if (envTruthy('OPENA2A_TELEMETRY_OPTOUT') || envTruthy('ARP_TELEMETRY_DISABLED')) return true;
  if (optOutMarkerExists()) return true;
  return false;
}

/** Whether the signature channel should run (default-on unless opted out). */
export function signatureTelemetryEnabled(config?: SignatureTelemetryConfig): boolean {
  return !isOptedOut(config);
}

/** Resolve the registry base URL for ingestion. */
export function resolveRegistryUrl(config?: SignatureTelemetryConfig): string {
  return config?.registryUrl || process.env.OPENA2A_REGISTRY_URL || DEFAULT_REGISTRY_URL;
}

/** Persist the opt-out marker (used by `arp telemetry opt-out`). Idempotent. */
export function writeOptOutMarker(): string {
  const home = opena2aHome();
  if (!existsSync(home)) mkdirSync(home, { recursive: true });
  const p = homePath(OPTOUT_MARKER_FILE);
  writeFileSync(p, new Date().toISOString() + '\n', { mode: 0o600 });
  return p;
}

/** Remove the opt-out marker (used by `arp telemetry opt-in`). Idempotent. */
export function clearOptOutMarker(): void {
  const p = homePath(OPTOUT_MARKER_FILE);
  if (existsSync(p)) {
    try {
      unlinkSync(p);
    } catch {
      // best effort
    }
  }
}
