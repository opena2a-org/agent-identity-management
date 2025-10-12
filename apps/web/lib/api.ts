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
  talks_to?: string[]
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
  agent_name?: string  // Optional - may be included by backend in some responses
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

// MCP Detection Types
export type DetectionMethod = 'manual' | 'claude_config' | 'sdk_import' | 'sdk_runtime' | 'direct_api' | 'sdk_integration'

export interface DetectionEvent {
  mcpServer: string
  detectionMethod: DetectionMethod
  confidence: number
  details?: Record<string, any>
  sdkVersion?: string
  timestamp: string
}

export interface DetectionReportRequest {
  detections: DetectionEvent[]
}

export interface DetectionReportResponse {
  success: boolean
  detectionsProcessed: number
  newMCPs: string[]
  existingMCPs: string[]
  message: string
}

export interface DetectedMCPSummary {
  name: string
  confidenceScore: number
  detectedBy: DetectionMethod[]
  firstDetected: string
  lastSeen: string
}

export interface DetectionStatusResponse {
  agentId: string
  sdkVersion?: string
  sdkInstalled: boolean
  autoDetectEnabled: boolean
  detectedMCPs: DetectedMCPSummary[]
  lastReportedAt?: string
}

class APIClient {
  private baseURL: string
  private token: string | null = null
  private refreshToken: string | null = null

  constructor(baseURL: string) {
    this.baseURL = baseURL
  }

  setToken(token: string, refreshToken?: string) {
    this.token = token
    if (typeof window !== 'undefined') {
      localStorage.setItem('auth_token', token)
      if (refreshToken) {
        this.refreshToken = refreshToken
        localStorage.setItem('refresh_token', refreshToken)
      }
    }
  }

  getToken(): string | null {
    if (this.token) return this.token
    if (typeof window !== 'undefined') {
      return localStorage.getItem('auth_token')
    }
    return null
  }

  getRefreshToken(): string | null {
    if (this.refreshToken) return this.refreshToken
    if (typeof window !== 'undefined') {
      return localStorage.getItem('refresh_token')
    }
    return null
  }

  clearToken() {
    this.token = null
    this.refreshToken = null
    if (typeof window !== 'undefined') {
      localStorage.removeItem('auth_token')
      localStorage.removeItem('refresh_token')
    }
  }

