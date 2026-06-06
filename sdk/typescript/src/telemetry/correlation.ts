/**
 * Causal-denial telemetry — correlation envelope.
 *
 * A correlation ID is minted at the earliest point in the request lifecycle
 * (content ingestion) and propagated best-effort to the injection detector and
 * the authorization call so that a single denied action's cause, intent, and
 * enforcement outcome can later be joined into one record.
 *
 * Propagation is ALWAYS best-effort: a missing ID degrades telemetry quality
 * but must never block enforcement or signing. Nothing here throws.
 */
import * as crypto from 'crypto';

/** Prefix marking an OpenA2A causal-denial-event correlation ID. */
export const CORRELATION_ID_PREFIX = 'cde_';

/** HTTP header that carries the correlation ID across process boundaries. */
export const CORRELATION_HEADER = 'x-opena2a-correlation-id';

/** W3C baggage key for in-process (trace-context) propagation. */
export const CORRELATION_BAGGAGE_KEY = 'opena2a.correlationId';

// Fixed-width, base36-encoded millisecond timestamp keeps IDs lexicographically
// ordered by time, which the async joiner relies on for time-window fallback.
const TS_WIDTH = 9;
const CORRELATION_ID_RE = /^cde_[0-9a-z]{9}_[0-9a-f]{12}$/;

/**
 * Mint a time-ordered correlation ID. Never throws — falls back to a
 * non-crypto random suffix if the crypto source is unavailable.
 */
export function mintCorrelationId(): string {
  const ts = Date.now().toString(36).padStart(TS_WIDTH, '0').slice(-TS_WIDTH);
  let rand: string;
  try {
    rand = crypto.randomBytes(6).toString('hex');
  } catch {
    // Best-effort fallback; correlation IDs are not a security boundary
    // (they only join records within one endpoint's local stream).
    rand = Math.floor(Math.random() * 0xffffffffffff)
      .toString(16)
      .padStart(12, '0')
      .slice(-12);
  }
  return `${CORRELATION_ID_PREFIX}${ts}_${rand}`;
}

/** True if the value is a well-formed correlation ID. */
export function isCorrelationId(value: unknown): value is string {
  return typeof value === 'string' && CORRELATION_ID_RE.test(value);
}

/**
 * Build the header object to attach to an outbound request (the Guard socket
 * call or the authorize call). Returns an empty object for an invalid ID so a
 * caller can always spread it safely.
 */
export function correlationHeaders(id: string | undefined): Record<string, string> {
  return isCorrelationId(id) ? { [CORRELATION_HEADER]: id } : {};
}

/**
 * Extract a correlation ID from a headers-like bag (case-insensitive).
 * Returns undefined when absent or malformed — never throws.
 */
export function extractCorrelationId(
  headers: Record<string, string | string[] | undefined> | undefined
): string | undefined {
  if (!headers) return undefined;
  try {
    for (const key of Object.keys(headers)) {
      if (key.toLowerCase() === CORRELATION_HEADER) {
        const raw = headers[key];
        const value = Array.isArray(raw) ? raw[0] : raw;
        return isCorrelationId(value) ? value : undefined;
      }
    }
  } catch {
    return undefined;
  }
  return undefined;
}
