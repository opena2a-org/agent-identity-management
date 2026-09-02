/**
 * AIM Client - Core SDK functionality for agent identity verification
 */

import type {
  AIMClientConfig,
  Agent,
  AgentCredentials,
  RegisterAgentOptions,
  VerificationResult,
  VerifyActionOptions,
} from '../types';
import { RiskLevel } from '../types';
import { SDK_VERSION } from '../version';
import { OAuthTokenManager, loadCredentialsFromEnv } from '../auth/oauth';
import { generateKeyPair, toBase64, createRequestSignature, fromBase64 } from '../crypto/ed25519';
import {
  AIMError,
  AuthenticationError,
  ActionDeniedError,
  VerificationError,
  ConfigurationError,
  NetworkError,
  parseAPIError,
} from '../exceptions';
import { LocalVerifier } from '../local';
import type { Atx } from '../local';
import {
  SandboxType,
  NetworkIsolation,
  FilesystemIsolation,
  ProcessIsolation,
  autoDetectIsolation,
  type IsolationPosture,
  type IsolationAttestationResult,
} from '../isolation/index';
import type { SecretsClient } from '../secrets';
import {
  CorrelationJoiner,
  CorrelatedRelay,
  correlationHeaders,
  mintCorrelationId,
  openA2AHome,
  type EnforcementInput,
  type RelayConfig,
} from '../telemetry';

const DEFAULT_BASE_URL = 'http://localhost:8080';
const DEFAULT_TIMEOUT = 30000;
// The enforcement deadline. ONE deadline, started when verifyAction's remote
// call begins, bounds the WHOLE verification: the OAuth token exchange (when
// one is needed) and the verify POST share the time left on it, never a
// fresh budget each. No production
// latency distribution for the verification routes has been measured, so the
// value is a choice, not a finding. UNMEASURED DEFAULT.
const DEFAULT_ENFORCEMENT_TIMEOUT = 5000;
const DEFAULT_ENFORCEMENT_SOURCE = 'aim-pdp';

/**
 * Read AIM_ENFORCEMENT_TIMEOUT_MS (integer milliseconds — the same spelling
 * and unit the Python SDK reads, so one .env value means one thing to both
 * SDKs). A missing, non-integer, zero or negative value yields undefined so
 * resolution falls through to the default.
 */
function envEnforcementTimeout(): number | undefined {
  const raw = process.env.AIM_ENFORCEMENT_TIMEOUT_MS?.trim();
  if (!raw || !/^\d+$/.test(raw)) {
    return undefined;
  }
  const millis = Number(raw);
  return millis > 0 ? millis : undefined;
}

/**
 * AIM Client for agent identity verification
 *
 * @example
 * ```typescript
 * import { AIMClient } from '@opena2a/aim-sdk';
 *
 * const client = new AIMClient({
 *   baseUrl: 'https://aim.example.com',
 *   apiKey: 'your-api-key',
 * });
 *
 * // Register an agent
 * const agent = await client.registerAgent({
 *   name: 'my-agent',
 *   displayName: 'My AI Agent',
 *   agentType: 'langchain',
 * });
 *
 * // Verify an action
 * const result = await client.verifyAction({
 *   action: 'file:read',
 *   resource: '/data/sensitive.json',
 * });
 * ```
 */
export class AIMClient {
  private readonly config: Required<Omit<AIMClientConfig, 'localVerification'>>;
  private tokenManager: OAuthTokenManager | null = null;
  private credentials: AgentCredentials | null = null;
  private agent: Agent | null = null;
  private _secrets: SecretsClient | null = null;

  // Spec-compliant local ATX verification (opt-in). When a verifier and a
  // resolved credential are present, verifyAction verifies the signed credential
  // offline instead of calling the central PDP. The remote POST stays as fallback.
  private readonly localVerifier: LocalVerifier | null;
  private localCredential: Atx | null = null;

