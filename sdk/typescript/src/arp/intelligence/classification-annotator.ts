/**
 * Classification annotation — the producer-side wiring that tags runtime
 * events with a cleared NanoMind-Guard classification label, for DETECTION.
 *
 * Three pieces:
 *
 *   1. `ClassificationProvider` — the injectable seam. `classify(ev)` returns a
 *      signed `NanoMindGuardResult` or null. This was the ONE coupling the
 *      ARP→AIM migration touched: pre-migration the provider wrapped
 *      hackmyagent's local NanoMind-Guard daemon client; post-migration (this
 *      copy) it wraps the SDK-owned Guard-socket client in `../guard/`.
 *      Nothing downstream of the interface changed.
 *
 *   2. `NanoMindGuardClassificationProvider` — the default impl over the
 *      SDK-owned `../guard/nanomind-guard-client.ts`. It adapts a daemon
 *      classify response into a `NanoMindGuardResult` ONLY when the daemon
 *      emitted the signed fields (signature, contentHash, timestamp,
 *      modelVersion). Without them it returns null — it never fabricates a
 *      signature, so an unsigned daemon degrades to "no classification" rather
 *      than planting an unverifiable label.
 *
 *   3. `ClassificationAnnotator` — buffered, off the hot path. For each event
 *      it calls the provider, routes the result through the already-built
 *      `verifyClassification` (Ed25519+ML-DSA-44 signature + the tier-keyed
 *      rejection matrix), and on the `valid` branch writes the cleared label to
 *      `event.data.classification`. A provider/Guard outage, an unsigned
 *      response, or a failed verification leaves the field untouched — the
 *      annotator degrades a signal, it never blocks and never plants an
 *      unverified label.
 *
 * Detection ≠ enforcement. Writing `event.data.classification` here feeds the
 * OFFLINE consumers (the sequence projector, the telemetry tee). The
 * coordinator's comply gate only acts on the field when the signed manifest
 * sets `comply.enforce === true`; with the default that read is a no-op, so
 * annotation can never, by itself, gate the agent. See `coordinator.ts`.
 */

import * as crypto from 'crypto';
import type {
  ARPEvent,
  NanoMindGuardResult,
  NanoMindGuardVerifyOptions,
  NanoMindGuardVerifyResult,
} from '../types';
import { applyVerifiedClassification } from './verify-classification';
import {
  sendClassify,
  type ClassifyResponse,
  type ClientOptions,
} from '../guard/nanomind-guard-client';

/**
 * The injectable classification source. The annotator depends only on this
 * interface; the concrete daemon vs. AIM-SDK choice is a construction detail.
 */
export interface ClassificationProvider {
  /**
   * Classify a runtime event. Returns a signed Guard result, or null when the
   * provider is unavailable or cannot produce a verifiable result. Must not
   * throw — every failure path resolves to null.
   */
  classify(ev: ARPEvent): Promise<NanoMindGuardResult | null>;
}

/** Maximum characters of event text sent to the Guard for classification. */
const MAX_CLASSIFY_TEXT = 4096;

/**
 * Build the text representation of an ARP event handed to the Guard. The Guard
 * classifies text; we give it the human description plus a bounded JSON of the
 * structured data, deterministically (sorted keys) so the same event yields
 * the same content hash.
 */
export function eventToClassifyText(event: ARPEvent): string {
  let dataStr = '';
  try {
    dataStr = JSON.stringify(event.data ?? {});
  } catch {
    dataStr = '';
  }
  const text = `${event.source} ${event.category} ${event.description} ${dataStr}`.trim();
  return text.length > MAX_CLASSIFY_TEXT ? text.slice(0, MAX_CLASSIFY_TEXT) : text;
}

/**
 * Pre-migration provider over the local NanoMind-Guard daemon. Adapts a daemon
 * classify response into a signed `NanoMindGuardResult`, or null.
 *
 * The daemon response is forwarded ONLY when it carries the signed envelope
 * the verifier needs (`signature`, `contentHash`, `timestamp`, `modelVersion`).
 * These fields are part of the signed payload, so the provider passes them
 * through verbatim — it never recomputes `contentHash` (a recomputed hash would
 * not match the signature) and never synthesizes a signature.
 */
export class NanoMindGuardClassificationProvider implements ClassificationProvider {
  constructor(private readonly clientOptions: ClientOptions = {}) {}

  async classify(ev: ARPEvent): Promise<NanoMindGuardResult | null> {
    const text = eventToClassifyText(ev);
    if (text.length === 0) return null;

    let response: ClassifyResponse | null;
    try {
      response = await sendClassify(text, this.clientOptions);
    } catch {
      // The daemon client is already null-on-failure; this catch is belt-and-
      // suspenders so a provider never throws into the annotator.
      return null;
    }
    if (response === null || response.ok !== true) return null;

    return toSignedResult(response as unknown as Record<string, unknown>);
  }
}

/**
 * Map a daemon ok-response bag to a `NanoMindGuardResult` iff it carries the
 * full signed envelope. Returns null on any missing / malformed field. Kept
 * exported for unit coverage of the adapter boundary.
 */
