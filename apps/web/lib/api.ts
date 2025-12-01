"use client";

import { toast } from "sonner";

const SESSION_EXPIRED_TOAST_ID = "session-expired";

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
  organizationId: string;
  name: string;
  displayName: string;
  description: string;
  agentType: "ai_agent" | "mcp_server";
  status: "pending" | "verified" | "suspended" | "revoked";
  version: string;
  trustScore: number;
  talksTo?: string[];
  capabilities?: any[];
  createdAt: string;
  updatedAt: string;
}

export interface Organization {
  id: string;
  name: string;
  maxAgents: number;
  isActive: boolean;
  createdAt: string;
  updatedAt: string;
}

export interface User {
  id: string;
  organizationId?: string;
  organizationName?: string;
  email: string;
  name: string;
  avatarUrl?: string;
  role: "admin" | "manager" | "member" | "viewer" | "pending";
  status: "active" | "pending_approval" | "suspended" | "deactivated";
  forcePasswordChange?: boolean;
  createdAt: string;
  provider?: string;
  lastLoginAt?: string;
  requestedAt?: string;
  pictureUrl?: string;
  isRegistrationRequest?: boolean;
}

export interface APIKey {
  id: string;
  agentId: string;
  name: string;
  prefix: string;
  lastUsedAt?: string;
  expiresAt?: string;
  isActive: boolean;
  createdAt: string;
  agentName?: string; // Optional - may be included by backend in some responses
}

export type TagCategory =
  | "resource_type"
  | "environment"
  | "agent_type"
  | "data_classification"
  | "custom";

export interface Tag {
  id: string;
  organizationId: string;
  key: string;
  value: string;
  category: TagCategory;
  description: string;
  color: string;
  createdAt: string;
  createdBy: string;
}

export interface CreateTagInput {
  key: string;
  value: string;
  category: TagCategory;
  description?: string;
  color?: string;
}

export interface AddTagsInput {
  tagIds: string[];
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
  agentId: string;
  mcpUrl: string;
  mcpName: string;
  capabilitiesFound: string[];
  connectionSuccessful: boolean;
  healthCheckPassed: boolean;
  connectionLatencyMs: number;
  timestamp: string; // ISO 8601 timestamp
  sdkVersion: string;
}

export interface AttestMCPRequest {
  attestation: AttestationPayload;
  signature: string; // Ed25519 signature (base64)
}

export interface AttestMCPResponse {
  success: boolean;
  attestationId: string;
  mcpConfidenceScore: number; // 0-100
  attestationCount: number;
  message: string;
}

export interface AttestationWithAgentDetails {
  id: string;
  agentId: string;
  agentName: string;
  agentTrustScore: number;
  verifiedAt: string; // ISO 8601 timestamp
  expiresAt: string; // ISO 8601 timestamp
  capabilitiesConfirmed: string[];
  connectionLatencyMs: number;
  healthCheckPassed: boolean;
  isValid: boolean;
}

export interface GetMCPAttestationsResponse {
  attestations: AttestationWithAgentDetails[];
  total: number;
  confidenceScore: number; // 0-100
  lastAttestedAt: string; // ISO 8601 timestamp
}

export interface ConnectedAgent {
  id: string;
  name: string;
  displayName: string;
  trustScore: number;
  status: string;
  lastAttestedAt?: string;
  attestationCount: number;
}

export interface GetConnectedAgentsResponse {
  agents: ConnectedAgent[];
  total: number;
}

export interface ConnectedMCPServer {
  id: string;
  organizationId?: string;
  name: string;
  description?: string;
  url: string;
  version?: string;
  publicKey?: string;
  status?: string;
  isVerified?: boolean;
  lastVerifiedAt?: string;
  verificationUrl?: string;
  capabilities?: string[];
  trustScore?: number;
  registeredByAgent?: string | null;
  createdBy?: string;
  createdAt?: string;
  updatedAt?: string;
  tags?: string[] | null;
  verificationMethod: string;
  attestationCount: number;
  confidenceScore: number;
  lastAttestedAt?: string;
}

