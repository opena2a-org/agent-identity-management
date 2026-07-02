/**
 * SequenceProjector — offline ordered in-scope action sequence assembly.
 *
 * The ARP runtime already captures everything an ordered sequence needs. Each
 * event the RuntimeTwin writes to `~/.opena2a/arp/events.jsonl`
 * (`runtime-twin.ts` `appendToLog`) carries `agentId`, `sessionId`,
 * `sequenceNum` (monotonic per session), `capability`, `toolName`, `argHash`,
 * and `l0Decision` — in arrival order, one JSON record per line, written off
 * any deny path. Order is only discarded LATER, at the anomaly-score step.
 *
 * This module stops discarding it. It is a pure offline reader: it groups the
 * log by `(agentId, sessionId)`, sorts by `sequenceNum`, keeps the in-scope
 * actions (an allowed event whose capability is inside the granted scope), and
 * projects each to the consistency-model tuple. The agent hot path is
 * unchanged — there is zero added inline work and zero added latency. This is
 * the "source A" corpus feed for the offline consistency signal.
 *
 * What this module does NOT do: it does not score the sequence. Scoring is the
 * CDS-owned offline consistency model's job. ARP carries the order; the model
 * judges it.
 *
 * === On the data-flow / taint link ===
 *
 * The consistency label for a write target (does this `send_money` recipient,
 * this `http_post` destination, serve the declared purpose?) usually depends on
 * what a PRIOR READ returned. The trainability spike found that a flat
 * `(purpose, action-args)` tuple is information-deficient precisely because it
 * lacks that read-output → write-arg link. We carry as much of that link as the
 * log substrate allows: for every sink action we record `taintSourceSeqs`, the
 * sequence numbers of prior source actions in the same session whose outputs
 * were available before it ran.
 *
 * This is provenance-by-ordering — the candidate-provenance channel — NOT
 * content-verified taint. The event log carries only `argHash` and
 * `responseSize`, never raw arguments or raw outputs, so we cannot prove a
 * specific byte of a read flowed into a specific write argument. We carry the
 * opportunity edges (which reads preceded this write within the session); a
 * downstream model or rule can ask "did this write depend on any prior read?".
 * Content-verified taint requires raw-argument capture and is deliberately
 * deferred — fabricating a proven-flow claim from hashes would be dishonest.
 */

import * as fs from 'fs';
import * as readline from 'readline';

/**
 * One record as written to the ordered event log. A superset of the
 * RuntimeTwin `BehavioralEvent`: the first block is what the twin writes
 * today; the optional second block is the enrichment a detection-mode tee
 * adds when it is available (classification from the annotator, egress/volume
 * from the network observation, objective from the capability justification).
 * Every enrichment field is optional and projects to `null` when absent, so
 * the projector is useful against the bare twin log on day one and gets richer
 * as the tee fills the fields in.
 */
export interface LoggedActionEvent {
  agentId: string;
  sessionId: string;
  sequenceNum: number;
  eventType: string;
  capability: string;
  toolName: string | null;
  argHash: string;
  timestampDelta: number;
  responseSize?: number;
  responseCode?: number;
  l0Decision: 'allow' | 'block' | 'alert';

  // --- optional enrichment (detection-mode tee) ---
  /** Cleared classification label (`event.data.classification`), once annotated. */
  classification?: string | null;
  /** Declared-purpose task objective the capability maps to, if resolvable. */
  objective?: string | null;
  /** Data domain the action touched (for the undeclared-data-domain tell). */
  dataScopeTouched?: string | null;
  /** Hashed external destination host (no path/secrets) — undeclared-egress tell. */
  egressTargetHash?: string | null;
  /** Observed egress/payload volume in bytes — covert-exfil volume tell. */
  volumeBytes?: number | null;
}

