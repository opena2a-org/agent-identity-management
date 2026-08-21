// --- Core ARP Types ---

/** An event emitted by a monitor */
export interface ARPEvent {
  /** Unique event ID */
  id: string;
  /** ISO timestamp */
  timestamp: string;
  /** Monitor that generated the event */
  source: MonitorType;
  /** Event category */
  category: EventCategory;
  /** Severity determined by L0 rules */
  severity: EventSeverity;
  /** Human-readable description */
  description: string;
  /** Structured event data (monitor-specific) */
  data: Record<string, unknown>;
  /** Which intelligence layer classified this event */
  classifiedBy:
    | 'L0-rules'
    | 'L0-comply'
    | 'L1-statistical'
    | 'L1-behavioral-risk'
    | 'L1-guard-anomaly'
    | 'L2-llm';
  /** LLM assessment (only if classified by L2) */
  llmAssessment?: LLMAssessment;
}

export type MonitorType = 'process' | 'network' | 'filesystem' | 'skill' | 'heartbeat' | 'prompt' | 'mcp-protocol' | 'a2a-protocol';
export type EventCategory = 'normal' | 'anomaly' | 'violation' | 'threat';
export type EventSeverity = 'info' | 'low' | 'medium' | 'high' | 'critical';

/** Result from LLM analysis of a suspicious event */
export interface LLMAssessment {
  /** Is this event consistent with the agent's declared purpose? */
  consistent: boolean;
  /** Confidence score 0-1 */
  confidence: number;
  /** 1-2 sentence reasoning */
  reasoning: string;
  /** Recommended action */
  recommendation: 'allow' | 'alert' | 'pause' | 'kill';
  /** Tokens used for this assessment */
  tokensUsed: number;
  /** Estimated cost in USD */
  estimatedCost: number;
}

// --- Configuration ---

export interface ARPConfig {
  /** Agent name (for logging and LLM context) */
  agentName: string;
  /** Agent's declared purpose / description */
  agentDescription?: string;
  /** Agent's declared capabilities (for LLM context) */
  declaredCapabilities?: string[];
  /** Data directory for logs and state */
  dataDir?: string;
  /** Which monitors to enable (default: all) */
  monitors?: MonitorConfig;
  /** Alert and enforcement rules */
  rules?: AlertRule[];
  /** Intelligence layer configuration */
  intelligence?: IntelligenceConfig;
  /** Application-level interceptors (zero-latency, 100% accuracy) */
  interceptors?: InterceptorConfig;
  /** AI-layer protection (prompt, MCP, A2A scanning) */
  aiLayer?: AILayerConfig;
  /** HTTP reverse proxy configuration */
  proxy?: ProxyConfig;
  /** GTIN (Global Threat Intelligence Network) opt-in configuration */
  gtin?: GTINConfig;
  /**
   * Structural signature telemetry (OFF by default, opt-in). Distinct from GTIN.
   * Emits only the structural shape of anomalous behaviors, with a local audit
   * log of every byte sent. Turn on with `enabled: true` or AIM_TELEMETRY=1.
   * Set `enabled: false` (or OPENA2A_TELEMETRY_OPTOUT, or `arp telemetry
   * opt-out`) to disable ALL OpenA2A telemetry.
   */
  signatureTelemetry?: SignatureTelemetryConfig;
}

/** Structural signature telemetry configuration (see telemetry/signature). */
export interface SignatureTelemetryConfig {
  /**
   * Master switch. Undefined (the default) means nobody has chosen and the
   * channel does NOT run. true opts in; false opts out of ALL telemetry and
   * beats any opt-in.
   */
  enabled?: boolean;
  /** Registry base URL override (default OPENA2A_REGISTRY_URL or https://api.oa2a.org). */
  registryUrl?: string;
}

export interface GTINConfig {
  /** Whether GTIN telemetry is enabled (default: false, opt-in only) */
  enabled: boolean;
  /** Sensor token override (auto-generated if not provided) */
  sensorToken?: string;
  /** Registry URL override (default: https://api.oa2a.org) */
  registryUrl?: string;
}

export interface MonitorConfig {
  process?: { enabled: boolean; intervalMs?: number };
  network?: { enabled: boolean; intervalMs?: number; allowedHosts?: string[] };
  filesystem?: { enabled: boolean; watchPaths?: string[]; allowedPaths?: string[] };
  skill?: { enabled: boolean };
  heartbeat?: { enabled: boolean; expectedUrl?: string; maxStaleMs?: number };
}

