/**
 * Causal-denial telemetry — local correlated-record writer.
 *
 * The full correlated record is authoritative and stays on the machine. It is
 * written to an append-only JSON-lines log, separate from the generic audit
 * log so the two schemas never mix.
 *
 * Every write is best-effort: a failure returns false and is swallowed. The
 * telemetry pipeline must never be a runtime dependency — if capture fails,
 * enforcement and signing continue unaffected (zero-failures principle).
 */
import * as fs from 'fs';
import * as os from 'os';
import * as path from 'path';
import * as crypto from 'crypto';
import type { CorrelatedRecord } from './correlated-record';

const CORRELATED_FILE = 'correlated-events.jsonl';
const MAX_RECORD_SIZE = 1024 * 1024; // 1MB per record
const MAX_LOG_SIZE = 50 * 1024 * 1024; // rotate at 50MB
const MAX_ROTATED_LOGS = 5;

/** Default OpenA2A data directory (~/.opena2a). */
export function defaultDataDir(): string {
  return path.join(os.homedir(), '.opena2a');
}

function rotateIfNeeded(filePath: string): void {
  try {
    const stat = fs.statSync(filePath);
    if (stat.size <= MAX_LOG_SIZE) return;

    const suffix = `${Date.now()}.${process.pid}.${crypto.randomBytes(4).toString('hex')}`;
    try {
      fs.renameSync(filePath, `${filePath}.${suffix}`);
    } catch {
      // Another process may have rotated already — safe to continue.
    }

    try {
      const dir = path.dirname(filePath);
      const base = path.basename(filePath);
      const rotated = fs
        .readdirSync(dir)
        .filter((f) => f.startsWith(base + '.') && f !== base)
        .sort()
        .reverse();
      for (const old of rotated.slice(MAX_ROTATED_LOGS)) {
        try {
          fs.unlinkSync(path.join(dir, old));
        } catch {
          // best-effort
        }
      }
    } catch {
      // cleanup is best-effort
    }
  } catch {
    // File doesn't exist yet — nothing to rotate.
  }
}

/**
 * Append a correlated record to the local log. Returns true on success, false
 * on any failure. Never throws.
 */
export function writeCorrelatedRecord(
  record: CorrelatedRecord,
  dataDir: string = defaultDataDir()
): boolean {
  try {
    let serialized = JSON.stringify(record);
    if (serialized.length > MAX_RECORD_SIZE) {
      // Preserve the authoritative enforcement fact + join key; drop the rest.
      const trimmed: CorrelatedRecord = {
        schemaVersion: record.schemaVersion,
        correlationId: record.correlationId,
        recordId: record.recordId,
        agentId: record.agentId,
        createdAt: record.createdAt,
        enforcement: record.enforcement,
        assembly: { ...record.assembly, completeness: 'partial-missing-both' },
      };
      serialized = JSON.stringify({ ...trimmed, _truncated: true });
    }

    fs.mkdirSync(dataDir, { recursive: true });
    const filePath = path.join(dataDir, CORRELATED_FILE);
    rotateIfNeeded(filePath);
    fs.appendFileSync(filePath, serialized + '\n', 'utf-8');
    return true;
  } catch {
    return false;
  }
}

export interface ReadOptions {
  since?: string;
  limit?: number;
}

/** Read correlated records back from the local log. Returns [] on any error. */
export function readCorrelatedRecords(
  dataDir: string = defaultDataDir(),
  options?: ReadOptions
): CorrelatedRecord[] {
  try {
    const filePath = path.join(dataDir, CORRELATED_FILE);
    if (!fs.existsSync(filePath)) return [];

    const lines = fs.readFileSync(filePath, 'utf-8').trim().split('\n').filter(Boolean);
    let records: CorrelatedRecord[] = [];
    for (const line of lines) {
      try {
        records.push(JSON.parse(line) as CorrelatedRecord);
      } catch {
        // Skip a corrupt line rather than failing the whole read.
      }
    }

    if (options?.since) {
      const since = new Date(options.since).getTime();
      records = records.filter((r) => new Date(r.createdAt).getTime() > since);
    }
    if (options?.limit && options.limit > 0) {
      records = records.slice(-options.limit);
    }
    return records;
  } catch {
    return [];
  }
}
