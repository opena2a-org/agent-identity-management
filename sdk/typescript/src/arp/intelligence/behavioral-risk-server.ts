/**
 * Behavioral risk IPC server (AIComply P1, producer side).
 *
 * Partner to `behavioral-risk.ts`. Listens on a unix domain socket (or
 * Windows named pipe) and answers risk signal requests by delegating to a
 * caller-supplied `BehavioralRiskScoreable`, which in production is a
 * `RuntimeTwin` instance running in the twin's own process.
 *
 * The server is deliberately narrow: one request per connection, no
 * session state, no authentication beyond filesystem permissions on the
 * socket path. The threat model assumes the socket is owned by the same
 * local user and is reachable only from processes with appropriate
 * filesystem access; cross-host or cross-user use is out of scope.
 *
 * Wire format (both directions, newline-delimited JSON):
 *
 *   request  {"kind":"risk_signal_request","version":1,"event":<ARPEvent>}
 *   response {"kind":"risk_signal_response","version":1,"score":...,
 *             "action":"allow|alert|throttle|suspend|kill","reason":"...",
 *             "source":"...","computedAtMs":...}
 *   error    {"kind":"risk_signal_error","version":1,"code":<code>,
 *             "reason":"..."}
 *
 * Any parse failure, unknown kind, or thrown exception inside the scorer
 * is converted to a structured `risk_signal_error` message. The server
 * never crashes on a malformed request; per-request isolation keeps a
 * bad client from affecting others.
 */

import * as net from 'net';
import * as fs from 'fs';
import * as path from 'path';
import type { ARPEvent } from '../types';
import {
  BEHAVIORAL_RISK_WIRE_VERSION,
  type BehavioralRiskScoreable,
  type BehavioralRiskUnavailableCode,
} from './behavioral-risk';

/**
 * Options accepted by `startBehavioralRiskServer`. Only `twin` is
 * required; everything else has a defensible default.
 */
export interface BehavioralRiskServerOptions {
  /**
   * Handle that knows how to score an ARP event. In production this is a
   * RuntimeTwin instance. In tests it can be any stub that returns a
   * deterministic risk score.
   */
  twin: BehavioralRiskScoreable;
  /**
   * Filesystem path where the server listens. On POSIX systems, a unix
   * domain socket; on Windows, a named pipe path. Any existing socket
   * file at this path is removed before binding (POSIX only).
   */
  socketPath: string;
  /**
   * Identifier written back into the `source` field of every response.
   * Helps the coordinator's audit log distinguish between multiple risk
   * sources during A/B rollout.
   */
  sourceName?: string;
  /**
   * Invoked whenever the server rejects a request with an error response.
   * Gets the code and reason. Useful for routing to a SIEM or surfacing
   * in an operator dashboard. Optional; defaults to a no-op.
   */
  onError?: (code: BehavioralRiskUnavailableCode, reason: string) => void;
}

/**
 * Handle returned by `startBehavioralRiskServer`. Callers hold this for
 * the lifetime of the coordinator and invoke `close()` on shutdown.
 */
export interface BehavioralRiskServerHandle {
  /** The listening socket path. */
  readonly socketPath: string;
  /**
   * Stop accepting new connections, tear down the listening socket, and
   * clean up the socket file on POSIX. Safe to call more than once.
   */
  close(): Promise<void>;
}

/**
 * Start the behavioral risk IPC server. Resolves once the server is
 * actively listening on the socket path so the caller can hand the path
 * to clients without a race against a half-open listener.
 *
 * Throws on bind failure (unrecoverable; the caller should decide
 * whether to degrade to an in-process source or fail the startup).
 */
