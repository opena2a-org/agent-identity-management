/**
 * SequenceLogWriter — the detection-mode tee that turns live ARP events into
 * `LoggedActionEvent` records the offline SequenceProjector consumes.
 *
 * The RuntimeTwin already writes its `BehavioralEvent` log; this writer is the
 * companion for hosts that run the proxy/coordinator path without a twin (the
 * ARP HTTP proxy). It owns a per-run `sessionId` and a monotonic sequence
 * counter, projects each event to the on-disk record shape — carrying the
 * enrichment the projector wants (classification once annotated, egress target
 * hash, volume) — and appends one JSON line per event.
 *
 * Append failures are swallowed: a logging hiccup must never disturb the proxy.
 * The conversion is exported standalone so it can be unit-tested without IO.
 */

import * as crypto from 'crypto';
import * as fs from 'fs';
import * as path from 'path';
import type { ARPEvent } from '../types';
import type { LoggedActionEvent } from './sequence-projector';

/** Mutable context threaded across a run so sequence/timing stay monotonic. */
export interface SequenceLogContext {
  agentId: string;
  sessionId: string;
  /** Next sequence number to assign (advanced by the converter). */
  seq: number;
  /** Wall-clock (ms) of the previous event, for the inter-event delta. */
  lastWallClock: number;
}

/** Common keys a monitor may use to carry the egress destination host. */
const HOST_KEYS = ['host', 'dest', 'destination', 'remoteHost', 'targetHost'];

/**
 * Map an ARP event's source hint to the behavioral event-type vocabulary,
 * mirroring `RuntimeTwin.convertEvent` so the two logs stay comparable.
 */
function deriveEventType(event: ARPEvent): string {
  const source = String(event.data?.source || event.data?.monitor || '');
  if (source.includes('network')) return 'EXTERNAL_CALL';
  if (source.includes('filesystem')) return 'MEMORY_READ';
  if (source.includes('mcp')) return 'MCP_CALL';
  if (source.includes('capability')) return 'CAPABILITY_CHECK';
  return 'TOOL_CALL';
}

/**
 * Derive the L0 decision, mirroring `RuntimeTwin.convertEvent`. Only `allow`
 * events are in-scope for the projector; a flagged event is a different signal.
 */
function deriveL0Decision(event: ARPEvent): 'allow' | 'block' | 'alert' {
  const initial = event.data?._initialAction;
  if (initial === 'kill' || initial === 'pause') return 'block';
  if (event.category === 'threat' || event.category === 'violation') return 'alert';
  if (event.severity === 'high' || event.severity === 'critical') return 'alert';
  return 'allow';
}

/**
 * Case-fold an egress host to its canonical identity before hashing. DNS host
 * names are case-insensitive and surrounding whitespace is never part of a
 * host, so `EVIL.COM`, `evil.com`, and `  evil.com  ` are the same target.
 * Folding here (and identically in the consumer's `hash_host`) is what stops an
 * attacker from evading egress-target grounding by varying host case. ASCII
 * trim + lowercase only; IDNA/punycode normalization is intentionally out of
 * scope for this bare-host token channel.
 */
function normalizeEgressHost(host: string): string {
  return host.trim().toLowerCase();
}

/** Hash an egress host (host only — never a path or query) for the egress tell. */
function deriveEgressTargetHash(event: ARPEvent): string | null {
  if (deriveEventType(event) !== 'EXTERNAL_CALL') return null;
  for (const key of HOST_KEYS) {
    const v = event.data?.[key];
    if (typeof v === 'string' && v.length > 0) {
      return crypto
        .createHash('sha256')
        .update(normalizeEgressHost(v))
        .digest('hex');
    }
  }
  return null;
}

/**
 * Project a live ARP event into the on-disk `LoggedActionEvent` shape. Pure:
 * advances and reads the supplied context but performs no IO. The context's
 * `seq` and `lastWallClock` are updated in place.
 */
export function arpEventToLoggedAction(
  event: ARPEvent,
  ctx: SequenceLogContext,
): LoggedActionEvent {
  const data = event.data ?? {};
  const wallClock = Date.parse(event.timestamp);
  const now = Number.isFinite(wallClock) ? wallClock : ctx.lastWallClock;
  const delta = ctx.lastWallClock > 0 ? Math.max(0, now - ctx.lastWallClock) : 0;
  ctx.lastWallClock = now;
  const sequenceNum = ctx.seq++;

  const responseSize = typeof data.responseSize === 'number' ? data.responseSize : 0;
  const classification = typeof data.classification === 'string' ? data.classification : null;

  return {
    agentId: ctx.agentId,
    sessionId: ctx.sessionId,
    sequenceNum,
    eventType: deriveEventType(event),
    capability: String(data.capability ?? data.type ?? 'unknown'),
    toolName: typeof data.toolName === 'string' ? data.toolName : null,
    argHash: crypto
      .createHash('sha256')
      .update(safeStringify(data))
      .digest('hex')
      .substring(0, 16),
    timestampDelta: delta,
    responseSize,
    responseCode: data.error ? 1 : 0,
    l0Decision: deriveL0Decision(event),
    classification,
    objective: typeof data.objective === 'string' ? data.objective : null,
    dataScopeTouched: typeof data.dataScope === 'string' ? data.dataScope : null,
    egressTargetHash: deriveEgressTargetHash(event),
    volumeBytes: responseSize,
  };
}

function safeStringify(value: unknown): string {
  try {
    return JSON.stringify(value);
  } catch {
    return '';
  }
}

/**
 * Owns the per-run session identity and the append-only sequence log. Use
 * `append` as the proxy's `onInScopeEvent` sink — it converts and writes one
 * record per event. Construction creates the parent directory lazily on first
 * write.
 */
export class SequenceLogWriter {
  private readonly ctx: SequenceLogContext;

  constructor(
    private readonly logPath: string,
    agentId: string,
    sessionId: string = crypto.randomUUID(),
  ) {
    this.ctx = { agentId, sessionId, seq: 1, lastWallClock: 0 };
  }

  /** The session id stamped on every record this writer emits. */
  get sessionId(): string {
    return this.ctx.sessionId;
  }

  /** Convert and append one event. Never throws. */
  append(event: ARPEvent): void {
    const record = arpEventToLoggedAction(event, this.ctx);
    try {
      const dir = path.dirname(this.logPath);
      if (!fs.existsSync(dir)) fs.mkdirSync(dir, { recursive: true });
      fs.appendFileSync(this.logPath, JSON.stringify(record) + '\n');
    } catch {
      // A logging failure must never disturb the proxy.
    }
  }
}
