/**
 * Vendored SecurityAST types — the artifact-graph vocabulary the AST drift
 * monitor compares runtime events against.
 *
 * Provenance: copied (types only, no runtime code) from
 * hackmyagent `src/nanomind-core/types.ts` as part of the ARP -> AIM SDK
 * runtime-protection migration. The compiler that PRODUCES SecurityAST
 * documents stays in hackmyagent (scan-time); this module only CONSUMES them
 * at runtime, so the type contract is duplicated here rather than importing
 * across the package boundary. If the producer schema changes shape, update
 * this file to match — the signature check on the AST document is the
 * compatibility backstop, not this type.
 */

// ============================================================================
// Abstract Security Tree
// ============================================================================

export interface SecurityAST {
  /** Artifact identity */
  artifactType: ArtifactType;
  contentHash: string;            // SHA-256 of the original artifact
  artifactPath?: string;          // File path (relative)
  artifactSize: number;           // Bytes

  /** Declarations: what the artifact SAYS it does */
  declaredPurpose: string;
  declaredCapabilities: Capability[];
  declaredConstraints: Constraint[];
  declaredDataAccess: DataAccessPattern[];

  /** Inferred: what NanoMind UNDERSTANDS it does */
  inferredCapabilities: Capability[];
  inferredRiskSurface: RiskSurface[];
  intentClassification: IntentClass;
  intentConfidence: number;       // 0-1

  /** Relationships */
  dependsOn: string[];            // Content hashes of referenced artifacts
  governedBy: string[];           // Content hashes of governing artifacts (SOUL, system prompt)

  /** Evidence: exact text regions supporting the classification */
  evidenceSpans: EvidenceSpan[];

  /** Cryptographic integrity */
  signature: string;              // Ed25519 signature of the AST (excluding this field)
  modelVersion: string;           // NanoMind version that produced this AST
  compiledAt: string;             // ISO 8601 timestamp
}

// ============================================================================
// Artifact Types
// ============================================================================

export type ArtifactType =
  | 'skill'
  | 'mcp_config'
  | 'soul'
  | 'system_prompt'
  | 'agent_config'
  | 'a2a_card'
  | 'credential_file'
  | 'source_code'
  | 'env_file'
  | 'unknown';

// ============================================================================
// Capabilities
// ============================================================================

export interface Capability {
  /** Capability identifier (e.g., "db.read", "api.call", "file.write") */
  name: string;
  /** Scope of the capability (e.g., "customers table", "weather API") */
  scope: string;
  /** Was this explicitly declared in the artifact? */
  declared: boolean;
  /** Was this inferred by NanoMind from the content? */
  inferred: boolean;
  /** Risk level of this capability */
  riskLevel: 'low' | 'medium' | 'high' | 'critical';
  /** Evidence: text span that declares or implies this capability */
  evidence?: string;
}

// ============================================================================
// Constraints
// ============================================================================

export interface Constraint {
  /** The constraint as written in the artifact */
  text: string;
  /** Governance domain (trust, oversight, data_handling, etc.) */
  domain: ConstraintDomain;
  /** How enforceable is this constraint? (0 = aspirational, 1 = enforced) */
  enforceability: number;
  /** How easy to bypass? (0 = robust, 1 = trivially bypassable) */
  bypassRisk: number;
  /** Specific weakness if bypassRisk > 0.5 */
  weakness?: string;
}

export type ConstraintDomain =
  | 'trust_hierarchy'
  | 'human_oversight'
  | 'data_handling'
  | 'action_reversibility'
  | 'capability_boundary'
  | 'identity_disclosure'
  | 'error_handling'
  | 'credential_management'
  | 'behavioral_constraint'
  | 'general';

// ============================================================================
// Data Access Patterns
// ============================================================================

export interface DataAccessPattern {
  /** What data type is accessed */
  dataType: string;         // "pii", "credentials", "financial", "general"
  /** How it's accessed */
  accessMode: 'read' | 'write' | 'delete' | 'transmit';
  /** Where it goes (if transmit) */
  destination?: string;
  /** Is this access declared in capabilities? */
  coveredByCapability: boolean;
}

// ============================================================================
// Risk Surface
// ============================================================================

export interface RiskSurface {
  /** What aspect of the artifact is risky */
  surface: string;
  /** Attack class from HMA taxonomy */
  attackClass: string;
  /** Confidence this is a real risk (0-1) */
  confidence: number;
  /** Specific text that creates this risk */
  evidence: string;
  /** How to mitigate */
  mitigation?: string;
}

// ============================================================================
// Intent Classification
// ============================================================================

export type IntentClass = 'benign' | 'suspicious' | 'malicious';

// ============================================================================
// Evidence Spans
// ============================================================================

export interface EvidenceSpan {
  /** Start character offset in original artifact */
  start: number;
  /** End character offset */
  end: number;
  /** The actual text */
  text: string;
  /** What this evidence supports */
  supports: string;         // e.g., "exfiltration_intent", "credential_exposure"
  /** Confidence this evidence is relevant */
  confidence: number;
}
