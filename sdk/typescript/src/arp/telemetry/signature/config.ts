/**
 * Signature telemetry configuration, the explicit opt-in, and the master opt-out.
 *
 * Posture: OPT-IN. The channel is disabled by default and starts only when the
 * operator turns it on explicitly. This reverses an earlier default-on,
 * opt-out posture for this SDK surface. Telemetry here is opt-in by explicit
 * configuration only, is never prompted for, and is disabled by default in
 * every distribution channel.
 *
 * Why the reversal: this SDK ships as a library inside other people's
 * production processes, and AIM's product claim is that identity and audit stay
 * on disk. A channel that phones home unless you find the switch is
 * inconsistent with both.
 *
 * Turning it ON requires one of (either is explicit and operator-chosen):
 *   1. Environment: AIM_TELEMETRY truthy.
 *   2. Config: `signatureTelemetry.enabled === true`.
 *
 * Turning it OFF is honored from three independent sources, and an opt-out
 * ALWAYS beats an opt-in — the switch only ever moves in the safe direction, so
 * a stale env var cannot re-enable a channel someone deliberately disabled:
 *   1. Environment: OPENA2A_TELEMETRY_OPTOUT / ARP_TELEMETRY_DISABLED truthy.
 *   2. A marker file at ~/.opena2a/telemetry-optout (written by `aim-arp telemetry opt-out`).
 *   3. Config: `signatureTelemetry.enabled === false`.
 *
 * `isOptedOut` deliberately keeps its original meaning — "has someone actively
 * refused?" — because it is ALSO the master switch gating the separate GTIN
 * runtime channel (see `arp/index.ts`) and the emitter's own pre-send re-check.
 * Ask `signatureTelemetryEnabled` for "should this channel run?"; the two are no
 * longer each other's negation.
 */

import { existsSync, mkdirSync, writeFileSync, unlinkSync } from 'fs';
import { opena2aHome, homePath, OPTOUT_MARKER_FILE } from './paths';
import type { SignatureTelemetryConfig } from '../../types';

export type { SignatureTelemetryConfig };

const DEFAULT_REGISTRY_URL = 'https://api.oa2a.org';

/**
 * Strict truthiness for opt-out env vars: only 1/true/yes/on count. Exported so
 * user-facing attribution (`optOutReason`) reads the SAME predicate as the
 * decision (`isOptedOut`) — a value like `0` must neither opt out nor be blamed.
 */
export function envTruthy(name: string): boolean {
  const v = process.env[name];
  if (!v) return false;
  const s = v.trim().toLowerCase();
  return s === '1' || s === 'true' || s === 'yes' || s === 'on';
}

/**
 * The ecosystem-wide opt-out, read in the OFF direction only.
 *
 * `opena2a.org/privacy` tells users verbatim to "Set the environment variable
 * OPENA2A_TELEMETRY=off in your shell profile", and `opena2a.org/telemetry`
 * repeats it as working on every OpenA2A surface. This is a VALUE predicate,
 * not a truthy-name one: the name being present means nothing, the value
 * off/0/false/no is the refusal. Reading it with `envTruthy` would treat
 * `OPENA2A_TELEMETRY=off` as unset and fail open on the one spelling we
 * publish. Kept byte-compatible with the canonical implementation in
 * `opena2a/packages/telemetry/src/config.ts` (`envOptOut`), trim included —
 * trailing whitespace is routine in .env files, docker-compose YAML and
 * command substitution, and an opt-out must never fail open over a stray space.
 *
 * Deliberately one-directional: `OPENA2A_TELEMETRY=on` does NOT opt in here.
 * That variable is an ecosystem CLI switch, while this SDK is a library inside
 * someone else's production process; opting in stays explicit and AIM-specific.
 */
export function ecosystemOptOut(): boolean {
  const v = process.env.OPENA2A_TELEMETRY;
  if (v === undefined) return false;
  const s = v.trim().toLowerCase();
  return s === 'off' || s === '0' || s === 'false' || s === 'no';
}

/** True if the opt-out marker file is present. */
export function optOutMarkerExists(): boolean {
  return existsSync(homePath(OPTOUT_MARKER_FILE));
}

/**
 * The master opt-out decision: has someone ACTIVELY refused telemetry? True if
 * any opt-out source is active. Consulted by the signature channel, by the
 * emitter's pre-send re-check, and (in index.ts) to gate the GTIN channel.
 *
 * Note this is NOT the negation of `signatureTelemetryEnabled`. Taking no action
 * at all leaves this false (nobody refused) while the channel still does not run
 * (nobody opted in).
 */
export function isOptedOut(config?: SignatureTelemetryConfig): boolean {
  if (config?.enabled === false) return true;
  if (envTruthy('OPENA2A_TELEMETRY_OPTOUT') || envTruthy('ARP_TELEMETRY_DISABLED')) return true;
  if (ecosystemOptOut()) return true;
  if (optOutMarkerExists()) return true;
  return false;
}

/**
 * Whether the operator has explicitly asked for the signature channel. Does not
 * consider opt-outs; `signatureTelemetryEnabled` applies those on top.
 */
export function isOptedIn(config?: SignatureTelemetryConfig): boolean {
  if (config?.enabled === true) return true;
  if (envTruthy('AIM_TELEMETRY')) return true;
  return false;
}

/**
 * Whether the signature channel should run. OFF unless explicitly enabled, and
 * an opt-out always wins over an opt-in.
 */
export function signatureTelemetryEnabled(config?: SignatureTelemetryConfig): boolean {
  if (isOptedOut(config)) return false;
  return isOptedIn(config);
}

/** Resolve the registry base URL for ingestion. */
export function resolveRegistryUrl(config?: SignatureTelemetryConfig): string {
  return config?.registryUrl || process.env.OPENA2A_REGISTRY_URL || DEFAULT_REGISTRY_URL;
}

/** Persist the opt-out marker (used by `aim-arp telemetry opt-out`). Idempotent. */
export function writeOptOutMarker(): string {
  const home = opena2aHome();
  if (!existsSync(home)) mkdirSync(home, { recursive: true });
  const p = homePath(OPTOUT_MARKER_FILE);
  writeFileSync(p, new Date().toISOString() + '\n', { mode: 0o600 });
  return p;
}

/** Remove the opt-out marker (used by `aim-arp telemetry opt-in`). Idempotent. */
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
