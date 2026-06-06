/**
 * Causal-denial telemetry — correlated-record schema.
 *
 * One record joins three signals around a single denied (or allowed) action:
 *   - enforcement: the authorization outcome — an OBSERVED FACT (from FGA).
 *   - intent:      what the agent was trying to do — an INFERENCE (NanoMind).
 *   - detection:   the injection cause — an INFERENCE, never ground truth.
 *
 * The record structurally separates observed fact from detection inference:
 * `observed: true` appears ONLY on enforcement. This enforces the rule that the
 * injection attribution is never labelled as the enforcement cause.
 *
 * Two tiers exist. The full {@link CorrelatedRecord} is authoritative and stays
 * local. The {@link SharedIndicator} is the only thing that may leave the
 * machine: anonymized, carrying no payloads, paths, credentials, identifiers,
 * or the correlation key.
 */
import * as crypto from 'crypto';

/** Schema version stamped on every record and indicator. */
export const TELEMETRY_SCHEMA_VERSION = '1.0';

export type EnforcementDecision = 'allow' | 'deny';

export type EnforcementOutcome =
  | 'ALLOW'
  | 'DENY'
  | 'DENY_INTENT'
  | 'DENY_CONTEXT'
  | 'DENY_CHAIN'
  | 'DENY_ATTRIBUTE'
  | 'ERROR';

/** Origin of the Threat Matrix technique ID on a detection inference. */
export type TechniqueSource = 'apia' | 'interim-mapping';

export type Completeness =
  | 'full'
  | 'partial-missing-detection'
  | 'partial-missing-intent'
  | 'partial-missing-both';

/** OBSERVED FACT — the authorization outcome. */
export interface EnforcementFact {
  readonly observed: true;
  decision: EnforcementDecision;
  outcome: EnforcementOutcome;
  deniedBy?: string;
  deniedReason?: string;
  capability: string;
  resource: string;
  scope?: string | null;
  /** Reference to a credential, never the secret itself. */
  credentialRef?: string | null;
  policyId?: string | null;
  occurredAt: string;
  traceId?: string | null;
  source: string;
}

/** INFERENCE — the classified intent of the action. */
export interface IntentInference {
  readonly observed: false;
  intentClass: string;
  confidence: number;
  blocked: boolean;
  source: string;
  modelVersion?: string | null;
}

/** INFERENCE — the injection cause. Attribution is always "inferred". */
export interface DetectionInference {
  readonly observed: false;
  readonly attribution: 'inferred';
  injectionDetected: boolean;
  attackClass?: string;
  techniqueId?: string;
  techniqueSource: TechniqueSource;
  confidence: number;
  detector: string;
  detectorVersion?: string | null;
  /** Content hash of the flagged input, never the content. */
  inputRef?: string;
  detectedAt: string;
}

export interface AssemblyMeta {
  joinedBy: string;
  completeness: Completeness;
  joinerVersion: string;
}

/** Full, authoritative correlated record. Stays local. */
export interface CorrelatedRecord {
  schemaVersion: string;
  correlationId: string;
  recordId: string;
  agentId: string;
  createdAt: string;
  enforcement: EnforcementFact;
  intent?: IntentInference;
  detection?: DetectionInference;
  assembly: AssemblyMeta;
}

/** Anonymized indicator — the only shape permitted to leave the machine. */
export interface SharedIndicator {
  schemaVersion: string;
  sensorToken: string;
  eventType: string;
  techniqueId?: string;
  techniqueSource?: TechniqueSource;
  detectionConfidence?: number;
  enforcementOutcome: EnforcementDecision;
  agentCategory: string;
  daySinceInstall: number;
  runtimeEnv: string;
  triggeredAt: string;
}

/** Caller-supplied parts. `observed`/`attribution` are set by the builder. */
export type EnforcementInput = Omit<EnforcementFact, 'observed'>;
export type IntentInput = Omit<IntentInference, 'observed'>;
export type DetectionInput = Omit<DetectionInference, 'observed' | 'attribution'>;

