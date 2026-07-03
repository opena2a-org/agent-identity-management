/**
 * SignatureEmitter — the producer (G2) for the structural signature channel.
 *
 * Pipeline, per anomalous ARP event:
 *   redact (FAIL CLOSED) -> aggregate identical shapes -> build signed wire
 *   request -> write the EXACT bytes to the local audit log BEFORE transmit ->
 *   POST to the registry -> on failure, buffer + retry next flush (FAIL OPEN).
 *
 * Invariants:
 *   - FAIL CLOSED on redaction: an un-mappable behavior is never transmitted.
 *   - FAIL OPEN on transport: a down/erroring registry never blocks, delays, or
 *     crashes the agent — the work is timer-driven, off the critical path.
 *   - Audit-before-transmit: the local record of the bytes always precedes send.
 */

import type { ARPEvent } from '../../types';
import { redactEvent, type OutcomeClass, type RedactedSignal } from './redaction';
import { behavioralHash } from './behavioral-hash';
import { buildSignedSubmission, SIGNATURE_INGEST_PATH } from './wire';
import { appendAuditRecord, queuedRecord } from './audit-log';
import { isOptedOut } from './config';
import { VERSION } from '../../index';

/** A queued signal with the number of occurrences it currently represents. */
interface QueuedSignal {
  signal: RedactedSignal;
  count: number;
}

export interface SignatureEmitterConfig {
  /** Registry base URL (e.g. https://api.oa2a.org). */
  registryUrl: string;
  /** Batch flush interval (ms). Default 30s. */
  batchIntervalMs?: number;
  /** Max buffered signals across retries before shedding oldest. Default 500. */
  maxBufferSize?: number;
}

const DEFAULT_BATCH_INTERVAL_MS = 30_000;
const DEFAULT_MAX_BUFFER = 500;
const SUBMIT_TIMEOUT_MS = 10_000;

export class SignatureEmitter {
  private readonly registryUrl: string;
  private readonly endpoint: string;
  private readonly batchIntervalMs: number;
  private readonly maxBufferSize: number;
  private queue: QueuedSignal[] = [];
  private flushTimer: ReturnType<typeof setInterval> | null = null;
  private stopped = false;

  constructor(config: SignatureEmitterConfig) {
    this.registryUrl = config.registryUrl.replace(/\/+$/, '');
    this.endpoint = this.registryUrl + SIGNATURE_INGEST_PATH;
    this.batchIntervalMs = config.batchIntervalMs ?? DEFAULT_BATCH_INTERVAL_MS;
    this.maxBufferSize = config.maxBufferSize ?? DEFAULT_MAX_BUFFER;
  }

  /** Start the periodic flush timer (also auto-starts on first queued event). */
  start(): void {
    if (this.flushTimer || this.stopped) return;
    this.flushTimer = setInterval(() => {
      this.flush().catch(() => {
        /* fail open */
      });
    }, this.batchIntervalMs);
    if (this.flushTimer && typeof this.flushTimer === 'object' && 'unref' in this.flushTimer) {
      this.flushTimer.unref();
    }
  }

  /**
   * Ingest an ARP event. Redacts fail-closed; only a successfully redacted
   * structural signal is queued. `outcome` defaults to a value derived from the
   * event (the emitter knows whether enforcement acted).
   */
  onEvent(event: ARPEvent, outcome?: OutcomeClass): void {
    if (this.stopped) return;
    const signal = redactEvent(event, outcome ?? deriveOutcome(event));
    if (!signal) return; // FAIL CLOSED — not reportable

    this.enqueue({ signal, count: 1 });
    if (!this.flushTimer) this.start();
  }

  private enqueue(entry: QueuedSignal): void {
    this.queue.push(entry);
    // Shed oldest if the buffer is saturated (a long registry outage). Audit the
    // drop so the customer can see nothing was silently lost.
    while (this.queue.length > this.maxBufferSize) {
      const dropped = this.queue.shift();
      if (dropped) {
        void appendAuditRecord({
          ts: new Date().toISOString(),
          phase: 'dropped',
          endpoint: this.endpoint,
          body: '',
          behavioralHash: behavioralHash(dropped.signal),
          techniqueId: dropped.signal.techniqueId,
          severity: dropped.signal.severity,
          outcome: dropped.signal.outcomeClass,
          detail: `buffer saturated; oldest signal shed (count=${dropped.count})`,
        });
      }
    }
  }

