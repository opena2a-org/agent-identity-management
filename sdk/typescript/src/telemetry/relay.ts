/**
 * Causal-denial telemetry — SDK -> Registry relay (the leg that ships indicators up).
 *
 * Reads full {@link CorrelatedRecord}s from the local authoritative log
 * (~/.opena2a/correlated-events.jsonl), reduces each to an anonymized
 * {@link SharedIndicator} via {@link toSharedIndicator} (which strips every
 * identifier — payloads, paths, credentials, resource/capability names,
 * correlationId, agentId, inputRef), and POSTs the denied-injection indicators
 * to the Registry's PUBLIC, count-only telemetry endpoint.
 *
 * CA decision (ARC-AIM-RUNTIME-PLAN.md §5.3/§5.4):
 *   - D-R1: targets the PUBLIC `POST /api/v1/telemetry/runtime` (count-only, no
 *     side-effects). The authenticated `/internal/telemetry/denial-correlated`
 *     path is reserved for a trusted internal aggregator — an end-user SDK does
 *     not hold an `internal:telemetry:write` service-account token.
 *   - D-R2: the indicator carries no package identity; `packageName` is attached
 *     at POST time from static relay config (the integrator's self-declared
 *     sensor package, like sensorToken), defaulting to a sentinel.
 *   - D-R3: the relay has its OWN master switch, default OFF, separate from
 *     telemetry capture. Sharing is a second, explicit opt-in.
 *   - D-R5: only `denied_injection_attempt` indicators are uploaded.
 *
 * Everything here is opt-in, consent-gated, off the enforcement critical path,
 * and best-effort: a failure is swallowed and never affects verifyAction.
 */
import { createHash, randomBytes } from 'crypto';
import { existsSync, mkdirSync, readFileSync, writeFileSync } from 'fs';
import { hostname, userInfo, homedir } from 'os';
import { join } from 'path';
import {
  toSharedIndicator,
  TELEMETRY_SCHEMA_VERSION,
  type CorrelatedRecord,
  type SharedIndicator,
  type SharedIndicatorContext,
} from './correlated-record';
import { readCorrelatedRecords } from './local-writer';

/** Default Registry base URL (shared with hackmyagent's GTIN forwarder). */
export const DEFAULT_REGISTRY_URL = 'https://api.oa2a.org';
/** Public, count-only runtime telemetry path. */
export const RUNTIME_TELEMETRY_PATH = '/api/v1/telemetry/runtime';
/** The only event type this relay uploads (CA D-R5). */
export const RELAY_EVENT_TYPE = 'denied_injection_attempt';
/** Sentinel package label used when the integrator declares none (CA D-R2). */
export const SENTINEL_PACKAGE_NAME = '_unattributed';

const DEFAULT_BATCH_SIZE = 50;
const DEFAULT_INTERVAL_MS = 60_000;
const DEFAULT_TIMEOUT_MS = 10_000;
const CURSOR_FILE = 'correlated-relay-cursor.json';

// Egress validation: the relay GUARANTEES the shape of what leaves the machine
// rather than trusting the server to reject malformed values after transmission.
// Record-derived optional fields (techniqueId, techniqueSource, confidence) are
// validated here and OMITTED if they do not match — a record tampered with
// locally cannot smuggle free text out via these fields.
const TECHNIQUE_ID_RE = /^T-\d{1,6}$/;
const VALID_TECHNIQUE_SOURCES = new Set(['apia', 'interim-mapping']);
const MAX_AGENT_CATEGORY = 128;
// Bound on same-millisecond boundary recordIds carried in the cursor. Sub-ms
// bursts beyond this (astronomically unlikely with ISO ms timestamps) may
// re-send a few count-only indicators on the next flush — acceptable for
// best-effort anonymized telemetry, and never a privacy or correctness hazard.
const MAX_BOUNDARY_IDS = 1000;

/** Cursor persisted between flushes so records are not re-uploaded. */
interface RelayCursor {
  /** ISO createdAt of the last fully-handled record. */
  lastCreatedAt: string;
  /** recordIds whose createdAt === lastCreatedAt (boundary dedup, capped at MAX_BOUNDARY_IDS). */
  lastRecordIds: string[];
}

