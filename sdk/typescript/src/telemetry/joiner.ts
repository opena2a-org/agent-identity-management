/**
 * Causal-denial telemetry — async joiner.
 *
 * Buffers the three parts of a denied action (enforcement fact, intent
 * inference, detection inference) keyed by correlation ID, and emits one
 * assembled record when they meet — fully when all parts arrive, or as a
 * partial when the join window elapses.
 *
 * The joiner runs OFF the enforcement path. It holds parts in memory and emits
 * via a sink callback (default: the local writer). It is deterministic: time is
 * supplied by an injectable clock, so production can drive it with a timer while
 * tests drive it with an explicit clock.
 *
 * An enforcement part is required to assemble a record (the fact anchors the
 * join). Parts that never receive an enforcement event within the window are
 * dropped as orphans rather than emitted.
 */
import {
  buildCorrelatedRecord,
  TELEMETRY_SCHEMA_VERSION,
  type CorrelatedRecord,
  type EnforcementInput,
  type IntentInput,
  type DetectionInput,
} from './correlated-record';
import { writeCorrelatedRecord } from './local-writer';

const DEFAULT_WINDOW_MS = 5000;

export interface EnforcementEvent {
  correlationId: string;
  agentId: string;
  enforcement: EnforcementInput;
}
export interface IntentEvent {
  correlationId: string;
  intent: IntentInput;
}
export interface DetectionEvent {
  correlationId: string;
  detection: DetectionInput;
}

export interface JoinerOptions {
  /** How long to hold partial joins before emitting/dropping. Default 5000ms. */
  windowMs?: number;
  /** Clock source (ms). Default Date.now. Injected for deterministic tests. */
  now?: () => number;
  /** Sink for assembled records. Default: append to the local log. */
  onRecord?: (record: CorrelatedRecord) => void;
  /** Stamped into assembly.joinerVersion. */
  joinerVersion?: string;
}

interface Buffer {
  agentId?: string;
  enforcement?: EnforcementInput;
  intent?: IntentInput;
  detection?: DetectionInput;
  firstSeenMs: number;
}

/**
 * Correlation-ID-keyed joiner. Call ingest* as parts arrive and flushExpired()
 * on a timer (or start()/stop() for a managed interval).
 */
export class CorrelationJoiner {
  private readonly windowMs: number;
  private readonly now: () => number;
  private readonly onRecord: (record: CorrelatedRecord) => void;
  private readonly joinerVersion: string;
  private readonly buffers = new Map<string, Buffer>();
  private timer: ReturnType<typeof setInterval> | undefined;

  /** Count of part-sets dropped for never receiving an enforcement fact. */
  public droppedOrphans = 0;

  constructor(options: JoinerOptions = {}) {
    this.windowMs = options.windowMs ?? DEFAULT_WINDOW_MS;
    this.now = options.now ?? Date.now;
    this.onRecord = options.onRecord ?? ((r) => void writeCorrelatedRecord(r));
    this.joinerVersion = options.joinerVersion ?? TELEMETRY_SCHEMA_VERSION;
  }

  /** Number of correlation IDs currently buffered. */
  get pending(): number {
    return this.buffers.size;
  }

  ingestEnforcement(event: EnforcementEvent): void {
    const buf = this.bufferFor(event.correlationId);
    buf.enforcement = event.enforcement;
    buf.agentId = event.agentId;
    this.tryEmitComplete(event.correlationId);
  }

  ingestIntent(event: IntentEvent): void {
    const buf = this.bufferFor(event.correlationId);
    buf.intent = event.intent;
    this.tryEmitComplete(event.correlationId);
  }

  ingestDetection(event: DetectionEvent): void {
    const buf = this.bufferFor(event.correlationId);
    buf.detection = event.detection;
    this.tryEmitComplete(event.correlationId);
  }

  /**
   * Emit/drop any buffers whose window has elapsed. Buffers with an enforcement
   * fact emit a partial record; buffers without one are dropped as orphans.
   */
  flushExpired(nowMs: number = this.now()): void {
    for (const [id, buf] of this.buffers) {
      if (nowMs - buf.firstSeenMs < this.windowMs) continue;
      if (buf.enforcement) {
        this.emit(id, buf, 'fallback:window');
      } else {
        this.droppedOrphans++;
      }
      this.buffers.delete(id);
    }
  }

  /** Start a managed flush interval (production use). */
  start(intervalMs: number = 1000): void {
    if (this.timer) return;
    this.timer = setInterval(() => this.flushExpired(), intervalMs);
    if (typeof this.timer.unref === 'function') this.timer.unref();
  }

  /** Stop the managed interval and flush whatever remains. */
  stop(): void {
    if (this.timer) {
      clearInterval(this.timer);
      this.timer = undefined;
    }
  }

  private bufferFor(correlationId: string): Buffer {
    let buf = this.buffers.get(correlationId);
    if (!buf) {
      buf = { firstSeenMs: this.now() };
      this.buffers.set(correlationId, buf);
    }
    return buf;
  }

  private tryEmitComplete(correlationId: string): void {
    const buf = this.buffers.get(correlationId);
    if (buf && buf.enforcement && buf.intent && buf.detection) {
      this.emit(correlationId, buf, 'correlationId');
      this.buffers.delete(correlationId);
    }
  }

  private emit(correlationId: string, buf: Buffer, joinedBy: string): void {
    // enforcement presence is guaranteed by both call sites.
    const record = buildCorrelatedRecord({
      correlationId,
      agentId: buf.agentId as string,
      enforcement: buf.enforcement as EnforcementInput,
      intent: buf.intent,
      detection: buf.detection,
      joinedBy,
      joinerVersion: this.joinerVersion,
    });
    this.onRecord(record);
  }
}

/**
 * A full record is the high-confidence training signal: all parts present, the
 * enforcement fact observed, and the technique source known. Partial or
 * fallback-joined records are telemetry-only.
 */
export function isHighConfidence(record: CorrelatedRecord): boolean {
  return (
    record.assembly.completeness === 'full' &&
    record.enforcement.observed === true &&
    record.assembly.joinedBy === 'correlationId' &&
    record.detection?.techniqueSource !== undefined
  );
}
