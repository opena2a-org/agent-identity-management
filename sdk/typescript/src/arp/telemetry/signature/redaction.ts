/**
 * Structural redaction (G2 of the sensor-network consent design).
 *
 * This is the privacy chokepoint of the producer. It reduces a runtime ARP
 * observation to a CLOSED SET of allowlisted structural tokens — the SHAPE of a
 * behavior — and nothing else. It is deliberately small and readable so that a
 * customer can audit it before trusting the default-on channel.
 *
 * Hard invariants (see opena2a-registry/docs/telemetry-behavioral-hash-spec.md):
 *   - Every output token comes from a fixed enum in THIS file. No value is ever
 *     copied out of the event's free-text fields (description, data values,
 *     paths, args, prompts, hostnames).
 *   - The mapping reads ONLY the event's `source` (a closed MonitorType enum),
 *     its `category`/`severity` (closed enums), and a SMALL allowlist of strict
 *     boolean / enum flags in `data`. Strings in `data` are never inspected for
 *     content — only a handful of explicit booleans the monitors set.
 *   - FAIL CLOSED: if a behavior cannot be reduced to allowlisted tokens, this
 *     returns `null` and the behavior is NOT reported. A missed signal is always
 *     preferable to a leaked payload.
 */

import type { ARPEvent, EventSeverity, MonitorType } from '../../types';

// --- Closed enums (the ONLY tokens that may ever be transmitted) -------------

/** Category of operation. Closed enum — mirrors the registry actionClass enum. */
export const ACTION_CLASSES = [
  'tool_invocation',
  'file_access',
  'network_egress',
  'process_spawn',
  'credential_access',
  'prompt_injection',
  'privilege_change',
  'data_staging',
] as const;
export type ActionClass = (typeof ACTION_CLASSES)[number];

/** Category of the target. Closed enum — mirrors the registry targetClass enum. */
export const TARGET_CLASSES = [
  'agent',
  'tool',
  'mcp_server',
  'file',
  'network',
  'credential',
  'model',
] as const;
export type TargetClass = (typeof TARGET_CLASSES)[number];

/** Kill-chain stage. Closed enum — the 9 canonical threat-matrix tactic ids. */
export const TACTIC_IDS = [
  'reconnaissance',
  'initial-access',
  'credential-harvest',
  'privilege-escalation',
  'lateral-movement',
  'persistence',
  'collection',
  'exfiltration',
  'impact',
] as const;
export type TacticId = (typeof TACTIC_IDS)[number];

/** Runtime outcome. Closed enum — mirrors the registry outcome enum. */
export const OUTCOME_CLASSES = ['detected', 'blocked', 'allowed'] as const;
export type OutcomeClass = (typeof OUTCOME_CLASSES)[number];

/** Severities the registry accepts (identical set to ARP EventSeverity). */
export const SEVERITIES = ['critical', 'high', 'medium', 'low', 'info'] as const;
export type Severity = (typeof SEVERITIES)[number];

/**
 * The fully-redacted structural signal. These are the ONLY fields that leave the
 * device (after hashing the first six into behavioralHash). There is no free-text
 * field anywhere in this type by construction.
 */
export interface RedactedSignal {
  tacticId: TacticId;
  /** Wire-form technique id, e.g. "ATM-T2001" (see TECHNIQUE_IDS note). */
  techniqueId: string;
  actionClass: ActionClass;
  targetClass: TargetClass;
  /** Ordered actionClass tokens joined by ">" for multi-step shapes, else "". */
  sequencePattern: string;
  outcomeClass: OutcomeClass;
  severity: Severity;
}

// --- Canonical technique ids -------------------------------------------------
//
// The canonical threat matrix (agent-threat-matrix/matrix.json) numbers
// techniques "T-1001".."T-9006". The telemetry wire schema's techniqueId regex
// (^[A-Z][A-Z0-9]{1,9}(-[A-Z0-9]{1,9}){0,3}$, registry domain/telemetry_signature.go)
// rejects a single-character first segment, so the bare canonical "T-1001" is NOT
// wire-valid. We therefore emit the spec's documented "ATM-T..." form: prefix
// "ATM-" and strip the canonical internal hyphen ("T-1001" -> "ATM-T1001"). The
// mapping is deterministic and reversible, so when G4/G5 lands canonical-enum
// validation it can normalize "ATM-T1001" <-> "T-1001" trivially.

