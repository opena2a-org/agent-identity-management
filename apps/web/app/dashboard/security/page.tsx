"use client";

import { useState, useEffect } from "react";
import {
  Shield,
  AlertTriangle,
  Activity,
  TrendingUp,
  AlertOctagon,
  Eye,
  XCircle,
  CheckCircle,
  AlertCircle,
  Bell,
  Clock,
  ChevronRight,
  Filter,
  X,
  Bot,
  Server,
} from "lucide-react";
import Link from "next/link";
import {
  LineChart,
  Line,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
  BarChart,
  Bar,
  Cell,
} from "recharts";
import { api } from "@/lib/api";
import ThreatDetailModal from "@/components/modals/threat-detail-modal";
import { formatDateTime } from "@/lib/date-utils";
import { getErrorMessage } from "@/lib/error-messages";
import { AuthGuard } from "@/components/auth-guard";

// Unified Security Activity for comprehensive activity feed
interface SecurityActivity {
  id: string;
  type: 'agent_created' | 'agent_verified' | 'agent_suspended' | 'mcp_registered' | 'mcp_verified' | 'verification_success' | 'verification_failed' | 'capability_granted' | 'capability_revoked' | 'api_key_created' | 'api_key_revoked' | 'audit_action';
  title: string;
  description: string;
  entityType: 'agent' | 'mcp' | 'verification' | 'capability' | 'api_key';
  entityId?: string;
  entityName: string;
  severity: 'info' | 'success' | 'warning' | 'error';
  timestamp: string;
  metadata?: Record<string, any>;
}

interface SecurityThreat {
  id: string;
  targetId: string; // Agent or MCP server ID
  targetName?: string; // Agent or MCP server name (optional, populated by backend)
  threatType: string;
  severity: "low" | "medium" | "high" | "critical";
  description: string;
  isBlocked: boolean;
  createdAt: string;
  resolvedAt?: string;
  // Additional fields for enhanced detail view
  source?: string;
  targetType?: string;
  title?: string;
}

interface SecurityIncident {
  id: string;
  title: string;
  severity: "low" | "medium" | "high" | "critical";
  status: "open" | "investigating" | "resolved";
  createdAt: string;
}

function StatCard({ stat }: { stat: any }) {
  return (
    <div className="bg-white dark:bg-gray-800 p-6 rounded-lg border border-gray-200 dark:border-gray-700 shadow-sm">
      <div className="flex items-center">
        <div className="flex-shrink-0">
          <stat.icon className="h-6 w-6 text-gray-400" />
        </div>
        <div className="ml-5 w-0 flex-1">
          <dl>
            <dt className="text-sm font-medium text-gray-500 dark:text-gray-400 truncate">
              {stat.name}
            </dt>
            <dd className="flex items-baseline">
              <div className="text-2xl font-semibold text-gray-900 dark:text-gray-100">
                {stat.value}
              </div>
              {stat.change && (
                <div
                  className={`ml-2 flex items-baseline text-sm font-semibold ${
                    stat.changeType === "positive"
                      ? "text-green-600"
                      : "text-red-600"
                  }`}
                >
                  {stat.change}
                </div>
              )}
            </dd>
          </dl>
        </div>
      </div>
    </div>
  );
}

function SeverityBadge({ severity }: { severity: string }) {
  const getSeverityStyles = (severity: string) => {
    switch (severity) {
      case "critical":
        return "bg-red-100 dark:bg-red-900/30 text-red-800 dark:text-red-300";
      case "high":
        return "bg-orange-100 dark:bg-orange-900/30 text-orange-800 dark:text-orange-300";
      case "medium":
        return "bg-yellow-100 dark:bg-yellow-900/30 text-yellow-800 dark:text-yellow-300";
      case "low":
        return "bg-blue-100 dark:bg-blue-900/30 text-blue-800 dark:text-blue-300";
      default:
        return "bg-gray-100 dark:bg-gray-700 text-gray-800 dark:text-gray-300";
    }
  };

  return (
    <span
      className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium capitalize ${getSeverityStyles(severity)}`}
    >
      {severity}
    </span>
  );
}

function StatusBadge({ status }: { status: string }) {
  const getStatusStyles = (status: string) => {
    switch (status) {
      case "active":
      case "open":
        return "bg-red-100 dark:bg-red-900/30 text-red-800 dark:text-red-300";
      case "mitigated":
      case "investigating":
        return "bg-yellow-100 dark:bg-yellow-900/30 text-yellow-800 dark:text-yellow-300";
      case "resolved":
        return "bg-green-100 dark:bg-green-900/30 text-green-800 dark:text-green-300";
      default:
        return "bg-gray-100 dark:bg-gray-700 text-gray-800 dark:text-gray-300";
    }
  };

  return (
    <span
      className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium capitalize ${getStatusStyles(status)}`}
    >
      {status}
    </span>
  );
}

