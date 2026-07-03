/**
 * NanoMind-Guard IPC client — JSON-Lines protocol over Unix domain socket.
 *
 * SDK-owned copy: this is the Guard-socket handoff of the ARP -> AIM SDK
 * runtime-protection migration. Provenance: copied from hackmyagent
 * `src/nanomind-core/inference/nanomind-guard-client.ts` (which remains the
 * scan-time client there); this copy is the runtime module's own path to the
 * daemon so the SDK carries no dependency on the scanner package. The wire
 * protocol is owned by the daemon — if it changes, both clients change.
 *
 * The NanoMind-Guard daemon (Python) loads the v3 NLM and the input-classifier
 * gate, and serves classification requests over an AF_UNIX socket. Direct
 * HuggingFace downloads and llama.cpp subprocess calls are deliberately not
 * offered because the daemon is the primitive that puts the v3.1
 * input-classifier gate in front of the NLM.
 *
 * Protocol: one request, one response, one connection.
 *   send: {"op": "classify", "text": "..."}\n
 *   recv: {"ok": true, ...}\n
 *
 * Socket absent or unreachable → returns null. Callers treat null as
 * "analyst unavailable" (the analyst is opt-in behind --nanomind, so a null
 * result is not a benign verdict — it is graceful unavailability).
 *
 * NEVER fall back to direct HF download on socket-absent. That bypasses the
 * input-classifier gate and silently emits the constant off-topic verdict
 * for any input during the gate's downtime — exactly the silent-FN failure
 * mode the gate exists to prevent.
 */

import { createConnection, Socket } from 'node:net';
import { lstatSync } from 'node:fs';

// ============================================================================
// Constants
// ============================================================================

/** Filesystem path used when neither callers nor env vars provide one. */
export const FALLBACK_SOCK_PATH = '/tmp/nanomind-guard.sock';

/**
 * Resolve the daemon socket path. Precedence: explicit override >
 * NANOMIND_GUARD_SOCK env var > FALLBACK_SOCK_PATH. Read per-call so test
 * harnesses can flip the env var between cases without re-importing.
 */
export function resolveSocketPath(override?: string): string {
  return override ?? process.env.NANOMIND_GUARD_SOCK ?? FALLBACK_SOCK_PATH;
}

/**
 * Check that the socket path is safe to connect to. Refuses symlinks (an
 * attacker who can write to the socket-path directory could otherwise
 * redirect HMA to an arbitrary socket they control, exfiltrating every
 * analyst prompt to that socket's owner).
 *
 * Returns true if the path is missing entirely (the daemon may not be
 * running — that's the caller's null-return case) or if it is a regular
 * Unix socket file. Returns false on symlink.
 */
function isSocketPathSafe(path: string): boolean {
  try {
    const stat = lstatSync(path);
    // Reject symlinks before connect(). isSocket() on a symlink to a real
    // socket would otherwise succeed silently.
    if (stat.isSymbolicLink()) return false;
    return true;
  } catch {
    // ENOENT / EACCES — let connect() surface the right error mode.
    return true;
  }
}

/**
 * Backwards-compatible export. Snapshots the env at module load. Setup
 * messages print this to tell the user what socket the daemon will bind to.
 * Per-request code uses resolveSocketPath() instead so tests can override.
 */
export const DEFAULT_SOCK_PATH = resolveSocketPath();

/**
 * Per-request timeout for classify. The daemon's NLM p50 is ~6s on bf16 MPS
 * (~400 tokens at ~64 ms/token; output length is structural regardless of
 * input size). 30s covers the long tail comfortably.
 */
export const DEFAULT_REQUEST_TIMEOUT_MS = 30_000;

/**
 * Healthz is gate-only (no NLM). p50 is <15 ms; 5s is generous and keeps the
 * status command from blocking when the daemon is wedged.
 */
export const DEFAULT_HEALTHZ_TIMEOUT_MS = 5_000;