  // Causal-denial telemetry — opt-in, best-effort, off the enforcement path.
  private readonly telemetryEnabled: boolean;
  private readonly enforcementSource: string;
  private readonly suppliedJoiner: CorrelationJoiner | null;
  private ownJoiner: CorrelationJoiner | undefined;
  // Stage-2 opt-in: upload anonymized indicators to the Registry.
  private readonly relayConfig: RelayConfig | null;
  private ownRelay: CorrelatedRelay | undefined;
  // Single resolved telemetry dir shared by the auto-joiner sink and the relay,
  // so the relay always reads exactly where the joiner writes.
  private readonly telemetryDir: string;
  // True when baseUrl fell through to the localhost default (neither the
  // `baseUrl` option nor a usable AIM_BASE_URL was provided). Used to add an
  // actionable hint to connection failures instead of a bare ECONNREFUSED.
  private readonly usedDefaultBaseUrl: boolean;

  constructor(config: AIMClientConfig = {}) {
    // Treat an empty/whitespace AIM_BASE_URL as unset so the flag and the
    // resolved baseUrl below agree: an empty env var must fall through to the
    // localhost default (not become a relative URL that fails to parse), and
    // the hint must reflect that.
    const envBaseUrl = process.env.AIM_BASE_URL?.trim() || undefined;
    this.usedDefaultBaseUrl = config.baseUrl == null && envBaseUrl === undefined;
    this.config = {
      baseUrl: config.baseUrl ?? envBaseUrl ?? DEFAULT_BASE_URL,
      organizationId: config.organizationId ?? process.env.AIM_ORGANIZATION_ID ?? '',
      apiKey: config.apiKey ?? process.env.AIM_API_KEY ?? '',
      autoRegister: config.autoRegister ?? true,
      timeout: config.timeout ?? DEFAULT_TIMEOUT,
      enforcementTimeout:
        config.enforcementTimeout ?? envEnforcementTimeout() ?? DEFAULT_ENFORCEMENT_TIMEOUT,
      debug: config.debug ?? process.env.AIM_DEBUG === 'true',
      headers: config.headers ?? {},
      telemetry: config.telemetry ?? {},
    };

    const telemetry = this.config.telemetry;
    this.telemetryEnabled = telemetry.enabled === true;
    this.enforcementSource = telemetry.enforcementSource ?? DEFAULT_ENFORCEMENT_SOURCE;
    this.suppliedJoiner = telemetry.joiner ?? null;
    this.relayConfig = telemetry.relay ?? null;
    // Resolve once: a relay.dataDir override (else ~/.opena2a) is the single
    // location both the auto-joiner sink and the relay use.
    this.telemetryDir = this.relayConfig?.dataDir ?? openA2AHome();

    // Opt-in local verification: build a verifier from cached trust anchors.
    // Absent this, verifyAction behaves exactly as before (remote POST).
    this.localVerifier = config.localVerification
      ? new LocalVerifier(config.localVerification)
      : null;

    // Try to load credentials from environment
    this.credentials = loadCredentialsFromEnv();
    if (this.credentials) {
      this.tokenManager = new OAuthTokenManager(this.config.baseUrl, this.credentials);
    }
  }

  /**
   * Lazy-initialized secrets client for identity-native credential management.
   */
  get secrets(): SecretsClient {
    if (!this._secrets) {
      // Dynamic import to avoid circular dependency and keep secrets optional
      // eslint-disable-next-line @typescript-eslint/no-var-requires
      const { SecretsClient: SC } = require('../secrets') as { SecretsClient: typeof SecretsClient };
      this._secrets = new SC({
        request: <T>(method: string, path: string, body?: unknown, useApiKey?: boolean) =>
          this.request<T>(method, path, body, useApiKey),
        getPrivateKey: () => this.credentials?.privateKey ?? null,
        getPublicKey: () => this.credentials?.publicKey ?? null,
      });
    }
    return this._secrets;
  }