/** A single projected in-scope action in the consistency-model tuple shape. */
export interface ProjectedAction {
  /** Per-session monotonic order key (the `sequenceNum` from the log). */
  seq: number;
  capability: string;
  tool: string | null;
  argHash: string;
  /** Declared-purpose objective, or null when unresolved. */
  objective: string | null;
  /** Cleared classification label, or null until annotated. */
  classification: string | null;
  /** Data domain touched, or null when unresolved. */
  dataScopeTouched: string | null;
  /** Hashed egress destination host, or null when not an egress action. */
  egressTargetHash: string | null;
  /** Observed volume in bytes, or null when unknown. */
  volumeBytes: number | null;
  /** Inter-event delta in ms (from the log's `timestampDelta`). */
  dtMs: number;
  /** Always true for emitted actions — they passed the in-scope filter. */
  inScope: true;
  /**
   * Provenance-by-ordering: sequence numbers of prior SOURCE actions in this
   * session whose outputs were available before this action ran. Populated
   * only for SINK actions (writes / egress). Empty for non-sinks and for sinks
   * with no prior source. See the module header — this is the candidate-flow
   * channel, not content-verified taint.
   */
  taintSourceSeqs: number[];
}

/** The per-session ordered, in-scope, projected action sequence. */
export interface InScopeActionSequence {
  agentId: string;
  sessionId: string;
  /** Purpose-vocabulary version the credential was issued under, if known. */
  vocabVersion: string | null;
  /** Reference to the declared purpose this sequence is judged against, if known. */
  purposeRef: string | null;
  actions: ProjectedAction[];
}

export interface SequenceProjectorOptions {
  /**
   * Capabilities the agent was granted (from the ATX credential / manifest).
   * An action is in-scope only when its capability is a member. A set
   * containing the single entry `'*'` matches any capability. When omitted,
   * the in-scope filter is `l0Decision === 'allow'` alone — every allowed
   * action is treated as in-scope. (Allowed-but-out-of-scope is itself a
   * signal; without the grant set we cannot compute it, so we degrade to
   * allow-only rather than inventing a scope.)
   */
  grantedScope?: ReadonlySet<string>;
  /** Vocabulary version stamped on every emitted sequence. */
  vocabVersion?: string | null;
  /** Declared-purpose reference stamped on every emitted sequence. */
  purposeRef?: string | null;
  /**
   * Resolve a capability to its declared-purpose objective. Overrides any
   * `objective` already on the log record. Returns null when unresolved.
   * Defaults to passing the log record's `objective` through.
   */
  capabilityToObjective?: (capability: string) => string | null;
}

/**
 * Event types whose actions consume arguments that may carry prior read
 * outputs — the SINK side of the provenance channel. A `MEMORY_WRITE` persists
 * state; an `EXTERNAL_CALL` egresses. These are where a tainted write target
 * would surface.
 */
const SINK_EVENT_TYPES: ReadonlySet<string> = new Set([
  'MEMORY_WRITE',
  'EXTERNAL_CALL',
]);

/**
 * Decide whether an action produced an output later actions could consume —
 * the SOURCE side of the provenance channel. Any action that returned a
 * non-empty response is a source; absent a response-size signal we
 * conservatively treat reads and protocol calls as sources too.
 */
function isSourceAction(ev: LoggedActionEvent): boolean {
  if (typeof ev.responseSize === 'number' && ev.responseSize > 0) return true;
  return (
    ev.eventType === 'MEMORY_READ' ||
    ev.eventType === 'MCP_CALL' ||
    ev.eventType === 'EXTERNAL_CALL'
  );
}

/**
 * Project a flat list of logged action records into per-session ordered
 * in-scope action sequences. Pure: no IO, deterministic, side-effect free.
 * Records missing the required identity/order fields are skipped (a corrupt
 * line must not abort the whole projection).
 */
