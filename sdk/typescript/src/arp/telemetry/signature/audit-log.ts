/**
 * Local customer audit log (G2) — the trust counterweight for any sending at all.
 *
 * BEFORE any payload is transmitted, the producer appends to this local,
 * append-only JSONL log EXACTLY the bytes it will send. A customer can therefore
 * verify with their own eyes (via `aim-arp telemetry log`) that no payload, prompt,
 * argument, path, secret, or PII ever leaves the device — the evidence behind
 * the claim, rather than a promise to be taken on trust.
 *
 * The log write is best-effort and FAILS OPEN: a logging failure never blocks or
 * crashes the agent. Writes happen on the emitter's flush path, off the agent's
 * critical path.
 */

import { appendFile, readFile, mkdir } from 'fs/promises';
import { existsSync } from 'fs';
import { opena2aHome, homePath, AUDIT_LOG_FILE } from './paths';
import type { TelemetrySignatureRequest } from './wire';

/** Phase of an audit record's lifecycle. */
export type AuditPhase = 'queued' | 'sent' | 'buffered' | 'failed' | 'dropped';

export interface AuditRecord {
  /** ISO timestamp the record was written. */
  ts: string;
  phase: AuditPhase;
  /** Registry URL the payload targets. */
  endpoint: string;
  /** The EXACT JSON body that will be / was POSTed. This is the auditable bytes. */
  body: string;
  /** Convenience-decoded structural fields (also present inside `body`). */
  behavioralHash: string;
  techniqueId: string;
  severity: string;
  outcome: string;
  /** Transport result detail, when known (HTTP status or error reason). */
  detail?: string;
}

async function ensureHome(): Promise<void> {
  const home = opena2aHome();
  if (!existsSync(home)) await mkdir(home, { recursive: true });
}

/**
 * Append one audit record. Returns true on a durable write, false if the write
 * failed. Never throws — a logging error is swallowed (it must never block or
 * crash the agent). The RETURN VALUE is load-bearing for the `queued` phase: the
 * emitter gates transmit on it, so bytes are never sent without a local audit
 * record (audit-before-transmit, enforced — not incidental).
 */
export async function appendAuditRecord(rec: AuditRecord): Promise<boolean> {
  try {
    await ensureHome();
    await appendFile(homePath(AUDIT_LOG_FILE), JSON.stringify(rec) + '\n', { mode: 0o600 });
    return true;
  } catch {
    // Fail open for the agent (never throw); fail CLOSED for transmit (caller
    // sees false and must not send unaudited bytes).
    return false;
  }
}

/** Build the pre-transmit (`queued`) audit record for a request. */
export function queuedRecord(
  request: TelemetrySignatureRequest,
  body: string,
  endpoint: string,
  ts: string,
): AuditRecord {
  return {
    ts,
    phase: 'queued',
    endpoint,
    body,
    behavioralHash: request.behavioralHash,
    techniqueId: request.techniqueId,
    severity: request.severity,
    outcome: request.outcome,
  };
}

/**
 * Read the last `limit` audit records (default 100) for `aim-arp telemetry log`.
 * Returns [] if the log does not exist yet.
 */
export async function readAuditRecords(limit = 100): Promise<AuditRecord[]> {
  const p = homePath(AUDIT_LOG_FILE);
  if (!existsSync(p)) return [];
  let content: string;
  try {
    content = await readFile(p, 'utf8');
  } catch {
    return [];
  }
  const lines = content.split('\n').filter((l) => l.trim().length > 0);
  const tail = lines.slice(-limit);
  const out: AuditRecord[] = [];
  for (const line of tail) {
    try {
      out.push(JSON.parse(line) as AuditRecord);
    } catch {
      // skip a corrupt line rather than fail the whole read
    }
  }
  return out;
}

/** Absolute path of the audit log (shown to the user by the CLI). */
export function auditLogPath(): string {
  return homePath(AUDIT_LOG_FILE);
}