/** Interceptor configuration — application-level hooks for zero-latency detection */
export interface InterceptorConfig {
  /** Hook child_process module for process spawn interception */
  process?: { enabled: boolean };
  /** Hook net.Socket for outbound connection interception */
  network?: { enabled: boolean; allowedHosts?: string[] };
  /** Hook fs module for file operation interception */
  filesystem?: { enabled: boolean; allowedPaths?: string[] };
}

export interface AlertRule {
  /** Rule name */
  name: string;
  /** Trigger condition */
  condition: AlertCondition;
  /** Action to take */
  action: 'log' | 'alert' | 'pause' | 'kill';
  /** Escalate to L2 LLM for confirmation before enforcement? */
  requireLlmConfirmation?: boolean;
}

export interface AlertCondition {
  /** Monitor source to match */
  source?: MonitorType;
  /** Category to match */
  category?: EventCategory;
  /** Minimum severity to trigger */
  minSeverity?: EventSeverity;
  /** Custom field match (e.g., { "data.host": "*.evil.com" }) */
  fieldMatch?: Record<string, string>;
  /** Threshold: trigger after N events in windowMs */
  threshold?: { count: number; windowMs: number };
}

// --- Intelligence Layer (the innovation) ---

export interface IntelligenceConfig {
  /** Enable LLM-assisted analysis (default: true) */
  enabled?: boolean;
  /**
   * Trusted NanoMind-Guard hybrid public key (Ed25519+ML-DSA-44), JSON-encoded
   * (an `EncodedHybridPublicKey`; optionally base64-wrapped for env-var
   * ergonomics). When set, the CLI proxy constructs the classification
   * annotator at start() and verifies every Guard result against this key
   * before writing `event.data.classification` for DETECTION.
   *
   * Resolution order (see `config/loader.ts`):
   * `intelligence.guardPublicKey ?? process.env.ARP_GUARD_PUBLIC_KEY ?? undefined`.
   * There is no fabricated default — an absent key means the annotator is not
   * built and classification stays null. A populated classification only ever
   * feeds detection; it never enables a deny (that stays gated on signed
   * `comply.enforce === true`).
   */
  guardPublicKey?: string;
  /** LLM adapter to use */
  adapter?: LLMAdapterType;
  /** Custom adapter config (API key, model, etc.) */
  adapterConfig?: Record<string, unknown>;
  /** Monthly budget in USD (default: 5.00) */
  budgetUsd?: number;
  /** Maximum tokens per single assessment (default: 300) */
  maxTokensPerCall?: number;
  /** Maximum L2 calls per hour (default: 20) */
  maxCallsPerHour?: number;
  /** Minimum L1 severity to escalate to L2 */
  minSeverityForLlm?: EventSeverity;
  /** Batch low-priority anomalies instead of individual calls */
  enableBatching?: boolean;
  /** Batch window in ms (default: 300000 = 5 min) */
  batchWindowMs?: number;
  /**
   * Deadline in milliseconds for a single behavioral risk IPC round-trip.
   * Enforced by `BehavioralRiskSource.getBehavioralRiskSignal`. Default is
   * 25 ms (see DEFAULT_BEHAVIORAL_RISK_TIMEOUT_MS in
   * `src/arp/intelligence/behavioral-risk.ts`). Only meaningful when a
   * behavioral risk source is attached to the coordinator; ignored
   * otherwise.
   */
  behavioralRiskTimeoutMs?: number;

  /**
   * Runtime twin (RuntimeTwin behavioral scorer) configuration. When
   * enabled, `AgentRuntimeProtection` constructs an in-process
   * `RuntimeTwin`, wraps it in `InProcessBehavioralRiskSource`, and
   * passes it to the
   * `IntelligenceCoordinator` so behavioral risk signals are fused into
   * every analyzed event.
   *
   * Default: enabled when `intelligence.enabled !== false`. Opt out by
   * setting `runtimeTwin.enabled = false`. Fleet federated learning is a
   * separate, explicit opt-in governed by `fleetEnabled`.
   *
   * The twin boots with a cold baseline and returns `NOT_READY` until it
   * has observed at least 100 events. During that warm-up the coordinator
   * records the unavailable signal and does not raise severity, matching
   * the record-and-ignore policy documented on `BehavioralRiskRecord`.
   */
  runtimeTwin?: {
    /** Default: true when intelligence is enabled. */
    enabled?: boolean;
    /** Default: false. Opt-in fleet federated learning participation. */
    fleetEnabled?: boolean;
    /** Default: 'general'. Agent category for fleet aggregation bucketing. */
    agentCategory?: string;
  };

