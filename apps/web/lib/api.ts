const API_URL = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080'

export interface Agent {
  id: string
  organization_id: string
  name: string
  display_name: string
  description: string
  agent_type: 'ai_agent' | 'mcp_server'
  status: 'pending' | 'verified' | 'suspended' | 'revoked'
  version: string
  trust_score: number
  created_at: string
  updated_at: string
}

export interface User {
  id: string
  organization_id: string
  email: string
  name: string
  avatar_url: string
  role: 'admin' | 'manager' | 'member' | 'viewer'
  created_at: string
}

export interface APIKey {
  id: string
  agent_id: string
  name: string
  prefix: string
  last_used_at?: string
  expires_at?: string
  is_active: boolean
  created_at: string
}

export type TagCategory = 'resource_type' | 'environment' | 'agent_type' | 'data_classification' | 'custom'

export interface Tag {
  id: string
  organization_id: string
  key: string
  value: string
  category: TagCategory
  description: string
  color: string
  created_at: string
  created_by: string
}

export interface CreateTagInput {
  key: string
  value: string
  category: TagCategory
  description?: string
  color?: string
}

export interface AddTagsInput {
  tag_ids: string[]
}

export interface AgentCapability {
  id: string
  agentId: string
  capabilityType: string
  capabilityScope?: Record<string, any>
  grantedBy?: string
  grantedAt: string
  revokedAt?: string
  createdAt: string
  updatedAt: string
}

export interface SDKToken {
  id: string
  userId: string
  organizationId: string
  tokenId: string
  deviceName?: string
  deviceFingerprint?: string
  ipAddress?: string
  userAgent?: string
  lastUsedAt?: string
  lastIpAddress?: string
  usageCount: number
  createdAt: string
  expiresAt: string
  revokedAt?: string
  revokeReason?: string
  metadata?: Record<string, any>
}

class APIClient {
  private baseURL: string
  private token: string | null = null

  constructor(baseURL: string) {
    this.baseURL = baseURL
  }

  setToken(token: string) {
    this.token = token
    if (typeof window !== 'undefined') {
      localStorage.setItem('aim_token', token)
    }
  }

  getToken(): string | null {
    if (this.token) return this.token
    if (typeof window !== 'undefined') {
      return localStorage.getItem('aim_token')
    }
    return null
  }

  clearToken() {
    this.token = null
    if (typeof window !== 'undefined') {
      localStorage.removeItem('aim_token')
    }
  }

  private async request<T>(
    endpoint: string,
    options: RequestInit = {}
  ): Promise<T> {
    const token = this.getToken()
    const headers: Record<string, string> = {
      'Content-Type': 'application/json',
      ...(options.headers as Record<string, string>),
    }

    if (token) {
      headers['Authorization'] = `Bearer ${token}`
    }

    const response = await fetch(`${this.baseURL}${endpoint}`, {
      ...options,
      headers,
      credentials: 'include', // Send cookies with requests
    })

    if (!response.ok) {
      const error = await response.json().catch(() => ({ message: 'Request failed' }))
      throw new Error(error.message || `HTTP ${response.status}`)
    }

    return response.json()
  }

  // Auth
  async login(provider: string): Promise<{ redirect_url: string }> {
    return this.request(`/api/v1/auth/login/${provider}`)
  }

  async getCurrentUser(): Promise<User> {
    return this.request('/api/v1/auth/me')
  }

  async logout(): Promise<void> {
    await this.request('/api/v1/auth/logout', { method: 'POST' })
    this.clearToken()
  }

  // Agents
  async listAgents(): Promise<{ agents: Agent[] }> {
    return this.request('/api/v1/agents')
  }

  async createAgent(data: Partial<Agent>): Promise<Agent> {
    return this.request('/api/v1/agents', {
      method: 'POST',
      body: JSON.stringify(data),
    })
  }

  async getAgent(id: string): Promise<Agent> {
    return this.request(`/api/v1/agents/${id}`)
  }

  async updateAgent(id: string, data: Partial<Agent>): Promise<Agent> {
    return this.request(`/api/v1/agents/${id}`, {
      method: 'PUT',
      body: JSON.stringify(data),
    })
  }

  async deleteAgent(id: string): Promise<void> {
    return this.request(`/api/v1/agents/${id}`, { method: 'DELETE' })
  }

  async verifyAgent(id: string): Promise<{ verified: boolean }> {
    return this.request(`/api/v1/agents/${id}/verify`, { method: 'POST' })
  }