  // Refresh access token using refresh token
  async refreshAccessToken(): Promise<{ access_token: string; refresh_token: string } | null> {
    const refreshToken = this.getRefreshToken()
    if (!refreshToken) {
      return null
    }

    try {
      const response = await fetch(`${this.baseURL}/api/v1/auth/refresh`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        credentials: 'include',
        body: JSON.stringify({ refresh_token: refreshToken }),
      })

      if (!response.ok) {
        // Refresh token is invalid or expired
        this.clearToken()
        return null
      }

      const data = await response.json()
      // Store new tokens (token rotation - old refresh token is now invalid)
      this.setToken(data.access_token, data.refresh_token)
      return data
    } catch (error) {
      // Network error or other issue
      this.clearToken()
      return null
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

    // Handle 204 No Content responses (e.g., DELETE operations)
    if (response.status === 204) {
      return undefined as T
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

  // Disable API key (sets is_active=false)
  async disableAPIKey(id: string): Promise<void> {
    return this.request(`/api/v1/api-keys/${id}/disable`, { method: 'PATCH' })
  }

  // Delete API key (only works if already disabled)
  async deleteAPIKey(id: string): Promise<void> {
    return this.request(`/api/v1/api-keys/${id}`, { method: 'DELETE' })
  }

  // Trust Score
  async getTrustScore(agentId: string): Promise<{ trust_score: number }> {
    return this.request(`/api/v1/trust-score/agents/${agentId}`)
  }

  // User management
  async getUsers(limit = 100, offset = 0): Promise<any[]> {
    const response = await this.request<{ users: any[] }>(`/api/v1/admin/users?limit=${limit}&offset=${offset}`)
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

  async approveDrift(alertId: string, approvedMcpServers: string[]): Promise<{ message: string }> {
    return this.request(`/api/v1/admin/alerts/${alertId}/approve-drift`, {
      method: 'POST',
      body: JSON.stringify({ approvedMcpServers })
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
      target_id: string
      target_name?: string
      threat_type: string
      severity: 'low' | 'medium' | 'high' | 'critical'
      title?: string
      description: string
      source?: string
      target_type?: string
      is_blocked: boolean
      created_at: string
      resolved_at?: string
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

  async getMCPServerCapabilities(id: string): Promise<{
    capabilities: Array<{
      id: string
      mcp_server_id: string
      name: string
      type: 'tool' | 'resource' | 'prompt'
      description: string
      schema: any
      detected_at: string
      last_verified_at?: string
      is_active: boolean
    }>
    total: number
    tools: any[]
    resources: any[]
    prompts: any[]
    counts: {
      tools: number
      resources: number
      prompts: number
    }
  }> {
    return this.request(`/api/v1/mcp-servers/${id}/capabilities`)
  }

  async getMCPServerAgents(id: string): Promise<{
    agents: Array<{
      id: string
      name: string
      display_name: string
      agent_type: string
      status: string
    }>
    total: number
  }> {
    return this.request(`/api/v1/mcp-servers/${id}/agents`)
  }

  // ========================================
  // Agent-MCP Relationship Management
  // ========================================

  // Get MCP servers an agent talks to
  async getAgentMCPServers(agentId: string): Promise<{
    agent_id: string
    agent_name: string
    talks_to: string[]
    total: number
  }> {
    return this.request(`/api/v1/agents/${agentId}/mcp-servers`)
  }

  // Add MCP servers to agent's talks_to list
  async addMCPServersToAgent(
    agentId: string,
    data: {
      mcp_server_ids: string[]
      detected_method?: string
      confidence?: number
      metadata?: Record<string, any>
    }
  ): Promise<{
    message: string
    talks_to: string[]
    added_servers: string[]
    total_count: number
  }> {
    return this.request(`/api/v1/agents/${agentId}/mcp-servers`, {
      method: 'PUT',
      body: JSON.stringify(data),
    })
  }

  // Remove a single MCP server from agent's talks_to list
  async removeMCPServerFromAgent(
    agentId: string,
    mcpServerId: string
  ): Promise<{
    message: string
    talks_to: string[]
    total_count: number
  }> {
    return this.request(`/api/v1/agents/${agentId}/mcp-servers/${mcpServerId}`, {
      method: 'DELETE',
    })
  }

  // Remove multiple MCP servers from agent's talks_to list (bulk)
  async bulkRemoveMCPServersFromAgent(
    agentId: string,
    mcpServerIds: string[]
  ): Promise<{
    message: string
    talks_to: string[]
    removed_servers: string[]
    total_count: number
  }> {
    return this.request(`/api/v1/agents/${agentId}/mcp-servers/bulk`, {
      method: 'DELETE',
      body: JSON.stringify({ mcp_server_ids: mcpServerIds }),
    })
  }

  // Auto-detect MCP servers from Claude Desktop config
  async detectAndMapMCPServers(
    agentId: string,
    data: {
      config_path: string
      auto_register?: boolean
      dry_run?: boolean
    }
  ): Promise<{
    detected_servers: Array<{
      name: string
      command: string
      args: string[]
      env?: Record<string, string>
      confidence: number
      source: string
      metadata?: Record<string, any>
    }>
    registered_count: number
    mapped_count: number
    total_talks_to: number
    dry_run: boolean
    errors_encountered?: string[]
  }> {
    return this.request(`/api/v1/agents/${agentId}/mcp-servers/detect`, {
      method: 'POST',
      body: JSON.stringify(data),
    })
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

  // SDK Download with automatic token refresh on 401
  async downloadSDK(sdkType: 'python' | 'go' | 'javascript' = 'python'): Promise<Blob> {
    const attemptDownload = async (token: string | null): Promise<Response> => {
      return fetch(`${this.baseURL}/api/v1/sdk/download?sdk=${sdkType}`, {
        method: 'GET',
        headers: {
          Authorization: `Bearer ${token}`,
        },
      })
    }

    // First attempt with current token
    let response = await attemptDownload(this.getToken())

    // If 401 Unauthorized, try to refresh token and retry
    if (response.status === 401) {
      const refreshed = await this.refreshAccessToken()

      if (!refreshed) {
        // Refresh failed - token is expired and can't be refreshed
        throw new Error('Your session has expired. Please sign in again to download the SDK.')
      }

      // Retry with new token
      response = await attemptDownload(this.getToken())
    }

    if (!response.ok) {
      const error = await response.json().catch(() => ({ error: 'Failed to download SDK' }))
      throw new Error(error.error || 'Failed to download SDK')
    }

    return response.blob()
  }

  // ========================================
  // MCP Detection (Phase 4: SDK + Direct API)
  // ========================================

  // Report MCP detections from SDK or Direct API
  async reportDetection(
    agentId: string,
    data: DetectionReportRequest
  ): Promise<DetectionReportResponse> {
    return this.request(`/api/v1/agents/${agentId}/detection/report`, {
      method: 'POST',
      body: JSON.stringify(data),
    })
  }

  // Get current detection status for an agent
  async getDetectionStatus(agentId: string): Promise<DetectionStatusResponse> {
    return this.request(`/api/v1/detection/agents/${agentId}/status`)
  }

  // ========================================
  // Capability Requests (Admin + User)
  // ========================================

  // List capability requests (admin only)
  async getCapabilityRequests(params?: {
    status?: 'pending' | 'approved' | 'rejected'
    agentId?: string
    limit?: number
    offset?: number
  }): Promise<any[]> {
    const queryParams = new URLSearchParams()
    if (params?.status) queryParams.append('status', params.status)
    if (params?.agentId) queryParams.append('agent_id', params.agentId)
    if (params?.limit) queryParams.append('limit', params.limit.toString())
    if (params?.offset) queryParams.append('offset', params.offset.toString())

    const query = queryParams.toString() ? `?${queryParams.toString()}` : ''
    return this.request(`/api/v1/admin/capability-requests${query}`)
  }

  // Get a single capability request by ID (admin only)
  async getCapabilityRequest(id: string): Promise<any> {
    return this.request(`/api/v1/admin/capability-requests/${id}`)
  }

  // Approve a capability request (admin only)
  async approveCapabilityRequest(id: string): Promise<{ message: string }> {
    return this.request(`/api/v1/admin/capability-requests/${id}/approve`, {
      method: 'POST'
    })
  }

  // Reject a capability request (admin only)
  async rejectCapabilityRequest(id: string): Promise<{ message: string }> {
    return this.request(`/api/v1/admin/capability-requests/${id}/reject`, {
      method: 'POST'
    })
  }

  // Create a capability request (any authenticated user)
  async createCapabilityRequest(data: {
    agent_id: string
    capability_type: string
    reason: string
  }): Promise<any> {
    return this.request('/api/v1/capability-requests', {
      method: 'POST',
      body: JSON.stringify(data)
    })
  }

  // ========================================
  // Security Policies (Admin Only)
  // ========================================

  // List all security policies for the organization
  async getSecurityPolicies(): Promise<any[]> {
    return this.request('/api/v1/admin/security-policies')
  }

  // Get a specific security policy by ID
  async getSecurityPolicy(policyId: string): Promise<any> {
    return this.request(`/api/v1/admin/security-policies/${policyId}`)
  }

  // Create a new security policy
  async createSecurityPolicy(data: {
    name: string
    description?: string
    policy_type: string
    enforcement_action: 'alert_only' | 'block_and_alert' | 'allow'
    severity_threshold: string
    rules?: Record<string, any>
    applies_to: string
    is_enabled: boolean
    priority: number
  }): Promise<any> {
    return this.request('/api/v1/admin/security-policies', {
      method: 'POST',
      body: JSON.stringify(data),
    })
  }

  // Update an existing security policy
  async updateSecurityPolicy(policyId: string, data: {
    name: string
    description?: string
    policy_type: string
    enforcement_action: 'alert_only' | 'block_and_alert' | 'allow'
    severity_threshold: string
    rules?: Record<string, any>
    applies_to: string
    is_enabled: boolean
    priority: number
  }): Promise<any> {
    return this.request(`/api/v1/admin/security-policies/${policyId}`, {
      method: 'PUT',
      body: JSON.stringify(data),
    })
  }

  // Delete a security policy
  async deleteSecurityPolicy(policyId: string): Promise<void> {
    return this.request(`/api/v1/admin/security-policies/${policyId}`, {
      method: 'DELETE',
    })
  }

  // Toggle policy enabled/disabled status
  async toggleSecurityPolicy(policyId: string, isEnabled: boolean): Promise<any> {
    return this.request(`/api/v1/admin/security-policies/${policyId}/toggle`, {
      method: 'PATCH',
      body: JSON.stringify({ isEnabled }),
    })
  }

  // ========================================
  // Compliance (Admin Only)
  // ========================================

  // Get compliance status overview
  async getComplianceStatus(): Promise<{
    compliance_level: string
    total_agents: number
    verified_agents: number
    verification_rate: number // Already in percentage (0-100)
    average_trust_score: number // Already in percentage (0-100)
    recent_audit_count: number
  }> {
    return this.request('/api/v1/compliance/status')
  }

  // Get compliance metrics
  async getComplianceMetrics(): Promise<{
    start_date: string
    end_date: string
    interval: string
    metrics: {
      period: {
        start: string
        end: string
        interval: string
      }
      agent_verification_trend: Array<{
        date: string
        verified: number
      }>
      trust_score_trend: Array<{
        date: string
        avg_score: number // 0-1 scale
      }>
    }
  }> {
    return this.request('/api/v1/compliance/metrics')
  }

  // Export audit log (returns CSV file as blob)
  async exportAuditLog(params?: {
    start_date?: string
    end_date?: string
    entity_type?: string
    action?: string
  }): Promise<Blob> {
    const queryParams = new URLSearchParams()
    if (params?.start_date) queryParams.append('start_date', params.start_date)
    if (params?.end_date) queryParams.append('end_date', params.end_date)
    if (params?.entity_type) queryParams.append('entity_type', params.entity_type)
    if (params?.action) queryParams.append('action', params.action)

    const query = queryParams.toString() ? `?${queryParams.toString()}` : ''
    const token = this.getToken()

    const response = await fetch(`${this.baseURL}/api/v1/compliance/audit-log/export${query}`, {
      method: 'GET',
      headers: {
        Authorization: `Bearer ${token}`,
      },
    })

    if (!response.ok) {
      const error = await response.json().catch(() => ({ error: 'Failed to export audit log' }))
      throw new Error(error.error || 'Failed to export audit log')
    }

    return response.blob()
  }

  // Get access review (users and their permissions)
  async getAccessReview(): Promise<{
    users: Array<{
      id: string
      email: string
      name: string
      role: string
      last_login: string
      created_at: string
      status: string
    }>
    total: number
  }> {
    return this.request('/api/v1/compliance/access-review')
  }

  // Run compliance check
  async runComplianceCheck(checkType: string = 'all'): Promise<{
    check_type: string
    passed: number
    failed: number
    total: number
    compliance_rate: number
    checks: Array<{
      name: string
      passed: boolean
    }>
  }> {
    return this.request('/api/v1/compliance/check', {
      method: 'POST',
      body: JSON.stringify({ check_type: checkType }),
    })
  }
}

export const api = new APIClient(API_URL)