/** Build the wire-form technique id from a canonical matrix id ("T-1001"). */
function atm(canonical: `T-${string}`): string {
  return `ATM-${canonical.replace('-', '')}`; // T-1001 -> ATM-T1001
}

/** Strict wire-format guard (mirrors the registry regex + 32-char ceiling). */
const RE_WIRE_TECHNIQUE = /^[A-Z][A-Z0-9]{1,9}(-[A-Z0-9]{1,9}){0,3}$/;
export function isWireTechniqueId(v: string): boolean {
  return v.length <= 32 && RE_WIRE_TECHNIQUE.test(v);
}

/**
 * Producer technique allowlist. A monitor may set `data.techniqueId` to override
 * the coarse source-derived mapping, but ONLY a value in this set (already in
 * wire form) is honored — a free-text or out-of-set value is ignored and the
 * coarse mapping is used instead. This guarantees techniqueId is never
 * attacker-controlled: it is either a constant from the table below or a member
 * of this allowlist.
 */
export const TECHNIQUE_ALLOWLIST: ReadonlySet<string> = new Set(
  (
    [
      'T-1001', 'T-1002', 'T-1003', 'T-1005',
      'T-2001', 'T-2002', 'T-2005',
      'T-3002', 'T-3005',
      'T-4001', 'T-4003',
      'T-5002', 'T-5003', 'T-5006',
      'T-6003', 'T-6004',
      'T-7001', 'T-7004',
      'T-8002', 'T-8003', 'T-8004',
      'T-9003',
    ] as const
  ).map(atm),
);

// --- Source -> structural shape (the closed mapping) -------------------------

interface SourceShape {
  actionClass: ActionClass;
  targetClass: TargetClass;
  tacticId: TacticId;
  techniqueCanonical: `T-${string}`;
}

/**
 * Base structural shape per monitor source. This is the floor: a coarse but
 * always-safe classification derived ONLY from the closed MonitorType enum.
 * `heartbeat` is intentionally absent — liveness is not a security behavior and
 * is dropped (fail closed).
 */
const SOURCE_SHAPE: Partial<Record<MonitorType, SourceShape>> = {
  process: { actionClass: 'process_spawn', targetClass: 'tool', tacticId: 'impact', techniqueCanonical: 'T-9003' },
  network: { actionClass: 'network_egress', targetClass: 'network', tacticId: 'exfiltration', techniqueCanonical: 'T-8002' },
  filesystem: { actionClass: 'file_access', targetClass: 'file', tacticId: 'collection', techniqueCanonical: 'T-7001' },
  prompt: { actionClass: 'prompt_injection', targetClass: 'model', tacticId: 'initial-access', techniqueCanonical: 'T-2001' },
  'mcp-protocol': { actionClass: 'tool_invocation', targetClass: 'mcp_server', tacticId: 'lateral-movement', techniqueCanonical: 'T-5003' },
  'a2a-protocol': { actionClass: 'tool_invocation', targetClass: 'agent', tacticId: 'lateral-movement', techniqueCanonical: 'T-5002' },
  skill: { actionClass: 'privilege_change', targetClass: 'tool', tacticId: 'privilege-escalation', techniqueCanonical: 'T-4001' },
};

/** Only anomalous categories are eligible; normal traffic is never reported. */
const REPORTABLE_CATEGORIES: ReadonlySet<string> = new Set(['anomaly', 'violation', 'threat']);

/** A strict boolean read — true ONLY for an exact `true`, never a truthy string. */
function flag(data: Record<string, unknown>, key: string): boolean {
  return data[key] === true;
}

/**
 * Refine the base source shape using a SMALL allowlist of strict structured
 * flags. Each branch reads only booleans / closed-enum scalars the monitors set
 * — never a free-text value. Anything unrecognized leaves the base shape intact.
 */