export function projectSequences(
  records: ReadonlyArray<LoggedActionEvent>,
  options: SequenceProjectorOptions = {},
): InScopeActionSequence[] {
  const grant = options.grantedScope;
  const grantMatchesAll = grant?.has('*') ?? false;

  // Group by (agentId, sessionId). Insertion order of first-seen groups is
  // preserved so the output is stable across runs on the same input.
  const groups = new Map<string, { agentId: string; sessionId: string; rows: LoggedActionEvent[] }>();
  for (const rec of records) {
    if (!isWellFormed(rec)) continue;
    // In-scope filter: allowed AND (no grant set → allow-only, or capability ∈ grant).
    if (rec.l0Decision !== 'allow') continue;
    if (grant && !grantMatchesAll && !grant.has(rec.capability)) continue;

    const key = `${rec.agentId} ${rec.sessionId}`;
    let group = groups.get(key);
    if (!group) {
      group = { agentId: rec.agentId, sessionId: rec.sessionId, rows: [] };
      groups.set(key, group);
    }
    group.rows.push(rec);
  }

  const sequences: InScopeActionSequence[] = [];
  for (const group of groups.values()) {
    // Sort by the monotonic per-session sequenceNum. A stable tiebreak on the
    // original index keeps duplicate sequence numbers in arrival order rather
    // than reordering them nondeterministically.
    const indexed = group.rows.map((row, i) => ({ row, i }));
    indexed.sort((a, b) => a.row.sequenceNum - b.row.sequenceNum || a.i - b.i);

    const sourceSeqsSoFar: number[] = [];
    const actions: ProjectedAction[] = [];
    for (const { row } of indexed) {
      const isSink = SINK_EVENT_TYPES.has(row.eventType);
      const taintSourceSeqs = isSink ? sourceSeqsSoFar.slice() : [];

      const objective = options.capabilityToObjective
        ? options.capabilityToObjective(row.capability)
        : row.objective ?? null;

      actions.push({
        seq: row.sequenceNum,
        capability: row.capability,
        tool: row.toolName ?? null,
        argHash: row.argHash,
        objective,
        classification: row.classification ?? null,
        dataScopeTouched: row.dataScopeTouched ?? null,
        egressTargetHash: row.egressTargetHash ?? null,
        volumeBytes:
          typeof row.volumeBytes === 'number'
            ? row.volumeBytes
            : typeof row.responseSize === 'number'
              ? row.responseSize
              : null,
        dtMs: row.timestampDelta,
        inScope: true,
        taintSourceSeqs,
      });

      // A source action's output is available to every LATER action in the
      // session — record it after projecting this row so an action is never
      // its own taint source.
      if (isSourceAction(row)) sourceSeqsSoFar.push(row.sequenceNum);
    }

    sequences.push({
      agentId: group.agentId,
      sessionId: group.sessionId,
      vocabVersion: options.vocabVersion ?? null,
      purposeRef: options.purposeRef ?? null,
      actions,
    });
  }

  return sequences;
}

function isWellFormed(rec: LoggedActionEvent): boolean {
  return (
    typeof rec?.agentId === 'string' &&
    rec.agentId.length > 0 &&
    typeof rec.sessionId === 'string' &&
    rec.sessionId.length > 0 &&
    typeof rec.sequenceNum === 'number' &&
    Number.isFinite(rec.sequenceNum) &&
    typeof rec.capability === 'string' &&
    typeof rec.argHash === 'string' &&
    (rec.l0Decision === 'allow' ||
      rec.l0Decision === 'block' ||
      rec.l0Decision === 'alert')
  );
}

/**
 * Read an append-only JSONL event log and project it into in-scope action
 * sequences. Streams the file line-by-line so a large log does not have to be
 * held in memory at once. Malformed JSON lines are skipped, not fatal — a
 * partially-written tail line (the agent may still be appending) must not abort
 * the projection. A missing log file yields an empty array.
 */
export async function projectSequencesFromFile(
  logPath: string,
  options: SequenceProjectorOptions = {},
): Promise<InScopeActionSequence[]> {
  let stream: fs.ReadStream;
  try {
    if (!fs.existsSync(logPath)) return [];
    stream = fs.createReadStream(logPath, { encoding: 'utf-8' });
  } catch {
    return [];
  }

  const records: LoggedActionEvent[] = [];
  const rl = readline.createInterface({ input: stream, crlfDelay: Infinity });
  for await (const line of rl) {
    const trimmed = line.trim();
    if (trimmed.length === 0) continue;
    try {
      const parsed = JSON.parse(trimmed) as LoggedActionEvent;
      records.push(parsed);
    } catch {
      // Skip a malformed / partially-written line; never abort the projection.
    }
  }

  return projectSequences(records, options);
}

/**
 * Convenience wrapper binding projector options once. Stateless beyond the
 * options it closes over — `project` / `projectFromFile` are safe to call
 * concurrently.
 */
export class SequenceProjector {
  constructor(private readonly options: SequenceProjectorOptions = {}) {}

  project(records: ReadonlyArray<LoggedActionEvent>): InScopeActionSequence[] {
    return projectSequences(records, this.options);
  }

  projectFromFile(logPath: string): Promise<InScopeActionSequence[]> {
    return projectSequencesFromFile(logPath, this.options);
  }
}