  /**
   * Log debug message if debug mode is enabled
   */
  private log(message: string, ...args: unknown[]): void {
    if (this.config.debug) {
      console.log(`[AIM] ${message}`, ...args);
    }
  }

  /**
   * Make an authenticated API request
   */
  private async request<T>(
    method: string,
    path: string,
    body?: unknown,
    useApiKey = false,
    extraHeaders?: Record<string, string>,
    deadlineAt?: number
  ): Promise<T> {
    const url = `${this.config.baseUrl}${path}`;
    const headers: Record<string, string> = {
      'Content-Type': 'application/json',
      'User-Agent': `AIM-SDK-TypeScript/${SDK_VERSION}`,
      ...this.config.headers,
      // Best-effort, non-auth extras (e.g. the correlation ID). Listed before
      // auth/signature headers below so they can never override them.
      ...extraHeaders,
    };

    // Add authentication
    if (useApiKey && this.config.apiKey) {
      headers['X-API-Key'] = this.config.apiKey;
    } else if (this.tokenManager) {
      // On the enforcement path the token exchange spends the SAME deadline
      // the verify POST will finish on; its abort surfaces as the one timeout
      // error the caller already knows.
      let token: string;
      try {
        token = await this.tokenManager.getAccessToken(deadlineAt);
      } catch (error) {
        if (error instanceof Error && error.name === 'AbortError') {
          throw new NetworkError('Request timed out', error);
        }
        throw error;
      }
      headers['Authorization'] = `Bearer ${token}`;
    } else if (this.config.apiKey) {
      headers['X-API-Key'] = this.config.apiKey;
    }

    // Add signature if we have credentials
    if (this.credentials) {
      const privateKey = fromBase64(this.credentials.privateKey);
      const { signature, timestamp } = await createRequestSignature(
        privateKey,
        method,
        path,
        body as string | object | undefined
      );
      headers['X-AIM-Signature'] = signature;
      headers['X-AIM-Timestamp'] = timestamp.toString();
      headers['X-AIM-Agent-ID'] = this.credentials.agentId;
    }

    this.log(`${method} ${path}`);

    const controller = new AbortController();
    // On the enforcement path the abort fires at the time LEFT on the shared
    // deadline (never a fresh budget per request); every other request keeps
    // the general per-request timeout exactly as before.
    const abortAfterMs =
      deadlineAt === undefined ? this.config.timeout : Math.max(deadlineAt - Date.now(), 0);
    const timeoutId = setTimeout(() => controller.abort(), abortAfterMs);

    try {
      const response = await fetch(url, {
        method,
        headers,
        body: body ? JSON.stringify(body) : undefined,
        signal: controller.signal,
      });

      clearTimeout(timeoutId);

      if (!response.ok) {
        const errorBody = await response.json().catch(() => ({}));
        throw parseAPIError(response.status, errorBody);
      }

      // Handle empty responses
      const text = await response.text();
      if (!text) {
        return {} as T;
      }

      return JSON.parse(text) as T;
    } catch (error) {
      clearTimeout(timeoutId);

      if (error instanceof AIMError) {
        throw error;
      }

      if (error instanceof Error) {
        if (error.name === 'AbortError') {
          throw new NetworkError('Request timed out', error);
        }
        const hint = this.usedDefaultBaseUrl
          ? ` (baseUrl defaulted to ${DEFAULT_BASE_URL} — no baseUrl option or AIM_BASE_URL env var was set)`
          : '';
        throw new NetworkError(`Network error: ${error.message}${hint}`, error);
      }

      throw new NetworkError('Unknown network error');
    }
  }