export interface GetAgentMCPServersResponse {
  mcpServers: ConnectedMCPServer[];
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

    if (response.status === 401) {
      this.clearToken();
      if (typeof window !== "undefined") {
        toast.error("Session expired", {
          id: SESSION_EXPIRED_TOAST_ID,
          description: "Please sign in again to continue.",
        });

        setTimeout(() => {
          window.location.replace("/auth/login");
        }, 1500);
      }
      throw new Error("Unauthorized");
    }

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
    requiresPasswordChange?: boolean;
  }> {
    const response = await this.request<{
      success: boolean;
      message: string;
      user?: User;
      accessToken?: string;
      refreshToken?: string;
      isApproved: boolean;
      requiresPasswordChange?: boolean;
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
  ): Promise<{ apiKey: string; message: string }> {
    return this.request(`/api/v1/agents/${id}/rotate-credentials`, {
      method: "POST",
    });
  }

  async adjustAgentTrustScore(
    id: string,
    score: number,
    reason: string
  ): Promise<{ success: boolean; newScore: number }> {
    return this.request(`/api/v1/agents/${id}/trust-score`, {
      method: "PUT",
      body: JSON.stringify({ score, reason }),
    });
  }

  async getAgentTrustScoreHistory(id: string): Promise<{
    agentId: string;
    history: Array<{
      timestamp: string;
      trustScore: number;
      reason: string;
      changedBy: string;
    }>;
  }> {
    return this.request(`/api/v1/agents/${id}/trust-score/history`);
  }

  // API Keys
  async listAPIKeys(): Promise<{ apiKeys: APIKey[] }> {
    return this.request("/api/v1/api-keys");
  }

  async createAPIKey(
    agentId: string,
    name: string
  ): Promise<{ apiKey: string; id: string }> {
    return this.request("/api/v1/api-keys", {
      method: "POST",
      body: JSON.stringify({ agentId: agentId, name }),
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
  async getTrustScore(agentId: string): Promise<{ trustScore: number }> {
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
  async getAlerts(limit = 100, offset = 0, status?: string): Promise<{
    alerts: any[];
    total: number;
    allCount: number;
    acknowledgedCount: number;
    unacknowledgedCount: number;
  }> {
    let url = `/api/v1/admin/alerts?limit=${limit}&offset=${offset}`;
    if (status && status !== 'all') {
      url += `&status=${status}`;
    }
    const response: any = await this.request(url);
    return {
      alerts: response.alerts || [],
      total: response.total || 0,
      allCount: response.allCount || 0,
      acknowledgedCount: response.acknowledgedCount || 0,
      unacknowledgedCount: response.unacknowledgedCount || 0,
    };
  }

  async acknowledgeAlert(alertId: string): Promise<void> {
    return this.request(`/api/v1/admin/alerts/${alertId}/acknowledge`, {
      method: "POST",
    });
  }

  async bulkAcknowledgeAlerts(userId: string): Promise<{
    message: string;
    acknowledgedCount: number;
    bulkAcknowledged: boolean;
  }> {
    return this.request(`/api/v1/admin/alerts/bulk-acknowledge`, {
      method: "POST",
      body: JSON.stringify({ userId: userId }),
    });
  }

  async getUnacknowledgedAlertCount(): Promise<number> {
    const alertsObj = await this.getAlerts(100, 0);
    return alertsObj.alerts.filter((a: any) => !a.isAcknowledged).length;
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

  async getPendingVerificationCount(): Promise<number> {
    try {
      const response = await this.getPendingVerifications({
        page: 1,
        pageSize: 1,
        status: "pending",
      });
      return response?.status_counts?.pending ?? 0;
    } catch (error) {
      console.error("Failed to fetch pending verification count:", error);
      return 0;
    }
  }

  // Dashboard stats - Viewer-accessible analytics endpoint
  async getDashboardStats(): Promise<{
    // Agent metrics
    totalAgents: number;
    verifiedAgents: number;
    pendingAgents: number;
    verificationRate: number;
    avgTrustScore: number;

    // MCP Server metrics
    totalMcpServers: number;
    activeMcpServers: number;

    // User metrics
    totalUsers: number;
    activeUsers: number;

    // Security metrics
    activeAlerts: number;
    criticalAlerts: number;
    securityIncidents: number;

    // Verification metrics (last 24 hours)
    totalVerifications?: number;
    successfulVerifications?: number;
    failedVerifications?: number;
    avgResponseTime?: number;

    // Organization
    organizationId: string;
  }> {
    return this.request("/api/v1/analytics/dashboard");
  }

  // Admin Dashboard stats - Admin-only endpoint with comprehensive platform metrics
  async getAdminDashboardStats(): Promise<{
    // Agent metrics
    totalAgents: number;
    verifiedAgents: number;
    pendingAgents: number;
    verificationRate: number;
    avgTrustScore: number;

    // MCP Server metrics
    totalMcpServers: number;
    activeMcpServers: number;

    // User metrics
    totalUsers: number;
    activeUsers: number;

    // Security metrics
    activeAlerts: number;
    criticalAlerts: number;
    securityIncidents: number;

    // Organization
    organizationId: string;
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
      monthYear: string;
    }>;
    currentStats: {
      totalVerified: number;
      totalPending: number;
      totalAgents: number;
    };
  }> {
    return this.request(
      `/api/v1/analytics/verification-activity?months=${months}`
    );
  }

  async getUsageStatistics(days = 30): Promise<{
    period: string;
    apiCalls: number;
    activeAgents: number;
    totalAgents: number;
    dataVolume: number;
    uptime: number;
    generatedAt: string;
  }> {
    return this.request(`/api/v1/analytics/usage?days=${days}`);
  }

  async getTrustScoreTrends(days = 30): Promise<{
    period: string;
    trends: Array<{
      date: string;
      avgScore: number;
      agentCount: number;
      scoresByRange: {
        excellent: number; // 90-100
        good: number; // 75-89
        fair: number; // 50-74
        poor: number; // 0-49
      };
    }>;
    summary: {
      overallAvg: number;
      trendDirection: "up" | "down" | "stable";
      changePercentage: number;
    };
  }> {
    return this.request(`/api/v1/analytics/trends?days=${days}`);
  }

  async getAgentActivity(limit = 50): Promise<{
    activities: Array<{
      id: string;
      agentId: string;
      agentName: string;
      action: string;
      status: "success" | "failure" | "pending";
      timestamp: string;
      details?: string;
    }>;
    summary: {
      totalActivities: number;
      successCount: number;
      failureCount: number;
      successRate: number;
    };
  }> {
    return this.request(`/api/v1/analytics/agents/activity?limit=${limit}`);
  }

  // Webhooks
  async listWebhooks(): Promise<
    Array<{
      id: string;
      organizationId: string;
      name: string;
      url: string;
      events: string[];
      isActive: boolean;
      secret: string;
      createdAt: string;
      lastTriggeredAt?: string;
      successCount: number;
      failureCount: number;
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
    organizationId: string;
    name: string;
    url: string;
    events: string[];
    isActive: boolean;
    secret: string;
    createdAt: string;
  }> {
    return this.request("/api/v1/webhooks", {
      method: "POST",
      body: JSON.stringify(data),
    });
  }

  async getWebhook(id: string): Promise<{
    id: string;
    organizationId: string;
    name: string;
    url: string;
    events: string[];
    isActive: boolean;
    secret: string;
    createdAt: string;
    lastTriggeredAt?: string;
    successCount: number;
    failureCount: number;
    deliveries: Array<{
      id: string;
      event: string;
      status: "success" | "failure";
      responseCode: number;
      timestamp: string;
      errorMessage?: string;
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
      isActive?: boolean;
    }
  ): Promise<{
    id: string;
    organizationId: string;
    name: string;
    url: string;
    events: string[];
    isActive: boolean;
    createdAt: string;
  }> {
    return this.request(`/api/v1/webhooks/${id}`, {
      method: "PUT",
      body: JSON.stringify(data),
    });
  }

  async testWebhook(id: string): Promise<{
    success: boolean;
    responseCode: number;
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
      agentId: string;
      agentName: string;
      action: string;
      status: "approved" | "denied" | "pending";
      durationMs: number;
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

  async getPendingVerifications(params?: {
    page?: number;
    pageSize?: number;
    status?: string;
    risk?: string;
    search?: string;
    searchField?: string;
  }): Promise<{
    verifications: Array<{
      id: string;
      agent_id: string;
      agent_name: string;
      action_type: string;
      resource: string;
      context: Record<string, any>;
      risk_level: string;
      trust_score: number;
      status: string;
      requested_at: string;
      expires_at: string;
    }>;
    pagination: {
      page: number;
      page_size: number;
      total: number;
      total_pages: number;
    };
    status_counts: {
      pending: number;
      approved: number;
      denied: number;
    };
  }> {
    const query = new URLSearchParams();
    if (params?.page) query.set("page", String(params.page));
    if (params?.pageSize) query.set("page_size", String(params.pageSize));
    if (params?.status) query.set("status", params.status);
    if (params?.risk) query.set("risk", params.risk);
    if (params?.search) query.set("search", params.search);
    if (params?.searchField) query.set("search_field", params.searchField);
    const suffix = query.toString() ? `?${query.toString()}` : "";
    return this.request(`/api/v1/admin/verifications/pending${suffix}`);
  }

  async approvePendingVerification(
    id: string,
    reason?: string
  ): Promise<any> {
    return this.request(`/api/v1/admin/verifications/${id}/approve`, {
      method: "POST",
      body: reason ? JSON.stringify({ reason }) : undefined,
    });
  }

  async denyPendingVerification(id: string, reason: string): Promise<any> {
    return this.request(`/api/v1/admin/verifications/${id}/deny`, {
      method: "POST",
      body: JSON.stringify({ reason }),
    });
  }

  // Security
  async getSecurityThreats(
    limit = 100,
    offset = 0
  ): Promise<{
    threats: Array<{
      id: string;
      targetId: string;
      targetName?: string;
      threatType: string;
      severity: "low" | "medium" | "high" | "critical";
      title?: string;
      description: string;
      source?: string;
      targetType?: string;
      isBlocked: boolean;
      createdAt: string;
      resolvedAt?: string;
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
      agentId: string;
      anomalyType: string;
      severity: string;
      description: string;
      detectedAt: string;
    }>;
    total: number;
  }> {
    return this.request(
      `/api/v1/security/anomalies?limit=${limit}&offset=${offset}`
    );
  }

  async getSecurityMetrics(): Promise<{
    totalThreats: number;
    activeThreats: number;
    totalAnomalies: number;
    totalIncidents: number;
    threatTrend: Array<{ date: string; count: number }>;
    severityDistribution: Array<{ severity: string; count: number }>;
  }> {
    return this.request("/api/v1/security/metrics");
  }

  // MCP Servers
  async listMCPServers(
    limit = 100,
    offset = 0
  ): Promise<{
    mcpServers: Array<{
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
      isVerified?: boolean;
      lastVerifiedAt?: string;
      createdAt: string;
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
    connectedAgents: Array<{
      id: string;
      name: string;
      displayName: string;
      status: string;
      trustScore: number;
      updatedAt: string;
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
      mcpServerId: string;
      name: string;
      type: "tool" | "resource" | "prompt";
      description: string;
      schema: any;
      detectedAt: string;
      lastVerifiedAt?: string;
      isActive: boolean;
      createdAt: string;
      updatedAt: string;
    }>;
    total: number;
  }> {
    return this.request(`/api/v1/mcp-servers/${id}/capabilities`);
  }

  async getMCPServerAgents(id: string): Promise<{
    agents: Array<{
      id: string;
      name: string;
      displayName: string;
      agentType: string;
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

  // Add MCP servers to agent's talksTo list
  async addMCPServersToAgent(
    agentId: string,
    data: {
      mcpServerIds: string[];
      detectedMethod?: string;
      confidence?: number;
      metadata?: Record<string, any>;
    }
  ): Promise<{
    message: string;
    talksTo: string[];
    addedServers: string[];
    totalCount: number;
  }> {
    return this.request(`/api/v1/agents/${agentId}/mcp-servers`, {
      method: "PUT",
      body: JSON.stringify(data),
    });
  }

  // Remove a single MCP server from agent's talksTo list
  async removeMCPServerFromAgent(
    agentId: string,
    mcpServerId: string
  ): Promise<{
    message: string;
    talksTo: string[];
    totalCount: number;
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
      configPath: string;
      autoRegister?: boolean;
      dryRun?: boolean;
    }
  ): Promise<{
    detectedServers: Array<{
      name: string;
      command: string;
      args: string[];
      env?: Record<string, string>;
      confidence: number;
      source: string;
      metadata?: Record<string, any>;
    }>;
    registeredCount: number;
    mappedCount: number;
    totalTalksTo: number;
    dryRun: boolean;
    errorsEncountered?: string[];
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
      body: JSON.stringify({ tagIds: tagIds }),
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
      body: JSON.stringify({ tagIds: tagIds }),
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
    if (params?.agentId) queryParams.append("agentId", params.agentId);
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
    policyType: string;
    enforcementAction: "alert_only" | "block_and_alert" | "allow";
    severityThreshold: string;
    rules?: Record<string, any>;
    appliesTo: string;
    isEnabled: boolean;
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
      policyType: string;
      enforcementAction: "alert_only" | "block_and_alert" | "allow";
      severityThreshold: string;
      rules?: Record<string, any>;
      appliesTo: string;
      isEnabled: boolean;
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
    complianceLevel: string;
    totalAgents: number;
    verifiedAgents: number;
    verificationRate: number; // Already in percentage (0-100)
    averageTrustScore: number; // Already in percentage (0-100)
    recentAuditCount: number;
  }> {
    return this.request("/api/v1/compliance/status");
  }

  // Get compliance metrics
  async getComplianceMetrics(): Promise<{
    startDate: string;
    endDate: string;
    interval: string;
    metrics: {
      period: {
        start: string;
        end: string;
        interval: string;
      };
      agentVerificationTrend: Array<{
        date: string;
        verified: number;
      }>;
      trustScoreTrend: Array<{
        date: string;
        avgScore: number; // 0-1 scale
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
      lastLogin: string;
      createdAt: string;
      status: string;
    }>;
    total: number;
  }> {
    return this.request("/api/v1/compliance/access-review");
  }

  // Run compliance check
  async runComplianceCheck(checkType: string = "all"): Promise<{
    checkType: string;
    passed: number;
    failed: number;
    total: number;
    complianceRate: number;
    checks: Array<{
      name: string;
      passed: boolean;
    }>;
  }> {
    return this.request("/api/v1/compliance/check", {
      method: "POST",
      body: JSON.stringify({ checkType: checkType }),
    });
  }

  // Get data retention information
  async getDataRetention(): Promise<{
    policies: Array<{
      id: string;
      dataType: string;
      retentionPeriodDays: number;
      description: string;
      autoDelete: boolean;
      createdAt: string;
    }>;
    storageMetrics: {
      totalRecords: number;
      oldestRecordDate: string;
      deletionCandidates: number;
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
      violationType: string;
      severity: string;
      description: string;
      affectedResource: string;
      detectedAt: string;
      remediated: boolean;
      remediationNotes?: string;
      remediatedBy?: string;
      remediatedAt?: string;
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
    violationId: string;
    remediatedAt: string;
  }> {
    return this.request(`/api/v1/compliance/remediate/${violationId}`, {
      method: "POST",
      body: JSON.stringify({
        remediationNotes: remediationNotes,
        remediationDate: remediationDate,
      }),
    });
  }

  // Resolve alert
  async resolveAlert(
    id: string,
    resolutionNotes: string
  ): Promise<{
    success: boolean;
    message: string;
  }> {
    return this.request(`/api/v1/admin/alerts/${id}/resolve`, {
      method: "POST",
      body: JSON.stringify({ resolutionNotes }),
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
      performedBy: string;
      performedByEmail: string;
      timestamp: string;
      details: string;
      ipAddress?: string;
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
