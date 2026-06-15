/**
 * AIM SDK Type Definitions
 */

import type { CorrelationJoiner, IntentInput, DetectionInput, RelayConfig } from '../telemetry';
import type { LocalVerificationConfig } from '../local';

/**
 * Agent types supported by AIM
 */
export enum AgentType {
  // LLM Provider-based agents
  CLAUDE = 'claude',
  GPT = 'gpt',
  GEMINI = 'gemini',
  LLAMA = 'llama',
  MISTRAL = 'mistral',
  COHERE = 'cohere',

  // Framework-based agents
  LANGCHAIN = 'langchain',
  LLAMAINDEX = 'llamaindex',
  AUTOGEN = 'autogen',
  CREWAI = 'crewai',
  LANGGRAPH = 'langgraph',
  HAYSTACK = 'haystack',
  SEMANTIC_KERNEL = 'semantic_kernel',

  // Copilot/Assistant types
  COPILOT = 'copilot',
  ASSISTANT = 'assistant',
  CHATBOT = 'chatbot',

  // Autonomous agents
  AUTOGPT = 'autogpt',
  BABYAGI = 'babyagi',

  // Generic types
  CUSTOM = 'custom',
  AI_AGENT = 'ai_agent',
}

/**
 * Risk levels for actions
 */
export enum RiskLevel {
  LOW = 'low',
  MEDIUM = 'medium',
  HIGH = 'high',
  CRITICAL = 'critical',
}

/**
 * Agent status
 */
export enum AgentStatus {
  PENDING = 'pending',
  VERIFIED = 'verified',
  SUSPENDED = 'suspended',
  REVOKED = 'revoked',
}

/**
 * Configuration options for AIM client
 */
export interface AIMClientConfig {
  /** AIM server base URL */
  baseUrl?: string;
  /** Organization ID */
  organizationId?: string;
  /** API key for authentication */
  apiKey?: string;
  /** Enable auto-registration of agent */
  autoRegister?: boolean;
  /** Request timeout in milliseconds */
  timeout?: number;
  /** Enable debug logging */
  debug?: boolean;
  /** Custom headers to include in requests */
  headers?: Record<string, string>;
  /** Causal-denial telemetry (opt-in, best-effort, off the enforcement path). */
  telemetry?: TelemetryConfig;
  /**
   * Spec-compliant local ATX verification (opt-in). When set, the client can
   * verify a resolved credential offline against these cached trust anchors
   * instead of calling the central PDP per action. See {@link LocalVerificationConfig}.
   */
  localVerification?: LocalVerificationConfig;
}

/**
 * Causal-denial telemetry configuration.
 *
 * Opt-in and best-effort: when disabled (the default) no correlation ID is
 * minted, no `x-opena2a-correlation-id` header is attached, and no record is
 * written. A telemetry failure never affects an action verification's result.
 */
export interface TelemetryConfig {
  /** Master switch. When false/absent, telemetry is fully inert. */
  enabled?: boolean;
  /**
   * Supply a joiner to control its lifecycle and sink. When omitted, the client
   * lazily creates one (writing to the local correlated-events log) and manages
   * an unref'd flush timer that does not keep the process alive.
   */
  joiner?: CorrelationJoiner;
  /** Stamped onto the enforcement fact's `source`. Default 'aim-pdp'. */
  enforcementSource?: string;
  /**
   * Stage-2 opt-in: ship anonymized indicators from the local log to the
   * Registry. Inert unless BOTH `enabled` (above, capture) and `relay.enabled`
   * (share) are true. Only anonymized `SharedIndicator`s leave the machine —
   * never full correlated records. When omitted, nothing is uploaded.
   */
  relay?: RelayConfig;
}

/**
 * Agent registration options
 */
export interface RegisterAgentOptions {
  /** Unique agent name */
  name: string;
  /** Human-readable display name */
  displayName?: string;
  /** Agent description */
  description?: string;
  /** Agent type */
  agentType?: AgentType | string;
  /** Agent version */
  version?: string;
  /** Capabilities the agent will use */
  capabilities?: string[];
  /** MCP servers the agent connects to */
  talksTo?: string[];
  /** Additional metadata */
  metadata?: Record<string, unknown>;
  /** Enable anonymous community intelligence telemetry (default: false) */
  communityIntelligenceOptIn?: boolean;
}