  /**
   * Guard anomaly (classification distribution drift) detector
   * configuration. When `baseline` is provided, `AgentRuntimeProtection`
   * constructs a `GuardAnomalyDetector` and passes it to the
   * `IntelligenceCoordinator` so every classified event contributes to
   * the drift window. Without a baseline the detector is not instantiated
   * and drift detection is inert.
   *
   * The baseline is intentionally injected rather than auto-bootstrapped
   * to prevent a compromised learning period from baking an attack into
   * the reference distribution. Callers should source the baseline from
   * the Registry-exported Guard training distribution in production
   * deployments, or from a snapshotted JSON file in pre-Registry
   * deployments.
   *
   * See `src/arp/intelligence/guard-anomaly.ts` for the chi-square drift
   * model, bounded-memory contract, and parameter semantics.
   */
  guardAnomaly?: {
    /** Default: true when a baseline is provided. */
    enabled?: boolean;
    /** Required. Keys are classification labels, values are counts or probabilities. */
    baseline?: Record<string, number>;
    /** Sliding window capacity. Default: 200. */
    windowSize?: number;
    /** Chi-square statistic threshold that triggers a drift alarm. Default: ~21.666. */
    alarmThreshold?: number;
    /** Laplace smoothing alpha. Default: 0.01. */
    smoothing?: number;
    /** Minimum observations before drift scoring activates. Default: max(30, windowSize / 4). */
    minObservations?: number;
    /** Audit source tag. Default: 'guard-anomaly'. */
    sourceName?: string;
  };
}

export type LLMAdapterType =
  | 'anthropic'   // Direct Anthropic API
  | 'openai'      // Direct OpenAI API
  | 'ollama'      // Local Ollama
  | 'agent-proxy' // Tap into the agent's own LLM (parasitic mode)
  | 'custom';     // User-provided adapter

/** Interface that LLM adapters must implement */
export interface LLMAdapter {
  /** Adapter name */
  readonly name: string;
  /** Send a micro-prompt and get a structured response */
  assess(prompt: string, maxTokens: number): Promise<LLMResponse>;
  /** Estimate cost for a given prompt length */
  estimateCost(inputTokens: number, outputTokens: number): number;
  /** Check if the adapter is available and configured */
  healthCheck(): Promise<boolean>;
}

export interface LLMResponse {
  content: string;
  inputTokens: number;
  outputTokens: number;
  model: string;
}

// --- Budget Tracking ---

export interface BudgetState {
  /** Total spent in current period (USD) */
  totalSpentUsd: number;
  /** Period start (ISO timestamp) */
  periodStart: string;
  /** Number of L2 calls made */
  totalCalls: number;
  /** Calls in current hour */
  callsThisHour: number;
  /** Hour start (ISO timestamp) */
  hourStart: string;
  /** Per-call cost history (last 100) */
  recentCosts: Array<{ timestamp: string; cost: number; tokens: number }>;
}

// --- Enforcement ---

export type EnforcementAction = 'log' | 'alert' | 'pause' | 'kill';

export interface EnforcementResult {
  action: EnforcementAction;
  targetPid?: number;
  success: boolean;
  reason: string;
  event: ARPEvent;
}

// --- AI-Layer Configuration ---

export interface AILayerConfig {
  /** Prompt scanning (injection, jailbreak, data leak detection) */
  prompt?: { enabled: boolean };
  /** MCP protocol scanning (parameter injection, path traversal, SSRF) */
  mcp?: { enabled: boolean; allowedTools?: string[] };
  /** A2A protocol scanning (identity spoofing, delegation abuse) */
  a2a?: { enabled: boolean; trustedAgents?: string[] };
}

// --- Proxy Configuration ---

export interface ProxyConfig {
  /** Port to listen on */
  port: number;
  /** Upstream targets */
  upstreams: ProxyUpstream[];
  /** Block requests on detection (default: false, alert only) */
  blockOnDetection?: boolean;
  /**
   * Optional path to a signed capability manifest YAML file. When set, the
   * proxy loads and verifies the manifest at `start()`. On any verification
   * failure the proxy enters a deny-all state and every request is answered
   * with 403. See `src/arp/crypto/manifest-loader.ts` for the wire format.
   */
  manifestPath?: string;
}

/**
 * Structured log emitted when a capability manifest is rejected at proxy
 * start. Passed to `ARPProxyDeps.onManifestRejection` so callers can route
 * the rejection to a SIEM or audit trail. The client that triggered the
 * request never sees these details; it only gets a generic 403 deny reason.
 */