export interface RelayConfig {
  /** Master switch for UPLOAD. Default false. Separate from telemetry capture. */
  enabled?: boolean;
  /** Registry base URL. Default https://api.oa2a.org. */
  registryUrl?: string;
  /** Integrator's self-declared sensor package (public). Default sentinel. */
  packageName?: string;
  /** Optional public package version label. */
  packageVersion?: string;
  /** Broad agent category (NOT identity). Optional. */
  agentCategory?: string;
  /** Data dir holding the local log + cursor. Default ~/.opena2a (OPENA2A_HOME aware). */
  dataDir?: string;
  /** Max indicators uploaded per flush. Default 50. */
  batchSize?: number;
  /** Managed flush interval (ms). Default 60000. */
  intervalMs?: number;
  /** Per-request timeout (ms). Default 10000. */
  timeoutMs?: number;
  /** Pre-derived sensor token. Default: sha256(host+user+salt). */
  sensorToken?: string;
  /** Injectable fetch (tests). Default global fetch. */
  fetchImpl?: typeof fetch;
  /** Optional sink for swallowed errors (best-effort observability). */
  onError?: (err: unknown) => void;
}

/** Outcome of a single flush, for tests/observability. Never thrown. */
export interface FlushResult {
  /** Records read fresh since the cursor (capped consideration). */
  considered: number;
  /** Indicators successfully POSTed this flush. */
  uploaded: number;
  /** True if a POST failed and the flush stopped early (cursor not advanced past it). */
  stoppedOnError: boolean;
}

/** Resolve the OpenA2A home dir (OPENA2A_HOME override, else ~/.opena2a). */
export function openA2AHome(): string {
  return process.env.OPENA2A_HOME || join(homedir(), '.opena2a');
}

/**
 * Stable per-device sensor token: sha256(hostname + username + persisted salt).
 * Uses the SAME salt file as hackmyagent's GTIN module so one device maps to one
 * anonymous sensor identity across tools. Best-effort; falls back to an ephemeral
 * token if the salt cannot be persisted.
 */
export function deriveSensorToken(dataDir: string = openA2AHome()): string {
  let salt: string;
  try {
    const saltPath = join(dataDir, 'gtin-sensor-salt');
    if (existsSync(saltPath)) {
      salt = readFileSync(saltPath, 'utf-8').trim();
    } else {
      salt = randomBytes(32).toString('hex');
      mkdirSync(dataDir, { recursive: true });
      writeFileSync(saltPath, salt, { mode: 0o600 });
    }
  } catch {
    // Cannot persist a salt — use an ephemeral one. Still anonymous; just not
    // stable across process restarts.
    salt = randomBytes(32).toString('hex');
  }

  let host = 'unknown';
  let user = 'unknown';
  try {
    host = hostname();
  } catch {
    /* keep default */
  }
  try {
    user = userInfo().username;
  } catch {
    /* keep default */
  }
  return createHash('sha256').update(`${host}|${user}|${salt}`).digest('hex');
}

/** Detect the runtime environment for the shared indicator. */
export function detectRuntimeEnv(): 'node' | 'python' | 'deno' {
  if (typeof (globalThis as Record<string, unknown>).Deno !== 'undefined') return 'deno';
  return 'node';
}

/**
 * Whole days since the relay was first used on this device. Marker file shared
 * with the GTIN module. Best-effort; returns 0 if it cannot be read/written.
 */
export function daysSinceInstall(dataDir: string = openA2AHome()): number {
  try {
    const markerPath = join(dataDir, 'gtin-install-date');
    let installDate: Date;
    if (existsSync(markerPath)) {
      installDate = new Date(readFileSync(markerPath, 'utf-8').trim());
      if (isNaN(installDate.getTime())) {
        installDate = new Date();
        writeFileSync(markerPath, installDate.toISOString(), { mode: 0o600 });
      }
    } else {
      installDate = new Date();
      mkdirSync(dataDir, { recursive: true });
      writeFileSync(markerPath, installDate.toISOString(), { mode: 0o600 });
    }
    const diffMs = Date.now() - installDate.getTime();
    return Math.max(0, Math.floor(diffMs / (24 * 60 * 60 * 1000)));
  } catch {
    return 0;
  }
}

/**
 * Ships locally-captured causal-denial indicators to the Registry.
 *
 * Call {@link start} for a managed unref'd flush timer, or {@link flushOnce}
 * to drive it manually (tests, one-shot CLI). Inert unless `enabled` is true.
 */
export class CorrelatedRelay {
  private readonly enabled: boolean;
  private readonly url: string;
  private readonly packageName: string;
  private readonly packageVersion?: string;
  private readonly agentCategory: string;
  private readonly dataDir: string;
  private readonly batchSize: number;
  private readonly intervalMs: number;
  private readonly timeoutMs: number;
  private readonly sensorToken: string;
  private readonly fetchImpl: typeof fetch;
  private readonly onError: (err: unknown) => void;

  private timer: ReturnType<typeof setInterval> | undefined;
  private flushing = false;

