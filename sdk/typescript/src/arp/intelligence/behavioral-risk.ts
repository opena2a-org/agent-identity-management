/**
 * Behavioral risk signal channel (AIComply P1, coordinator hot path).
 *
 * The `IntelligenceCoordinator` needs a way to ask the behavioral twin
 * (`RuntimeTwin` in `runtime-twin.ts`) "how anomalous does this event
 * look against the agent's behavioral baseline?" without importing the
 * twin directly into the comply path. Direct coupling would re-introduce the
 * layering break that the L0-comply gate was added to prevent: the twin
 * would sit inline with the crypto verifier, and a bug in the twin would
 * cascade into every classified event.
 *
 * The solution here is an IPC contract. The coordinator holds a
 * `BehavioralRiskSource` that abstracts transport. Two implementations
 * ship in this module:
 *
 *   - `InProcessBehavioralRiskSource` wraps any object that exposes
 *     `scoreARPEvent(event)`. This is the single-process fast path: no
 *     serialization, no socket, just a direct call guarded by a try/catch.
 *     `RuntimeTwin` satisfies the interface via its readonly
 *     scoreARPEvent method, but tests can inject any stub.
 *
 *   - `UnixSocketBehavioralRiskSource` speaks newline-delimited JSON over
 *     a unix domain socket (or a Windows named pipe via node `net`). This
 *     is the cross-process variant, used when the twin runs in its own
 *     daemon to isolate a crashy baseline from the enforcement path.
 *
 * Every caller-visible method is bounded:
 *
 *   - `getBehavioralRiskSignal(event, timeoutMs)` enforces a deadline on
 *     every call. The unix socket transport destroys the socket on timeout
 *     and resolves to `{status: 'unavailable', code: 'TIMEOUT'}`; the
 *     in-process source has no IO but still wraps the call in a bounded
 *     try/catch so a throwing twin cannot take down the coordinator.
 *
 *   - A five-failure circuit breaker opens the unix socket transport for
 *     30 seconds on repeated unavailable outcomes. A wedged IPC source
 *     therefore cannot cascade-block the coordinator indefinitely: after
 *     the breaker opens, calls resolve instantly with a cached error until
 *     cooldown expires.
 *
 * Parse-to-deny (CR-001) is honored on every path: the public surface
 * never throws. Every error branch resolves to a typed
 * `{status: 'unavailable', code, reason}` so the coordinator can route on
 * the code without string scraping. What "deny" means when the twin is
 * unavailable is the coordinator's policy, not this module's: the
 * coordinator records the unavailable signal on `event.data.behavioralRisk`
 * for downstream audit and leaves severity untouched. An IPC outage is
 * not evidence of threat, and raising severity on every event during twin
 * startup would be a regression; security-paranoid deployments can layer
 * a stricter policy on top of the recorded signal.
 */

import * as net from 'net';
import * as os from 'os';
import * as path from 'path';
import type { ARPEvent } from '../types';

/**
 * Shape returned by the twin's on-demand scorer. Mirrors RuntimeTwin's
 * internal AnomalyResult so this module does not need to import the twin
 * directly, and so test stubs can produce it without depending on the
 * full RuntimeTwin class surface.
 */
export interface BehavioralRiskScore {
  /** Normalized anomaly score in [0, 1]. Higher is riskier. */
  score: number;
  /** Response mapping derived from the score band. */
  action: 'allow' | 'alert' | 'throttle' | 'suspend' | 'kill';
  /** Human-readable reason for the action, safe to log. */
  reason: string;
}

/**
 * Minimum interface an in-process twin handle must satisfy to be plugged
 * into `InProcessBehavioralRiskSource`. RuntimeTwin.scoreARPEvent matches
 * this shape by construction.
 */
export interface BehavioralRiskScoreable {
  scoreARPEvent(event: ARPEvent): BehavioralRiskScore | null;
}

/**
 * Ok branch returned to the coordinator when the twin scored the event.
 * The `source` string identifies which twin produced the signal (for
 * audit) and `computedAtMs` is the wall clock at the time of scoring so
 * callers can age out old values if they cache.
 */
export interface BehavioralRiskSignal extends BehavioralRiskScore {
  source: string;
  computedAtMs: number;
}

/**
 * Discrete error codes on the unavailable branch. Kept as a closed union
 * so the coordinator can route on the code without parsing strings.
 */
export type BehavioralRiskUnavailableCode =
  | 'NOT_READY'        // baseline not yet trained
  | 'TIMEOUT'          // deadline exceeded on the hot path
  | 'TRANSPORT_ERROR'  // socket refused, closed, reset, or otherwise failed
  | 'PARSE_ERROR'      // malformed response from the server
  | 'CIRCUIT_OPEN'     // circuit breaker tripped, fast-failing
  | 'DISABLED'         // caller passed a no-op source
  | 'INTERNAL_ERROR';  // exception inside scoreARPEvent, never reaches caller