export async function startBehavioralRiskServer(
  options: BehavioralRiskServerOptions,
): Promise<BehavioralRiskServerHandle> {
  const { twin, socketPath, sourceName = 'nanomind-l1-ipc', onError } = options;

  // On POSIX, a stale socket file from a crashed previous run blocks
  // bind. We remove it defensively. On Windows named pipes, the path is
  // in its own namespace and this step is a no-op.
  if (process.platform !== 'win32') {
    try {
      const dir = path.dirname(socketPath);
      if (!fs.existsSync(dir)) fs.mkdirSync(dir, { recursive: true });
      if (fs.existsSync(socketPath)) fs.unlinkSync(socketPath);
    } catch {
      // If we cannot clean up the stale file, let listen() surface the
      // real error below.
    }
  }

  const server = net.createServer((socket) => {
    let buffer = '';
    let handled = false;

    const sendAndClose = (payload: Record<string, unknown>) => {
      if (handled) return;
      handled = true;
      try {
        socket.write(JSON.stringify(payload) + '\n');
      } catch {
        // If the client already hung up, there is nothing useful to do.
      }
      socket.end();
    };

    const sendError = (code: BehavioralRiskUnavailableCode, reason: string) => {
      onError?.(code, reason);
      sendAndClose({
        kind: 'risk_signal_error',
        version: BEHAVIORAL_RISK_WIRE_VERSION,
        code,
        reason,
      });
    };

    socket.on('data', (chunk: Buffer) => {
      if (handled) return;
      buffer += chunk.toString('utf8');
      const nl = buffer.indexOf('\n');
      if (nl < 0) {
        // Cap unparsed buffer to keep a misbehaving client from
        // exhausting server memory. One request per connection means a
        // single line; anything beyond a reasonable ceiling is hostile.
        if (buffer.length > 64 * 1024) {
          sendError('PARSE_ERROR', 'request exceeded maximum size');
        }
        return;
      }
      const line = buffer.slice(0, nl);
      handleRequest(line, twin, sourceName, sendAndClose, sendError);
    });

    socket.on('error', () => {
      // Drop per-connection errors; nothing to log that the client will
      // see. A wedged client eventually times out.
    });
  });

  server.on('error', (err) => {
    // Per-connection errors bubble up here after a catastrophic failure.
    // We defer to the caller's onError hook so operators can spot
    // repeating listen-level failures.
    onError?.('TRANSPORT_ERROR', `server error: ${(err as Error).message}`);
  });

  await new Promise<void>((resolve, reject) => {
    server.once('error', reject);
    server.listen(socketPath, () => {
      server.removeListener('error', reject);
      resolve();
    });
  });

  let closed = false;
  return {
    socketPath,
    close: async () => {
      if (closed) return;
      closed = true;
      await new Promise<void>((resolve) => {
        server.close(() => resolve());
      });
      if (process.platform !== 'win32') {
        try {
          if (fs.existsSync(socketPath)) fs.unlinkSync(socketPath);
        } catch {
          // Nothing actionable; the socket file may already be gone.
        }
      }
    },
  };
}

/**
 * Parse and dispatch a single request line. Extracted so the connection
 * handler stays readable and so the request path can be unit tested in
 * isolation later if needed. Every failure path resolves to a
 * `risk_signal_error` rather than propagating an exception: the server
 * must never crash on a malformed request.
 */
function handleRequest(
  line: string,
  twin: BehavioralRiskScoreable,
  sourceName: string,
  sendAndClose: (payload: Record<string, unknown>) => void,
  sendError: (code: BehavioralRiskUnavailableCode, reason: string) => void,
): void {
  let msg: unknown;
  try {
    msg = JSON.parse(line);
  } catch (err) {
    sendError('PARSE_ERROR', `invalid json: ${(err as Error).message}`);
    return;
  }
  if (msg === null || typeof msg !== 'object') {
    sendError('PARSE_ERROR', 'request is not a json object');
    return;
  }
  const obj = msg as Record<string, unknown>;
  if (obj.version !== BEHAVIORAL_RISK_WIRE_VERSION) {
    sendError('PARSE_ERROR', `unsupported wire version: ${JSON.stringify(obj.version)}`);
    return;
  }
  if (obj.kind !== 'risk_signal_request') {
    sendError('PARSE_ERROR', `unexpected kind: ${JSON.stringify(obj.kind)}`);
    return;
  }
  const event = obj.event;
  if (!isArpEventLike(event)) {
    sendError('PARSE_ERROR', 'event field is missing required ARPEvent shape');
    return;
  }
  let result;
  try {
    result = twin.scoreARPEvent(event);
  } catch (err) {
    sendError('INTERNAL_ERROR', `twin threw: ${(err as Error).message ?? String(err)}`);
    return;
  }
  if (result === null) {
    sendError('NOT_READY', 'twin baseline not yet trained');
    return;
  }
  sendAndClose({
    kind: 'risk_signal_response',
    version: BEHAVIORAL_RISK_WIRE_VERSION,
    score: result.score,
    action: result.action,
    reason: result.reason,
    source: sourceName,
    computedAtMs: Date.now(),
  });
}

/**
 * Narrow runtime check for an ARPEvent-shaped request payload. Only
 * validates the fields the twin's scoreARPEvent path actually reads,
 * which is by design: stricter validation would reject valid events
 * whose extra fields the server does not care about.
 */
function isArpEventLike(v: unknown): v is ARPEvent {
  if (v === null || typeof v !== 'object') return false;
  const e = v as Record<string, unknown>;
  if (typeof e.id !== 'string') return false;
  if (typeof e.timestamp !== 'string') return false;
  if (typeof e.source !== 'string') return false;
  if (typeof e.category !== 'string') return false;
  if (typeof e.severity !== 'string') return false;
  if (typeof e.description !== 'string') return false;
  if (e.data === null || typeof e.data !== 'object') return false;
  return true;
}
