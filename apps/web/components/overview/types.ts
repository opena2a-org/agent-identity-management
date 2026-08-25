import type { Agent } from "@/lib/api";

export interface StatsData {
  totalAgents: number;
  verifiedAgents: number;
  pendingAgents: number;
  verificationRate: number;
  avgTrustScore: number;
  totalMcpServers: number;
  activeMcpServers: number;
  totalUsers: number;
  activeUsers: number;
  activeAlerts: number;
  criticalAlerts: number;
  securityIncidents: number;
  organizationId: string;
  /** Client-side index of the agents list, for lookups by id. */
  agentsById?: Record<string, Agent>;
}

export interface VerificationStatistics {
  totalVerifications: number;
  successCount: number;
  failedCount: number;
  pendingCount: number;
  uniqueAgentsVerified: number;
}

export interface VerificationActivityMonth {
  month: string;
  verified: number;
  pending: number;
  monthYear: string;
}

export interface SecurityMetrics {
  securityScore: number;
  securityStatus: string;
  lastIncidentAt: string | null;
  actionsBlocked: number;
  actionsBlockedToday: number;
  agentsMonitored: number;
  agentsTrusted: number;
  trustPercentage: number;
  actionsToday: number;
  requiresAttention: number;
  averageTrustScore: number;
  mcpServersTotal: number;
  mcpServersVerified: number;
  totalThreats: number;
  activeThreats: number;
  blockedThreats: number;
  highSeverityCount: number;
  openIncidents: number;
  riskByCategory: Array<{ category: string; blocked: number; riskLevel: string }>;
  recentBlockedActions: Array<{ id: string; agentId: string; agentName: string; attemptedCapability: string; details: string; trustImpact: number; createdAt: string }>;
}

export interface Violation {
  id: string;
  agentId: string;
  agentName?: string;
  attemptedCapability: string;
  registeredCapabilities: string[];
  severity: string;
  trustScoreImpact: number;
  isBlocked: boolean;
  createdAt: string;
}

export interface VerificationEvent {
  id: string;
  agentId: string;
  agentName?: string;
  verificationType: string;
  status: string;
  trustScore: number;
  startedAt: string;
  createdAt: string;
}

export interface ComplianceStatus {
  complianceLevel: string;
  totalAgents: number;
  verifiedAgents: number;
  verificationRate: number;
  averageTrustScore: number;
  recentAuditCount: number;
}

/** Extra data the Security and Executive lenses read; null when the endpoint failed or the role may not read it. */
export interface LensData {
  security: SecurityMetrics | null;
  violations: { violations: Violation[]; total: number } | null;
  events: VerificationEvent[] | null;
  compliance: ComplianceStatus | null;
}