  /** Drain the queue: aggregate identical shapes, sign, audit, transmit. */
  async flush(): Promise<void> {
    // Honor a runtime opt-out (env / marker written after start): stop sending
    // immediately and discard anything queued. Cheap (env read + one file stat).
    if (isOptedOut()) {
      this.queue = [];
      return;
    }
    if (this.queue.length === 0) return;
    const batch = this.queue.splice(0);

    // Aggregate identical structural shapes into one submission, SUMMING counts
    // (so occurrences are preserved across aggregation and across retries).
    const groups = new Map<string, QueuedSignal>();
    for (const entry of batch) {
      const key = behavioralHash(entry.signal) + '|' + entry.signal.outcomeClass + '|' + entry.signal.severity;
      const g = groups.get(key);
      if (g) g.count += entry.count;
      else groups.set(key, { signal: entry.signal, count: entry.count });
    }

    for (const entry of groups.values()) {
      const ok = await this.submitOne(entry.signal, entry.count);
      if (!ok) {
        // FAIL OPEN: re-buffer for the next flush, PRESERVING the occurrence
        // count (bounded by enqueue shedding).
        this.enqueue(entry);
      }
    }
  }

  /** Build, audit (before transmit), and POST a single aggregated signal. */
  private async submitOne(signal: RedactedSignal, count: number): Promise<boolean> {
    let built;
    try {
      built = await buildSignedSubmission(signal, { count });
    } catch (err) {
      // Signing/state failure: audit and treat as not-sent (fail open).
      void appendAuditRecord({
        ts: new Date().toISOString(),
        phase: 'failed',
        endpoint: this.endpoint,
        body: '',
        behavioralHash: behavioralHash(signal),
        techniqueId: signal.techniqueId,
        severity: signal.severity,
        outcome: signal.outcomeClass,
        detail: 'build/sign failed: ' + errMessage(err),
      });
      return false;
    }

    const ts = new Date().toISOString();
    // AUDIT BEFORE TRANSMIT — enforced, not incidental. If the local record of
    // the exact bytes cannot be written, we DO NOT transmit (fail closed for the
    // leak invariant; fail open for the agent — the signal re-buffers and retries
    // next flush, so a transient disk error recovers). Bytes never leave the
    // machine without a local audit record.
    const audited = await appendAuditRecord(queuedRecord(built.request, built.body, this.endpoint, ts));
    if (!audited) {
      return false; // re-buffer; never send unaudited bytes
    }

    try {
      const controller = new AbortController();
      const timer = setTimeout(() => controller.abort(), SUBMIT_TIMEOUT_MS);
      const res = await fetch(this.endpoint, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'User-Agent': `OpenA2A-ARP/${VERSION}`,
        },
        body: built.body,
        signal: controller.signal,
      });
      clearTimeout(timer);

      if (!res.ok) {
        const detail = `HTTP ${res.status}`;
        await appendAuditRecord({ ...queuedRecord(built.request, built.body, this.endpoint, new Date().toISOString()), phase: 'failed', detail });
        // 4xx (e.g. ingestion disabled / schema reject) is not retryable; 5xx is.
        return res.status >= 400 && res.status < 500 ? true : false;
      }
      await appendAuditRecord({ ...queuedRecord(built.request, built.body, this.endpoint, new Date().toISOString()), phase: 'sent', detail: `HTTP ${res.status}` });
      return true;
    } catch (err) {
      await appendAuditRecord({ ...queuedRecord(built.request, built.body, this.endpoint, new Date().toISOString()), phase: 'buffered', detail: errMessage(err) });
      return false;
    }
  }

  /** Flush and stop. After shutdown no new events are accepted. */
  async shutdown(): Promise<void> {
    this.stopped = true;
    if (this.flushTimer) {
      clearInterval(this.flushTimer);
      this.flushTimer = null;
    }
    await this.flush().catch(() => {
      /* fail open */
    });
  }

  getQueueLength(): number {
    return this.queue.length;
  }

  isRunning(): boolean {
    return !this.stopped;
  }
}

/** Map an event to a runtime outcome from explicit structured flags only. */
export function deriveOutcome(event: ARPEvent): OutcomeClass {
  const data = event.data ?? {};
  if (data['blocked'] === true || data['enforced'] === true) return 'blocked';
  const action = data['enforcementAction'];
  if (action === 'pause' || action === 'kill' || action === 'block') return 'blocked';
  if (data['allowed'] === true) return 'allowed';
  return 'detected';
}

function errMessage(err: unknown): string {
  const m = err instanceof Error ? err.message : String(err);
  return m.length > 200 ? m.slice(0, 200) : m;
}
