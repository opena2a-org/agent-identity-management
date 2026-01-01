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
  A2ASkillAttestation,
  A2AConsensusResult,
} from './types';

const A2A_BASE_PATH = '/api/v1/a2a';

export class A2AClient {
  private readonly aimClient: AIMClient;
  private readonly agentId: string;

  constructor(aimClient: AIMClient, agentId?: string) {
    this.aimClient = aimClient;
    // Use provided agentId or get from AIMClient credentials
    this.agentId = agentId ?? (aimClient as unknown as { credentials?: { agentId?: string } }).credentials?.agentId ?? '';
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

  // ==================== Multi-Agent Skill Attestation ====================

  /**
   * Attest to another agent's skill capability.
   *
   * Creates a signed attestation that contributes to multi-agent consensus
   * verification. Skills become "verified" when they reach the consensus
   * threshold (3+ unique attesters, 2+ unique owners).
   *
   * @param targetAgentId Agent whose skill is being attested
   * @param skillId ID of the skill being attested
   * @param attestationType Type of attestation
   * @param confidence Confidence level (0.0 to 1.0)
   * @param evidence Optional evidence supporting the attestation
   */
  async attestSkill(
    targetAgentId: string,
    skillId: string,
    attestationType: 'SKILL_VERIFICATION' | 'QUALITY' | 'SECURITY' = 'SKILL_VERIFICATION',
    confidence = 1.0,
    evidence?: Record<string, unknown>
  ): Promise<A2ASkillAttestation> {
    return this.post<A2ASkillAttestation>(`${A2A_BASE_PATH}/attestations`, {
      attestedAgentId: targetAgentId,
      skillId,
      attestationType,
      confidence,
      evidence: evidence ?? {},
    });
  }

  /**
   * Get attestations for an agent's skill(s).
   *
   * @param agentId Agent to get attestations for
   * @param skillId Optional specific skill (all skills if not specified)
   */
  async getSkillAttestations(agentId: string, skillId?: string): Promise<A2ASkillAttestation[]> {
    const path = skillId
      ? `${A2A_BASE_PATH}/agents/${agentId}/attestations?skillId=${skillId}`
      : `${A2A_BASE_PATH}/agents/${agentId}/attestations`;
    const response = await this.get<{ attestations: A2ASkillAttestation[] }>(path);
    return response.attestations ?? [];
  }

  /**
   * Get the consensus verification status for a skill.
   *
   * Returns whether a skill has reached the verification threshold
   * through multi-agent consensus.
   *
   * @param agentId Agent that owns the skill
   * @param skillId Skill to check
   */
  async getConsensusStatus(agentId: string, skillId: string): Promise<A2AConsensusResult> {
    return this.get<A2AConsensusResult>(
      `${A2A_BASE_PATH}/agents/${agentId}/skills/${skillId}/consensus`
    );
  }

  /**
   * Revoke a previously made attestation.
   *
   * @param attestationId ID of the attestation to revoke
   * @param reason Reason for revocation
   */
  async revokeAttestation(attestationId: string, reason: string): Promise<A2ASkillAttestation> {
    return this.post<A2ASkillAttestation>(
      `${A2A_BASE_PATH}/attestations/${attestationId}/revoke`,
      { reason }
    );
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

  /**
   * Get trust relationship with a specific peer agent.
   */
  async getPeerTrust(peerAgentId: string): Promise<A2APeerTrust> {
    return this.get<A2APeerTrust>(
      `${A2A_BASE_PATH}/agents/${this.getAgentId()}/peers/${peerAgentId}/trust`
    );
  }

  /**
   * Compute and update the A2A trust score for this agent.
   */
  async computeTrustScore(): Promise<A2ATrustScore> {
    return this.post<A2ATrustScore>(
      `${A2A_BASE_PATH}/agents/${this.getAgentId()}/trust-score/compute`,
      {}
    );
  }

  // ==================== Consent Management (GDPR/PSD2) ====================

  /**
   * Record consent for cross-agent data sharing.
   *
   * @param userId - ID of the user granting consent
   * @param recipientAgentId - ID of the agent receiving data
   * @param scope - Consent scopes (e.g., ['pii', 'payment'])
   * @param purpose - Purpose of the consent
   * @param dataTypes - Types of data being shared
   * @param consentMethod - How consent was obtained (explicit_consent, contract, etc.)
   * @param expiresInHours - Hours until consent expires
   */
  async recordConsent(
    userId: string,
    recipientAgentId: string,
    scope: string[],
    purpose: string,
    dataTypes: string[],
    consentMethod = 'explicit_consent',
    expiresInHours = 24
  ): Promise<A2AConsent> {
    return this.post<A2AConsent>(`${A2A_BASE_PATH}/consent`, {
      userId,
      grantorAgentId: this.agentId,
      recipientAgentId,
      scope,
      purpose,
      dataTypes,
      consentMethod,
      expiresInHours,
    });
  }

  /**
   * Check if consent exists for a specific scope.
   */
  async checkConsent(
    userId: string,
    recipientAgentId: string,
    scope: string
  ): Promise<{ hasConsent: boolean; consent?: A2AConsent }> {
    const query = `userId=${encodeURIComponent(userId)}&grantorAgentId=${encodeURIComponent(this.agentId)}&recipientAgentId=${encodeURIComponent(recipientAgentId)}&scope=${encodeURIComponent(scope)}`;
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
  async listUserConsents(userId: string, includeRevoked = false): Promise<A2AConsent[]> {
    const response = await this.get<{ consents: A2AConsent[] }>(
      `${A2A_BASE_PATH}/consent/user/${encodeURIComponent(userId)}?includeRevoked=${includeRevoked}`
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

  // ==================== Task Logging ====================

  /**
   * Log an A2A task for audit trail.
   *
   * @param targetAgentId - ID of the target agent
   * @param taskId - External task ID
   * @param taskType - Type of task
   * @param status - Task status (SUBMITTED, WORKING, COMPLETED, FAILED)
   */
  async logTask(
    targetAgentId: string,
    taskId: string,
    taskType: string,
    status: string
  ): Promise<{ id: string; createdAt: string }> {
    return this.post<{ id: string; createdAt: string }>(`${A2A_BASE_PATH}/tasks`, {
      targetAgentId,
      taskId,
      taskType,
      status,
    });
  }

  /**
   * Get task history for interactions with a target agent.
   *
   * @param targetAgentId - ID of the target agent
   * @param limit - Maximum number of results
   */
  async getTaskHistory(
    targetAgentId: string,
    limit = 50
  ): Promise<Array<{ id: string; taskId: string; taskType: string; status: string; createdAt: string }>> {
    const response = await this.get<{ tasks: Array<{ id: string; taskId: string; taskType: string; status: string; createdAt: string }> }>(
      `${A2A_BASE_PATH}/tasks/${targetAgentId}?limit=${limit}`
    );
    return response.tasks ?? [];
  }

  /**
   * Update the state of an A2A task.
   *
   * @param taskId - Task ID to update
   * @param state - New state (SUBMITTED, WORKING, INPUT_NEEDED, COMPLETED, FAILED, CANCELLED)
   * @param errorCode - Optional error code if failed
   * @param errorMessage - Optional error message if failed
   */
  async updateTaskState(
    taskId: string,
    state: string,
    errorCode?: string,
    errorMessage?: string
  ): Promise<{ updated: boolean }> {
    const body: Record<string, string> = { state };
    if (errorCode) {
      body.errorCode = errorCode;
    }
    if (errorMessage) {
      body.errorMessage = errorMessage;
    }
    return this.put<{ updated: boolean }>(`${A2A_BASE_PATH}/tasks/${taskId}/state`, body);
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
   * Get skills for a specific agent.
   */
  async getSkills(agentId: string): Promise<A2ASkill[]> {
    const response = await this.get<{ skills: A2ASkill[] }>(
      `${A2A_BASE_PATH}/agents/${agentId}/skills`
    );
    return response.skills ?? [];
  }

  /**
   * Search for skills across all agents.
   */
  async searchSkills(query: string, limit = 20): Promise<Array<Record<string, unknown>>> {
    const response = await this.get<{ skills: Array<Record<string, unknown>> }>(
      `${A2A_BASE_PATH}/skills/search?q=${encodeURIComponent(query)}&limit=${limit}`
    );
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
