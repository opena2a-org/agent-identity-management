"use client";

// Runtime API URL configuration
// CRITICAL: This function MUST be called ONLY in browser context (client-side)
// to ensure proper URL detection for environment-agnostic deployments
const getApiUrl = (): string => {
  // Defense: If somehow called during SSR, throw clear error
  if (typeof window === "undefined") {
    throw new Error(
      "getApiUrl() MUST be called in browser context only. Check your component for SSR issues."
    );
  }

  // 1. Check for runtime config (set by server via script injection)
  if ((window as any).__RUNTIME_CONFIG__?.apiUrl) {
    console.log(
      "[API] Using runtime config URL:",
      (window as any).__RUNTIME_CONFIG__.apiUrl
    );
    return (window as any).__RUNTIME_CONFIG__.apiUrl;
  }

  // 2. Auto-detect from window location (PRIMARY method for environment-agnostic deployment)
  // IMPORTANT: Do this BEFORE checking process.env because Next.js bakes env vars at build time
  const { protocol, hostname } = window.location;

  // Match both 'aim-frontend' and 'aim-dev-frontend' or any variant with '-frontend'
  if (hostname.includes("-frontend")) {
    const backendHost = hostname.replace("-frontend", "-backend");
    const detectedUrl = `${protocol}//${backendHost}`;
    console.log("[API] Auto-detected URL from hostname:", detectedUrl);
    return detectedUrl;
  }

  // 3. Check for NEXT_PUBLIC_API_URL environment variable
  if (process.env.NEXT_PUBLIC_API_URL) {
    console.log(
      "[API] Using NEXT_PUBLIC_API_URL:",
      process.env.NEXT_PUBLIC_API_URL
    );
    return process.env.NEXT_PUBLIC_API_URL;
  }

  // 4. Fallback to localhost for local development
  console.log("[API] Using localhost fallback (local development)");
  return "http://localhost:8080";
};

export interface Agent {
  id: string;
  organization_id: string;
  name: string;
  display_name: string;
  description: string;
  agent_type: "ai_agent" | "mcp_server";
  status: "pending" | "verified" | "suspended" | "revoked";
  version: string;
  trust_score: number;
  talks_to?: string[];
  created_at: string;
  updated_at: string;
}

export interface Organization {
  id: string;
  name: string;
  plan: "community" | "pro" | "enterprise";
  max_agents: number;
  is_active: boolean;
  created_at: string;
  updated_at: string;
}

export interface User {
  id: string;
  organization_id?: string;
  organization_name?: string;
  email: string;
  name: string;
  avatar_url?: string;
  role: "admin" | "manager" | "member" | "viewer" | "pending";
  status: "active" | "pending_approval" | "suspended" | "deactivated";
  force_password_change?: boolean;
  created_at: string;
  provider?: string;
  last_login_at?: string;
  requested_at?: string;
  picture_url?: string;
  is_registration_request?: boolean;
}

export interface APIKey {
  id: string;
  agent_id: string;
  name: string;
  prefix: string;
  last_used_at?: string;
  expires_at?: string;
  is_active: boolean;
  created_at: string;
  agent_name?: string; // Optional - may be included by backend in some responses
}

export type TagCategory =
  | "resource_type"
  | "environment"
  | "agent_type"
  | "data_classification"
  | "custom";

export interface Tag {
  id: string;
  organization_id: string;
  key: string;
  value: string;
  category: TagCategory;
  description: string;
  color: string;
  created_at: string;
  created_by: string;
}

export interface CreateTagInput {
  key: string;
  value: string;
  category: TagCategory;
  description?: string;
  color?: string;
}

export interface AddTagsInput {
  tag_ids: string[];
}

export interface AgentCapability {
  id: string;
  agentId: string;
  capabilityType: string;
  capabilityScope?: Record<string, any>;
  grantedBy?: string;
  grantedAt: string;
  revokedAt?: string;
  createdAt: string;
  updatedAt: string;
}

export interface SDKToken {
  id: string;
  userId: string;
  organizationId: string;
  tokenId: string;
  deviceName?: string;
  deviceFingerprint?: string;
  ipAddress?: string;
  userAgent?: string;
  lastUsedAt?: string;
  lastIpAddress?: string;
  usageCount: number;
  createdAt: string;
  expiresAt: string;
  revokedAt?: string;
  revokeReason?: string;
  metadata?: Record<string, any>;
}

// MCP Detection Types
export type DetectionMethod =
  | "manual"
  | "claude_config"
  | "sdk_import"
  | "sdk_runtime"
  | "direct_api"
  | "sdk_integration";

export interface DetectionEvent {
  mcpServer: string;
  detectionMethod: DetectionMethod;
  confidence: number;
  details?: Record<string, any>;
  sdkVersion?: string;
  timestamp: string;
}

export interface DetectionReportRequest {
  detections: DetectionEvent[];
}

export interface DetectionReportResponse {
  success: boolean;
  detectionsProcessed: number;
  newMCPs: string[];
  existingMCPs: string[];
  message: string;
}

export interface DetectedMCPSummary {
  name: string;
  confidenceScore: number;
  detectedBy: DetectionMethod[];
  firstDetected: string;
  lastSeen: string;
}

export interface DetectionStatusResponse {
  agentId: string;
  sdkVersion?: string;
  sdkInstalled: boolean;
  autoDetectEnabled: boolean;
  protocol?: string; // SDK-detected protocol: "mcp", "a2a", "oauth", etc.
  detectedMCPs: DetectedMCPSummary[];
  lastReportedAt?: string;
}

// ✅ Agent Attestation Types (Phase 5: Revolutionary Zero-Effort MCP Verification)
export interface AttestationPayload {
  agent_id: string;
  mcp_url: string;
  mcp_name: string;
  capabilities_found: string[];
  connection_successful: boolean;
  health_check_passed: boolean;
  connection_latency_ms: number;
  timestamp: string; // ISO 8601 timestamp
  sdk_version: string;
}

export interface AttestMCPRequest {
  attestation: AttestationPayload;
  signature: string; // Ed25519 signature (base64)
}

export interface AttestMCPResponse {
  success: boolean;
  attestation_id: string;
  mcp_confidence_score: number; // 0-100
  attestation_count: number;
  message: string;
}