/**
 * Hard cap on a single response. Daemon's longest legitimate response is the
 * NLM-path body (analysis + verdict + evidence + remediation, bounded by
 * NANOMIND_GUARD_MAX_NEW_TOKENS=512); 256 KB is well above the worst case
 * and below an OOM threshold for a CLI.
 */
const MAX_RESPONSE_BYTES = 256 * 1024;

/**
 * Hard cap on payload sent to the daemon. The daemon enforces 1 MB on its
 * side (NANOMIND_GUARD_MAX_BYTES); we mirror it client-side so we fail fast
 * without round-tripping a giant prompt.
 */
const MAX_REQUEST_BYTES = 1 * 1024 * 1024;

// ============================================================================
// Wire types
// ============================================================================

export type GateLabel = 'off-topic' | 'security-artifact' | 'classifier-error' | string;
export type GateReason =
  | 'lr'
  | 'stego-prefilter'
  | 'bidi-prefilter'
  | 'classifier-exception'
  | 'empty-input'
  | 'malformed-wrapper'
  | 'nested-wrapper'
  | string;

export type DaemonErrorCode =
  | 'ERR_BAD_REQUEST'
  | 'ERR_EMPTY_INPUT'
  | 'ERR_INPUT_TOO_LARGE'
  | 'ERR_UNKNOWN_OP'
  | 'ERR_INTERNAL'
  | 'ERR_TIMEOUT';

export interface ClassifyOkResponse {
  ok: true;
  /** none | <attack class>; constant "none" on gate-bypass path */
  predictedAttackClass: string;
  /** null on bypass path (binary gate decision), number on NLM path */
  confidence: number | null;
  classification: string;
  /** "input-classifier-gate" on bypass, "nlm" on pass-through or fail-CLOSED */
  source: 'input-classifier-gate' | 'nlm';
  gateLabel: GateLabel;
  gateReason: GateReason;
  gateProbaOffTopic: number | null;
  gateThreshold: number;
  gateLatencyMs: number;
  nlmInvoked: boolean;
  nlmLatencyMs: number | null;
  nlmTokenCount: number | null;
  /** NLM path carries severity, analysis, verdict, evidence, remediation */
  [k: string]: unknown;
}

export interface DaemonErrorResponse {
  ok: false;
  error: DaemonErrorCode;
  message: string;
}

export type ClassifyResponse = ClassifyOkResponse | DaemonErrorResponse;

export interface HealthzResponse {
  ok: boolean;
  daemonState: 'ready' | 'degraded';
  gateProbe: {
    input: string;
    label: string | null;
    expected: string;
    passed: boolean;
  };
  uptimeSec: number;
  requestsServed: number;
  modelPath: string;
  classifierPath: string;
  classifierThreshold: number;
  embedder: string;
}

export interface ClientOptions {
  socketPath?: string;
  timeoutMs?: number;
}

// ============================================================================
// Type guards
// ============================================================================

export function isClassifyOk(
  response: ClassifyResponse,
): response is ClassifyOkResponse {
  return response.ok === true;
}

export function isDaemonError(
  response: ClassifyResponse,
): response is DaemonErrorResponse {
  return response.ok === false;
}

// ============================================================================
// Core IPC
// ============================================================================

/**
 * Send one JSON-Lines request, read one response line, close.
 *
 * Returns null when the daemon is unreachable (ENOENT, ECONNREFUSED, timeout,
 * malformed response). Returns the parsed response otherwise.
 */