  /**
   * Register a new agent
   */
  async registerAgent(options: RegisterAgentOptions): Promise<Agent> {
    // Generate key pair for the agent
    const keyPair = await generateKeyPair();
    const publicKey = toBase64(keyPair.publicKey);
    const privateKey = toBase64(keyPair.privateKey);

    const payload = {
      name: options.name,
      displayName: options.displayName ?? options.name,
      description: options.description ?? '',
      agentType: options.agentType ?? 'custom',
      version: options.version ?? '1.0.0',
      capabilities: options.capabilities ?? [],
      talksTo: options.talksTo ?? [],
      metadata: options.metadata ?? {},
      communityIntelligenceOptIn: options.communityIntelligenceOptIn ?? false,
      publicKey,
    };

    this.log('Registering agent:', options.name);

    const response = await this.request<Record<string, unknown>>(
      'POST',
      '/api/v1/agents',
      payload,
      true // Use API key for registration
    );

    // Handle both response formats:
    // 1. Wrapped format: { agent: {...}, credentials: { agentId: "..." } }
    // 2. Flat format (actual server): { id, name, displayName, trustScore, ... }
    let agentData: Agent;
    let agentId: string;

    if (response.agent && typeof response.agent === 'object') {
      // Wrapped format
      agentData = response.agent as Agent;
      const creds = response.credentials as { agentId: string } | undefined;
      agentId = creds?.agentId ?? agentData.id;
    } else if (response.id || response.agentId) {
      // Flat format from the actual AIM server
      agentId = (response.id ?? response.agentId) as string;
      agentData = response as unknown as Agent;
    } else {
      throw new AIMError('Unexpected response format from server: missing agent data');
    }

    // Store credentials
    this.credentials = {
      agentId,
      privateKey,
      publicKey,
      organizationId: this.config.organizationId,
      createdAt: new Date().toISOString(),
    };

    // Initialize token manager
    this.tokenManager = new OAuthTokenManager(this.config.baseUrl, this.credentials);

    this.agent = agentData;
    this.log('Agent registered:', this.agent.id);

    return this.agent;
  }

  /**
   * Get the current agent
   */
  async getAgent(): Promise<Agent | null> {
    if (!this.credentials) {
      return null;
    }

    if (this.agent) {
      return this.agent;
    }

    try {
      this.agent = await this.request<Agent>('GET', `/api/v1/agents/${this.credentials.agentId}`);
      return this.agent;
    } catch {
      return null;
    }
  }

  /**
   * Verify an action.
   *
   * When local verification is configured (a {@link LocalVerificationConfig} was
   * passed to the constructor) AND a resolved ATX credential is available (via
   * {@link setLocalCredential} or the `atx` argument), the signed credential is
   * verified offline and authorization is decided from its signed claims — no
   * call to the central PDP. Otherwise this falls back to the remote
   * `POST /api/v1/verify` decision (the original behavior).
   *
   * @param atx Optional ATX credential to verify locally for this call. Overrides
   *   any credential set via {@link setLocalCredential}.
   */
  async verifyAction(options: VerifyActionOptions, atx?: Atx): Promise<VerificationResult> {
    const credential = atx ?? this.localCredential;
    if (this.localVerifier && credential) {
      return this.verifyActionLocally(options, credential);
    }

    // ONE deadline over the whole remote verification call, started on entry:
    // the token exchange (when one is needed) and the verify POST are both
    // aborted by the time left on it.
    const deadlineAt = Date.now() + this.config.enforcementTimeout;

    if (!this.credentials) {
      throw new AuthenticationError('No credentials available. Register an agent first.');
    }

    const payload = {
      agentId: this.credentials.agentId,
      action: options.action,
      resource: options.resource,
      resourceType: options.resourceType,
      context: options.context ?? {},
      riskLevel: options.riskLevel,
    };

    this.log('Verifying action:', options.action);

    // Mint the correlation ID at the earliest ingestion point and propagate it
    // best-effort. Minting/headers are skipped entirely when telemetry is off.
    const correlationId = this.telemetryEnabled ? mintCorrelationId() : undefined;

    const result = await this.request<VerificationResult>(
      'POST',
      '/api/v1/verify',
      payload,
      false,
      correlationHeaders(correlationId),
      deadlineAt
    );

    // Record the enforcement outcome (allow OR deny) BEFORE the deny throw, so
    // the causal-denial case is captured. Never affects the result below.
    if (correlationId) {
      this.recordVerificationTelemetry(correlationId, options, result);
    }

    if (!result.actionAllowed) {
      throw new ActionDeniedError(
        options.action,
        result.denialReason ?? 'Action not allowed',
        result.trustScore
      );
    }

    return result;
  }