/**
 * Registered agent information
 */
export interface Agent {
  id: string;
  name: string;
  displayName: string;
  description?: string;
  agentType: string;
  version: string;
  status: AgentStatus;
  trustScore: number;
  organizationId: string;
  capabilities: string[];
  talksTo: string[];
  metadata?: Record<string, unknown>;
  publicKey?: string;
  createdAt: string;
  updatedAt: string;
  lastActive?: string;
}

/**
 * Verification result
 */
export interface VerificationResult {
  /** Whether the verification was successful */
  verified: boolean;
  /** Agent ID */
  agentId: string;
  /** Agent name */
  agentName: string;
  /** Current trust score */
  trustScore: number;
  /** Risk level of the action */
  riskLevel: RiskLevel;
  /** Whether the action is allowed */
  actionAllowed: boolean;
  /** Reason for denial if action not allowed */
  denialReason?: string;
  /** Verification timestamp */
  timestamp: string;
  /** Verification event ID */
  eventId?: string;
}

/**
 * Action verification options
 */
export interface VerifyActionOptions {
  /** Action being performed */
  action: string;
  /** Resource being accessed */
  resource?: string;
  /** Resource type */
  resourceType?: string;
  /** Additional context */
  context?: Record<string, unknown>;
  /** Risk level override */
  riskLevel?: RiskLevel;
  /**
   * Optional causal-denial telemetry inputs for this action. The future
   * ARP/Guard runtime module populates these (classified intent + injection
   * detection) so the joiner can assemble a full correlated record; until then
   * callers may supply them directly. Ignored unless telemetry is enabled.
   */
  telemetry?: {
    /** Classified intent inference from the NanoMind intent daemon. */
    intent?: IntentInput;
    /** Injection-cause inference from NanoMind-Guard. */
    detection?: DetectionInput;
  };
}

/**
 * Credentials for agent authentication
 */
export interface AgentCredentials {
  agentId: string;
  privateKey: string;
  publicKey: string;
  organizationId: string;
  createdAt: string;
}

/**
 * OAuth token response
 */
export interface TokenResponse {
  accessToken: string;
  tokenType: string;
  expiresIn: number;
  scope?: string;
}

/**
 * API error response
 */
export interface APIError {
  error: string;
  message: string;
  statusCode: number;
  details?: Record<string, unknown>;
}

/**
 * Webhook event types
 */
export enum WebhookEvent {
  // Alert events
  ALERT_CREATED = 'alert.created',
  ALERT_ACKNOWLEDGED = 'alert.acknowledged',

  // Agent events
  AGENT_CREATED = 'agent.created',
  AGENT_VERIFIED = 'agent.verified',
  AGENT_UPDATED = 'agent.updated',
  AGENT_SUSPENDED = 'agent.suspended',
  AGENT_DELETED = 'agent.deleted',

  // Trust score events
  TRUST_SCORE_CHANGED = 'trust_score.changed',
  TRUST_SCORE_LOW = 'trust_score.low',

  // Security events
  SECURITY_VIOLATION = 'security.violation',
  COMPLIANCE_VIOLATION = 'compliance.violation',
  CAPABILITY_DRIFT = 'security.capability_drift',
  DATA_EXFILTRATION = 'security.data_exfiltration',

  // API key events
  API_KEY_CREATED = 'api_key.created',
  API_KEY_REVOKED = 'api_key.revoked',
  API_KEY_EXPIRED = 'api_key.expired',
}

/**
 * Webhook payload
 */
export interface WebhookPayload {
  id: string;
  event: WebhookEvent;
  timestamp: string;
  organizationId: string;
  data: Record<string, unknown>;
}

/**
 * MCP server information
 */
export interface MCPServer {
  id: string;
  name: string;
  displayName: string;
  description?: string;
  serverType: string;
  endpoint?: string;
  capabilities: string[];
  trustScore: number;
  status: string;
}