async function sendRequest(
  payload: Record<string, unknown>,
  opts: ClientOptions,
): Promise<unknown | null> {
  const socketPath = resolveSocketPath(opts.socketPath);
  const timeoutMs = opts.timeoutMs ?? DEFAULT_REQUEST_TIMEOUT_MS;

  if (!isSocketPathSafe(socketPath)) {
    // Symlink at the socket path — refuse without connecting. Treat as
    // "daemon unavailable" rather than crashing, so analyst calls fall back
    // to the existing null-return contract.
    return null;
  }

  const requestLine = JSON.stringify(payload) + '\n';
  const requestBytes = Buffer.byteLength(requestLine, 'utf8');
  if (requestBytes > MAX_REQUEST_BYTES) {
    // Mirror the daemon's MAX_BYTES limit client-side. The daemon would reject
    // this anyway; failing here avoids round-tripping a giant buffer.
    return null;
  }

  return new Promise<unknown | null>((resolve) => {
    let settled = false;
    const chunks: Buffer[] = [];
    let totalBytes = 0;

    const settle = (value: unknown | null): void => {
      if (settled) return;
      settled = true;
      clearTimeout(timer);
      try {
        socket.removeAllListeners();
        socket.destroy();
      } catch {
        // ignore — socket already closing
      }
      resolve(value);
    };

    const timer = setTimeout(() => settle(null), timeoutMs);

    let socket: Socket;
    try {
      socket = createConnection({ path: socketPath });
    } catch {
      // Synchronous creation failures (invalid path) return null.
      clearTimeout(timer);
      resolve(null);
      return;
    }

    socket.setNoDelay(true);

    socket.on('error', () => settle(null));
    socket.on('timeout', () => settle(null));

    socket.on('connect', () => {
      socket.write(requestLine, 'utf8');
    });

    socket.on('data', (chunk: Buffer) => {
      chunks.push(chunk);
      totalBytes += chunk.length;
      if (totalBytes > MAX_RESPONSE_BYTES) {
        // Daemon should never write this much; treat as protocol violation.
        settle(null);
        return;
      }
      // Look for the first newline across the buffered chunks. Daemon always
      // terminates a response with "\n".
      const buf = Buffer.concat(chunks, totalBytes);
      const nl = buf.indexOf(0x0a);
      if (nl < 0) return;

      const line = buf.subarray(0, nl).toString('utf8');
      let parsed: unknown;
      try {
        parsed = JSON.parse(line);
      } catch {
        settle(null);
        return;
      }
      if (typeof parsed !== 'object' || parsed === null) {
        settle(null);
        return;
      }
      settle(parsed);
    });

    socket.on('end', () => {
      // Peer closed without a complete line. The data handler resolves on
      // newline; if we never saw one, return null.
      if (!settled) settle(null);
    });

    socket.on('close', () => {
      if (!settled) settle(null);
    });
  });
}

// ============================================================================
// Public API
// ============================================================================

/**
 * Classify a single text input via the daemon. Returns the daemon response
 * (ClassifyOkResponse or DaemonErrorResponse), or null if the daemon is
 * unreachable / unresponsive.
 *
 * Callers treat null as "analyst unavailable" — NOT "benign verdict". The
 * analyst is opt-in behind --nanomind; falling back to a benign default
 * here would silently emit `none` during a transient daemon outage, which
 * defeats the gate.
 */
export async function sendClassify(
  text: string,
  opts: ClientOptions = {},
): Promise<ClassifyResponse | null> {
  if (typeof text !== 'string' || text.length === 0 || text.trim().length === 0) {
    // Mirror the daemon's ERR_EMPTY_INPUT guard client-side. The daemon would
    // reject this with ERR_EMPTY_INPUT; returning null here matches the
    // "analyst unavailable for this input" contract without a round-trip.
    return null;
  }
  const result = await sendRequest({ op: 'classify', text }, opts);
  if (result === null) return null;
  return validateClassifyResponse(result);
}

/**
 * Probe daemon health. Returns null on unreachable / unresponsive /
 * malformed response.
 */
export async function sendHealthz(
  opts: ClientOptions = {},
): Promise<HealthzResponse | null> {
  const timeoutMs = opts.timeoutMs ?? DEFAULT_HEALTHZ_TIMEOUT_MS;
  const result = await sendRequest({ op: 'healthz' }, { ...opts, timeoutMs });
  if (result === null) return null;
  return validateHealthzResponse(result);
}

/**
 * Convenience wrapper: returns true if /healthz reports ok within the
 * timeout. Used by status display and gating checks.
 */