export function toSignedResult(
  bag: Record<string, unknown>,
): NanoMindGuardResult | null {
  const classification = bag.classification;
  const confidence = bag.confidence;
  const modelVersion = bag.modelVersion;
  const contentHash = bag.contentHash;
  const timestamp = bag.timestamp;
  const signature = bag.signature;

  if (typeof classification !== 'string' || classification.length === 0) return null;
  if (typeof confidence !== 'number' || !Number.isFinite(confidence)) return null;
  if (typeof modelVersion !== 'string' || modelVersion.length === 0) return null;
  if (typeof contentHash !== 'string' || !/^[0-9a-fA-F]{64}$/.test(contentHash)) return null;
  if (typeof timestamp !== 'number' || !Number.isFinite(timestamp)) return null;
  if (signature === null || typeof signature !== 'object') return null;

  return {
    classification,
    confidence,
    modelVersion,
    contentHash,
    timestamp,
    signature: signature as NanoMindGuardResult['signature'],
  };
}

/** Item queued for buffered annotation. */
interface QueueItem {
  event: ARPEvent;
  onDone?: (result: NanoMindGuardVerifyResult | null) => void;
}

export interface ClassificationAnnotatorOptions {
  /** Max concurrent provider calls. Default 4. Keeps the off-hot-path pump bounded. */
  concurrency?: number;
  /** Hard cap on the buffered queue. Excess enqueues are dropped (detection is best-effort). Default 1000. */
  maxQueue?: number;
}

/**
 * Buffered annotator. `annotate` is the direct (awaitable) form; `enqueue` is
 * the off-hot-path form a hot caller uses fire-and-forget. Both share the same
 * verify-and-write path. Provider/verification failure leaves
 * `event.data.classification` untouched.
 */
export class ClassificationAnnotator {
  private readonly queue: QueueItem[] = [];
  private active = 0;
  private readonly concurrency: number;
  private readonly maxQueue: number;
  private drainWaiters: Array<() => void> = [];

  constructor(
    private readonly provider: ClassificationProvider,
    private readonly verifyOptions: NanoMindGuardVerifyOptions,
    options: ClassificationAnnotatorOptions = {},
  ) {
    this.concurrency = Math.max(1, options.concurrency ?? 4);
    this.maxQueue = Math.max(1, options.maxQueue ?? 1000);
  }

  /**
   * Classify and, on a verified result, write the cleared label to
   * `event.data.classification`. Returns the verify result, or null when the
   * provider had nothing to offer (outage / unsigned / unavailable). Never
   * throws.
   */
  async annotate(event: ARPEvent): Promise<NanoMindGuardVerifyResult | null> {
    let result: NanoMindGuardResult | null;
    try {
      result = await this.provider.classify(event);
    } catch {
      return null;
    }
    if (result === null) return null;

    // Bind the label to THIS event. The signed result carries the contentHash
    // the Guard hashed; it must equal the hash of the text we would classify
    // for this event. Without this check a valid, fresh, signed result for some
    // other event could be applied here (a confused-deputy / replay within the
    // freshness window). On mismatch we degrade to null rather than plant a
    // label that does not provably belong to the event. (The Guard must hash
    // the output of `eventToClassifyText`; see that function.)
    if (result.contentHash !== classifyTextHash(event)) return null;

    try {
      return await applyVerifiedClassification(event.data, result, this.verifyOptions);
    } catch {
      // Verification helper is total by contract, but never let an unexpected
      // throw escape the detection path.
      return null;
    }
  }

  /**
   * Enqueue an event for buffered annotation off the hot path. Returns
   * immediately. When the queue is full the event is dropped (detection is
   * best-effort; a dropped annotation only loses a signal, never correctness).
   */
  enqueue(
    event: ARPEvent,
    onDone?: (result: NanoMindGuardVerifyResult | null) => void,
  ): void {
    if (this.queue.length >= this.maxQueue) {
      onDone?.(null);
      return;
    }
    this.queue.push({ event, onDone });
    this.pump();
  }

  /** Resolves when the queue is empty and no annotation is in flight. */
  drain(): Promise<void> {
    if (this.queue.length === 0 && this.active === 0) return Promise.resolve();
    return new Promise((resolve) => this.drainWaiters.push(resolve));
  }

  private pump(): void {
    while (this.active < this.concurrency && this.queue.length > 0) {
      const item = this.queue.shift() as QueueItem;
      this.active++;
      // Detach: never block the enqueuing caller.
      void this.annotate(item.event)
        .then((result) => item.onDone?.(result))
        .catch(() => item.onDone?.(null))
        .finally(() => {
          this.active--;
          if (this.queue.length > 0) this.pump();
          else if (this.active === 0) this.settleDrain();
        });
    }
  }

  private settleDrain(): void {
    const waiters = this.drainWaiters;
    this.drainWaiters = [];
    for (const w of waiters) w();
  }
}

/**
 * Compute the SHA-256 hex digest of the classify text for an event. Exposed so
 * a signer (the daemon, or a test harness) can produce a `contentHash` that
 * matches what this side would expect for the same event.
 */
export function classifyTextHash(event: ARPEvent): string {
  return crypto.createHash('sha256').update(eventToClassifyText(event)).digest('hex');
}