  constructor(config: RelayConfig = {}) {
    this.enabled = config.enabled === true;
    this.url = (config.registryUrl ?? DEFAULT_REGISTRY_URL).replace(/\/+$/, '') + RUNTIME_TELEMETRY_PATH;
    this.packageName = config.packageName?.trim() || SENTINEL_PACKAGE_NAME;
    this.packageVersion = config.packageVersion?.trim() || undefined;
    this.agentCategory = (config.agentCategory?.trim() || 'unspecified').slice(0, MAX_AGENT_CATEGORY);
    this.dataDir = config.dataDir ?? openA2AHome();
    this.batchSize = config.batchSize && config.batchSize > 0 ? config.batchSize : DEFAULT_BATCH_SIZE;
    this.intervalMs = config.intervalMs && config.intervalMs > 0 ? config.intervalMs : DEFAULT_INTERVAL_MS;
    this.timeoutMs = config.timeoutMs && config.timeoutMs > 0 ? config.timeoutMs : DEFAULT_TIMEOUT_MS;
    this.sensorToken = config.sensorToken || deriveSensorToken(this.dataDir);
    this.fetchImpl = config.fetchImpl ?? globalThis.fetch;
    this.onError = config.onError ?? (() => {});
  }

  /** Start a managed flush interval. No-op when disabled or already running. */
  start(): void {
    if (!this.enabled || this.timer) return;
    this.timer = setInterval(() => {
      void this.flushOnce();
    }, this.intervalMs);
    if (typeof this.timer.unref === 'function') this.timer.unref();
  }

  /** Stop the managed interval. Does not flush. */
  stop(): void {
    if (this.timer) {
      clearInterval(this.timer);
      this.timer = undefined;
    }
  }

  /**
   * Read fresh records, upload eligible indicators, advance the cursor. Never
   * throws. Returns a summary. Re-entrancy-guarded so overlapping timer ticks
   * cannot double-send.
   */
  async flushOnce(): Promise<FlushResult> {
    const result: FlushResult = { considered: 0, uploaded: 0, stoppedOnError: false };
    if (!this.enabled || this.flushing) return result;
    this.flushing = true;
    try {
      const cursor = this.readCursor();
      const fresh = this.readFresh(cursor);
      result.considered = fresh.length;
      if (fresh.length === 0) return result;

      // Build the sensor-level context once; it is identical for every record.
      const ctx: SharedIndicatorContext = {
        sensorToken: this.sensorToken,
        agentCategory: this.agentCategory,
        daySinceInstall: daysSinceInstall(this.dataDir),
        runtimeEnv: detectRuntimeEnv(),
      };

      let lastCreatedAt = cursor?.lastCreatedAt;
      let boundaryIds: string[] = cursor ? [...cursor.lastRecordIds] : [];

      for (const rec of fresh) {
        if (result.uploaded >= this.batchSize) break;

        const indicator = toSharedIndicator(rec, ctx);
        if (indicator.eventType === RELAY_EVENT_TYPE) {
          const outcome = await this.post(indicator);
          if (outcome === 'failed') {
            // Transient failure: leave this record (and the rest) for the next
            // flush; do NOT advance the cursor past it.
            result.stoppedOnError = true;
            break;
          }
          if (outcome === 'uploaded') result.uploaded++;
          // 'rejected' (4xx): non-retryable, advance the cursor without counting.
        }

        // Record handled (uploaded, or skipped because ineligible). Advance.
        if (rec.createdAt === lastCreatedAt) {
          boundaryIds.push(rec.recordId);
          // Bound the boundary set (see MAX_BOUNDARY_IDS). Keep the most recent.
          if (boundaryIds.length > MAX_BOUNDARY_IDS) {
            boundaryIds = boundaryIds.slice(-MAX_BOUNDARY_IDS);
          }
        } else {
          lastCreatedAt = rec.createdAt;
          boundaryIds = [rec.recordId];
        }
      }

      if (lastCreatedAt && lastCreatedAt !== cursor?.lastCreatedAt) {
        this.writeCursor({ lastCreatedAt, lastRecordIds: boundaryIds });
      } else if (lastCreatedAt && boundaryIds.length !== (cursor?.lastRecordIds.length ?? 0)) {
        this.writeCursor({ lastCreatedAt, lastRecordIds: boundaryIds });
      }
    } catch (err) {
      this.onError(err);
    } finally {
      this.flushing = false;
    }
    return result;
  }