  /**
   * Cache a resolved ATX credential for the agent. This is the output of AAP
   * credential resolution (the only step that touches the network); once cached,
   * {@link verifyAction} / {@link verifyActionLocally} verify it offline per
   * action. Pass `null` to clear it (falling back to the remote PDP path).
   */
  setLocalCredential(atx: Atx | null): void {
    this.localCredential = atx;
  }

  /**
   * The configured local verifier, or null when local verification is not enabled.
   * Use it directly for credential verification without the action-authorization
   * adaptation that {@link verifyActionLocally} performs.
   */
  getLocalVerifier(): LocalVerifier | null {
    return this.localVerifier;
  }

  /**
   * Verify an action against a locally-held ATX credential — fully offline,
   * sub-millisecond after the first call. The credential's signature, expiry,
   * revocation, and issuer trust are checked against the cached anchors, then a
   * local broker policy authorizes the action from the credential's *signed*
   * capabilities (v1.0 capabilities are forgeable and are refused by default).
   *
   * Mirrors {@link verifyAction}'s contract: returns a {@link VerificationResult}
   * on allow and throws {@link ActionDeniedError} on deny.
   *
   * @param atx The ATX to verify. Falls back to the credential set via
   *   {@link setLocalCredential}.
   */
  async verifyActionLocally(options: VerifyActionOptions, atx?: Atx): Promise<VerificationResult> {
    if (!this.localVerifier) {
      throw new ConfigurationError(
        'Local verification is not configured. Pass `localVerification` (trust anchors) to the AIMClient constructor.',
      );
    }
    const credential = atx ?? this.localCredential;
    if (!credential) {
      throw new VerificationError(
        'No local ATX credential available. Resolve one and call setLocalCredential(), or pass it to verifyActionLocally().',
      );
    }

    this.log('Verifying action locally:', options.action);

    // Mint the correlation ID at the earliest ingestion point (telemetry off
    // unless enabled), exactly as the remote path does.
    const correlationId = this.telemetryEnabled ? mintCorrelationId() : undefined;

    const auth = await this.localVerifier.authorize(credential, { action: options.action });

    const result: VerificationResult = {
      verified: auth.verified,
      agentId: auth.agentId,
      // An ATX carries no display name, so report the agent's DID rather than
      // aliasing the opaque agentId as a "name" (which would misrepresent the
      // field and diverge from the remote path's real agentName).
      agentName: auth.agentDid,
      trustScore: auth.trustScore,
      riskLevel: options.riskLevel ?? RiskLevel.LOW,
      actionAllowed: auth.actionAllowed,
      denialReason: auth.denialReason,
      timestamp: new Date().toISOString(),
    };

    // Record the enforcement outcome (allow OR deny) BEFORE the deny throw, so the
    // causal-denial case is captured. Best-effort; never affects the result.
    if (correlationId) {
      this.recordVerificationTelemetry(correlationId, options, result);
    }

    if (!result.actionAllowed) {
      throw new ActionDeniedError(
        options.action,
        result.denialReason ?? 'Action not allowed',
        result.trustScore,
      );
    }

    return result;
  }