function refineShape(base: SourceShape, source: MonitorType, data: Record<string, unknown>): SourceShape {
  if (source === 'network') {
    // DNS egress is a distinct, well-known exfil channel.
    if (flag(data, 'dns') || flag(data, 'dnsLookup') || data['protocol'] === 'dns' || data['type'] === 'dns') {
      return { actionClass: 'network_egress', targetClass: 'network', tacticId: 'exfiltration', techniqueCanonical: 'T-8003' };
    }
    return base;
  }
  if (source === 'filesystem') {
    // Only an EXPLICIT structured flag escalates to credential access. We never
    // inspect a path string to infer this (that would read attacker-influenced
    // bytes); a monitor that knows it touched a credential sets the boolean.
    if (flag(data, 'credentialAccess') || flag(data, 'envProbe')) {
      return { actionClass: 'credential_access', targetClass: 'credential', tacticId: 'credential-harvest', techniqueCanonical: 'T-3002' };
    }
    if (flag(data, 'configAccess')) {
      return { actionClass: 'credential_access', targetClass: 'credential', tacticId: 'credential-harvest', techniqueCanonical: 'T-3005' };
    }
    return base;
  }
  if (source === 'process') {
    // Code-eval is malicious code deployment; keep T-9003 but it is explicit.
    if (flag(data, 'eval') || flag(data, 'evalDetected')) {
      return { actionClass: 'process_spawn', targetClass: 'tool', tacticId: 'impact', techniqueCanonical: 'T-9003' };
    }
    return base;
  }
  return base;
}

/**
 * Build a sequencePattern from a structured `data.sequence` array, if present.
 * Each element must be a recognized MonitorType (closed enum) — anything else
 * makes the whole sequence empty (fail closed; we never join free text).
 *
 * NOTE: element CONTENT is never attacker-controlled (each maps to a closed
 * actionClass token), but element ORDER is preserved (the chain order is the
 * signal). That ordering is the one residual low-bandwidth channel; it is bounded
 * here to <=16 steps over an <=8-symbol alphabet. We deliberately do NOT sort —
 * sorting would destroy the kill-chain ordering that makes the shape meaningful.
 */
function buildSequencePattern(data: Record<string, unknown>): string {
  const seq = data['sequence'];
  if (!Array.isArray(seq) || seq.length === 0 || seq.length > 16) return '';
  const tokens: ActionClass[] = [];
  for (const step of seq) {
    if (typeof step !== 'string') return '';
    const shape = SOURCE_SHAPE[step as MonitorType];
    if (!shape) return ''; // unrecognized step -> drop the whole pattern
    tokens.push(shape.actionClass);
  }
  return tokens.join('>');
}

function isSeverity(v: EventSeverity): v is Severity {
  return (SEVERITIES as readonly string[]).includes(v);
}

/**
 * Reduce an ARP event to a redacted structural signal, or return `null` if it
 * cannot be safely reduced (FAIL CLOSED). `outcome` is supplied by the emitter,
 * which alone knows whether runtime enforcement blocked the behavior.
 */
export function redactEvent(event: ARPEvent, outcome: OutcomeClass): RedactedSignal | null {
  // Gate 1: only anomalous behaviors are eligible.
  if (!REPORTABLE_CATEGORIES.has(event.category)) return null;

  // Gate 2: the source must have a known structural shape (drops heartbeat and
  // any future/unknown source).
  const base = SOURCE_SHAPE[event.source];
  if (!base) return null;

  // Gate 3: severity must be in the closed set (it always is, but assert it).
  if (!isSeverity(event.severity)) return null;

  const data = event.data ?? {};
  const shape = refineShape(base, event.source, data);

  // Optional precise override from the monitor, allowlisted only.
  let techniqueId = atm(shape.techniqueCanonical);
  const override = data['techniqueId'];
  if (typeof override === 'string' && TECHNIQUE_ALLOWLIST.has(override)) {
    techniqueId = override;
  }

  // Final defensive assertion: every token must be in its enum and wire-valid.
  if (!ACTION_CLASSES.includes(shape.actionClass)) return null;
  if (!TARGET_CLASSES.includes(shape.targetClass)) return null;
  if (!TACTIC_IDS.includes(shape.tacticId)) return null;
  if (!isWireTechniqueId(techniqueId)) return null;
  if (!OUTCOME_CLASSES.includes(outcome)) return null;

  return {
    tacticId: shape.tacticId,
    techniqueId,
    actionClass: shape.actionClass,
    targetClass: shape.targetClass,
    sequencePattern: buildSequencePattern(data),
    outcomeClass: outcome,
    severity: event.severity,
  };
}
