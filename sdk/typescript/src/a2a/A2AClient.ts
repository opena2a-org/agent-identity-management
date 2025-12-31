/**
 * A2A Client - Agent-to-Agent protocol operations
 *
 * Provides secure agent-to-agent communication with:
 * - Agent card registration and discovery
 * - Intent-based agent routing
 * - Ed25519 request signing and verification
 * - Trust score management
 * - GDPR/PSD2 compliant consent management
 * - Security policy enforcement
 *
 * @example
 * ```typescript
 * import { AIMClient, A2AClient } from '@opena2a/aim-sdk';
 *
 * const aimClient = new AIMClient({ baseUrl: 'https://aim.example.com' });
 * const a2a = new A2AClient(aimClient);
 *
 * // Register agent card
 * const card = await a2a.registerAgentCard('https://myagent.example.com/.well-known/agent.json');
 *
 * // Find agents by intent
 * const agents = await a2a.routeByIntent('analyze code quality');
 *
 * // Check security before calling
 * const security = await a2a.checkSecurity(targetAgentId);
 * if (security.allowed) {
 *   // Make the A2A call
 * }
 * ```
 */

import type { AIMClient } from '../client/AIMClient';
import type {
  A2AAgentCard,
  A2ATrustScore,
  A2APeerTrust,
  A2AConsent,
  A2ARequestSignature,
  A2ASecurityCheckResult,
  A2ASecuritySettings,
  A2ASecurityViolation,
  CapableAgent,
  A2ASkill,
} from './types';

const A2A_BASE_PATH = '/api/v1/a2a';

export class A2AClient {
  private readonly aimClient: AIMClient;

  constructor(aimClient: AIMClient) {
    this.aimClient = aimClient;
  }

  // ==================== Agent Card Operations ====================

  /**
   * Register an A2A agent card for the current agent.
   */
  async registerAgentCard(cardUrl: string): Promise<A2AAgentCard> {
    return this.post<A2AAgentCard>(`${A2A_BASE_PATH}/cards`, { cardUrl });
  }

  /**
   * Get an agent card by agent ID.
   */
  async getAgentCard(agentId: string): Promise<A2AAgentCard> {
    return this.get<A2AAgentCard>(`${A2A_BASE_PATH}/cards/${agentId}`);
  }

  /**
   * Update the current agent's card.
   */
  async updateAgentCard(card: Partial<A2AAgentCard>): Promise<A2AAgentCard> {
    const agentId = card.agentId ?? this.getAgentId();
    return this.put<A2AAgentCard>(`${A2A_BASE_PATH}/cards/${agentId}`, card);
  }

  /**
   * Refresh AIM attestation for an agent card.
   */
  async refreshAttestation(agentId: string): Promise<A2AAgentCard> {
    return this.post<A2AAgentCard>(`${A2A_BASE_PATH}/cards/${agentId}/attestation`, {});
  }

  /**
   * List all agent cards.
   */
  async listAgentCards(limit = 100, offset = 0): Promise<A2AAgentCard[]> {
    const response = await this.get<{ cards: A2AAgentCard[] }>(
      `${A2A_BASE_PATH}/cards?limit=${limit}&offset=${offset}`
    );
    return response.cards ?? [];
  }

  // ==================== Intent-Based Discovery ====================

  /**
   * Route a natural language intent to capable agents.
   *
   * Uses full-text search to find agents with skills matching the intent.
   * Returns agents ranked by relevance and trust score.
   *
   * @param intent Natural language description of what you need (e.g., "analyze code quality")
   * @param minTrustScore Minimum trust score required (0.0 to 1.0)
   */
  async routeByIntent(intent: string, minTrustScore = 0): Promise<CapableAgent[]> {
    const response = await this.post<{ agents: CapableAgent[] }>(
      `${A2A_BASE_PATH}/discovery/route`,
      { intent, minTrustScore }
    );
    return response.agents ?? [];
  }

  /**
   * Find agents capable of handling specific skills.
   */
  async getCapableAgents(skillIds: string[], minTrustScore = 0): Promise<CapableAgent[]> {
    const response = await this.post<{ agents: CapableAgent[] }>(
      `${A2A_BASE_PATH}/discovery/capable`,
      { skillIds, minTrustScore }
    );
    return response.agents ?? [];
  }

  // ==================== Security Policy Operations ====================

  /**
   * Check security policy before making an A2A call.
   *
   * Evaluates security settings to determine if an A2A call is allowed.
   * In "monitor" mode, violations are logged but allowed.
   * In "strict" mode, violations block the request.
   */
  async checkSecurity(targetAgentId: string, skillId?: string): Promise<A2ASecurityCheckResult> {
    const body: Record<string, string> = { targetAgentId };
    if (skillId) {
      body.skillId = skillId;
    }
    return this.post<A2ASecurityCheckResult>(`${A2A_BASE_PATH}/security/check`, body);
  }