export interface AttestationWithAgentDetails {
  id: string;
  agent_id: string;
  agent_name: string;
  agent_trust_score: number;
  verified_at: string; // ISO 8601 timestamp
  expires_at: string; // ISO 8601 timestamp
  capabilities_confirmed: string[];
  connection_latency_ms: number;
  health_check_passed: boolean;
  is_valid: boolean;
}

export interface GetMCPAttestationsResponse {
  attestations: AttestationWithAgentDetails[];
  total: number;
  confidence_score: number; // 0-100
  last_attested_at: string; // ISO 8601 timestamp
}

export interface ConnectedAgent {
  id: string;
  name: string;
  display_name: string;
  trust_score: number;
  status: string;
  last_attested_at?: string;
  attestation_count: number;
}

export interface GetConnectedAgentsResponse {
  agents: ConnectedAgent[];
  total: number;
}

export interface ConnectedMCPServer {
  id: string;
  organization_id?: string;
  name: string;
  description?: string;
  url: string;
  version?: string;
  public_key?: string;
  status?: string;
  is_verified?: boolean;
  last_verified_at?: string;
  verification_url?: string;
  capabilities?: string[];
  trust_score?: number;
  registered_by_agent?: string | null;
  created_by?: string;
  created_at?: string;
  updated_at?: string;
  tags?: string[] | null;
  verification_method: string;
  attestation_count: number;
  confidence_score: number;
  last_attested_at?: string;
}

export interface GetAgentMCPServersResponse {
  mcp_servers: ConnectedMCPServer[];
  total: number;
}

class APIClient {
  private token: string | null = null;
  private refreshToken: string | null = null;
  private _baseURL: string | null = null;

  constructor() {
    // Constructor does nothing - baseURL is lazily initialized on first use
  }

  // Lazy getter that initializes baseURL only once, on first access (client-side only)
  private get baseURL(): string {
    if (!this._baseURL) {
      this._baseURL = getApiUrl(); // Will throw if called during SSR
    }
    return this._baseURL;
  }

  setToken(token: string, refreshToken?: string) {
    this.token = token;
    if (typeof window !== "undefined") {
      localStorage.setItem("auth_token", token);
      if (refreshToken) {
        this.refreshToken = refreshToken;
        localStorage.setItem("refresh_token", refreshToken);
      }
    }
  }

  getToken(): string | null {
    if (this.token) return this.token;
    if (typeof window !== "undefined") {
      return localStorage.getItem("auth_token");
    }
    return null;
  }

  getRefreshToken(): string | null {
    if (this.refreshToken) return this.refreshToken;
    if (typeof window !== "undefined") {
      return localStorage.getItem("refresh_token");
    }
    return null;
  }

  clearToken() {
    this.token = null;
    this.refreshToken = null;
    if (typeof window !== "undefined") {
      localStorage.removeItem("auth_token");
      localStorage.removeItem("refresh_token");
    }
  }

