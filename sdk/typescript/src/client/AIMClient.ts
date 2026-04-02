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
import { OAuthTokenManager, loadCredentialsFromEnv } from '../auth/oauth';
import { generateKeyPair, toBase64, createRequestSignature, fromBase64 } from '../crypto/ed25519';
import {
  AIMError,
  AuthenticationError,
  ActionDeniedError,
  VerificationError,
  NetworkError,
  parseAPIError,
} from '../exceptions';

const DEFAULT_BASE_URL = 'http://localhost:8080';
const DEFAULT_TIMEOUT = 30000;

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
  private readonly config: Required<AIMClientConfig>;
  private tokenManager: OAuthTokenManager | null = null;
  private credentials: AgentCredentials | null = null;
  private agent: Agent | null = null;

  constructor(config: AIMClientConfig = {}) {
    this.config = {
      baseUrl: config.baseUrl ?? process.env.AIM_BASE_URL ?? DEFAULT_BASE_URL,
      organizationId: config.organizationId ?? process.env.AIM_ORGANIZATION_ID ?? '',
      apiKey: config.apiKey ?? process.env.AIM_API_KEY ?? '',
      autoRegister: config.autoRegister ?? true,
      timeout: config.timeout ?? DEFAULT_TIMEOUT,
      debug: config.debug ?? process.env.AIM_DEBUG === 'true',
      headers: config.headers ?? {},
    };

    // Try to load credentials from environment
    this.credentials = loadCredentialsFromEnv();
    if (this.credentials) {
      this.tokenManager = new OAuthTokenManager(this.config.baseUrl, this.credentials);
    }
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
    useApiKey = false
  ): Promise<T> {
    const url = `${this.config.baseUrl}${path}`;
    const headers: Record<string, string> = {
      'Content-Type': 'application/json',
      'User-Agent': 'AIM-SDK-TypeScript/1.0.0',
      ...this.config.headers,
    };

    // Add authentication
    if (useApiKey && this.config.apiKey) {
      headers['X-API-Key'] = this.config.apiKey;
    } else if (this.tokenManager) {
      const token = await this.tokenManager.getAccessToken();
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
    const timeoutId = setTimeout(() => controller.abort(), this.config.timeout);

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
        throw new NetworkError(`Network error: ${error.message}`, error);
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
   * Verify an action
   */
  async verifyAction(options: VerifyActionOptions): Promise<VerificationResult> {
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

    const result = await this.request<VerificationResult>('POST', '/api/v1/verify', payload);

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