  // API Keys
  async listAPIKeys(): Promise<{ api_keys: APIKey[] }> {
    return this.request('/api/v1/api-keys')
  }

  async createAPIKey(agentId: string, name: string): Promise<{ api_key: string; id: string }> {
    return this.request('/api/v1/api-keys', {
      method: 'POST',
      body: JSON.stringify({ agent_id: agentId, name }),
    })
  }

  async revokeAPIKey(id: string): Promise<void> {
    return this.request(`/api/v1/api-keys/${id}`, { method: 'DELETE' })
  }

  // Trust Score
  async getTrustScore(agentId: string): Promise<{ trust_score: number }> {
    return this.request(`/api/v1/trust-score/agents/${agentId}`)
  }

  // User management
  async getUsers(limit = 100, offset = 0): Promise<any[]> {
    const response = await this.request(`/api/v1/admin/users?limit=${limit}&offset=${offset}`)
    return response.users || []
  }

  async updateUserRole(userId: string, role: string): Promise<void> {
    return this.request(`/api/v1/admin/users/${userId}/role`, {
      method: 'PUT',
      body: JSON.stringify({ role })
    })
  }

  async approveUser(userId: string): Promise<void> {
    return this.request(`/api/v1/admin/users/${userId}/approve`, {
      method: 'POST'
    })
  }

  async rejectUser(userId: string, reason?: string): Promise<void> {
    return this.request(`/api/v1/admin/users/${userId}/reject`, {
      method: 'POST',
      body: JSON.stringify({ reason: reason || '' })
    })
  }

  // Organization settings
  async getOrganizationSettings(): Promise<{ auto_approve_sso: boolean }> {
    return this.request('/api/v1/admin/organization/settings')
  }

  async updateOrganizationSettings(autoApproveSSO: boolean): Promise<void> {
    return this.request('/api/v1/admin/organization/settings', {
      method: 'PUT',
      body: JSON.stringify({ auto_approve_sso: autoApproveSSO })
    })
  }

  // Audit logs
  async getAuditLogs(limit = 100, offset = 0): Promise<any[]> {
    const response: any = await this.request(`/api/v1/admin/audit-logs?limit=${limit}&offset=${offset}`)
    return response.logs || []
  }

  // Alerts
  async getAlerts(limit = 100, offset = 0): Promise<any[]> {
    const response: any = await this.request(`/api/v1/admin/alerts?limit=${limit}&offset=${offset}`)
    return response.alerts || []
  }

  async acknowledgeAlert(alertId: string): Promise<void> {
    return this.request(`/api/v1/admin/alerts/${alertId}/acknowledge`, {
      method: 'POST'
    })
  }

  async getUnacknowledgedAlertCount(): Promise<number> {
    const alerts = await this.getAlerts(100, 0)
    return alerts.filter((a: any) => !a.is_acknowledged).length
  }

  // Dashboard stats - Viewer-accessible analytics endpoint
  async getDashboardStats(): Promise<{
    // Agent metrics
    total_agents: number
    verified_agents: number
    pending_agents: number
    verification_rate: number
    avg_trust_score: number

    // MCP Server metrics
    total_mcp_servers: number
    active_mcp_servers: number

    // User metrics
    total_users: number
    active_users: number

    // Security metrics
    active_alerts: number
    critical_alerts: number
    security_incidents: number

    // Verification metrics (last 24 hours)
    total_verifications?: number
    successful_verifications?: number
    failed_verifications?: number
    avg_response_time?: number

    // Organization
    organization_id: string
  }> {
    return this.request('/api/v1/analytics/dashboard')
  }

  // Verifications
  async listVerifications(limit = 100, offset = 0): Promise<{
    verifications: Array<{
      id: string
      agent_id: string
      agent_name: string
      action: string
      status: 'approved' | 'denied' | 'pending'
      duration_ms: number
      timestamp: string
      metadata: any
    }>
    total: number
  }> {
    return this.request(`/api/v1/verifications?limit=${limit}&offset=${offset}`)
  }

  async getVerificationDetails(id: string): Promise<any> {
    return this.request(`/api/v1/verifications/${id}`)
  }

  async approveVerification(id: string): Promise<any> {
    return this.request(`/api/v1/verifications/${id}/approve`, {
      method: 'POST'
    })
  }

  async denyVerification(id: string): Promise<any> {
    return this.request(`/api/v1/verifications/${id}/deny`, {
      method: 'POST'
    })
  }