export interface ManifestRejectionLog {
  /** Discrete error code from the loader (e.g. SIGNATURE_INVALID, EXPIRED). */
  code: string;
  /** Absolute or relative path the loader attempted to read, if any. */
  manifestPath?: string;
  /** Short, non-sensitive reason string for the rejection. */
  reason?: string;
  /** ISO timestamp when the rejection was observed. */
  timestamp: string;
}

export interface ProxyUpstream {
  /** Path prefix to route (e.g., '/openai', '/mcp', '/a2a') */
  pathPrefix: string;
  /** Upstream target URL (e.g., 'http://localhost:3003') */
  target: string;
  /** Protocol hint for request/response parsing */
  protocol: 'openai-api' | 'mcp-http' | 'a2a' | 'passthrough';
}

// --- Monitor Interface ---

export interface Monitor {
  readonly type: MonitorType;
  start(): Promise<void>;
  stop(): Promise<void>;
  isRunning(): boolean;
}

// --- Capability Manifest (AIComply P1, brief Section 5.4) ---

/**
 * Capability tier that bounds what ARP is permitted to do on behalf of an agent.
 * Higher tiers imply broader permitted classes and stricter signing requirements.
 * Per AIComply brief Section 5.4 / AC-010 (seven key hierarchies).
 */
export type CapabilityTier = 'minimal' | 'read' | 'execute' | 'mutate' | 'privileged';

/**
 * Action to take when an agent attempts a class listed in `prohibited_classes`
 * or a class outside `permitted_classes`.
 * Maps onto the existing ARP enforcement actions.
 */
export type ComplyOnViolation = 'log' | 'alert' | 'pause' | 'kill' | 'deny';

/**
 * Compliance envelope for a capability manifest.
 * Allow-list and deny-list of classification classes (as produced by NanoMind-Guard)
 * with a violation policy. If a classification is not in `permitted_classes` and
 * not in `prohibited_classes`, the default policy is `deny` (parse-to-deny, CR-001).
 */
export interface ComplyEnvelope {
  /** Classes the agent is explicitly allowed to operate on */
  permitted_classes: string[];
  /** Classes the agent is explicitly forbidden from (takes precedence over permitted) */
  prohibited_classes: string[];
  /** Action on violation (allow/prohibited class, or unknown class under parse-to-deny) */
  on_violation: ComplyOnViolation;
  /**
   * Whether the coordinator's comply gate actively enforces this envelope.
   *
   * Default (absent or `false`): DETECTION ONLY. A populated
   * `event.data.classification` is recorded for offline consumers (the
   * sequence projector and the telemetry tee) and the comply gate is a no-op
   * — it does not run `applyComplyGate`, write a `comply` decision, raise
   * severity via `on_violation`, or short-circuit the stack. This is the
   * load-bearing default: populating a classification (e.g. from a model) must
   * not silently become a hot-path DENY control.
   *
   * Scope note: this guarantee is about the comply / deny path specifically.
   * The guard-anomaly drift detector — when an operator attaches a baseline —
   * is a separate, opt-in DETECTION signal that also reads the classification
   * label and may raise severity to `anomaly`/`medium` on distribution drift;
   * it never denies or kills. The detection-mode wiring in the ARP proxy
   * deliberately constructs its coordinator WITHOUT a guard-anomaly or
   * behavioral-risk source, so a classification reaching that coordinator only
   * ever meets the `enforce`-gated comply gate.
   *
   * `true`: the operator has explicitly opted this agent into AIComply
   * enforcement. The comply gate evaluates the permitted/prohibited lists and
   * routes `on_violation` as a deterministic class→action policy. This is an
   * operator choice that lives in the signed manifest payload, never a
   * runtime side effect of wiring classification.
   */
  enforce?: boolean;
}

// --- NanoMind-Guard Classification (AIComply P1, producer side) ---

/**
 * Wire shape of a classification result emitted by NanoMind-Guard.
 *
 * NanoMind-Guard is the producer of the classification labels that the ARP
 * coordinator's L0-comply gate consumes at `event.data.classification`. Every
 * emitted result is hybrid-signed with the high-throughput ML-DSA-44 variant
 * (Ed25519+ML-DSA-44) so the coordinator can verify that a classification
 * actually originated from the Guard and has not been tampered with in transit.
 *
 * ML-DSA-44 is deliberately chosen over ML-DSA-65 (used for capability
 * manifests) to keep signing cost low on the per-event hot path. Manifests
 * are loaded once at startup; classifications are signed per event.
 *
 * Wire format on disk or on the network is a JSON object with the
 * `signature` field as an `EncodedHybridSignature`. The signed payload is
 * the result object with the `signature` field removed, serialized to
 * canonical JSON (sorted keys, no whitespace) by
 * `canonicalizeGuardResultPayload` in `verify-classification.ts`.
 */