export async function isDaemonHealthy(opts: ClientOptions = {}): Promise<boolean> {
  const response = await sendHealthz(opts);
  return response !== null && response.ok === true;
}

// ============================================================================
// Response validation
// ============================================================================

const ALLOWED_SEVERITIES = new Set([
  'critical', 'high', 'medium', 'low', 'info', 'none', '',
]);
const ALLOWED_GATE_LABELS = new Set([
  'off-topic', 'security-artifact', 'classifier-error',
]);
const STRING_FIELD_HARD_CAP = 64 * 1024;  // analysis/verdict/evidence/remediation
const SHORT_FIELD_HARD_CAP = 256;  // attackClass / severity / classification / source

function validateClassifyResponse(raw: unknown): ClassifyResponse | null {
  if (typeof raw !== 'object' || raw === null) return null;
  const obj = raw as Record<string, unknown>;
  if (typeof obj.ok !== 'boolean') return null;

  if (obj.ok === false) {
    if (typeof obj.error !== 'string' || typeof obj.message !== 'string') {
      return null;
    }
    if (obj.error.length > SHORT_FIELD_HARD_CAP) return null;
    if (obj.message.length > STRING_FIELD_HARD_CAP) return null;
    return {
      ok: false,
      error: obj.error as DaemonErrorCode,
      message: obj.message,
    };
  }

  // ok === true — short-field checks. We forward the body for the rich NLM
  // fields (analysis/verdict/evidence/remediation) but cap their length to
  // keep a hostile daemon from poisoning downstream renderers via unbounded
  // strings.
  if (typeof obj.predictedAttackClass !== 'string') return null;
  if (obj.predictedAttackClass.length > SHORT_FIELD_HARD_CAP) return null;
  if (typeof obj.source !== 'string') return null;
  if (obj.source.length > SHORT_FIELD_HARD_CAP) return null;

  // confidence: null OR finite number in [0, 1]
  if (obj.confidence !== null && obj.confidence !== undefined) {
    if (typeof obj.confidence !== 'number') return null;
    if (!Number.isFinite(obj.confidence)) return null;
    if (obj.confidence < 0 || obj.confidence > 1) return null;
  }
  if (typeof obj.classification === 'string'
      && obj.classification.length > SHORT_FIELD_HARD_CAP) return null;
  if (typeof obj.severity === 'string') {
    if (obj.severity.length > SHORT_FIELD_HARD_CAP) return null;
    // Bound severity to the recognized set; unknown severities are dropped
    // to '' rather than rejecting the whole response (forward-compat).
    if (!ALLOWED_SEVERITIES.has(obj.severity.toLowerCase())) {
      obj.severity = '';
    }
  }
  if (typeof obj.gateLabel === 'string' && obj.gateLabel.length > SHORT_FIELD_HARD_CAP) return null;
  if (typeof obj.gateLabel === 'string' && !ALLOWED_GATE_LABELS.has(obj.gateLabel)) {
    obj.gateLabel = '';
  }

  // Cap the rich NLM fields. Hostile daemon cannot drive the renderer to
  // walk a multi-megabyte string in `analysis`/`verdict`/etc.
  for (const field of ['analysis', 'verdict', 'evidence', 'remediation']) {
    const v = obj[field];
    if (typeof v === 'string' && v.length > STRING_FIELD_HARD_CAP) {
      obj[field] = v.slice(0, STRING_FIELD_HARD_CAP);
    }
  }
  return obj as unknown as ClassifyOkResponse;
}

function validateHealthzResponse(raw: unknown): HealthzResponse | null {
  if (typeof raw !== 'object' || raw === null) return null;
  const obj = raw as Record<string, unknown>;
  if (typeof obj.ok !== 'boolean') return null;
  if (typeof obj.daemonState !== 'string') return null;
  if (typeof obj.gateProbe !== 'object' || obj.gateProbe === null) return null;
  return obj as unknown as HealthzResponse;
}