export interface BuildCorrelatedRecordInput {
  correlationId: string;
  agentId: string;
  enforcement: EnforcementInput;
  intent?: IntentInput;
  detection?: DetectionInput;
  /** How the parts were joined; defaults to "correlationId". */
  joinedBy?: string;
  joinerVersion?: string;
}

function deriveCompleteness(hasIntent: boolean, hasDetection: boolean): Completeness {
  if (hasIntent && hasDetection) return 'full';
  if (!hasIntent && !hasDetection) return 'partial-missing-both';
  return hasDetection ? 'partial-missing-intent' : 'partial-missing-detection';
}

function newRecordId(): string {
  try {
    return crypto.randomUUID();
  } catch {
    return `rec_${Date.now().toString(36)}_${Math.floor(Math.random() * 1e9).toString(36)}`;
  }
}

/**
 * Assemble a correlated record. The builder owns the observed/attribution flags
 * so the fact/inference separation cannot be bypassed by a caller.
 */
export function buildCorrelatedRecord(input: BuildCorrelatedRecordInput): CorrelatedRecord {
  const intent: IntentInference | undefined = input.intent
    ? { ...input.intent, observed: false }
    : undefined;
  const detection: DetectionInference | undefined = input.detection
    ? { ...input.detection, observed: false, attribution: 'inferred' }
    : undefined;

  return {
    schemaVersion: TELEMETRY_SCHEMA_VERSION,
    correlationId: input.correlationId,
    recordId: newRecordId(),
    agentId: input.agentId,
    createdAt: new Date().toISOString(),
    enforcement: { ...input.enforcement, observed: true },
    intent,
    detection,
    assembly: {
      joinedBy: input.joinedBy ?? 'correlationId',
      completeness: deriveCompleteness(Boolean(intent), Boolean(detection)),
      joinerVersion: input.joinerVersion ?? TELEMETRY_SCHEMA_VERSION,
    },
  };
}

/** Defensive invariant check: only enforcement may be observed. */
export function assertObservedInvariant(record: CorrelatedRecord): void {
  if (record.enforcement.observed !== true) {
    throw new Error('enforcement fact must be observed:true');
  }
  if (record.intent && (record.intent as { observed: boolean }).observed !== false) {
    throw new Error('intent must be an inference (observed:false)');
  }
  if (record.detection && (record.detection as { observed: boolean }).observed !== false) {
    throw new Error('detection must be an inference (observed:false)');
  }
}

export interface SharedIndicatorContext {
  sensorToken: string;
  agentCategory: string;
  daySinceInstall: number;
  runtimeEnv: string;
  /** Override the derived event type if needed. */
  eventType?: string;
}

/**
 * Reduce a full record to the anonymized shared indicator. Sensitive fields
 * (correlationId, agentId, resource, capability, credentialRef, inputRef,
 * deniedReason, traceId) are intentionally NOT carried over.
 */
export function toSharedIndicator(
  record: CorrelatedRecord,
  ctx: SharedIndicatorContext
): SharedIndicator {
  const decision = record.enforcement.decision;
  const injected = record.detection?.injectionDetected === true;
  const eventType =
    ctx.eventType ?? (injected && decision === 'deny' ? 'denied_injection_attempt' : `${decision}_action`);

  return {
    schemaVersion: TELEMETRY_SCHEMA_VERSION,
    sensorToken: ctx.sensorToken,
    eventType,
    techniqueId: record.detection?.techniqueId,
    techniqueSource: record.detection?.techniqueSource,
    detectionConfidence: record.detection?.confidence,
    enforcementOutcome: decision,
    agentCategory: ctx.agentCategory,
    daySinceInstall: ctx.daySinceInstall,
    runtimeEnv: ctx.runtimeEnv,
    triggeredAt: new Date().toISOString(),
  };
}