/**
 * Discriminated union a caller routes on. The ok branch carries the full
 * signal; the unavailable branch carries a code and a short reason.
 */
export type BehavioralRiskResult =
  | { status: 'ok'; signal: BehavioralRiskSignal }
  | { status: 'unavailable'; code: BehavioralRiskUnavailableCode; reason: string };

/**
 * Abstract transport the coordinator holds. All error paths resolve to an
 * `unavailable` result; `getBehavioralRiskSignal` must never throw and
 * must honor the timeout deadline.
 */
export interface BehavioralRiskSource {
  /**
   * Ask the twin for the behavioral risk signal associated with `event`.
   * Must resolve within `timeoutMs`. Must never throw. Must never block
   * the caller beyond the timeout: implementations that wait on IO must
   * tear down any pending work on deadline.
   */
  getBehavioralRiskSignal(event: ARPEvent, timeoutMs: number): Promise<BehavioralRiskResult>;
  /**
   * Release any transport resources. Coordinator calls this on shutdown.
   * Safe to call repeatedly.
   */
  close(): Promise<void>;
}

/**
 * Default timeout for a single risk signal request. Chosen to keep the
 * comply hot path well under a frame budget even under transport jitter.
 * The in-process source ignores this; the unix socket source enforces it.
 */
export const DEFAULT_BEHAVIORAL_RISK_TIMEOUT_MS = 25;

/**
 * Number of consecutive unavailable outcomes that open the circuit
 * breaker on the unix socket source. A wedged twin daemon will hit this
 * after five calls and stop consuming coordinator time.
 */
export const CIRCUIT_BREAKER_THRESHOLD = 5;

/**
 * Cooldown before the circuit breaker allows a single probe call to try
 * the transport again. 30 seconds is long enough to absorb transient
 * restarts without burning too many coordinator cycles on probes.
 */
export const CIRCUIT_BREAKER_COOLDOWN_MS = 30_000;

/**
 * IPC wire format version. Clients and servers both advertise `version: 1`
 * in every message; mismatched versions are treated as PARSE_ERROR rather
 * than silently accepted, so a future protocol change cannot be downgraded.
 */
export const BEHAVIORAL_RISK_WIRE_VERSION = 1;

/**
 * Resolve the platform-appropriate default socket path for the behavioral
 * risk server. Unix-like systems get a socket under
 * `~/.opena2a/arp/behavioral-risk-<agentId>.sock`; Windows gets a named
 * pipe in the `\\.\pipe\opena2a-arp-behavioral-risk-<agentId>` namespace.
 * Callers are free to override this.
 */
export function defaultBehavioralRiskSocketPath(agentId: string): string {
  const safe = agentId.replace(/[^a-zA-Z0-9_-]/g, '_');
  if (process.platform === 'win32') {
    return `\\\\.\\pipe\\opena2a-arp-behavioral-risk-${safe}`;
  }
  return path.join(os.homedir(), '.opena2a', 'arp', `behavioral-risk-${safe}.sock`);
}

/**
 * In-process risk source. Wraps a handle that implements
 * `scoreARPEvent`. Intended for single-process deployments where the
 * twin and the coordinator live in the same Node process. Synchronous by
 * construction, so the timeout is only a safety net against a
 * misbehaving twin that throws.
 *
 * Decoupling rationale: the coordinator holds this as a
 * `BehavioralRiskSource`, not as a `RuntimeTwin`. The comply path has no
 * knowledge of twin internals and a bug in the twin surfaces as an
 * `unavailable` result, not a cascade failure.
 */
export class InProcessBehavioralRiskSource implements BehavioralRiskSource {
  private readonly twin: BehavioralRiskScoreable;
  private readonly sourceName: string;

  constructor(twin: BehavioralRiskScoreable, sourceName = 'nanomind-l1-inproc') {
    this.twin = twin;
    this.sourceName = sourceName;
  }

  async getBehavioralRiskSignal(
    event: ARPEvent,
    _timeoutMs: number,
  ): Promise<BehavioralRiskResult> {
    // Synchronous call wrapped in try/catch. The twin's scoreARPEvent is
    // contract-bound to return null on not-ready; we map that to the
    // NOT_READY code so the coordinator can tell apart "no baseline" from
    // a runtime error.
    try {
      const result = this.twin.scoreARPEvent(event);
      if (result === null) {
        return {
          status: 'unavailable',
          code: 'NOT_READY',
          reason: 'behavioral twin baseline is not yet trained',
        };
      }
      return {
        status: 'ok',
        signal: {
          score: result.score,
          action: result.action,
          reason: result.reason,
          source: this.sourceName,
          computedAtMs: Date.now(),
        },
      };
    } catch (err) {
      // A throwing twin is a twin bug, not a threat signal. Surface it as
      // INTERNAL_ERROR; the coordinator's unavailable policy handles the
      // rest.
      return {
        status: 'unavailable',
        code: 'INTERNAL_ERROR',
        reason: `scoreARPEvent threw: ${(err as Error).message ?? String(err)}`,
      };
    }
  }

