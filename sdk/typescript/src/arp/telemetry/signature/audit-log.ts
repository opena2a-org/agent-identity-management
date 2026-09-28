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

const AUDIT_PHASES: ReadonlySet<string> = new Set<AuditPhase>([
  'queued',
  'sent',
  'buffered',
  'failed',
  'dropped',
]);

/**
 * True when a parsed line has the fields the CLI renders. A line that parses as
 * JSON but lacks them (a partial write, another writer's schema) is corrupt for
 * this reader: rendering it threw a raw TypeError on `padEnd`/`toUpperCase`.
 */
function isAuditRecord(v: unknown): v is AuditRecord {
  if (typeof v !== 'object' || v === null) return false;
  const r = v as Record<string, unknown>;
  return (
    typeof r.ts === 'string' &&
    typeof r.phase === 'string' &&
    AUDIT_PHASES.has(r.phase) &&
    typeof r.techniqueId === 'string' &&
    typeof r.severity === 'string' &&
    typeof r.outcome === 'string'
  );
}

/** What a read of the audit log found, including what it could not use. */
export interface AuditLogRead {
  /** Audit records from the examined tail, oldest first. */
  records: AuditRecord[];
  /** Non-empty lines in the examined tail that are not audit records. */
  unparseable: number;
  /**
   * Set when the log exists but could not be read (the error code, e.g.
   * `EACCES`). `records` is then empty, which does NOT mean nothing was sent.
   */
  readError?: string;
}

/**
 * Read the last `limit` lines of the audit log (default 100) and say what they
 * held. The audit log is the surface the disclosure tells users to trust, so a
 * corrupt or unreadable log must be reported as such: collapsing it to "no
 * records" made `telemetry log` say nothing had been sent and `status` say
 * "Sent: 0" over a log that held lines it could not parse (#416).
 */
export async function readAuditLog(limit = 100): Promise<AuditLogRead> {
  const p = homePath(AUDIT_LOG_FILE);
  if (!existsSync(p)) return { records: [], unparseable: 0 };
  let content: string;
  try {
    content = await readFile(p, 'utf8');
  } catch (err) {
    const code = (err as NodeJS.ErrnoException | undefined)?.code;
    return { records: [], unparseable: 0, readError: code || 'unreadable' };
  }
  const lines = content.split('\n').filter((l) => l.trim().length > 0);
  const tail = lines.slice(-limit);
  const records: AuditRecord[] = [];
  let unparseable = 0;
  for (const line of tail) {
    let parsed: unknown;
    try {
      parsed = JSON.parse(line);
    } catch {
      unparseable++;
      continue;
    }
    if (isAuditRecord(parsed)) records.push(parsed);
    else unparseable++;
  }
  return { records, unparseable };
}

/**
 * Read the last `limit` audit records (default 100). Returns [] if the log does
 * not exist yet. Corrupt lines are skipped; use `readAuditLog` to learn how
 * many there were and whether the file could be read at all.
 */
export async function readAuditRecords(limit = 100): Promise<AuditRecord[]> {
  return (await readAuditLog(limit)).records;
}

/** Absolute path of the audit log (shown to the user by the CLI). */
export function auditLogPath(): string {
  return homePath(AUDIT_LOG_FILE);
}