export interface NanoMindGuardResult {
  /** Classification label the Guard produced for this event (non-empty). */
  classification: string;
  /** Model confidence in the classification, in [0, 1]. */
  confidence: number;
  /** Semver of the NanoMind-Guard model that produced the result. */
  modelVersion: string;
  /** SHA-256 hex digest of the event payload that was classified. */
  contentHash: string;
  /** Unix ms timestamp when the Guard signed the result. */
  timestamp: number;
  /** Hybrid Ed25519+ML-DSA-44 signature in the wire-encoded form. */
  signature: import('./crypto/types').EncodedHybridSignature;
}

/**
 * Error codes returned by `verifyClassification` when a NanoMind-Guard
 * result is rejected. Kept as a discrete union so callers can route on the
 * reason without parsing strings. Stable surface.
 */
export type NanoMindGuardVerifyErrorCode =
  | 'SCHEMA_ERROR'
  | 'ALGORITHM_UNSUPPORTED'
  | 'KEY_FORMAT_ERROR'
  | 'SIGNATURE_INVALID'
  | 'STALE'
  | 'FUTURE_DATED'
  | 'TIER_REJECTED'
  | 'UNKNOWN_CLASSIFICATION'
  | 'ABSOLUTE_DENY';

/**
 * Result of `verifyClassification`. On the valid branch the `classification`
 * field is the cleared label the caller writes into `event.data.classification`
 * before handing the event to the coordinator. On the invalid branch `code`
 * and `reason` describe the rejection.
 */
export type NanoMindGuardVerifyResult =
  | {
      valid: true;
      classification: string;
      tier: CapabilityTier;
      confidence: number;
    }
  | {
      valid: false;
      code: NanoMindGuardVerifyErrorCode;
      reason: string;
    };

/**
 * Configuration passed to `verifyClassification`. The caller supplies the
 * trusted Guard public key (Ed25519+ML-DSA-44), the capability manifest that
 * bounds the agent, and optional freshness parameters.
 *
 * A clock override is exposed so the unit tests can drive the freshness
 * check without mocking the global clock.
 */
export interface NanoMindGuardVerifyOptions {
  /** Trusted Guard hybrid public key (Ed25519+ML-DSA-44). */
  guardPublicKey: import('./crypto/types').EncodedHybridPublicKey;
  /** Capability manifest whose `tier` keys the rejection matrix. */
  manifest: CapabilityManifest;
  /**
   * Maximum age of a Guard signature before it is rejected as STALE.
   * Default: 5 minutes (300_000 ms). Per-event signing is expected to be
   * well under a second, so anything older is a replay suspect.
   */
  maxAgeMs?: number;
  /**
   * Tolerance for signatures that appear to be timestamped slightly in the
   * future (clock skew between producer and consumer). Default: 30 seconds.
   * Anything beyond this is rejected as FUTURE_DATED.
   */
  futureSkewMs?: number;
  /** Clock override. Defaults to `Date.now`. Used by the test harness. */
  now?: () => number;
}

/**
 * Capability manifest for an AIComply-governed agent.
 * Must be signed with an Ed25519+ML-DSA-65 hybrid signature before ARP will load it
 * (see `src/arp/crypto/hybrid-signing.ts`). Invalid or missing signatures trigger
 * parse-to-deny via the existing CR-001 code path.
 *
 * Wire format (YAML on disk) carries the fields below as the signed payload, plus a
 * detached `signature` block. The runtime shape (this interface) is what the loader
 * returns after verification.
 */
export interface CapabilityManifest {
  /** Manifest format version (semver). Current supported: "1.0.0". */
  version: string;
  /** Stable identifier for the governed agent (matches ARPConfig.agentName). */
  agentId: string;
  /** Capability tier for this agent. */
  tier: CapabilityTier;
  /** Compliance envelope (permitted/prohibited classes, violation policy). */
  comply: ComplyEnvelope;
  /** ISO timestamp when the manifest was issued. */
  issuedAt: string;
  /** Optional ISO timestamp when the manifest expires. */
  expiresAt?: string;
  /** Base64-encoded Ed25519 public key used to co-sign the manifest. */
  ed25519PublicKey: string;
  /** Base64-encoded ML-DSA-65 public key used to co-sign the manifest. */
  mldsa65PublicKey: string;
}