  /**
   * Get organization's A2A security settings.
   */
  async getSecuritySettings(): Promise<A2ASecuritySettings> {
    return this.get<A2ASecuritySettings>(`${A2A_BASE_PATH}/security/settings`);
  }

  /**
   * Update organization's A2A security settings.
   */
  async updateSecuritySettings(settings: Partial<A2ASecuritySettings>): Promise<A2ASecuritySettings> {
    return this.put<A2ASecuritySettings>(`${A2A_BASE_PATH}/security/settings`, settings);
  }

  /**
   * Get recent security violations.
   */
  async getSecurityViolations(limit = 100): Promise<A2ASecurityViolation[]> {
    const response = await this.get<{ violations: A2ASecurityViolation[] }>(
      `${A2A_BASE_PATH}/security/violations?limit=${limit}`
    );
    return response.violations ?? [];
  }

  // ==================== Request Signing & Verification ====================

  /**
   * Sign an outbound A2A request using Ed25519.
   */
  async signRequest(method: string, path: string, body?: string): Promise<A2ARequestSignature> {
    return this.post<A2ARequestSignature>(`${A2A_BASE_PATH}/sign`, { method, path, body });
  }

  /**
   * Verify an incoming A2A request signature.
   */
  async verifyRequest(
    signature: string,
    publicKey: string,
    timestamp: number,
    nonce: string,
    method: string,
    path: string,
    body?: string
  ): Promise<{ valid: boolean; agentId?: string; error?: string }> {
    return this.post('/api/v1/a2a/verify', {
      signature,
      publicKey,
      timestamp,
      nonce,
      method,
      path,
      body,
    });
  }

  // ==================== Trust Score Operations ====================

  /**
   * Get trust score for another agent.
   */
  async getTrustScore(targetAgentId: string): Promise<A2ATrustScore> {
    return this.get<A2ATrustScore>(`${A2A_BASE_PATH}/trust/${targetAgentId}`);
  }

  /**
   * Update trust score for another agent.
   */
  async updateTrustScore(
    targetAgentId: string,
    score: number,
    confidence: number,
    factors?: Record<string, number>
  ): Promise<A2ATrustScore> {
    return this.put<A2ATrustScore>(`${A2A_BASE_PATH}/trust/${targetAgentId}`, {
      score,
      confidence,
      factors,
    });
  }

  /**
   * Record a successful interaction with another agent.
   */
  async recordSuccessfulInteraction(
    targetAgentId: string,
    taskType: string,
    durationMs: number
  ): Promise<A2ATrustScore> {
    return this.post<A2ATrustScore>(`${A2A_BASE_PATH}/trust/${targetAgentId}/interaction`, {
      success: true,
      taskType,
      durationMs,
    });
  }

  /**
   * Record a failed interaction with another agent.
   */
  async recordFailedInteraction(
    targetAgentId: string,
    taskType: string,
    errorReason: string
  ): Promise<A2ATrustScore> {
    return this.post<A2ATrustScore>(`${A2A_BASE_PATH}/trust/${targetAgentId}/interaction`, {
      success: false,
      taskType,
      errorReason,
    });
  }

  /**
   * List all peer trust relationships.
   */
  async listPeerTrusts(): Promise<A2APeerTrust[]> {
    const response = await this.get<{ peers: A2APeerTrust[] }>(`${A2A_BASE_PATH}/peers`);
    return response.peers ?? [];
  }

  // ==================== Consent Management (GDPR/PSD2) ====================

  /**
   * Record consent for cross-agent data sharing.
   */
  async recordConsent(
    userId: string,
    targetAgentId: string,
    purpose: string,
    dataTypes: string[],
    legalBasis: string,
    expiresAt?: Date
  ): Promise<A2AConsent> {
    return this.post<A2AConsent>(`${A2A_BASE_PATH}/consent`, {
      userId,
      targetAgentId,
      purpose,
      dataTypes,
      legalBasis,
      expiresAt: expiresAt?.toISOString(),
    });
  }

  /**
   * Check if consent exists for a specific operation.
   */
  async checkConsent(
    userId: string,
    targetAgentId: string,
    purpose: string,
    dataType: string
  ): Promise<{ hasConsent: boolean; consent?: A2AConsent }> {
    const query = `userId=${userId}&targetAgentId=${targetAgentId}&purpose=${purpose}&dataType=${dataType}`;
    return this.get<{ hasConsent: boolean; consent?: A2AConsent }>(
      `${A2A_BASE_PATH}/consent/check?${query}`
    );
  }

  /**
   * Revoke consent for cross-agent data sharing.
   */
  async revokeConsent(consentId: string, reason: string): Promise<A2AConsent> {
    return this.post<A2AConsent>(`${A2A_BASE_PATH}/consent/${consentId}/revoke`, { reason });
  }

  /**
   * List all consents for a user.
   */
  async listUserConsents(userId: string): Promise<A2AConsent[]> {
    const response = await this.get<{ consents: A2AConsent[] }>(
      `${A2A_BASE_PATH}/consent?userId=${userId}`
    );
    return response.consents ?? [];
  }