  /**
   * Feed the enforcement outcome (and any supplied intent/detection) into the
   * correlation joiner. Fully best-effort: a missing joiner, missing agent, or
   * any thrown error is swallowed so action verification is never affected.
   */
  private recordVerificationTelemetry(
    correlationId: string,
    options: VerifyActionOptions,
    result: VerificationResult
  ): void {
    try {
      const joiner = this.getJoiner();
      const agentId = this.credentials?.agentId;
      if (!joiner || !agentId) return;

      const enforcement: EnforcementInput = {
        decision: result.actionAllowed ? 'allow' : 'deny',
        outcome: result.actionAllowed ? 'ALLOW' : 'DENY',
        deniedReason: result.actionAllowed ? undefined : result.denialReason,
        capability: options.action,
        resource: options.resource ?? '',
        occurredAt: result.timestamp ?? new Date().toISOString(),
        traceId: result.eventId ?? null,
        source: this.enforcementSource,
      };
      joiner.ingestEnforcement({ correlationId, agentId, enforcement });

      const tele = options.telemetry;
      if (tele?.intent) joiner.ingestIntent({ correlationId, intent: tele.intent });
      if (tele?.detection) joiner.ingestDetection({ correlationId, detection: tele.detection });
    } catch (error) {
      this.log('telemetry error (ignored):', error);
    }
  }

  /**
   * Resolve the joiner: a caller-supplied one wins; otherwise lazily create and
   * start an internally managed joiner with an unref'd flush timer. Returns null
   * when telemetry is disabled.
   */
  private getJoiner(): CorrelationJoiner | null {
    if (!this.telemetryEnabled) return null;
    // Spin up the upload relay alongside the joiner when stage-2 sharing is on.
    this.ensureRelay();
    if (this.suppliedJoiner) return this.suppliedJoiner;
    if (!this.ownJoiner) {
      // Write to the same dir the relay reads, so an enabled relay actually
      // finds the records this joiner produces.
      this.ownJoiner = new CorrelationJoiner({ dataDir: this.telemetryDir });
      this.ownJoiner.start();
    }
    return this.ownJoiner;
  }

  /**
   * Lazily start the managed upload relay. No-op unless telemetry capture AND
   * relay sharing are both opted in (two-stage consent). The relay drains the
   * local correlated-events log to the Registry on its own unref'd timer.
   */
  private ensureRelay(): void {
    if (!this.telemetryEnabled || this.relayConfig?.enabled !== true) return;
    if (this.ownRelay) return;
    // Pin the relay to the same resolved dir as the auto-joiner sink.
    this.ownRelay = new CorrelatedRelay({ ...this.relayConfig, dataDir: this.telemetryDir });
    this.ownRelay.start();
  }

  /**
   * Stop the internally managed telemetry joiner timer and upload relay, if
   * created. A caller-supplied joiner is left untouched (the caller owns its
   * lifecycle).
   */
  closeTelemetry(): void {
    if (this.ownJoiner) {
      this.ownJoiner.stop();
      this.ownJoiner = undefined;
    }
    if (this.ownRelay) {
      this.ownRelay.stop();
      this.ownRelay = undefined;
    }
  }

  /**
   * Decorator for verifying actions before execution
   */
  verify(action: string, options?: Partial<VerifyActionOptions>) {
    return <T extends (...args: unknown[]) => Promise<unknown>>(
      _target: unknown,
      _propertyKey: string,
      descriptor: TypedPropertyDescriptor<T>
    ): TypedPropertyDescriptor<T> => {
      const originalMethod = descriptor.value!;

      descriptor.value = (async (...args: unknown[]) => {
        await this.verifyAction({
          action,
          ...options,
          context: {
            ...options?.context,
            args: args.length > 0 ? args : undefined,
          },
        });
        return originalMethod.apply(this, args);
      }) as T;

      return descriptor;
    };
  }

