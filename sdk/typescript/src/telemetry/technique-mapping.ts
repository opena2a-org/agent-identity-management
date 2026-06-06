/**
 * Causal-denial telemetry — interim attack-class to Threat Matrix mapping.
 *
 * Until the injection analyzer (APIA) emits a precise technique ID, detection
 * inferences carry an interim mapping from a coarse injection attack class to a
 * representative Threat Matrix technique. Each entry is derived from the real
 * matrix taxonomy (agent-threat-matrix matrix.json v1.1): the attack class maps
 * to a matrix attackClass node that genuinely exists, and the technique ID is a
 * member of that node's technique set.
 *
 * This is an INTERIM, judgment-based correspondence. The authoritative
 * attack-class to technique map is owned by Threat Matrix governance, and the
 * canonical technique registry is the Registry (`GET /api/v1/threat-matrix`).
 * The snapshot below is for offline, best-effort validation only; a record's
 * `techniqueSource` stays `interim-mapping` so a trained detector can supersede
 * it with `apia` and no schema change.
 */
import type { DetectionInput, TechniqueSource } from './correlated-record';

/** Threat Matrix snapshot this mapping was derived against. */
export const MATRIX_SNAPSHOT_VERSION = '1.1';

/** Valid technique-ID format. */
export const TECHNIQUE_ID_RE = /^T-\d{4}$/;

/**
 * Snapshot of the 61 canonical technique IDs (matrix.json v1.1). The Registry
 * remains the source of truth; this is for offline best-effort validation.
 */
export const KNOWN_TECHNIQUE_IDS: ReadonlySet<string> = new Set([
  'T-1001', 'T-1002', 'T-1003', 'T-1004', 'T-1005', 'T-1006', 'T-1007',
  'T-2001', 'T-2002', 'T-2003', 'T-2004', 'T-2005', 'T-2006', 'T-2007', 'T-2008', 'T-2009',
  'T-3001', 'T-3002', 'T-3003', 'T-3004', 'T-3005', 'T-3006',
  'T-4001', 'T-4002', 'T-4003', 'T-4004', 'T-4005', 'T-4006', 'T-4007',
  'T-5001', 'T-5002', 'T-5003', 'T-5004', 'T-5005', 'T-5006',
  'T-6001', 'T-6002', 'T-6003', 'T-6004', 'T-6005', 'T-6006', 'T-6007',
  'T-7001', 'T-7002', 'T-7003', 'T-7004', 'T-7005', 'T-7006', 'T-7007',
  'T-8001', 'T-8002', 'T-8003', 'T-8004', 'T-8005', 'T-8006',
  'T-9001', 'T-9002', 'T-9003', 'T-9004', 'T-9005', 'T-9006',
]);

export interface InterimMappingEntry {
  /** Representative Threat Matrix technique for this attack class. */
  techniqueId: string;
  /** Matrix attackClass node this technique was drawn from (provenance). */
  matrixAttackClass: string;
  /** Why this correspondence (one line). */
  note: string;
}

/**
 * Interim map: injection attack class -> representative technique.
 * Keys are normalized (lowercase, hyphenated). Every techniqueId is a member of
 * the cited matrixAttackClass in matrix.json v1.1 (verified).
 */
export const INTERIM_ATTACK_CLASS_MAP: Readonly<Record<string, InterimMappingEntry>> = {
  // External content achieving override of agent behavior.
  indirect: { techniqueId: 'T-4002', matrixAttackClass: 'SOUL-HIJACK', note: 'external content override' },
  // Instructions injected directly into the prompt / system prompt.
  direct: { techniqueId: 'T-1003', matrixAttackClass: 'SOUL-INJECT', note: 'system-prompt injection' },
  // Injection carried in retrieved (RAG) content.
  'rag-embedded': { techniqueId: 'T-2002', matrixAttackClass: 'RAG-POISON', note: 'poisoned retrieval content' },
  // Gradual displacement of safety instructions across turns.
  'multi-turn': { techniqueId: 'T-2004', matrixAttackClass: 'SOUL-DRIFT', note: 'conversational drift' },
  // Malicious content arriving via a tool/MCP return channel.
  'tool-output': { techniqueId: 'T-4007', matrixAttackClass: 'FAKETOOL-INJECT', note: 'tool impersonation/injection' },
  // Overriding harm-avoidance / safety constraints.
  jailbreak: { techniqueId: 'T-2001', matrixAttackClass: 'SOUL-HV', note: 'harm-avoidance override' },
};

// Common aliases normalized onto the canonical keys above.
const ALIASES: Readonly<Record<string, string>> = {
  'prompt-injection': 'direct',
  'direct-injection': 'direct',
  'indirect-injection': 'indirect',
  'indirect-prompt-injection': 'indirect',
  rag: 'rag-embedded',
  'rag-poisoning': 'rag-embedded',
  multiturn: 'multi-turn',
  'tool-injection': 'tool-output',
};

function normalize(attackClass: string): string {
  const key = attackClass.trim().toLowerCase().replace(/[\s_]+/g, '-');
  return ALIASES[key] ?? key;
}

/** True if the ID is well-formed AND present in the snapshot. */
export function isValidTechniqueId(id: unknown): id is string {
  return typeof id === 'string' && TECHNIQUE_ID_RE.test(id) && KNOWN_TECHNIQUE_IDS.has(id);
}

/** True if the ID merely matches the T-NNNN format (Registry may know newer IDs). */
export function isTechniqueIdFormat(id: unknown): id is string {
  return typeof id === 'string' && TECHNIQUE_ID_RE.test(id);
}

/**
 * Map a coarse attack class to its interim technique entry, or undefined when
 * unknown. Never guesses — an unmapped class yields no technique ID.
 */
export function mapAttackClass(attackClass: string | undefined): InterimMappingEntry | undefined {
  if (!attackClass) return undefined;
  return INTERIM_ATTACK_CLASS_MAP[normalize(attackClass)];
}

/**
 * Build the technique fields for a DetectionInput from an attack class. Returns
 * `techniqueSource: 'interim-mapping'` always; `techniqueId` is set only when
 * the class is known and maps to a valid technique.
 */
export function interimTechniqueFields(
  attackClass: string | undefined
): Pick<DetectionInput, 'techniqueId' | 'techniqueSource'> {
  const source: TechniqueSource = 'interim-mapping';
  const entry = mapAttackClass(attackClass);
  if (entry && isValidTechniqueId(entry.techniqueId)) {
    return { techniqueId: entry.techniqueId, techniqueSource: source };
  }
  return { techniqueSource: source };
}