  // Security
  async getSecurityThreats(limit = 100, offset = 0): Promise<{
    threats: Array<{
      id: string
      agent_id: string
      threat_type: string
      severity: 'low' | 'medium' | 'high' | 'critical'
      description: string
      status: 'active' | 'mitigated' | 'resolved'
      detected_at: string
    }>
    total: number
  }> {
    return this.request(`/api/v1/security/threats?limit=${limit}&offset=${offset}`)
  }

  async getSecurityAnomalies(limit = 100, offset = 0): Promise<{
    anomalies: Array<{
      id: string
      agent_id: string
      anomaly_type: string
      severity: string
      description: string
      detected_at: string
    }>
    total: number
  }> {
    return this.request(`/api/v1/security/anomalies?limit=${limit}&offset=${offset}`)
  }

  async getSecurityIncidents(limit = 100, offset = 0): Promise<{
    incidents: Array<{
      id: string
      title: string
      severity: 'low' | 'medium' | 'high' | 'critical'
      status: 'open' | 'investigating' | 'resolved'
      created_at: string
    }>
    total: number
  }> {
    return this.request(`/api/v1/security/incidents?limit=${limit}&offset=${offset}`)
  }

  async getSecurityMetrics(): Promise<{
    total_threats: number
    active_threats: number
    total_anomalies: number
    total_incidents: number
    threat_trend: Array<{ date: string; count: number }>
    severity_distribution: Array<{ severity: string; count: number }>
  }> {
    return this.request('/api/v1/security/metrics')
  }

  // MCP Servers
  async listMCPServers(limit = 100, offset = 0): Promise<{
    mcp_servers: Array<{
      id: string
      name: string
      url: string
      status: 'active' | 'inactive' | 'pending'
      verification_status: 'verified' | 'unverified' | 'failed'
      last_verified_at?: string
      created_at: string
    }>
    total: number
  }> {
    return this.request(`/api/v1/mcp-servers?limit=${limit}&offset=${offset}`)
  }

  async createMCPServer(data: {
    name: string
    url: string
    description?: string
  }): Promise<any> {
    return this.request('/api/v1/mcp-servers', {
      method: 'POST',
      body: JSON.stringify(data),
    })
  }

  async getMCPServer(id: string): Promise<any> {
    return this.request(`/api/v1/mcp-servers/${id}`)
  }

  async updateMCPServer(id: string, data: any): Promise<any> {
    return this.request(`/api/v1/mcp-servers/${id}`, {
      method: 'PUT',
      body: JSON.stringify(data),
    })
  }

  async deleteMCPServer(id: string): Promise<void> {
    return this.request(`/api/v1/mcp-servers/${id}`, { method: 'DELETE' })
  }

  async verifyMCPServer(id: string): Promise<{ verified: boolean }> {
    return this.request(`/api/v1/mcp-servers/${id}/verify`, { method: 'POST' })
  }

  // Verification Events (Real-time Monitoring)
  async getRecentVerificationEvents(minutes = 15): Promise<{
    events: Array<{
      id: string
      agentId: string
      agentName: string
      protocol: string
      verificationType: string
      status: string
      confidence: number
      trustScore: number
      durationMs: number
      initiatorType: string
      startedAt: string
      completedAt: string | null
      createdAt: string
    }>
  }> {
    return this.request(`/api/v1/verification-events/recent?minutes=${minutes}`)
  }

  async getVerificationStatistics(period: '24h' | '7d' | '30d' = '24h'): Promise<{
    totalVerifications: number
    successCount: number
    failedCount: number
    pendingCount: number
    timeoutCount: number
    successRate: number
    avgDurationMs: number
    avgConfidence: number
    avgTrustScore: number
    verificationsPerMinute: number
    uniqueAgentsVerified: number
    protocolDistribution: { [key: string]: number }
    typeDistribution: { [key: string]: number }
    initiatorDistribution: { [key: string]: number }
  }> {
    return this.request(`/api/v1/verification-events/statistics?period=${period}`)
  }

  // OAuth / SSO Registration
  async listPendingRegistrations(limit = 50, offset = 0): Promise<{
    requests: Array<{
      id: string
      email: string
      firstName: string
      lastName: string
      oauthProvider: 'google' | 'microsoft' | 'okta'
      oauthUserId: string
      status: 'pending' | 'approved' | 'rejected'
      requestedAt: string
      reviewedAt?: string
      reviewedBy?: string
      rejectionReason?: string
      profilePictureUrl?: string
      oauthEmailVerified: boolean
    }>
    total: number
    limit: number
    offset: number
  }> {
    return this.request(`/api/v1/admin/registration-requests?limit=${limit}&offset=${offset}`)
  }