  /**
   * Records not yet handled, in stable (createdAt, recordId) order. Assumes the
   * local log's `createdAt` is roughly monotonic (it is — records are appended
   * in assembly order by a single writer). A record appended out of order with
   * a `createdAt` earlier than the cursor is skipped; this is acceptable
   * best-effort data loss for anonymized count telemetry, never a correctness
   * hazard for the records that are sent.
   */
  private readFresh(cursor: RelayCursor | undefined): CorrelatedRecord[] {
    // `since` filters createdAt > since; subtract 1ms so the boundary timestamp
    // is included, then dedup precisely against the boundary recordIds.
    const sinceParam = cursor
      ? new Date(new Date(cursor.lastCreatedAt).getTime() - 1).toISOString()
      : undefined;
    const records = readCorrelatedRecords(this.dataDir, sinceParam ? { since: sinceParam } : undefined);

    const cutoff = cursor ? new Date(cursor.lastCreatedAt).getTime() : -Infinity;
    const fresh = records.filter((r) => {
      const t = new Date(r.createdAt).getTime();
      if (t > cutoff) return true;
      if (t === cutoff && cursor) return !cursor.lastRecordIds.includes(r.recordId);
      return false;
    });

    fresh.sort((a, b) => {
      const ta = new Date(a.createdAt).getTime();
      const tb = new Date(b.createdAt).getTime();
      if (ta !== tb) return ta - tb;
      return a.recordId < b.recordId ? -1 : a.recordId > b.recordId ? 1 : 0;
    });
    return fresh;
  }

  /**
   * POST one indicator. Never throws.
   *   - 'uploaded': 2xx — counted and cursor advances.
   *   - 'rejected': 4xx — non-retryable (bad/duplicate indicator); cursor
   *      advances but it is NOT counted as an upload.
   *   - 'failed':   5xx / network / timeout — transient; cursor stays put.
   */
  private async post(indicator: SharedIndicator): Promise<'uploaded' | 'rejected' | 'failed'> {
    const body: Record<string, unknown> = {
      schemaVersion: TELEMETRY_SCHEMA_VERSION,
      sensorToken: indicator.sensorToken,
      eventType: indicator.eventType,
      packageName: this.packageName,
      runtimeEnv: indicator.runtimeEnv,
      triggeredAt: indicator.triggeredAt,
      daySinceInstall: indicator.daySinceInstall,
      enforcementOutcome: indicator.enforcementOutcome,
      agentCategory: indicator.agentCategory,
    };
    if (this.packageVersion) body.packageVersion = this.packageVersion;
    // Validate record-derived optional fields before egress; omit on mismatch
    // so malformed/tampered values never leave the machine.
    if (indicator.techniqueId && TECHNIQUE_ID_RE.test(indicator.techniqueId)) {
      body.techniqueId = indicator.techniqueId;
    }
    if (indicator.techniqueSource && VALID_TECHNIQUE_SOURCES.has(indicator.techniqueSource)) {
      body.techniqueSource = indicator.techniqueSource;
    }
    if (
      typeof indicator.detectionConfidence === 'number' &&
      indicator.detectionConfidence >= 0 &&
      indicator.detectionConfidence <= 1
    ) {
      body.detectionConfidence = indicator.detectionConfidence;
    }

    const controller = new AbortController();
    const timeout = setTimeout(() => controller.abort(), this.timeoutMs);
    try {
      const res = await this.fetchImpl(this.url, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'User-Agent': 'OpenA2A-AIM-SDK-Relay',
        },
        body: JSON.stringify(body),
        signal: controller.signal,
      });
      // 4xx (bad/duplicate indicator) is non-retryable from our side: treat as
      // handled so the cursor advances. Only 5xx / network errors block.
      if (res.ok) return 'uploaded';
      if (res.status >= 400 && res.status < 500) {
        this.onError(new Error(`relay rejected indicator: HTTP ${res.status}`));
        return 'rejected';
      }
      this.onError(new Error(`relay upload failed: HTTP ${res.status}`));
      return 'failed';
    } catch (err) {
      this.onError(err);
      return 'failed';
    } finally {
      clearTimeout(timeout);
    }
  }

  private cursorPath(): string {
    return join(this.dataDir, CURSOR_FILE);
  }

  private readCursor(): RelayCursor | undefined {
    try {
      const raw = readFileSync(this.cursorPath(), 'utf-8');
      const parsed = JSON.parse(raw) as Partial<RelayCursor>;
      if (typeof parsed.lastCreatedAt === 'string' && Array.isArray(parsed.lastRecordIds)) {
        return { lastCreatedAt: parsed.lastCreatedAt, lastRecordIds: parsed.lastRecordIds };
      }
    } catch {
      // No cursor yet, or corrupt — start from the beginning.
    }
    return undefined;
  }

  private writeCursor(cursor: RelayCursor): void {
    try {
      mkdirSync(this.dataDir, { recursive: true });
      writeFileSync(this.cursorPath(), JSON.stringify(cursor), { mode: 0o600 });
    } catch (err) {
      this.onError(err);
    }
  }
}