function SecurityPageSkeleton() {
  return (
    <div className="space-y-6">
      {/* Header Skeleton */}
      <div className="space-y-2">
        <div className="animate-pulse bg-gray-200 dark:bg-gray-700 h-8 w-48 rounded"></div>
        <div className="animate-pulse bg-gray-200 dark:bg-gray-700 h-4 w-96 rounded"></div>
      </div>

      {/* Stats Cards Skeleton */}
      <div className="grid grid-cols-1 gap-5 sm:grid-cols-2 lg:grid-cols-4">
        {[...Array(4)].map((_, i) => (
          <div
            key={i}
            className="bg-white dark:bg-gray-800 p-6 rounded-lg border border-gray-200 dark:border-gray-700 shadow-sm"
          >
            <div className="flex items-center">
              <div className="flex-shrink-0">
                <div className="animate-pulse bg-gray-200 dark:bg-gray-700 h-6 w-6 rounded"></div>
              </div>
              <div className="ml-5 w-0 flex-1">
                <div className="space-y-2">
                  <div className="animate-pulse bg-gray-200 dark:bg-gray-700 h-4 w-24 rounded"></div>
                  <div className="flex items-baseline gap-2">
                    <div className="animate-pulse bg-gray-200 dark:bg-gray-700 h-8 w-16 rounded"></div>
                    <div className="animate-pulse bg-gray-200 dark:bg-gray-700 h-4 w-12 rounded"></div>
                  </div>
                </div>
              </div>
            </div>
          </div>
        ))}
      </div>

      {/* Security Events Table Skeleton */}
      <div className="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 shadow-sm">
        <div className="p-6 border-b border-gray-200 dark:border-gray-700">
          <div className="flex items-center justify-between">
            <div className="animate-pulse bg-gray-200 dark:bg-gray-700 h-6 w-48 rounded"></div>
            <div className="animate-pulse bg-gray-200 dark:bg-gray-700 h-5 w-5 rounded"></div>
          </div>
        </div>
        <div className="overflow-x-auto">
          <table className="min-w-full divide-y divide-gray-200 dark:divide-gray-700">
            <thead className="bg-gray-50 dark:bg-gray-800">
              <tr>
                <th className="px-6 py-3">
                  <div className="animate-pulse bg-gray-200 dark:bg-gray-700 h-4 w-16 rounded"></div>
                </th>
                <th className="px-6 py-3">
                  <div className="animate-pulse bg-gray-200 dark:bg-gray-700 h-4 w-24 rounded"></div>
                </th>
                <th className="px-6 py-3">
                  <div className="animate-pulse bg-gray-200 dark:bg-gray-700 h-4 w-20 rounded"></div>
                </th>
                <th className="px-6 py-3">
                  <div className="animate-pulse bg-gray-200 dark:bg-gray-700 h-4 w-16 rounded"></div>
                </th>
                <th className="px-6 py-3">
                  <div className="animate-pulse bg-gray-200 dark:bg-gray-700 h-4 w-20 rounded"></div>
                </th>
              </tr>
            </thead>
            <tbody className="bg-white dark:bg-gray-900 divide-y divide-gray-200 dark:divide-gray-700">
              {[...Array(8)].map((_, rowIndex) => (
                <tr key={rowIndex}>
                  <td className="px-6 py-4">
                    <div className="animate-pulse bg-gray-200 dark:bg-gray-700 h-6 w-20 rounded-full"></div>
                  </td>
                  <td className="px-6 py-4">
                    <div className="animate-pulse bg-gray-200 dark:bg-gray-700 h-4 w-32 rounded"></div>
                  </td>
                  <td className="px-6 py-4">
                    <div className="animate-pulse bg-gray-200 dark:bg-gray-700 h-4 w-24 rounded"></div>
                  </td>
                  <td className="px-6 py-4">
                    <div className="animate-pulse bg-gray-200 dark:bg-gray-700 h-6 w-16 rounded-full"></div>
                  </td>
                  <td className="px-6 py-4">
                    <div className="animate-pulse bg-gray-200 dark:bg-gray-700 h-4 w-20 rounded"></div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  );
}

function ErrorDisplay({
  message,
  onRetry,
}: {
  message: string;
  onRetry: () => void;
}) {
  const is403 = message.includes("403");

  return (
    <div className="flex items-center justify-center min-h-[400px]">
      <div className="flex flex-col items-center gap-4 max-w-md text-center px-4">
        <Shield
          className={`h-16 w-16 ${is403 ? "text-amber-500" : "text-red-500"}`}
        />
        <div className="space-y-2">
          <h3 className="text-2xl font-bold text-gray-900 dark:text-gray-100">
            {is403 ? "Access Restricted" : "Failed to Load Security Data"}
          </h3>
          {is403 ? (
            <div className="space-y-3">
              <p className="text-base text-gray-600 dark:text-gray-400">
                Security monitoring is only available to <strong>Admin</strong>{" "}
                and <strong>Manager</strong> roles.
              </p>
              <p className="text-sm text-gray-500 dark:text-gray-500">
                To view security threats, incidents, and metrics, please contact
                your organization administrator to upgrade your account
                permissions.
              </p>
            </div>
          ) : (
            <p className="text-sm text-gray-500 dark:text-gray-400">
              {message}
            </p>
          )}
        </div>
        {!is403 && (
          <button
            onClick={onRetry}
            className="px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 transition-colors"
          >
            Retry
          </button>
        )}
      </div>
    </div>
  );
}

export default function SecurityPage() {
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [threats, setThreats] = useState<SecurityThreat[]>([]);
  const [metrics, setMetrics] = useState<any>(null);
  const [alerts, setAlerts] = useState<any[]>([]);
  const [alertCounts, setAlertCounts] = useState({ all: 0, acknowledged: 0, unacknowledged: 0 });
  const [auditLogs, setAuditLogs] = useState<any[]>([]);
  const [agents, setAgents] = useState<any[]>([]);
  const [mcpServers, setMcpServers] = useState<any[]>([]);
  const [verificationEvents, setVerificationEvents] = useState<any[]>([]);

  // Filter states
  const [severityFilter, setSeverityFilter] = useState<string>("all");
  const [statusFilter, setStatusFilter] = useState<string>("all");
  const [targetTypeFilter, setTargetTypeFilter] = useState<string>("all");
  const [showFilters, setShowFilters] = useState(false);

  // Modal states
  const [selectedThreat, setSelectedThreat] = useState<SecurityThreat | null>(
    null
  );
  const [showThreatModal, setShowThreatModal] = useState(false);

  const fetchSecurityData = async () => {
    try {
      setLoading(true);
      setError(null);
      const [threatsData, metricsData, alertsData, auditData, agentsData, mcpData, verificationData] = await Promise.all([
        api.getSecurityThreats(),
        api.getSecurityMetrics(),
        api.getAlerts(10, 0),
        api.getAuditLogs(100, 0),
        api.listAgents().catch(() => ({ agents: [] })),
        api.listMCPServers(50, 0).catch(() => ({ mcpServers: [] })),
        api.getRecentVerificationEvents(60).catch(() => ({ events: [] })), // Last 60 minutes
      ]);
      setThreats(threatsData.threats || []);
      setMetrics(metricsData);
      setAlerts(alertsData.alerts || []);
      setAlertCounts({
        all: alertsData.allCount || 0,
        acknowledged: alertsData.acknowledgedCount || 0,
        unacknowledged: alertsData.unacknowledgedCount || 0,
      });
      setAuditLogs(auditData || []);
      setAgents(agentsData.agents || []);
      setMcpServers(mcpData.mcpServers || []);
      setVerificationEvents(verificationData.events || []);
    } catch (err) {
      console.error("Failed to fetch security data:", err);
      const errorMessage = getErrorMessage(err, {
        resource: "security data",
        action: "load",
      });
      setError(errorMessage);
    } finally {
      setLoading(false);
    }
  };

  // Filter threats based on selected filters
  const filteredThreats = threats.filter((threat) => {
    if (severityFilter !== "all" && threat.severity !== severityFilter) return false;
    if (statusFilter !== "all") {
      const isResolved = threat.isBlocked || !!threat.resolvedAt;
      if (statusFilter === "active" && isResolved) return false;
      if (statusFilter === "resolved" && !isResolved) return false;
    }
    if (targetTypeFilter !== "all") {
      const targetType = threat.targetType?.toLowerCase() || "agent";
      if (targetTypeFilter === "agent" && (targetType === "mcp_server" || targetType === "mcp")) return false;
      if (targetTypeFilter === "mcp" && targetType !== "mcp_server" && targetType !== "mcp") return false;
    }
    return true;
  });

  const activeFiltersCount = [severityFilter, statusFilter, targetTypeFilter].filter(f => f !== "all").length;

  const clearAllFilters = () => {
    setSeverityFilter("all");
    setStatusFilter("all");
    setTargetTypeFilter("all");
  };

  useEffect(() => {
    fetchSecurityData();
  }, []);

  // Create unified security activities from multiple data sources
  const getUnifiedSecurityActivities = (): SecurityActivity[] => {
    const activities: SecurityActivity[] = [];

    // Add agent activities (recent creations/updates)
    agents.forEach((agent: any) => {
      // Agent creation
      if (agent.createdAt) {
        const isRecent = new Date(agent.createdAt) > new Date(Date.now() - 7 * 24 * 60 * 60 * 1000); // Last 7 days
        if (isRecent) {
          activities.push({
            id: `agent-created-${agent.id}`,
            type: 'agent_created',
            title: 'Agent Registered',
            description: `Agent "${agent.displayName || agent.name}" was registered`,
            entityType: 'agent',
            entityId: agent.id,
            entityName: agent.displayName || agent.name,
            severity: 'success',
            timestamp: agent.createdAt,
            metadata: { status: agent.status, trustScore: agent.trustScore },
          });
        }
      }
      // Agent verification
      if (agent.status === 'verified' && agent.updatedAt) {
        const isRecent = new Date(agent.updatedAt) > new Date(Date.now() - 7 * 24 * 60 * 60 * 1000);
        if (isRecent && agent.createdAt !== agent.updatedAt) {
          activities.push({
            id: `agent-verified-${agent.id}`,
            type: 'agent_verified',
            title: 'Agent Verified',
            description: `Agent "${agent.displayName || agent.name}" was verified`,
            entityType: 'agent',
            entityId: agent.id,
            entityName: agent.displayName || agent.name,
            severity: 'success',
            timestamp: agent.updatedAt,
            metadata: { trustScore: agent.trustScore },
          });
        }
      }
      // Agent suspended
      if (agent.status === 'suspended' && agent.updatedAt) {
        activities.push({
          id: `agent-suspended-${agent.id}`,
          type: 'agent_suspended',
          title: 'Agent Suspended',
          description: `Agent "${agent.displayName || agent.name}" was suspended`,
          entityType: 'agent',
          entityId: agent.id,
          entityName: agent.displayName || agent.name,
          severity: 'warning',
          timestamp: agent.updatedAt,
        });
      }
    });

    // Add MCP server activities
    mcpServers.forEach((mcp: any) => {
      // MCP registration
      if (mcp.createdAt) {
        const isRecent = new Date(mcp.createdAt) > new Date(Date.now() - 7 * 24 * 60 * 60 * 1000);
        if (isRecent) {
          activities.push({
            id: `mcp-registered-${mcp.id}`,
            type: 'mcp_registered',
            title: 'MCP Server Registered',
            description: `MCP server "${mcp.name}" was registered`,
            entityType: 'mcp',
            entityId: mcp.id,
            entityName: mcp.name,
            severity: 'success',
            timestamp: mcp.createdAt,
            metadata: { url: mcp.url, status: mcp.status },
          });
        }
      }
      // MCP verification
      if (mcp.isVerified && mcp.lastVerifiedAt) {
        const isRecent = new Date(mcp.lastVerifiedAt) > new Date(Date.now() - 7 * 24 * 60 * 60 * 1000);
        if (isRecent) {
          activities.push({
            id: `mcp-verified-${mcp.id}`,
            type: 'mcp_verified',
            title: 'MCP Server Verified',
            description: `MCP server "${mcp.name}" verification completed`,
            entityType: 'mcp',
            entityId: mcp.id,
            entityName: mcp.name,
            severity: 'success',
            timestamp: mcp.lastVerifiedAt,
          });
        }
      }
    });

    // Add verification events
    verificationEvents.forEach((event: any) => {
      const isSuccess = event.status === 'success' || event.status === 'approved';
      activities.push({
        id: `verification-${event.id}`,
        type: isSuccess ? 'verification_success' : 'verification_failed',
        title: isSuccess ? 'Verification Passed' : 'Verification Failed',
        description: `${event.verificationType || 'Verification'} for "${event.agentName || 'Agent'}"`,
        entityType: 'verification',
        entityId: event.agentId,
        entityName: event.agentName || 'Unknown Agent',
        severity: isSuccess ? 'success' : 'error',
        timestamp: event.createdAt || event.completedAt || event.startedAt,
        metadata: {
          trustScore: event.trustScore,
          durationMs: event.durationMs,
          verificationType: event.verificationType,
        },
      });
    });

    // Add meaningful audit log activities
    const meaningfulActions = ['create', 'verify', 'suspend', 'reactivate', 'revoke', 'grant', 'attest'];
    const meaningfulResourceTypes = ['agent', 'mcp_server', 'api_key', 'agent_capability'];

    auditLogs
      .filter((log: any) =>
        meaningfulActions.includes(log.action) ||
        meaningfulResourceTypes.includes(log.resourceType)
      )
      .forEach((log: any) => {
        const actionMap: Record<string, { title: string; severity: 'info' | 'success' | 'warning' | 'error' }> = {
          'create': { title: 'Created', severity: 'success' },
          'verify': { title: 'Verified', severity: 'success' },
          'attest': { title: 'Attested', severity: 'success' },
          'grant': { title: 'Capability Granted', severity: 'info' },
          'revoke': { title: 'Revoked', severity: 'warning' },
          'suspend': { title: 'Suspended', severity: 'warning' },
          'reactivate': { title: 'Reactivated', severity: 'success' },
          'delete': { title: 'Deleted', severity: 'error' },
        };

        const actionInfo = actionMap[log.action] || { title: log.action, severity: 'info' as const };
        const resourceTypeName = log.resourceType?.replace(/_/g, ' ') || 'Resource';

        activities.push({
          id: `audit-${log.id}`,
          type: 'audit_action',
          title: `${actionInfo.title}`,
          description: `${resourceTypeName} ${actionInfo.title.toLowerCase()}`,
          entityType: log.resourceType === 'mcp_server' ? 'mcp' : log.resourceType === 'api_key' ? 'api_key' : log.resourceType === 'agent_capability' ? 'capability' : 'agent',
          entityId: log.resourceId,
          entityName: log.agentName || log.resourceId?.substring(0, 8) || 'Unknown',
          severity: actionInfo.severity,
          timestamp: log.createdAt || log.timestamp,
          metadata: {
            performedBy: log.userName || log.userId,
            action: log.action,
            resourceType: log.resourceType,
          },
        });
      });

    // Sort by timestamp (most recent first) and deduplicate by similar activities
    const sortedActivities = activities.sort((a, b) =>
      new Date(b.timestamp).getTime() - new Date(a.timestamp).getTime()
    );

    // Deduplicate: remove audit logs that duplicate agent/MCP/verification events
    const seen = new Set<string>();
    return sortedActivities.filter(activity => {
      // For audit actions, check if we already have a specific event for this
      if (activity.type === 'audit_action' && activity.entityId) {
        const key = `${activity.entityType}-${activity.entityId}`;
        if (seen.has(key)) return false;
      }
      // Mark entity as seen for non-audit activities
      if (activity.type !== 'audit_action' && activity.entityId) {
        seen.add(`${activity.entityType}-${activity.entityId}`);
      }
      return true;
    });
  };

  const securityActivities = getUnifiedSecurityActivities();

  // Handler for viewing threat details
  const handleViewThreat = (threat: SecurityThreat) => {
    setSelectedThreat(threat);
    setShowThreatModal(true);
  };

  if (loading) {
    return <SecurityPageSkeleton />;
  }

  if (error && !metrics) {
    return <ErrorDisplay message={error} onRetry={fetchSecurityData} />;
  }

  const stats = [
    {
      name: "Active Threats",
      value: metrics?.activeThreats?.toLocaleString() || "0",
      icon: AlertOctagon,
    },
    {
      name: "Unacknowledged Alerts",
      value: alertCounts.unacknowledged.toLocaleString(),
      icon: Bell,
    },
    {
      name: "Blocked Threats",
      value: metrics?.blockedThreats?.toLocaleString() || "0",
      icon: CheckCircle,
    },
    {
      name: "Total Threats",
      value: metrics?.totalThreats?.toLocaleString() || "0",
      icon: AlertTriangle,
    },
  ];

  return (
    <AuthGuard>
      <div className="space-y-6">
        {/* Header */}
        <div>
          <h1 className="text-2xl font-bold text-gray-900 dark:text-white">
            Security Dashboard
          </h1>
        </div>

      <div className="grid grid-cols-1 gap-5 sm:grid-cols-2 lg:grid-cols-4">
        {stats.map((stat) => (
          <StatCard key={stat.name} stat={stat} />
        ))}
      </div>

      {/* Charts */}
      <div className="grid grid-cols-1 gap-6 lg:grid-cols-2">
        {/* Threat Trend */}
        <div className="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 shadow-sm p-6">
          <div className="flex items-center justify-between mb-4">
            <h3 className="text-lg font-medium text-gray-900 dark:text-gray-100">
              Threat Trend (30 Days)
            </h3>
            <TrendingUp className="h-5 w-5 text-gray-400" />
          </div>
          <div className="h-64">
            <ResponsiveContainer width="100%" height="100%">
              <LineChart data={metrics?.threatTrend || []}>
                <defs>
                  <linearGradient id="threatGradient" x1="0" y1="0" x2="0" y2="1">
                    <stop offset="5%" stopColor="#ef4444" stopOpacity={0.3}/>
                    <stop offset="95%" stopColor="#ef4444" stopOpacity={0}/>
                  </linearGradient>
                </defs>
                <CartesianGrid
                  strokeDasharray="3 3"
                  className="stroke-gray-200 dark:stroke-gray-700"
                />
                <XAxis
                  dataKey="date"
                  className="text-xs text-gray-500 dark:text-gray-400"
                  stroke="#9CA3AF"
                  tick={{ fontSize: 11 }}
                />
                <YAxis
                  className="text-xs text-gray-500 dark:text-gray-400"
                  stroke="#9CA3AF"
                  tick={{ fontSize: 11 }}
                />
                <Tooltip
                  contentStyle={{
                    backgroundColor: "#fff",
                    border: "1px solid #e5e7eb",
                    borderRadius: "0.5rem",
                    boxShadow: "0 4px 6px -1px rgb(0 0 0 / 0.1)",
                  }}
                />
                <Line
                  type="monotone"
                  dataKey="count"
                  stroke="#ef4444"
                  strokeWidth={2}
                  name="Threats"
                  dot={{ fill: "#ef4444", strokeWidth: 2, r: 4 }}
                  activeDot={{ r: 6, fill: "#ef4444", stroke: "#fff", strokeWidth: 2 }}
                  fill="url(#threatGradient)"
                />
              </LineChart>
            </ResponsiveContainer>
          </div>
        </div>

        {/* Severity Distribution */}
        <div className="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 shadow-sm p-6">
          <div className="flex items-center justify-between mb-4">
            <h3 className="text-lg font-medium text-gray-900 dark:text-gray-100">
              Severity Distribution
            </h3>
            <Shield className="h-5 w-5 text-gray-400" />
          </div>
          <div className="h-64">
            <ResponsiveContainer width="100%" height="100%">
              <BarChart data={metrics?.severityDistribution || []}>
                <CartesianGrid
                  strokeDasharray="3 3"
                  className="stroke-gray-200 dark:stroke-gray-700"
                />
                <XAxis
                  dataKey="severity"
                  className="text-xs text-gray-500 dark:text-gray-400"
                  stroke="#9CA3AF"
                />
                <YAxis
                  className="text-xs text-gray-500 dark:text-gray-400"
                  stroke="#9CA3AF"
                />
                <Tooltip
                  contentStyle={{
                    backgroundColor: "#fff",
                    border: "1px solid #e5e7eb",
                    borderRadius: "0.5rem",
                    boxShadow: "0 1px 3px 0 rgb(0 0 0 / 0.1)",
                  }}
                />
                <Bar dataKey="count" name="Count" radius={[4, 4, 0, 0]}>
                  {(metrics?.severityDistribution || []).map(
                    (entry: any, index: number) => {
                      const colors: Record<string, string> = {
                        critical: "#dc2626",
                        Critical: "#dc2626",
                        high: "#f97316",
                        High: "#f97316",
                        medium: "#eab308",
                        Medium: "#eab308",
                        low: "#22c55e",
                        Low: "#22c55e",
                        warning: "#8b5cf6",
                        Warning: "#8b5cf6",
                      };
                      return (
                        <Cell
                          key={`cell-${index}`}
                          fill={colors[entry.severity] || "#6366f1"}
                        />
                      );
                    }
                  )}
                </Bar>
              </BarChart>
            </ResponsiveContainer>
          </div>
        </div>
      </div>

      {/* Recent Threats */}
      <div className="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 shadow-sm">
        <div className="p-6 border-b border-gray-200 dark:border-gray-700">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-3">
              <h3 className="text-lg font-medium text-gray-900 dark:text-gray-100">
                Recent Threats
              </h3>
              <span className="text-sm text-gray-500 dark:text-gray-400">
                ({filteredThreats.length} of {threats.length})
              </span>
            </div>
            <div className="flex items-center gap-2">
              <button
                onClick={() => setShowFilters(!showFilters)}
                className={`flex items-center gap-2 px-3 py-1.5 text-sm rounded-lg border transition-colors ${
                  showFilters || activeFiltersCount > 0
                    ? 'bg-blue-50 border-blue-200 text-blue-700 dark:bg-blue-900/20 dark:border-blue-800 dark:text-blue-400'
                    : 'bg-white border-gray-200 text-gray-700 hover:bg-gray-50 dark:bg-gray-800 dark:border-gray-700 dark:text-gray-300 dark:hover:bg-gray-700'
                }`}
              >
                <Filter className="h-4 w-4" />
                Filters
                {activeFiltersCount > 0 && (
                  <span className="flex items-center justify-center w-5 h-5 text-xs font-medium bg-blue-600 text-white rounded-full">
                    {activeFiltersCount}
                  </span>
                )}
              </button>
              {activeFiltersCount > 0 && (
                <button
                  onClick={clearAllFilters}
                  className="flex items-center gap-1 px-2 py-1.5 text-sm text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-300"
                >
                  <X className="h-4 w-4" />
                  Clear
                </button>
              )}
            </div>
          </div>

          {/* Filter Panel */}
          {showFilters && (
            <div className="mt-4 p-4 bg-gray-50 dark:bg-gray-900/50 rounded-lg border border-gray-200 dark:border-gray-700">
              <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
                {/* Severity Filter */}
                <div>
                  <label className="block text-xs font-medium text-gray-500 dark:text-gray-400 mb-1.5">
                    Severity
                  </label>
                  <select
                    value={severityFilter}
                    onChange={(e) => setSeverityFilter(e.target.value)}
                    className="w-full px-3 py-2 text-sm bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg focus:ring-2 focus:ring-blue-500 focus:border-transparent"
                  >
                    <option value="all">All Severities</option>
                    <option value="critical">Critical</option>
                    <option value="high">High</option>
                    <option value="medium">Medium</option>
                    <option value="low">Low</option>
                  </select>
                </div>

                {/* Status Filter */}
                <div>
                  <label className="block text-xs font-medium text-gray-500 dark:text-gray-400 mb-1.5">
                    Status
                  </label>
                  <select
                    value={statusFilter}
                    onChange={(e) => setStatusFilter(e.target.value)}
                    className="w-full px-3 py-2 text-sm bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg focus:ring-2 focus:ring-blue-500 focus:border-transparent"
                  >
                    <option value="all">All Status</option>
                    <option value="active">Active</option>
                    <option value="resolved">Resolved/Blocked</option>
                  </select>
                </div>

                {/* Target Type Filter */}
                <div>
                  <label className="block text-xs font-medium text-gray-500 dark:text-gray-400 mb-1.5">
                    Target Type
                  </label>
                  <select
                    value={targetTypeFilter}
                    onChange={(e) => setTargetTypeFilter(e.target.value)}
                    className="w-full px-3 py-2 text-sm bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg focus:ring-2 focus:ring-blue-500 focus:border-transparent"
                  >
                    <option value="all">All Types</option>
                    <option value="agent">Agents Only</option>
                    <option value="mcp">MCP Servers Only</option>
                  </select>
                </div>
              </div>
            </div>
          )}
        </div>
        <div className="overflow-hidden">
          <table className="w-full divide-y divide-gray-200 dark:divide-gray-700">
            <thead className="bg-gray-50 dark:bg-gray-800">
              <tr>
                <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider w-[30%]">
                  Threat Type
                </th>
                <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider w-[20%]">
                  Target
                </th>
                <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider w-[12%]">
                  Severity
                </th>
                <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider w-[12%]">
                  Status
                </th>
                <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider w-[18%]">
                  Detected At
                </th>
                <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider w-[8%]">
                  Actions
                </th>
              </tr>
            </thead>
            <tbody className="bg-white dark:bg-gray-900 divide-y divide-gray-200 dark:divide-gray-700">
              {filteredThreats.length === 0 && (
                <tr>
                  <td colSpan={6} className="px-4 py-8 text-center">
                    <AlertTriangle className="h-8 w-8 mx-auto mb-2 text-gray-300 dark:text-gray-600" />
                    <p className="text-sm text-gray-500 dark:text-gray-400">
                      {threats.length === 0 ? "No threats detected" : "No threats match the current filters"}
                    </p>
                    {activeFiltersCount > 0 && (
                      <button
                        onClick={clearAllFilters}
                        className="mt-2 text-sm text-blue-600 dark:text-blue-400 hover:underline"
                      >
                        Clear all filters
                      </button>
                    )}
                  </td>
                </tr>
              )}
              {filteredThreats?.slice(0, 10)?.map((threat) => (
                <tr
                  key={threat?.id}
                  className="hover:bg-gray-50 dark:hover:bg-gray-800 transition-colors"
                >
                  <td className="px-4 py-3">
                    <div className="text-sm font-medium text-gray-900 dark:text-gray-100 break-words">
                      {threat?.threatType}
                    </div>
                    <div className="text-xs text-gray-500 dark:text-gray-400 line-clamp-2">
                      {threat?.description}
                    </div>
                  </td>
                  <td className="px-4 py-3">
                    <div className="flex items-center gap-2">
                      <span className={`text-xs px-2 py-0.5 rounded-full ${
                        threat?.targetType === 'mcp_server' || threat?.targetType === 'mcp'
                          ? 'bg-purple-100 text-purple-700 dark:bg-purple-900/30 dark:text-purple-400'
                          : 'bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400'
                      }`}>
                        {threat?.targetType === 'mcp_server' || threat?.targetType === 'mcp' ? 'MCP' : 'Agent'}
                      </span>
                      <div
                        className="text-sm text-gray-900 dark:text-gray-100 truncate"
                        title={threat?.targetName || threat?.targetId}
                      >
                        {threat?.targetName ||
                          `${threat?.targetId?.substring(0, 8)}...`}
                      </div>
                    </div>
                  </td>
                  <td className="px-4 py-3 whitespace-nowrap">
                    <SeverityBadge severity={threat?.severity} />
                  </td>
                  <td className="px-4 py-3 whitespace-nowrap">
                    <StatusBadge
                      status={threat?.isBlocked ? "resolved" : "active"}
                    />
                  </td>
                  <td className="px-4 py-3 whitespace-nowrap">
                    <div className="text-xs text-gray-500 dark:text-gray-400">
                      {threat?.createdAt && formatDateTime(threat.createdAt)}
                    </div>
                  </td>
                  <td className="px-4 py-3 whitespace-nowrap">
                    <button
                      onClick={() => handleViewThreat(threat)}
                      className="p-1 text-gray-400 hover:text-blue-600 dark:hover:text-blue-400 transition-colors"
                      title="View threat details"
                    >
                      <Eye className="h-4 w-4" />
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>

      {/* Two-column layout for Alerts and Audit Activity */}
      <div className="grid grid-cols-1 gap-6 lg:grid-cols-2">
        {/* Recent Security Alerts */}
        <div className="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 shadow-sm">
          <div className="p-6 border-b border-gray-200 dark:border-gray-700">
            <div className="flex items-center justify-between">
              <div>
                <h3 className="text-lg font-medium text-gray-900 dark:text-gray-100">
                  Security Alerts
                </h3>
                <p className="text-xs text-gray-500 dark:text-gray-400 mt-1">
                  {alertCounts.unacknowledged} unacknowledged of {alertCounts.all} total
                </p>
              </div>
              <Link
                href="/dashboard/admin/alerts"
                className="flex items-center text-sm text-blue-600 dark:text-blue-400 hover:underline"
              >
                View all <ChevronRight className="h-4 w-4 ml-1" />
              </Link>
            </div>
          </div>
          <div className="divide-y divide-gray-200 dark:divide-gray-700">
            {alerts.length === 0 ? (
              <div className="p-8 text-center">
                <Bell className="h-12 w-12 mx-auto mb-3 text-gray-300 dark:text-gray-600" />
                <p className="text-sm text-gray-500 dark:text-gray-400">No security alerts</p>
              </div>
            ) : (
              alerts.slice(0, 5).map((alert) => (
                <div key={alert.id} className="p-4 hover:bg-gray-50 dark:hover:bg-gray-800/50 transition-colors">
                  <div className="flex items-start gap-3">
                    <div className={`mt-0.5 p-1.5 rounded-full ${
                      alert.severity === 'critical' ? 'bg-red-100 dark:bg-red-900/30' :
                      alert.severity === 'high' ? 'bg-orange-100 dark:bg-orange-900/30' :
                      alert.severity === 'medium' ? 'bg-yellow-100 dark:bg-yellow-900/30' :
                      'bg-blue-100 dark:bg-blue-900/30'
                    }`}>
                      <AlertTriangle className={`h-3.5 w-3.5 ${
                        alert.severity === 'critical' ? 'text-red-600 dark:text-red-400' :
                        alert.severity === 'high' ? 'text-orange-600 dark:text-orange-400' :
                        alert.severity === 'medium' ? 'text-yellow-600 dark:text-yellow-400' :
                        'text-blue-600 dark:text-blue-400'
                      }`} />
                    </div>
                    <div className="flex-1 min-w-0">
                      <div className="flex items-center gap-2">
                        <p className="text-sm font-medium text-gray-900 dark:text-gray-100 truncate">
                          {alert.title}
                        </p>
                        <SeverityBadge severity={alert.severity} />
                      </div>
                      <p className="text-xs text-gray-500 dark:text-gray-400 mt-0.5 line-clamp-1">
                        {alert.description}
                      </p>
                      <div className="flex items-center gap-2 mt-1">
                        <Clock className="h-3 w-3 text-gray-400" />
                        <span className="text-xs text-gray-400">
                          {formatDateTime(alert.createdAt)}
                        </span>
                        {!alert.isAcknowledged && (
                          <span className="text-xs px-1.5 py-0.5 bg-amber-100 dark:bg-amber-900/30 text-amber-700 dark:text-amber-400 rounded">
                            Unacknowledged
                          </span>
                        )}
                      </div>
                    </div>
                  </div>
                </div>
              ))
            )}
          </div>
        </div>

        {/* Agent/MCP Activities */}
        <div className="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 shadow-sm">
          <div className="p-6 border-b border-gray-200 dark:border-gray-700">
            <div className="flex items-center justify-between">
              <div>
                <h3 className="text-lg font-medium text-gray-900 dark:text-gray-100">
                  Agent & MCP Activities
                </h3>
                <p className="text-xs text-gray-500 dark:text-gray-400 mt-1">
                  {securityActivities.length > 0
                    ? `${securityActivities.length} recent activities`
                    : 'Recent agent and MCP server actions'}
                </p>
              </div>
              <Activity className="h-5 w-5 text-gray-400" />
            </div>
          </div>
          <div className="divide-y divide-gray-200 dark:divide-gray-700 max-h-[400px] overflow-y-auto">
            {securityActivities.length === 0 ? (
              <div className="p-8 text-center">
                <Bot className="h-12 w-12 mx-auto mb-3 text-gray-300 dark:text-gray-600" />
                <p className="text-sm text-gray-500 dark:text-gray-400">No recent activities</p>
                <p className="text-xs text-gray-400 dark:text-gray-500 mt-1">
                  Agent registrations, verifications, and MCP server activities will appear here
                </p>
              </div>
            ) : (
              securityActivities.slice(0, 10).map((activity) => {
                const severityStyles = {
                  success: {
                    bg: 'bg-green-100 dark:bg-green-900/30',
                    icon: 'text-green-600 dark:text-green-400',
                  },
                  error: {
                    bg: 'bg-red-100 dark:bg-red-900/30',
                    icon: 'text-red-600 dark:text-red-400',
                  },
                  warning: {
                    bg: 'bg-amber-100 dark:bg-amber-900/30',
                    icon: 'text-amber-600 dark:text-amber-400',
                  },
                  info: {
                    bg: 'bg-blue-100 dark:bg-blue-900/30',
                    icon: 'text-blue-600 dark:text-blue-400',
                  },
                };

                const style = severityStyles[activity.severity];
                const isMcp = activity.entityType === 'mcp';
                const isVerification = activity.entityType === 'verification';

                return (
                  <div key={activity.id} className="p-4 hover:bg-gray-50 dark:hover:bg-gray-800/50 transition-colors">
                    <div className="flex items-start gap-3">
                      <div className={`mt-0.5 p-1.5 rounded-full ${style.bg}`}>
                        {activity.severity === 'success' ? (
                          <CheckCircle className={`h-3.5 w-3.5 ${style.icon}`} />
                        ) : activity.severity === 'error' ? (
                          <XCircle className={`h-3.5 w-3.5 ${style.icon}`} />
                        ) : activity.severity === 'warning' ? (
                          <AlertCircle className={`h-3.5 w-3.5 ${style.icon}`} />
                        ) : (
                          <Activity className={`h-3.5 w-3.5 ${style.icon}`} />
                        )}
                      </div>
                      <div className="flex-1 min-w-0">
                        <div className="flex items-center gap-2 flex-wrap">
                          <span className={`text-xs px-2 py-0.5 rounded-full ${
                            isMcp
                              ? 'bg-purple-100 text-purple-700 dark:bg-purple-900/30 dark:text-purple-400'
                              : isVerification
                              ? 'bg-indigo-100 text-indigo-700 dark:bg-indigo-900/30 dark:text-indigo-400'
                              : 'bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400'
                          }`}>
                            {isMcp ? (
                              <span className="flex items-center gap-1">
                                <Server className="h-3 w-3" />
                                MCP
                              </span>
                            ) : isVerification ? (
                              <span className="flex items-center gap-1">
                                <Shield className="h-3 w-3" />
                                Verify
                              </span>
                            ) : (
                              <span className="flex items-center gap-1">
                                <Bot className="h-3 w-3" />
                                Agent
                              </span>
                            )}
                          </span>
                          <span className="text-sm font-medium text-gray-900 dark:text-gray-100 truncate">
                            {activity.title}
                          </span>
                        </div>
                        <p className="text-xs text-gray-500 dark:text-gray-400 mt-0.5 truncate">
                          {activity.entityName}
                          {activity.metadata?.trustScore && (
                            <span className="ml-2 text-gray-400">
                              Trust: {Math.round(activity.metadata.trustScore)}%
                            </span>
                          )}
                        </p>
                        <div className="flex items-center gap-2 mt-1">
                          <Clock className="h-3 w-3 text-gray-400" />
                          <span className="text-xs text-gray-400">
                            {formatDateTime(activity.timestamp)}
                          </span>
                          {activity.metadata?.verificationType && (
                            <span className="text-xs px-1.5 py-0.5 bg-gray-100 dark:bg-gray-700 text-gray-600 dark:text-gray-400 rounded">
                              {activity.metadata.verificationType}
                            </span>
                          )}
                          {activity.metadata?.durationMs && (
                            <span className="text-xs text-gray-400">
                              {activity.metadata.durationMs}ms
                            </span>
                          )}
                        </div>
                      </div>
                    </div>
                  </div>
                );
              })
            )}
          </div>
        </div>
      </div>

      {/* Recent Incidents - Hidden until incident management workflow is implemented */}
      {/*
      <div className="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 shadow-sm">
        <div className="p-6 border-b border-gray-200 dark:border-gray-700">
          <div className="flex items-center justify-between">
            <h3 className="text-lg font-medium text-gray-900 dark:text-gray-100">Security Incidents</h3>
            <AlertOctagon className="h-5 w-5 text-gray-400" />
          </div>
        </div>
        <div className="overflow-x-auto">
          <table className="min-w-full divide-y divide-gray-200 dark:divide-gray-700">
            <thead className="bg-gray-50 dark:bg-gray-800">
              <tr>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                  Title
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                  Severity
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                  Status
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                  Created At
                </th>
              </tr>
            </thead>
            <tbody className="bg-white dark:bg-gray-900 divide-y divide-gray-200 dark:divide-gray-700">
              {incidents.map((incident) => (
                <tr key={incident.id} className="hover:bg-gray-50 dark:hover:bg-gray-800 transition-colors">
                  <td className="px-6 py-4 whitespace-nowrap">
                    <div className="text-sm font-medium text-gray-900 dark:text-gray-100">
                      {incident.title}
                    </div>
                  </td>
                  <td className="px-6 py-4 whitespace-nowrap">
                    <SeverityBadge severity={incident.severity} />
                  </td>
                  <td className="px-6 py-4 whitespace-nowrap">
                    <StatusBadge status={incident.status} />
                  </td>
                  <td className="px-6 py-4 whitespace-nowrap">
                    <div className="text-sm text-gray-500 dark:text-gray-400">
                      {formatDateTime(incident.createdAt)}
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>
      */}

      {/* Modals */}
      <ThreatDetailModal
        isOpen={showThreatModal}
        onClose={() => {
          setShowThreatModal(false);
          setSelectedThreat(null);
        }}
        threat={selectedThreat}
      />
    </div>
    </AuthGuard>
  );
}