  // ==================== Policy Evaluation ====================

  /**
   * Evaluate cross-agent policy for an operation.
   */
  async evaluatePolicy(
    targetAgentId: string,
    action: string,
    resource: string,
    context?: Record<string, unknown>
  ): Promise<{ allowed: boolean; reason?: string }> {
    return this.post(`${A2A_BASE_PATH}/policies/evaluate`, {
      targetAgentId,
      action,
      resource,
      context,
    });
  }

  // ==================== Skill Operations ====================

  /**
   * Register a skill for the current agent.
   */
  async registerSkill(skill: Omit<A2ASkill, 'id'>): Promise<A2ASkill> {
    return this.post<A2ASkill>(`${A2A_BASE_PATH}/skills`, skill);
  }

  /**
   * List all skills for the current agent.
   */
  async listSkills(): Promise<A2ASkill[]> {
    const response = await this.get<{ skills: A2ASkill[] }>(`${A2A_BASE_PATH}/skills`);
    return response.skills ?? [];
  }

  /**
   * Delete a skill.
   */
  async deleteSkill(skillId: string): Promise<void> {
    await this.delete(`${A2A_BASE_PATH}/skills/${skillId}`);
  }

  // ==================== URL-Based Discovery ====================

  /**
   * Discover agent by URL (fetch and register their agent card).
   */
  async discoverAgent(agentUrl: string): Promise<A2AAgentCard> {
    const cardUrl = agentUrl.endsWith('/')
      ? `${agentUrl}.well-known/agent.json`
      : `${agentUrl}/.well-known/agent.json`;
    return this.registerAgentCard(cardUrl);
  }

  // ==================== Helper Methods ====================

  private getAgentId(): string {
    const credentials = this.aimClient.getCredentials();
    if (!credentials) {
      throw new Error('No credentials available. Register an agent first.');
    }
    return credentials.agentId;
  }

  private getHeaders(): Record<string, string> {
    const headers: Record<string, string> = {
      'Content-Type': 'application/json',
    };

    const credentials = this.aimClient.getCredentials();
    if (credentials) {
      headers['Authorization'] = `Bearer ${credentials.agentId}`;
    }

    return headers;
  }

  private async get<T>(path: string): Promise<T> {
    const baseUrl = (this.aimClient as unknown as { config: { baseUrl: string } }).config.baseUrl;
    const response = await fetch(`${baseUrl}${path}`, {
      method: 'GET',
      headers: this.getHeaders(),
    });

    if (!response.ok) {
      throw new Error(`A2A request failed: ${response.status} ${response.statusText}`);
    }

    return (await response.json()) as T;
  }

  private async post<T>(path: string, body: unknown): Promise<T> {
    const baseUrl = (this.aimClient as unknown as { config: { baseUrl: string } }).config.baseUrl;
    const response = await fetch(`${baseUrl}${path}`, {
      method: 'POST',
      headers: this.getHeaders(),
      body: JSON.stringify(body),
    });

    if (!response.ok) {
      throw new Error(`A2A request failed: ${response.status} ${response.statusText}`);
    }

    return (await response.json()) as T;
  }

  private async put<T>(path: string, body: unknown): Promise<T> {
    const baseUrl = (this.aimClient as unknown as { config: { baseUrl: string } }).config.baseUrl;
    const response = await fetch(`${baseUrl}${path}`, {
      method: 'PUT',
      headers: this.getHeaders(),
      body: JSON.stringify(body),
    });

    if (!response.ok) {
      throw new Error(`A2A request failed: ${response.status} ${response.statusText}`);
    }

    return (await response.json()) as T;
  }

  private async delete(path: string): Promise<void> {
    const baseUrl = (this.aimClient as unknown as { config: { baseUrl: string } }).config.baseUrl;
    const response = await fetch(`${baseUrl}${path}`, {
      method: 'DELETE',
      headers: this.getHeaders(),
    });

    if (!response.ok) {
      throw new Error(`A2A request failed: ${response.status} ${response.statusText}`);
    }
  }
}

/**
 * Create an A2A client from an AIM client.
 */
export function createA2AClient(aimClient: AIMClient): A2AClient {
  return new A2AClient(aimClient);
}

/**
 * Simple intent-based agent discovery.
 *
 * @example
 * ```typescript
 * import { findAgentForIntent, AIMClient } from '@opena2a/aim-sdk';
 *
 * const client = new AIMClient();
 * const agent = await findAgentForIntent(client, 'analyze code quality');
 * console.log('Found agent:', agent?.name);
 * ```
 */
export async function findAgentForIntent(
  aimClient: AIMClient,
  intent: string,
  minTrustScore = 0
): Promise<CapableAgent | null> {
  const a2a = new A2AClient(aimClient);
  const agents = await a2a.routeByIntent(intent, minTrustScore);
  return agents.length > 0 ? agents[0] : null;
}