  // Refresh access token using refresh token
  async refreshAccessToken(): Promise<{
    access_token: string;
    refresh_token: string;
  } | null> {
    const refreshToken = this.getRefreshToken();
    if (!refreshToken) {
      return null;
    }

    try {
      const response = await fetch(`${this.baseURL}/api/v1/auth/refresh`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        credentials: "include",
        body: JSON.stringify({ refresh_token: refreshToken }),
      });

      if (!response.ok) {
        // Refresh token is invalid or expired
        this.clearToken();
        return null;
      }

      const data = await response.json();
      // Store new tokens (token rotation - old refresh token is now invalid)
      this.setToken(data?.access_token, data?.refresh_token);
      return data;
    } catch (error) {
      // Network error or other issue
      this.clearToken();
      return null;
    }
  }

  private async request<T>(
    endpoint: string,
    options: RequestInit = {}
  ): Promise<T> {
    const token = this.getToken();
    const headers: Record<string, string> = {
      "Content-Type": "application/json",
      ...(options.headers as Record<string, string>),
    };

    if (token) {
      headers["Authorization"] = `Bearer ${token}`;
    }

    const response = await fetch(`${this.baseURL}${endpoint}`, {
      ...options,
      headers,
      credentials: "include", // Send cookies with requests
    });

    if (!response.ok) {
      const error = await response
        .json()
        .catch(() => ({ message: "Request failed" }));

      // Backend can return either 'error' or 'message' field
      const errorMessage =
        error?.error || error?.message || `HTTP ${response.status}`;
      throw new Error(errorMessage);
    }

    // Handle 204 No Content responses (e.g., DELETE operations)
    if (response.status === 204) {
      return undefined as T;
    }

    // Check if response has content before parsing JSON
    const contentType = response.headers.get("content-type");
    if (contentType && contentType.includes("application/json")) {
      try {
        return await response.json();
      } catch (err) {
        // JSON parsing failed, but response was successful
        console.warn("Failed to parse JSON response:", err);
        return undefined as T;
      }
    }

    // No JSON content, return undefined
    return undefined as T;
  }

  // Auth
  async login(provider: string): Promise<{ redirect_url: string }> {
    return this.request(`/api/v1/oauth/${provider}/login`);
  }

  async getCurrentUser(): Promise<User> {
    return this.request("/api/v1/auth/me");
  }

  async getCurrentOrganization(): Promise<Organization> {
    return this.request("/api/v1/organizations/current");
  }

  async logout(): Promise<void> {
    await this.request("/api/v1/auth/logout", { method: "POST" });
    this.clearToken();
  }

  async changePassword(data: {
    email: string;
    currentPassword: string;
    newPassword: string;
  }): Promise<{
    success: boolean;
    user?: User;
    accessToken?: string;
    refreshToken?: string;
    message?: string;
  }> {
    // Use public endpoint for forced password changes (no auth required)
    // Backend expects: email, oldPassword, newPassword
    const payload = {
      email: data.email,
      oldPassword: data.currentPassword,
      newPassword: data.newPassword,
    };

    const response = await fetch(
      `${this.baseURL}/api/v1/public/change-password`,
      {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        credentials: "include",
        body: JSON.stringify(payload),
      }
    );

    if (!response.ok) {
      const error = await response.json();
      throw new Error(error.error || "Failed to change password");
    }

    const data_response = await response.json();

    // Store tokens if password change was successful
    if (data_response.success && data_response.accessToken) {
      this.setToken(data_response.accessToken, data_response.refreshToken);
    }

    return data_response;
  }

  // Public Registration & Login (Email/Password)
  async register(data: {
    email: string;
    firstName: string;
    lastName: string;
    password: string;
    provider: string;
  }): Promise<{
    success: boolean;
    message: string;
    requestId: string;
  }> {
    const response = await this.request<{
      success: boolean;
      message: string;
      requestId: string;
    }>("/api/v1/public/register", {
      method: "POST",
      body: JSON.stringify(data),
    });
    return response;
  }

  async loginWithPassword(data: { email: string; password: string }): Promise<{
    success: boolean;
    message: string;
    user?: User;
    accessToken?: string;
    refreshToken?: string;
    isApproved: boolean;
  }> {
    const response = await this.request<{
      success: boolean;
      message: string;
      user?: User;
      accessToken?: string;
      refreshToken?: string;
      isApproved: boolean;
    }>("/api/v1/public/login", {
      method: "POST",
      body: JSON.stringify(data),
    });

    // If login successful and user is approved, store tokens
    if (response.success && response.isApproved && response.accessToken) {
      this.setToken(response.accessToken, response.refreshToken);
    }

    return response;
  }

  async checkRegistrationStatus(requestId: string): Promise<{
    status: "pending" | "approved" | "rejected";
    message: string;
  }> {
    return this.request(`/api/v1/public/register/${requestId}/status`);
  }

  async forgotPassword(data: { email: string }): Promise<{
    success: boolean;
    message: string;
  }> {
    return this.request("/api/v1/public/forgot-password", {
      method: "POST",
      body: JSON.stringify(data),
    });
  }

  async resetPassword(data: {
    resetToken: string;
    newPassword: string;
    confirmPassword: string;
  }): Promise<{
    success: boolean;
    message: string;
  }> {
    return this.request("/api/v1/public/reset-password", {
      method: "POST",
      body: JSON.stringify(data),
    });
  }

  // Agents
  async listAgents(): Promise<{ agents: Agent[] }> {
    return this.request("/api/v1/agents");
  }

  async createAgent(data: Partial<Agent>): Promise<Agent> {
    return this.request("/api/v1/agents", {
      method: "POST",
      body: JSON.stringify(data),
    });
  }

  async getAgent(id: string): Promise<Agent> {
    return this.request(`/api/v1/agents/${id}`);
  }

  async updateAgent(id: string, data: Partial<Agent>): Promise<Agent> {
    return this.request(`/api/v1/agents/${id}`, {
      method: "PUT",
      body: JSON.stringify(data),
    });
  }

  async deleteAgent(id: string): Promise<void> {
    return this.request(`/api/v1/agents/${id}`, { method: "DELETE" });
  }

  async verifyAgent(id: string): Promise<{ verified: boolean }> {
    return this.request(`/api/v1/agents/${id}/verify`, { method: "POST" });
  }

  async suspendAgent(
    id: string
  ): Promise<{ success: boolean; message: string }> {
    return this.request(`/api/v1/agents/${id}/suspend`, { method: "POST" });
  }

  async reactivateAgent(
    id: string
  ): Promise<{ success: boolean; message: string }> {
    return this.request(`/api/v1/agents/${id}/reactivate`, { method: "POST" });
  }

  async rotateAgentCredentials(
    id: string
  ): Promise<{ api_key: string; message: string }> {
    return this.request(`/api/v1/agents/${id}/rotate-credentials`, {
      method: "POST",
    });
  }

  async adjustAgentTrustScore(
    id: string,
    score: number,
    reason: string
  ): Promise<{ success: boolean; new_score: number }> {
    return this.request(`/api/v1/agents/${id}/trust-score`, {
      method: "PUT",
      body: JSON.stringify({ score, reason }),
    });
  }

  async getAgentTrustScoreHistory(id: string): Promise<{
    agent_id: string;
    history: Array<{
      timestamp: string;
      trust_score: number;
      reason: string;
      changed_by: string;
    }>;
  }> {
    return this.request(`/api/v1/agents/${id}/trust-score/history`);
  }

  // API Keys
  async listAPIKeys(): Promise<{ api_keys: APIKey[] }> {
    return this.request("/api/v1/api-keys");
  }

  async createAPIKey(
    agentId: string,
    name: string
  ): Promise<{ api_key: string; id: string }> {
    return this.request("/api/v1/api-keys", {
      method: "POST",
      body: JSON.stringify({ agent_id: agentId, name }),
    });
  }

  // Disable API key (sets is_active=false)
  async disableAPIKey(id: string): Promise<void> {
    return this.request(`/api/v1/api-keys/${id}/disable`, { method: "PATCH" });
  }

  // Delete API key (only works if already disabled)
  async deleteAPIKey(id: string): Promise<void> {
    return this.request(`/api/v1/api-keys/${id}`, { method: "DELETE" });
  }

  // Trust Score
  async getTrustScore(agentId: string): Promise<{ trust_score: number }> {
    return this.request(`/api/v1/trust-score/agents/${agentId}`);
  }

  async getTrustScoreBreakdown(agentId: string): Promise<{
    agentId: string;
    agentName: string;
    overall: number;
    factors: {
      verificationStatus: number;
      uptime: number;
      successRate: number;
      securityAlerts: number;
      compliance: number;
      age: number;
      driftDetection: number;
      userFeedback: number;
    };
    weights: {
      verificationStatus: number;
      uptime: number;
      successRate: number;
      securityAlerts: number;
      compliance: number;
      age: number;
      driftDetection: number;
      userFeedback: number;
    };
    contributions: {
      verificationStatus: number;
      uptime: number;
      successRate: number;
      securityAlerts: number;
      compliance: number;
      age: number;
      driftDetection: number;
      userFeedback: number;
    };
    confidence: number;
    calculatedAt: string;
  }> {
    return this.request(`/api/v1/trust-score/agents/${agentId}/breakdown`);
  }

  // User management
  async getUsers(limit = 100, offset = 0): Promise<any[]> {
    const response = await this.request<{ users: any[] }>(
      `/api/v1/admin/users?limit=${limit}&offset=${offset}`
    );
    return response.users || [];
  }

  async updateUserRole(userId: string, role: string): Promise<void> {
    return this.request(`/api/v1/admin/users/${userId}/role`, {
      method: "PUT",
      body: JSON.stringify({ role }),
    });
  }

  async deactivateUser(userId: string): Promise<void> {
    return this.request(`/api/v1/admin/users/${userId}/deactivate`, {
      method: "POST",
    });
  }

  async activateUser(userId: string): Promise<void> {
    return this.request(`/api/v1/admin/users/${userId}/activate`, {
      method: "POST",
    });
  }

  async approveRegistrationRequest(requestId: string): Promise<void> {
    return this.request(
      `/api/v1/admin/registration-requests/${requestId}/approve`,
      {
        method: "POST",
      }
    );
  }

  async rejectRegistrationRequest(requestId: string): Promise<void> {
    return this.request(
      `/api/v1/admin/registration-requests/${requestId}/reject`,
      {
        method: "POST",
      }
    );
  }

  async approveUser(userId: string): Promise<void> {
    return this.request(`/api/v1/admin/users/${userId}/approve`, {
      method: "POST",
    });
  }

  async rejectUser(userId: string, reason?: string): Promise<void> {
    return this.request(`/api/v1/admin/users/${userId}/reject`, {
      method: "POST",
      body: JSON.stringify({ reason: reason || "" }),
    });
  }

  // Audit logs
  async getAuditLogs(limit = 100, offset = 0): Promise<any[]> {
    const response: any = await this.request(
      `/api/v1/admin/audit-logs?limit=${limit}&offset=${offset}`
    );
    return response.logs || [];
  }

  // Alerts
  async getAlerts(limit = 100, offset = 0): Promise<any[]> {
    const response: any = await this.request(
      `/api/v1/admin/alerts?limit=${limit}&offset=${offset}`
    );
    return response.alerts || [];
  }

  async acknowledgeAlert(alertId: string): Promise<void> {
    return this.request(`/api/v1/admin/alerts/${alertId}/acknowledge`, {
      method: "POST",
    });
  }

  async getUnacknowledgedAlertCount(): Promise<number> {
    const alerts = await this.getAlerts(100, 0);
    return alerts.filter((a: any) => !a.is_acknowledged).length;
  }

  async getPendingCapabilityRequestsCount(): Promise<number> {
    try {
      const requests = await this.getCapabilityRequests({ status: "pending" });
      return requests.length;
    } catch (error) {
      console.error("Failed to fetch pending capability requests count:", error);
      return 0;
    }
  }

  // Dashboard stats - Viewer-accessible analytics endpoint
  async getDashboardStats(): Promise<{
    // Agent metrics
    total_agents: number;
    verified_agents: number;
    pending_agents: number;
    verification_rate: number;
    avg_trust_score: number;

    // MCP Server metrics
    total_mcp_servers: number;
    active_mcp_servers: number;

    // User metrics
    total_users: number;
    active_users: number;

    // Security metrics
    active_alerts: number;
    critical_alerts: number;
    security_incidents: number;

    // Verification metrics (last 24 hours)
    total_verifications?: number;
    successful_verifications?: number;
    failed_verifications?: number;
    avg_response_time?: number;

    // Organization
    organization_id: string;
  }> {
    return this.request("/api/v1/analytics/dashboard");
  }

  // Admin Dashboard stats - Admin-only endpoint with comprehensive platform metrics
  async getAdminDashboardStats(): Promise<{
    // Agent metrics
    total_agents: number;
    verified_agents: number;
    pending_agents: number;
    verification_rate: number;
    avg_trust_score: number;

    // MCP Server metrics
    total_mcp_servers: number;
    active_mcp_servers: number;

    // User metrics
    total_users: number;
    active_users: number;

    // Security metrics
    active_alerts: number;
    critical_alerts: number;
    security_incidents: number;

    // Organization
    organization_id: string;
  }> {
    return this.request("/api/v1/admin/dashboard/stats");
  }

  // Verification Activity - Get monthly verification activity data
  async getVerificationActivity(months = 6): Promise<{
    period: string;
    activity: Array<{
      month: string;
      verified: number;
      pending: number;
      month_year: string;
    }>;
    current_stats: {
      total_verified: number;
      total_pending: number;
      total_agents: number;
    };
  }> {
    return this.request(
      `/api/v1/analytics/verification-activity?months=${months}`
    );
  }

  async getUsageStatistics(days = 30): Promise<{
    period: string;
    api_calls: number;
    active_agents: number;
    total_agents: number;
    data_volume: number;
    uptime: number;
    generated_at: string;
  }> {
    return this.request(`/api/v1/analytics/usage?days=${days}`);
  }

  async getTrustScoreTrends(days = 30): Promise<{
    period: string;
    trends: Array<{
      date: string;
      avg_score: number; // ✅ FIXED: Backend returns avg_score, not avg_trust_score
      agent_count: number;
      scores_by_range: {
        excellent: number; // 90-100
        good: number; // 75-89
        fair: number; // 50-74
        poor: number; // 0-49
      };
    }>;
    summary: {
      overall_avg: number;
      trend_direction: "up" | "down" | "stable";
      change_percentage: number;
    };
  }> {
    return this.request(`/api/v1/analytics/trends?days=${days}`);
  }

  async getAgentActivity(limit = 50): Promise<{
    activities: Array<{
      id: string;
      agent_id: string;
      agent_name: string;
      action: string;
      status: "success" | "failure" | "pending";
      timestamp: string;
      details?: string;
    }>;
    summary: {
      total_activities: number;
      success_count: number;
      failure_count: number;
      success_rate: number;
    };
  }> {
    return this.request(`/api/v1/analytics/agents/activity?limit=${limit}`);
  }

  // Webhooks
  async listWebhooks(): Promise<
    Array<{
      id: string;
      organization_id: string;
      name: string;
      url: string;
      events: string[];
      is_active: boolean;
      secret: string;
      created_at: string;
      last_triggered_at?: string;
      success_count: number;
      failure_count: number;
    }>
  > {
    const response = await this.request<{ webhooks: any[] }>(
      "/api/v1/webhooks"
    );
    return response.webhooks || [];
  }

  async createWebhook(data: {
    name: string;
    url: string;
    events: string[];
    secret?: string;
  }): Promise<{
    id: string;
    organization_id: string;
    name: string;
    url: string;
    events: string[];
    is_active: boolean;
    secret: string;
    created_at: string;
  }> {
    return this.request("/api/v1/webhooks", {
      method: "POST",
      body: JSON.stringify(data),
    });
  }

  async getWebhook(id: string): Promise<{
    id: string;
    organization_id: string;
    name: string;
    url: string;
    events: string[];
    is_active: boolean;
    secret: string;
    created_at: string;
    last_triggered_at?: string;
    success_count: number;
    failure_count: number;
    deliveries: Array<{
      id: string;
      event: string;
      status: "success" | "failure";
      response_code: number;
      timestamp: string;
      error_message?: string;
    }>;
  }> {
    return this.request(`/api/v1/webhooks/${id}`);
  }

  async deleteWebhook(id: string): Promise<void> {
    return this.request(`/api/v1/webhooks/${id}`, { method: "DELETE" });
  }

  async updateWebhook(
    id: string,
    data: {
      name?: string;
      url?: string;
      events?: string[];
      is_active?: boolean;
    }
  ): Promise<{
    id: string;
    organization_id: string;
    name: string;
    url: string;
    events: string[];
    is_active: boolean;
    created_at: string;
  }> {
    return this.request(`/api/v1/webhooks/${id}`, {
      method: "PUT",
      body: JSON.stringify(data),
    });
  }

  async testWebhook(id: string): Promise<{
    success: boolean;
    response_code: number;
    message: string;
  }> {
    return this.request(`/api/v1/webhooks/${id}/test`, { method: "POST" });
  }

  // Verifications
  async listVerifications(
    limit = 100,
    offset = 0
  ): Promise<{
    verifications: Array<{
      id: string;
      agent_id: string;
      agent_name: string;
      action: string;
      status: "approved" | "denied" | "pending";
      duration_ms: number;
      timestamp: string;
      metadata: any;
    }>;
    total: number;
  }> {
    return this.request(
      `/api/v1/verifications?limit=${limit}&offset=${offset}`
    );
  }

  async getVerificationDetails(id: string): Promise<any> {
    return this.request(`/api/v1/verifications/${id}`);
  }

  async approveVerification(id: string): Promise<any> {
    return this.request(`/api/v1/verifications/${id}/approve`, {
      method: "POST",
    });
  }

  async denyVerification(id: string): Promise<any> {
    return this.request(`/api/v1/verifications/${id}/deny`, {
      method: "POST",
    });
  }

  // Security
  async getSecurityThreats(
    limit = 100,
    offset = 0
  ): Promise<{
    threats: Array<{
      id: string;
      target_id: string;
      target_name?: string;
      threat_type: string;
      severity: "low" | "medium" | "high" | "critical";
      title?: string;
      description: string;
      source?: string;
      target_type?: string;
      is_blocked: boolean;
      created_at: string;
      resolved_at?: string;
    }>;
    total: number;
  }> {
    return this.request(
      `/api/v1/security/threats?limit=${limit}&offset=${offset}`
    );
  }

  async getSecurityAnomalies(
    limit = 100,
    offset = 0
  ): Promise<{
    anomalies: Array<{
      id: string;
      agent_id: string;
      anomaly_type: string;
      severity: string;
      description: string;
      detected_at: string;
    }>;
    total: number;
  }> {
    return this.request(
      `/api/v1/security/anomalies?limit=${limit}&offset=${offset}`
    );
  }

  async getSecurityMetrics(): Promise<{
    total_threats: number;
    active_threats: number;
    total_anomalies: number;
    total_incidents: number;
    threat_trend: Array<{ date: string; count: number }>;
    severity_distribution: Array<{ severity: string; count: number }>;
  }> {
    return this.request("/api/v1/security/metrics");
  }

  // MCP Servers
  async listMCPServers(
    limit = 100,
    offset = 0
  ): Promise<{
    mcp_servers: Array<{
      id: string;
      name: string;
      url: string;
      status:
        | "active"
        | "inactive"
        | "pending"
        | "verified"
        | "suspended"
        | "revoked";
      is_verified?: boolean;
      last_verified_at?: string;
      created_at: string;
    }>;
    total: number;
  }> {
    return this.request(`/api/v1/mcp-servers?limit=${limit}&offset=${offset}`);
  }

  async createMCPServer(data: {
    name: string;
    url: string;
    description?: string;
  }): Promise<any> {
    return this.request("/api/v1/mcp-servers", {
      method: "POST",
      body: JSON.stringify(data),
    });
  }

  async getMCPServer(id: string): Promise<any> {
    return this.request(`/api/v1/mcp-servers/${id}`);
  }

  async getMCPServerConnectedAgents(id: string): Promise<{
    connected_agents: Array<{
      id: string;
      name: string;
      display_name: string;
      status: string;
      trust_score: number;
      updated_at: string;
    }>;
    count: number;
  }> {
    return this.request(`/api/v1/mcp-servers/${id}/agents`);
  }

  async updateMCPServer(id: string, data: any): Promise<any> {
    return this.request(`/api/v1/mcp-servers/${id}`, {
      method: "PUT",
      body: JSON.stringify(data),
    });
  }

  async deleteMCPServer(id: string): Promise<void> {
    return this.request(`/api/v1/mcp-servers/${id}`, { method: "DELETE" });
  }

  async verifyMCPServer(id: string): Promise<{ verified: boolean }> {
    return this.request(`/api/v1/mcp-servers/${id}/verify`, { method: "POST" });
  }

  async getMCPServerCapabilities(id: string): Promise<{
    capabilities: Array<{
      id: string;
      mcp_server_id: string;
      name: string;
      type: "tool" | "resource" | "prompt";
      description: string;
      schema: any;
      detected_at: string;
      last_verified_at?: string;
      is_active: boolean;
      created_at: string;
      updated_at: string;
    }>;
    total: number;
  }> {
    return this.request(`/api/v1/mcp-servers/${id}/capabilities`);
  }

  async getMCPServerAgents(id: string): Promise<{
    agents: Array<{
      id: string;
      name: string;
      display_name: string;
      agent_type: string;
      status: string;
    }>;
    total: number;
  }> {
    return this.request(`/api/v1/mcp-servers/${id}/agents`);
  }

  // ========================================
  // MCP Agent Attestation (New Approach)
  // ========================================

  /**
   * Submit cryptographically signed attestation from a verified agent
   * @param mcpServerId MCP server ID to attest
   * @param request Attestation data and Ed25519 signature
   */
  async attestMCP(
    mcpServerId: string,
    request: AttestMCPRequest
  ): Promise<AttestMCPResponse> {
    return this.request(`/api/v1/mcp-servers/${mcpServerId}/attest`, {
      method: "POST",
      body: JSON.stringify(request),
    });
  }

  /**
   * Get all agent attestations for an MCP server
   * @param mcpServerId MCP server ID
   */
  async getMCPAttestations(
    mcpServerId: string
  ): Promise<GetMCPAttestationsResponse> {
    return this.request(`/api/v1/mcp-servers/${mcpServerId}/attestations`);
  }

  /**
   * Get all agents connected to an MCP server (with attestation details)
   * @param mcpServerId MCP server ID
   */
  async getConnectedAgentsForMCP(
    mcpServerId: string
  ): Promise<GetConnectedAgentsResponse> {
    return this.request(`/api/v1/mcp-servers/${mcpServerId}/agents`);
  }

  // ========================================
  // Agent-MCP Relationship Management
  // ========================================

  /**
   * Get MCP servers an agent is connected to (with attestation details)
   * @param agentId Agent ID
   */
  async getAgentMCPServers(
    agentId: string
  ): Promise<GetAgentMCPServersResponse> {
    return this.request(`/api/v1/agents/${agentId}/mcp-servers`);
  }

  // Add MCP servers to agent's talks_to list
  async addMCPServersToAgent(
    agentId: string,
    data: {
      mcp_server_ids: string[];
      detected_method?: string;
      confidence?: number;
      metadata?: Record<string, any>;
    }
  ): Promise<{
    message: string;
    talks_to: string[];
    added_servers: string[];
    total_count: number;
  }> {
    return this.request(`/api/v1/agents/${agentId}/mcp-servers`, {
      method: "PUT",
      body: JSON.stringify(data),
    });
  }

  // Remove a single MCP server from agent's talks_to list
  async removeMCPServerFromAgent(
    agentId: string,
    mcpServerId: string
  ): Promise<{
    message: string;
    talks_to: string[];
    total_count: number;
  }> {
    return this.request(
      `/api/v1/agents/${agentId}/mcp-servers/${mcpServerId}`,
      {
        method: "DELETE",
      }
    );
  }

  // Auto-detect MCP servers from Claude Desktop config
  async detectAndMapMCPServers(
    agentId: string,
    data: {
      config_path: string;
      auto_register?: boolean;
      dry_run?: boolean;
    }
  ): Promise<{
    detected_servers: Array<{
      name: string;
      command: string;
      args: string[];
      env?: Record<string, string>;
      confidence: number;
      source: string;
      metadata?: Record<string, any>;
    }>;
    registered_count: number;
    mapped_count: number;
    total_talks_to: number;
    dry_run: boolean;
    errors_encountered?: string[];
  }> {
    return this.request(`/api/v1/agents/${agentId}/mcp-servers/detect`, {
      method: "POST",
      body: JSON.stringify(data),
    });
  }

  // Verification Events (Real-time Monitoring)
  async getRecentVerificationEvents(minutes = 15): Promise<{
    events: Array<{
      id: string;
      agentId: string;
      agentName: string;
      protocol: string;
      verificationType: string;
      status: string;
      confidence: number;
      trustScore: number;
      durationMs: number;
      initiatorType: string;
      startedAt: string;
      completedAt: string | null;
      createdAt: string;
    }>;
  }> {
    return this.request(
      `/api/v1/verification-events/recent?minutes=${minutes}`
    );
  }

  async getVerificationStatistics(
    period: "24h" | "7d" | "30d" = "24h"
  ): Promise<{
    totalVerifications: number;
    successCount: number;
    failedCount: number;
    pendingCount: number;
    timeoutCount: number;
    successRate: number;
    avgDurationMs: number;
    avgConfidence: number;
    avgTrustScore: number;
    verificationsPerMinute: number;
    uniqueAgentsVerified: number;
    protocolDistribution: { [key: string]: number };
    typeDistribution: { [key: string]: number };
    initiatorDistribution: { [key: string]: number };
  }> {
    return this.request(
      `/api/v1/verification-events/statistics?period=${period}`
    );
  }

  // OAuth / SSO Registration
  async listPendingRegistrations(
    limit = 50,
    offset = 0
  ): Promise<{
    requests: Array<{
      id: string;
      email: string;
      firstName: string;
      lastName: string;
      oauthProvider: "google" | "microsoft" | "okta";
      oauthUserId: string;
      status: "pending" | "approved" | "rejected";
      requestedAt: string;
      reviewedAt?: string;
      reviewedBy?: string;
      rejectionReason?: string;
      profilePictureUrl?: string;
      oauthEmailVerified: boolean;
    }>;
    total: number;
    limit: number;
    offset: number;
  }> {
    return this.request(
      `/api/v1/admin/registration-requests?limit=${limit}&offset=${offset}`
    );
  }

  async approveRegistration(id: string): Promise<{
    message: string;
    user: {
      id: string;
      email: string;
      role: string;
      status: string;
    };
  }> {
    return this.request(`/api/v1/admin/registration-requests/${id}/approve`, {
      method: "POST",
    });
  }

  async rejectRegistration(
    id: string,
    reason: string
  ): Promise<{
    message: string;
  }> {
    return this.request(`/api/v1/admin/registration-requests/${id}/reject`, {
      method: "POST",
      body: JSON.stringify({ reason }),
    });
  }

  // Tags
  async listTags(category?: TagCategory): Promise<Tag[]> {
    const url = category ? `/api/v1/tags?category=${category}` : "/api/v1/tags";
    return this.request(url);
  }

  async createTag(data: CreateTagInput): Promise<Tag> {
    return this.request("/api/v1/tags", {
      method: "POST",
      body: JSON.stringify(data),
    });
  }

  async updateTag(id: string, data: Partial<CreateTagInput>): Promise<Tag> {
    return this.request(`/api/v1/tags/${id}`, {
      method: "PUT",
      body: JSON.stringify(data),
    });
  }

  async deleteTag(id: string): Promise<void> {
    return this.request(`/api/v1/tags/${id}`, { method: "DELETE" });
  }

  // Agent Tags
  async getAgentTags(agentId: string): Promise<Tag[]> {
    return this.request(`/api/v1/agents/${agentId}/tags`);
  }

  async addTagsToAgent(agentId: string, tagIds: string[]): Promise<void> {
    return this.request(`/api/v1/agents/${agentId}/tags`, {
      method: "POST",
      body: JSON.stringify({ tag_ids: tagIds }),
    });
  }

  async removeTagFromAgent(agentId: string, tagId: string): Promise<void> {
    return this.request(`/api/v1/agents/${agentId}/tags/${tagId}`, {
      method: "DELETE",
    });
  }

  async suggestTagsForAgent(agentId: string): Promise<Tag[]> {
    return this.request(`/api/v1/agents/${agentId}/tags/suggestions`);
  }

  // Agent Capabilities
  async getAgentCapabilities(
    agentId: string,
    activeOnly: boolean = true
  ): Promise<AgentCapability[]> {
    return this.request(
      `/api/v1/agents/${agentId}/capabilities?activeOnly=${activeOnly}`
    );
  }

  async getLatestCapabilityReport(agentId: string): Promise<any> {
    return this.request(
      `/api/v1/detection/agents/${agentId}/capabilities/latest`
    );
  }

  async getAgentViolations(
    agentId: string,
    limit: number = 10,
    offset: number = 0
  ): Promise<{ violations: any[]; total: number }> {
    return this.request(
      `/api/v1/agents/${agentId}/violations?limit=${limit}&offset=${offset}`
    );
  }

  async getAgentKeyVault(agentId: string): Promise<any> {
    return this.request(`/api/v1/agents/${agentId}/key-vault`);
  }

  // MCP Server Tags
  async getMCPServerTags(mcpServerId: string): Promise<Tag[]> {
    return this.request(`/api/v1/mcp-servers/${mcpServerId}/tags`);
  }

  async addTagsToMCPServer(
    mcpServerId: string,
    tagIds: string[]
  ): Promise<void> {
    return this.request(`/api/v1/mcp-servers/${mcpServerId}/tags`, {
      method: "POST",
      body: JSON.stringify({ tag_ids: tagIds }),
    });
  }

  async removeTagFromMCPServer(
    mcpServerId: string,
    tagId: string
  ): Promise<void> {
    return this.request(`/api/v1/mcp-servers/${mcpServerId}/tags/${tagId}`, {
      method: "DELETE",
    });
  }

  async suggestTagsForMCPServer(mcpServerId: string): Promise<Tag[]> {
    return this.request(`/api/v1/mcp-servers/${mcpServerId}/tags/suggestions`);
  }

  // SDK Tokens
  async listSDKTokens(includeRevoked = false): Promise<{ tokens: SDKToken[] }> {
    return this.request(
      `/api/v1/users/me/sdk-tokens?include_revoked=${includeRevoked}`
    );
  }

  async getActiveSDKTokenCount(): Promise<{ count: number }> {
    return this.request("/api/v1/users/me/sdk-tokens/count");
  }

  async revokeSDKToken(tokenId: string, reason: string): Promise<void> {
    return this.request(`/api/v1/users/me/sdk-tokens/${tokenId}/revoke`, {
      method: "POST",
      body: JSON.stringify({ reason }),
    });
  }

  async revokeAllSDKTokens(reason: string): Promise<void> {
    return this.request("/api/v1/users/me/sdk-tokens/revoke-all", {
      method: "POST",
      body: JSON.stringify({ reason }),
    });
  }

  // SDK Download with automatic token refresh on 401
  async downloadSDK(
    sdkType: "python" | "go" | "javascript" = "python"
  ): Promise<Blob> {
    const attemptDownload = async (token: string | null): Promise<Response> => {
      return fetch(`${this.baseURL}/api/v1/sdk/download?sdk=${sdkType}`, {
        method: "GET",
        headers: {
          Authorization: `Bearer ${token}`,
        },
      });
    };

    // First attempt with current token
    let response = await attemptDownload(this.getToken());

    // If 401 Unauthorized, try to refresh token and retry
    if (response.status === 401) {
      const refreshed = await this.refreshAccessToken();

      if (!refreshed) {
        // Refresh failed - token is expired and can't be refreshed
        throw new Error(
          "Your session has expired. Please sign in again to download the SDK."
        );
      }

      // Retry with new token
      response = await attemptDownload(this.getToken());
    }

    if (!response.ok) {
      const error = await response
        .json()
        .catch(() => ({ error: "Failed to download SDK" }));
      throw new Error(error.error || "Failed to download SDK");
    }

    return response.blob();
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
      method: "POST",
      body: JSON.stringify(data),
    });
  }

  // Get current detection status for an agent
  async getDetectionStatus(agentId: string): Promise<DetectionStatusResponse> {
    return this.request(`/api/v1/detection/agents/${agentId}/status`);
  }

  // ========================================
  // Capability Requests (Admin + User)
  // ========================================

  // List capability requests (admin only)
  async getCapabilityRequests(params?: {
    status?: "pending" | "approved" | "rejected";
    agentId?: string;
    limit?: number;
    offset?: number;
  }): Promise<any[]> {
    const queryParams = new URLSearchParams();
    if (params?.status) queryParams.append("status", params.status);
    if (params?.agentId) queryParams.append("agent_id", params.agentId);
    if (params?.limit) queryParams.append("limit", params.limit.toString());
    if (params?.offset) queryParams.append("offset", params.offset.toString());

    const query = queryParams.toString() ? `?${queryParams.toString()}` : "";
    return this.request(`/api/v1/admin/capability-requests${query}`);
  }

  // Get a single capability request by ID (admin only)
  async getCapabilityRequest(id: string): Promise<any> {
    return this.request(`/api/v1/admin/capability-requests/${id}`);
  }

  // Approve a capability request (admin only)
  async approveCapabilityRequest(id: string): Promise<{ message: string }> {
    return this.request(`/api/v1/admin/capability-requests/${id}/approve`, {
      method: "POST",
    });
  }

  // Reject a capability request (admin only)
  async rejectCapabilityRequest(id: string): Promise<{ message: string }> {
    return this.request(`/api/v1/admin/capability-requests/${id}/reject`, {
      method: "POST",
    });
  }

  // ========================================
  // Security Policies (Admin Only)
  // ========================================

  // List all security policies for the organization
  async getSecurityPolicies(): Promise<any[]> {
    return this.request("/api/v1/admin/security-policies");
  }

  // Get a specific security policy by ID
  async getSecurityPolicy(policyId: string): Promise<any> {
    return this.request(`/api/v1/admin/security-policies/${policyId}`);
  }

  // Create a new security policy
  async createSecurityPolicy(data: {
    name: string;
    description?: string;
    policy_type: string;
    enforcement_action: "alert_only" | "block_and_alert" | "allow";
    severity_threshold: string;
    rules?: Record<string, any>;
    applies_to: string;
    is_enabled: boolean;
    priority: number;
  }): Promise<any> {
    return this.request("/api/v1/admin/security-policies", {
      method: "POST",
      body: JSON.stringify(data),
    });
  }

  // Update an existing security policy
  async updateSecurityPolicy(
    policyId: string,
    data: {
      name: string;
      description?: string;
      policy_type: string;
      enforcement_action: "alert_only" | "block_and_alert" | "allow";
      severity_threshold: string;
      rules?: Record<string, any>;
      applies_to: string;
      is_enabled: boolean;
      priority: number;
    }
  ): Promise<any> {
    return this.request(`/api/v1/admin/security-policies/${policyId}`, {
      method: "PUT",
      body: JSON.stringify(data),
    });
  }

  // Delete a security policy
  async deleteSecurityPolicy(policyId: string): Promise<void> {
    return this.request(`/api/v1/admin/security-policies/${policyId}`, {
      method: "DELETE",
    });
  }

  // Toggle policy enabled/disabled status
  async toggleSecurityPolicy(
    policyId: string,
    isEnabled: boolean
  ): Promise<any> {
    return this.request(`/api/v1/admin/security-policies/${policyId}/toggle`, {
      method: "PATCH",
      body: JSON.stringify({ isEnabled }),
    });
  }

  // ========================================
  // Compliance (Admin Only)
  // ========================================

  // Get compliance status overview
  async getComplianceStatus(): Promise<{
    compliance_level: string;
    total_agents: number;
    verified_agents: number;
    verification_rate: number; // Already in percentage (0-100)
    average_trust_score: number; // Already in percentage (0-100)
    recent_audit_count: number;
  }> {
    return this.request("/api/v1/compliance/status");
  }

  // Get compliance metrics
  async getComplianceMetrics(): Promise<{
    start_date: string;
    end_date: string;
    interval: string;
    metrics: {
      period: {
        start: string;
        end: string;
        interval: string;
      };
      agent_verification_trend: Array<{
        date: string;
        verified: number;
      }>;
      trust_score_trend: Array<{
        date: string;
        avg_score: number; // 0-1 scale
      }>;
    };
  }> {
    return this.request("/api/v1/compliance/metrics");
  }

  // Get access review (users and their permissions)
  async getAccessReview(): Promise<{
    users: Array<{
      id: string;
      email: string;
      name: string;
      role: string;
      last_login: string;
      created_at: string;
      status: string;
    }>;
    total: number;
  }> {
    return this.request("/api/v1/compliance/access-review");
  }

  // Run compliance check
  async runComplianceCheck(checkType: string = "all"): Promise<{
    check_type: string;
    passed: number;
    failed: number;
    total: number;
    compliance_rate: number;
    checks: Array<{
      name: string;
      passed: boolean;
    }>;
  }> {
    return this.request("/api/v1/compliance/check", {
      method: "POST",
      body: JSON.stringify({ check_type: checkType }),
    });
  }

  // Get data retention information
  async getDataRetention(): Promise<{
    policies: Array<{
      id: string;
      data_type: string;
      retention_period_days: number;
      description: string;
      auto_delete: boolean;
      created_at: string;
    }>;
    storage_metrics: {
      total_records: number;
      oldest_record_date: string;
      deletion_candidates: number;
    };
  }> {
    return this.request("/api/v1/compliance/data-retention");
  }

  // Get compliance violations
  async getComplianceViolations(
    framework?: string,
    severity?: string
  ): Promise<{
    violations: Array<{
      id: string;
      framework: string;
      violation_type: string;
      severity: string;
      description: string;
      affected_resource: string;
      detected_at: string;
      remediated: boolean;
      remediation_notes?: string;
      remediated_by?: string;
      remediated_at?: string;
    }>;
    total: number;
    filters: {
      framework: string;
      severity: string;
    };
  }> {
    const params = new URLSearchParams();
    if (framework) params.append("framework", framework);
    if (severity) params.append("severity", severity);

    const queryString = params.toString();
    const url = queryString
      ? `/api/v1/compliance/violations?${queryString}`
      : "/api/v1/compliance/violations";

    return this.request(url);
  }

  // Remediate a compliance violation
  async remediateViolation(
    violationId: string,
    remediationNotes: string,
    remediationDate?: string
  ): Promise<{
    message: string;
    violation_id: string;
    remediated_at: string;
  }> {
    return this.request(`/api/v1/compliance/remediate/${violationId}`, {
      method: "POST",
      body: JSON.stringify({
        remediation_notes: remediationNotes,
        remediation_date: remediationDate,
      }),
    });
  }

  // Resolve alert
  async resolveAlert(
    id: string,
    resolution_notes: string
  ): Promise<{
    success: boolean;
    message: string;
  }> {
    return this.request(`/api/v1/admin/alerts/${id}/resolve`, {
      method: "POST",
      body: JSON.stringify({ resolution_notes }),
    });
  }

  // Get agent audit logs
  async getAgentAuditLogs(
    agentId: string,
    limit: number = 50
  ): Promise<{
    logs: Array<{
      id: string;
      action: string;
      performed_by: string;
      performed_by_email: string;
      timestamp: string;
      details: string;
      ip_address?: string;
    }>;
    total: number;
  }> {
    return this.request(`/api/v1/agents/${agentId}/audit-logs?limit=${limit}`);
  }
}

// Lazy singleton instance - created ONLY on first access in browser
let _apiInstance: APIClient | null = null;

function getAPIClient(): APIClient {
  if (!_apiInstance) {
    console.log("[API] Creating APIClient instance for the first time");
    _apiInstance = new APIClient();
  }
  return _apiInstance;
}

// Export a Proxy that lazily creates the real APIClient on first property access
export const api = new Proxy({} as APIClient, {
  get(target, prop) {
    const instance = getAPIClient();
    const value = (instance as any)[prop];

    // Bind methods to the instance to preserve 'this' context
    if (typeof value === "function") {
      return value.bind(instance);
    }

    return value;
  },
  set(target, prop, value) {
    const instance = getAPIClient();
    (instance as any)[prop] = value;
    return true;
  },
});