  async close(): Promise<void> {
    // No transport resources to release.
  }
}

/**
 * Unix-socket (or Windows named pipe) risk source. Opens a fresh
 * connection per request. Rationale: per-request connections keep the
 * transport state machine trivial (no pooling, no reconnection dance) and
 * the request cost is dominated by the twin's scoring latency, not the
 * connect itself. If perf measurement later shows connect-per-request is
 * a bottleneck, a pooled variant can be added without changing the
 * public BehavioralRiskSource interface.
 *
 * Circuit breaker semantics:
 *   - After `CIRCUIT_BREAKER_THRESHOLD` consecutive unavailable outcomes,
 *     the breaker opens and subsequent calls fast-fail with
 *     `CIRCUIT_OPEN` without touching the socket.
 *   - After `CIRCUIT_BREAKER_COOLDOWN_MS`, the breaker half-opens: the
 *     next call is allowed through. A successful call resets the failure
 *     counter and closes the breaker. A failed probe leaves the breaker
 *     open and restarts the cooldown.
 */
export class UnixSocketBehavioralRiskSource implements BehavioralRiskSource {
  private readonly socketPath: string;
  private readonly sourceName: string;
  private readonly now: () => number;
  private consecutiveFailures = 0;
  private circuitOpen = false;
  private circuitOpenedAtMs = 0;

  constructor(
    socketPath: string,
    options: { sourceName?: string; now?: () => number } = {},
  ) {
    this.socketPath = socketPath;
    this.sourceName = options.sourceName ?? 'nanomind-l1-ipc';
    this.now = options.now ?? Date.now;
  }

  async getBehavioralRiskSignal(
    event: ARPEvent,
    timeoutMs: number,
  ): Promise<BehavioralRiskResult> {
    // Fast path: breaker open. Check cooldown. If still open, fast-fail.
    // Otherwise allow a single probe through.
    if (this.circuitOpen) {
      const elapsed = this.now() - this.circuitOpenedAtMs;
      if (elapsed < CIRCUIT_BREAKER_COOLDOWN_MS) {
        return {
          status: 'unavailable',
          code: 'CIRCUIT_OPEN',
          reason: `circuit breaker open, ${CIRCUIT_BREAKER_COOLDOWN_MS - elapsed} ms remaining`,
        };
      }
      // Half-open: let the next call through. The breaker stays flagged
      // open until the call actually resolves; on success we close it.
    }

    const result = await this.roundTrip(event, timeoutMs);
    this.recordOutcome(result);
    return result;
  }

  private roundTrip(
    event: ARPEvent,
    timeoutMs: number,
  ): Promise<BehavioralRiskResult> {
    return new Promise<BehavioralRiskResult>((resolve) => {
      let settled = false;
      let socket: net.Socket | null = null;
      const chunks: Buffer[] = [];

      const settle = (r: BehavioralRiskResult) => {
        if (settled) return;
        settled = true;
        clearTimeout(timer);
        if (socket && !socket.destroyed) {
          socket.destroy();
        }
        resolve(r);
      };

      const timer = setTimeout(() => {
        settle({
          status: 'unavailable',
          code: 'TIMEOUT',
          reason: `behavioral risk ipc exceeded ${timeoutMs} ms deadline`,
        });
      }, timeoutMs);
      if (typeof timer.unref === 'function') timer.unref();

      try {
        socket = net.createConnection(this.socketPath);
      } catch (err) {
        settle({
          status: 'unavailable',
          code: 'TRANSPORT_ERROR',
          reason: `failed to initiate connection: ${(err as Error).message}`,
        });
        return;
      }

      socket.on('connect', () => {
        const request = JSON.stringify({
          kind: 'risk_signal_request',
          version: BEHAVIORAL_RISK_WIRE_VERSION,
          event,
        }) + '\n';
        try {
          socket!.write(request);
        } catch (err) {
          settle({
            status: 'unavailable',
            code: 'TRANSPORT_ERROR',
            reason: `failed to write request: ${(err as Error).message}`,
          });
        }
      });

      socket.on('data', (chunk: Buffer) => {
        chunks.push(chunk);
        const buf = Buffer.concat(chunks).toString('utf8');
        const nl = buf.indexOf('\n');
        if (nl < 0) return;
        const line = buf.slice(0, nl);
        settle(this.parseResponse(line));
      });

      socket.on('error', (err: Error) => {
        settle({
          status: 'unavailable',
          code: 'TRANSPORT_ERROR',
          reason: `socket error: ${err.message}`,
        });
      });

      socket.on('close', () => {
        settle({
          status: 'unavailable',
          code: 'TRANSPORT_ERROR',
          reason: 'socket closed before response received',
        });
      });
    });
  }