  /**
   * Create a verified action wrapper
   */
  createVerifiedAction<T extends (...args: unknown[]) => Promise<unknown>>(
    action: string,
    fn: T,
    options?: Partial<VerifyActionOptions>
  ): T {
    return (async (...args: unknown[]) => {
      await this.verifyAction({
        action,
        ...options,
        context: {
          ...options?.context,
          args: args.length > 0 ? args : undefined,
        },
      });
      return fn(...args);
    }) as T;
  }

  /**
   * Update agent metadata
   */
  async updateAgent(updates: Partial<RegisterAgentOptions>): Promise<Agent> {
    if (!this.credentials) {
      throw new AuthenticationError('No credentials available. Register an agent first.');
    }

    this.agent = await this.request<Agent>(
      'PUT',
      `/api/v1/agents/${this.credentials.agentId}`,
      updates
    );

    return this.agent;
  }

  /**
   * Report capabilities
   */
  async reportCapabilities(capabilities: string[]): Promise<void> {
    if (!this.credentials) {
      throw new AuthenticationError('No credentials available. Register an agent first.');
    }

    await this.request('POST', `/api/v1/agents/${this.credentials.agentId}/capabilities/report`, {
      capabilities,
    });
  }

  /**
   * Get agent trust score
   */
  async getTrustScore(): Promise<number> {
    const agent = await this.getAgent();
    return agent?.trustScore ?? 0;
  }

  /**
   * Check if the client is authenticated
   */
  isAuthenticated(): boolean {
    return this.credentials !== null;
  }

  /**
   * Get the current credentials
   */
  getCredentials(): AgentCredentials | null {
    return this.credentials;
  }

  /**
   * Set credentials (for loading from storage)
   */
  setCredentials(credentials: AgentCredentials): void {
    this.credentials = credentials;
    this.tokenManager = new OAuthTokenManager(this.config.baseUrl, credentials);
  }

  /**
   * Submit an isolation attestation reporting the agent's runtime isolation posture.
   * Contributes to the Execution Isolation trust factor (Factor 9).
   */
  async attestIsolation(options: {
    sandbox?: SandboxType;
    network?: NetworkIsolation;
    filesystem?: FilesystemIsolation;
    process?: ProcessIsolation;
    autoDetect?: boolean;
  } = {}): Promise<IsolationAttestationResult> {
    let posture: IsolationPosture;

    if (options.autoDetect) {
      posture = await autoDetectIsolation();
    } else {
      posture = {
        sandbox: options.sandbox ?? SandboxType.None,
        network: options.network ?? NetworkIsolation.None,
        filesystem: options.filesystem ?? FilesystemIsolation.None,
        process: options.process ?? ProcessIsolation.None,
      };
    }

    const payload: Record<string, string> = {
      sandbox: posture.sandbox,
      network: posture.network,
      filesystem: posture.filesystem,
      process: posture.process,
    };

    if (!this.credentials) {
      throw new AuthenticationError('No credentials available. Register an agent first.');
    }

    const response = await this.request<IsolationAttestationResult>(
      'POST',
      `/api/v1/sdk-api/agents/${this.credentials.agentId}/isolation-attestation`,
      payload,
    );

    return response;
  }

  /**
   * Clear all authentication state
   */
  logout(): void {
    this.credentials = null;
    this.tokenManager = null;
    this.agent = null;
  }
}

/**
 * Create and configure an AIM client instance
 */
export function createClient(config?: AIMClientConfig): AIMClient {
  return new AIMClient(config);
}

/**
 * Register an agent with minimal configuration
 */
export async function registerAgent(
  name: string,
  options?: Partial<RegisterAgentOptions> & { config?: AIMClientConfig }
): Promise<{ client: AIMClient; agent: Agent; credentials: AgentCredentials }> {
  const client = new AIMClient(options?.config);
  const agent = await client.registerAgent({
    name,
    ...options,
  });

  const credentials = client.getCredentials();
  if (!credentials) {
    throw new VerificationError('Failed to get credentials after registration');
  }

  return { client, agent, credentials };
}