  async approveRegistration(id: string): Promise<{
    message: string
    user: {
      id: string
      email: string
      role: string
      status: string
    }
  }> {
    return this.request(`/api/v1/admin/registration-requests/${id}/approve`, {
      method: 'POST'
    })
  }

  async rejectRegistration(id: string, reason: string): Promise<{
    message: string
  }> {
    return this.request(`/api/v1/admin/registration-requests/${id}/reject`, {
      method: 'POST',
      body: JSON.stringify({ reason })
    })
  }

  // Tags
  async listTags(category?: TagCategory): Promise<Tag[]> {
    const url = category
      ? `/api/v1/tags?category=${category}`
      : '/api/v1/tags'
    return this.request(url)
  }

  async createTag(data: CreateTagInput): Promise<Tag> {
    return this.request('/api/v1/tags', {
      method: 'POST',
      body: JSON.stringify(data),
    })
  }

  async deleteTag(id: string): Promise<void> {
    return this.request(`/api/v1/tags/${id}`, { method: 'DELETE' })
  }

  // Agent Tags
  async getAgentTags(agentId: string): Promise<Tag[]> {
    return this.request(`/api/v1/agents/${agentId}/tags`)
  }

  async addTagsToAgent(agentId: string, tagIds: string[]): Promise<void> {
    return this.request(`/api/v1/agents/${agentId}/tags`, {
      method: 'POST',
      body: JSON.stringify({ tag_ids: tagIds }),
    })
  }

  async removeTagFromAgent(agentId: string, tagId: string): Promise<void> {
    return this.request(`/api/v1/agents/${agentId}/tags/${tagId}`, {
      method: 'DELETE',
    })
  }

  async suggestTagsForAgent(agentId: string): Promise<Tag[]> {
    return this.request(`/api/v1/agents/${agentId}/tags/suggestions`)
  }

  // Agent Capabilities
  async getAgentCapabilities(agentId: string, activeOnly: boolean = true): Promise<AgentCapability[]> {
    return this.request(`/api/v1/agents/${agentId}/capabilities?activeOnly=${activeOnly}`)
  }

  // MCP Server Tags
  async getMCPServerTags(mcpServerId: string): Promise<Tag[]> {
    return this.request(`/api/v1/mcp-servers/${mcpServerId}/tags`)
  }

  async addTagsToMCPServer(mcpServerId: string, tagIds: string[]): Promise<void> {
    return this.request(`/api/v1/mcp-servers/${mcpServerId}/tags`, {
      method: 'POST',
      body: JSON.stringify({ tag_ids: tagIds }),
    })
  }

  async removeTagFromMCPServer(mcpServerId: string, tagId: string): Promise<void> {
    return this.request(`/api/v1/mcp-servers/${mcpServerId}/tags/${tagId}`, {
      method: 'DELETE',
    })
  }

  async suggestTagsForMCPServer(mcpServerId: string): Promise<Tag[]> {
    return this.request(`/api/v1/mcp-servers/${mcpServerId}/tags/suggestions`)
  }

  // SDK Tokens
  async listSDKTokens(includeRevoked = false): Promise<{ tokens: SDKToken[] }> {
    return this.request(`/api/v1/users/me/sdk-tokens?include_revoked=${includeRevoked}`)
  }

  async getActiveSDKTokenCount(): Promise<{ count: number }> {
    return this.request('/api/v1/users/me/sdk-tokens/count')
  }

  async revokeSDKToken(tokenId: string, reason: string): Promise<void> {
    return this.request(`/api/v1/users/me/sdk-tokens/${tokenId}/revoke`, {
      method: 'POST',
      body: JSON.stringify({ reason })
    })
  }

  async revokeAllSDKTokens(reason: string): Promise<void> {
    return this.request('/api/v1/users/me/sdk-tokens/revoke-all', {
      method: 'POST',
      body: JSON.stringify({ reason })
    })
  }

  // SDK Download
  async downloadSDK(): Promise<Blob> {
    const response = await fetch(`${this.baseURL}/api/v1/sdk/download`, {
      method: 'GET',
      headers: {
        Authorization: `Bearer ${this.getToken()}`,
      },
    })

    if (!response.ok) {
      const error = await response.json()
      throw new Error(error.error || 'Failed to download SDK')
    }

    return response.blob()
  }
}

export const api = new APIClient(API_URL)