  private parseResponse(line: string): BehavioralRiskResult {
    let msg: unknown;
    try {
      msg = JSON.parse(line);
    } catch (err) {
      return {
        status: 'unavailable',
        code: 'PARSE_ERROR',
        reason: `invalid json response: ${(err as Error).message}`,
      };
    }
    if (msg === null || typeof msg !== 'object') {
      return {
        status: 'unavailable',
        code: 'PARSE_ERROR',
        reason: 'response body is not an object',
      };
    }
    const obj = msg as Record<string, unknown>;
    if (obj.version !== BEHAVIORAL_RISK_WIRE_VERSION) {
      return {
        status: 'unavailable',
        code: 'PARSE_ERROR',
        reason: `unsupported wire version: ${JSON.stringify(obj.version)}`,
      };
    }
    if (obj.kind === 'risk_signal_error') {
      const code = isUnavailableCode(obj.code) ? obj.code : 'TRANSPORT_ERROR';
      const reason = typeof obj.reason === 'string' ? obj.reason : 'server reported error';
      return { status: 'unavailable', code, reason };
    }
    if (obj.kind !== 'risk_signal_response') {
      return {
        status: 'unavailable',
        code: 'PARSE_ERROR',
        reason: `unexpected message kind: ${JSON.stringify(obj.kind)}`,
      };
    }
    if (
      typeof obj.score !== 'number' ||
      !Number.isFinite(obj.score) ||
      obj.score < 0 ||
      obj.score > 1
    ) {
      return {
        status: 'unavailable',
        code: 'PARSE_ERROR',
        reason: `score must be a finite number in [0,1], got ${JSON.stringify(obj.score)}`,
      };
    }
    if (!isRiskAction(obj.action)) {
      return {
        status: 'unavailable',
        code: 'PARSE_ERROR',
        reason: `action must be one of allow|alert|throttle|suspend|kill, got ${JSON.stringify(obj.action)}`,
      };
    }
    return {
      status: 'ok',
      signal: {
        score: obj.score,
        action: obj.action,
        reason: typeof obj.reason === 'string' ? obj.reason : '',
        source:
          typeof obj.source === 'string' && obj.source.length > 0
            ? obj.source
            : this.sourceName,
        computedAtMs:
          typeof obj.computedAtMs === 'number' && Number.isFinite(obj.computedAtMs)
            ? obj.computedAtMs
            : this.now(),
      },
    };
  }

  private recordOutcome(result: BehavioralRiskResult): void {
    if (result.status === 'ok') {
      this.consecutiveFailures = 0;
      this.circuitOpen = false;
      return;
    }
    // CIRCUIT_OPEN outcomes do not increment the failure counter; they
    // are already a symptom of the breaker being open. Counting them
    // would extend the cooldown indefinitely every time a caller probed
    // while the breaker is still cooling.
    if (result.code === 'CIRCUIT_OPEN') return;
    this.consecutiveFailures++;
    if (this.consecutiveFailures >= CIRCUIT_BREAKER_THRESHOLD) {
      this.circuitOpen = true;
      this.circuitOpenedAtMs = this.now();
    }
  }

  /**
   * Test-only accessor for the current breaker state. Exposed so tests
   * can assert breaker opens at the threshold and closes on a successful
   * probe without scraping internals through reflection.
   */
  _getBreakerStateForTest(): { open: boolean; failures: number; openedAtMs: number } {
    return {
      open: this.circuitOpen,
      failures: this.consecutiveFailures,
      openedAtMs: this.circuitOpenedAtMs,
    };
  }

  async close(): Promise<void> {
    // No pooled resources.
  }
}

function isUnavailableCode(v: unknown): v is BehavioralRiskUnavailableCode {
  return (
    v === 'NOT_READY' ||
    v === 'TIMEOUT' ||
    v === 'TRANSPORT_ERROR' ||
    v === 'PARSE_ERROR' ||
    v === 'CIRCUIT_OPEN' ||
    v === 'DISABLED' ||
    v === 'INTERNAL_ERROR'
  );
}

function isRiskAction(v: unknown): v is BehavioralRiskScore['action'] {
  return v === 'allow' || v === 'alert' || v === 'throttle' || v === 'suspend' || v === 'kill';
}
