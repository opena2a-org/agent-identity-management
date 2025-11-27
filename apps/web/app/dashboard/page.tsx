"use client";

import { useState, useEffect, Suspense } from "react";
import { useSearchParams } from "next/navigation";
import {
  Shield,
  Users,
  Activity,
  TrendingUp,
  AlertTriangle,
  CheckCircle,
  Clock,
  Network,
  Loader2,
  AlertCircle,
} from "lucide-react";
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
} from "recharts";
import { api } from "@/lib/api";
import { getDashboardPermissions, type UserRole } from "@/lib/permissions";
import { TimezoneIndicator } from "@/components/timezone-indicator";
import { getErrorMessage } from "@/lib/error-messages";
import {
  DashboardSkeleton,
  ChartSkeleton,
} from "@/components/ui/content-loaders";
import { AuthGuard } from "@/components/auth-guard";
import { ActivityTimeline } from "@/components/analytics/activity-timeline";

interface DashboardStats {
  // Backend returns these exact fields (camelCase from Go JSON tags)
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
}

interface TrustScoreTrend {
  date: string;
  weekStart?: string; // Only for weekly data
  avgScore: number;
  agentCount: number;
}

interface TrustScoreTrendsData {
  period: string;
  currentAverage: number;
  dataType: "weekly" | "daily";
  trends: TrustScoreTrend[];
}

interface VerificationActivityData {
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
                  className={`ml-2 flex items-baseline text-sm font-semibold ${stat.changeType === "positive"
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

function ErrorDisplay({
  message,
  onRetry,
}: {
  message: string;
  onRetry: () => void;
}) {
  return (
    <div className="flex items-center justify-center min-h-[400px]">
      <div className="flex flex-col items-center gap-4 max-w-md text-center">
        <AlertCircle className="h-12 w-12 text-red-500" />
        <h3 className="text-lg font-semibold text-gray-900 dark:text-gray-100">
          Something went wrong!
        </h3>
        <p className="text-sm text-gray-500 dark:text-gray-400">{message}</p>
        <button
          onClick={onRetry}
          className="px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 transition-colors"
        >
          Retry
        </button>
      </div>
    </div>
  );
}

interface AuditLog {
  id: string;
  action: string;
  resourceType: string;
  resourceId: string;
  userId: string;
  metadata: any;
  timestamp: string;
}

function DashboardContent() {
  const searchParams = useSearchParams();
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [dashboardData, setDashboardData] = useState<DashboardStats | null>(
    null
  );
  const [verificationActivity, setVerificationActivity] =
    useState<VerificationActivityData | null>(null);
  const [activityLoading, setActivityLoading] = useState(true);
  const [userRole, setUserRole] = useState<UserRole>("viewer");
  const [auditLogs, setAuditLogs] = useState<AuditLog[]>([]);
  const [logsLoading, setLogsLoading] = useState(false);
  const [recentVerificationEvents, setRecentVerificationEvents] = useState<any[]>([]);
  const [currentUser, setCurrentUser] = useState<{
    id: string;
    email: string;
  } | null>(null);

  // Extract user info from JWT token
  useEffect(() => {
    const token = api.getToken();
    if (token) {
      try {
        const payload = JSON.parse(atob(token.split(".")[1]));
        setUserRole((payload.role as UserRole) || "viewer");
        // Extract user ID and email from JWT
        setCurrentUser({
          id: payload.sub || payload.userId || "",
          email: payload.email || "",
        });
      } catch (e) {
        console.error("Failed to decode JWT token:", e);
        setUserRole("viewer");
      }
    }
  }, []);

  const fetchDashboardData = async () => {
    try {
      setLoading(true);
      setError(null);
      const data = await api.getDashboardStats();
      setDashboardData(data);
    } catch (err) {
      console.error("Failed to fetch dashboard data:", err);
      const errorMessage = getErrorMessage(err, {
        resource: "dashboard data",
        action: "load",
      });
      setError(errorMessage);
    } finally {
      setLoading(false);
    }
  };


  const fetchVerificationActivity = async () => {
    try {
      setActivityLoading(true);
      console.log(
        "fetchVerificationActivity: Fetching verification activity data"
      );
      const data = await api.getVerificationActivity(6); // Request 6 months of data
      console.log("fetchVerificationActivity data:", data);

      // Check if we actually have valid data
      if (data && data.activity && data.activity.length > 0) {
        setVerificationActivity(data);
      } else {
        console.warn("No verification activity data returned from API");
        setVerificationActivity(null); // Set to null to show "no data" state
      }
    } catch (err) {
      console.error("Failed to fetch verification activity:", err);
      setVerificationActivity(null); // Set to null to show error state
    } finally {
      setActivityLoading(false);
    }
  };

  const fetchAuditLogs = async () => {
    try {
      setLogsLoading(true);
      // Fetch more logs to get past the excessive "view" actions
      // Most recent 50 logs are mostly "view + alerts" (automated polling)
      // Need to fetch 500+ to get interesting integration test data (create, verify, etc.)
      const logs = await api.getAuditLogs(500, 0);

      // Filter out ALL "view" actions - they're not meaningful for Recent Activity
      // Only show actual changes: create, update, delete, verify, grant, revoke, etc.
      const filtered = logs.filter((log: AuditLog) => {
        // Exclude ALL view actions completely
        if (log.action === "view") return false;

        // Also exclude automated system actions that aren't interesting
        if (log.resourceType === "dashboard_stats") return false;
        if (
          log.resourceType === "organization_settings" &&
          log.action === "view"
        )
          return false;

        // Keep only meaningful actions: create, update, delete, verify, grant, revoke, suspend, acknowledge
        return true;
      });

      // Sort by timestamp DESC (most recent first) to show latest activities
      const sorted = filtered.sort((a, b) => {
        return (
          new Date(b.timestamp).getTime() - new Date(a.timestamp).getTime()
        );
      });

      // Take first 10 most recent activities (agent/MCP creates, verifications, etc.)
      setAuditLogs(sorted.slice(0, 10));
    } catch (err) {
      console.error("Failed to fetch audit logs:", err);
      // Fail silently - keep empty array
    } finally {
      setLogsLoading(false);
    }
  };

  const fetchRecentVerificationEvents = async () => {
    try {
      // Fetch verification events from the last 15 minutes
      const data = await api.getRecentVerificationEvents(15);
      setRecentVerificationEvents(data.events || []);
    } catch (err) {
      console.error("Failed to fetch recent verification events:", err);
      // Fail silently - keep empty array
    }
  };

  useEffect(() => {
    // Check if OAuth returned with a token
    const token = searchParams.get("token");
    if (token) {
      api.setToken(token);
      // Clean up URL
      window.history.replaceState({}, "", "/dashboard");
    }
    console.log("runing useEffect");
    fetchDashboardData();
    fetchVerificationActivity();
    fetchAuditLogs();
    fetchRecentVerificationEvents();
  }, [searchParams]);

  if (loading) {
    return <DashboardSkeleton />;
  }

  if (error && !dashboardData) {
    return <ErrorDisplay message={error} onRetry={fetchDashboardData} />;
  }

  const data = dashboardData!;

  // Get role-based permissions
  const permissions = getDashboardPermissions(userRole);

  // Helper function to format audit log event name with entity details
  const formatEventName = (log: AuditLog): string => {
    const action = log.action.charAt(0).toUpperCase() + log.action.slice(1);
    const resource = log.resourceType.replace(/_/g, " ");

    // Extract entity name from metadata for more meaningful display
    let entityName = "";
    if (log.metadata) {
      // Try to get specific entity name from metadata
      entityName =
        log.metadata.agent_name ||
        log.metadata.server_name ||
        log.metadata.mcp_name ||
        log.metadata.key_name ||
        log.metadata.tag_name ||
        "";
    }

    // Format with entity name if available
    const entityDisplay = entityName ? ` "${entityName}"` : "";

    // Special handling for specific action types
    if (log.action === "view") {
      return `Viewed ${resource}${entityDisplay}`;
    } else if (log.action === "create") {
      return `Created ${resource}${entityDisplay}`;
    } else if (log.action === "verify") {
      return `Verified ${resource}${entityDisplay}`;
    } else if (log.action === "update") {
      return `Updated ${resource}${entityDisplay}`;
    } else if (log.action === "delete") {
      return `Deleted ${resource}${entityDisplay}`;
    } else if (log.action === "grant") {
      return `Granted ${resource}${entityDisplay}`;
    } else if (log.action === "revoke") {
      return `Revoked ${resource}${entityDisplay}`;
    } else if (log.action === "suspend") {
      return `Suspended ${resource}${entityDisplay}`;
    } else if (log.action === "acknowledge") {
      return `Acknowledged ${resource}${entityDisplay}`;
    } else if (log.resourceType === "agent_action") {
      // For agent actions, use the action name as the event
      return action.replace(/_/g, " ");
    }

    return `${action} ${resource}${entityDisplay}`;
  };

  // Helper function to get WHO initiated the action (user, agent, or MCP)
  const getInitiatedBy = (log: AuditLog): string => {
    // Check metadata for agent or MCP context
    if (log.metadata) {
      // If action was initiated by an agent
      if (log.metadata.registered_by_agent || log.metadata.acting_agent_name) {
        return `Agent: ${log.metadata.registered_by_agent || log.metadata.acting_agent_name}`;
      }
      // If action was initiated by an MCP server
      if (log.metadata.mcp_server || log.metadata.server_name) {
        return `MCP: ${log.metadata.mcp_server || log.metadata.server_name}`;
      }
      // If we have user email in metadata
      if (log.metadata.user_email) {
        return log.metadata.user_email;
      }
      // If we have display_name in metadata
      if (log.metadata.displayName) {
        return log.metadata.displayName;
      }
    }

    // Check if this is the current user and show their email
    if (log.userId && currentUser) {
      if (log.userId === currentUser.id) {
        return currentUser.email;
      }
    }

    // Fallback: show user ID if available
    if (log.userId) {
      const shortId = log.userId.split("-")[0];
      return `User ${shortId}`;
    }

    // Last resort
    return "System";
  };

  // Helper function to categorize the event type
  const getEventType = (log: AuditLog): string => {
    if (log.resourceType.includes("agent")) {
      return "Agent Management";
    } else if (log.resourceType.includes("mcp")) {
      return "MCP Servers";
    } else if (log.resourceType.includes("auth") || log.action === "login") {
      return "Authentication";
    } else if (
      log.resourceType.includes("alert") ||
      log.resourceType.includes("security")
    ) {
      return "Security";
    } else if (log.resourceType.includes("api_key")) {
      return "API Keys";
    } else if (log.resourceType.includes("user")) {
      return "User Management";
    } else if (log.action === "view") {
      return "View";
    }
    return "System";
  };

  // Helper function to format relative time
  const formatRelativeTime = (timestamp: string): string => {
    const now = new Date();
    const then = new Date(timestamp);
    const diffMs = now.getTime() - then.getTime();
    const diffSecs = Math.floor(diffMs / 1000);
    const diffMins = Math.floor(diffSecs / 60);
    const diffHours = Math.floor(diffMins / 60);
    const diffDays = Math.floor(diffHours / 24);

    if (diffSecs < 10) return "Just now";
    if (diffSecs < 60) return `${diffSecs} seconds ago`;
    if (diffMins < 60)
      return `${diffMins} minute${diffMins > 1 ? "s" : ""} ago`;
    if (diffHours < 24)
      return `${diffHours} hour${diffHours > 1 ? "s" : ""} ago`;
    if (diffDays < 7) return `${diffDays} day${diffDays > 1 ? "s" : ""} ago`;

    return then.toLocaleDateString();
  };

  // Prepare required stat cards
  const allStats = [
    {
      name: "Total Agents",
      value: data?.totalAgents?.toLocaleString() || 0,
      icon: Users,
      permission: "canViewAgentStats" as const,
    },
    {
      name: "Verified Agents",
      value: data?.verifiedAgents?.toLocaleString() || 0,
      icon: CheckCircle,
      permission: "canViewAgentStats" as const,
    },
    {
      name: "Trust Score Average",
      value: data?.avgTrustScore
        ? `${(data.avgTrustScore * 100).toFixed(0)}%`
        : "0%",
      icon: TrendingUp,
      permission: "canViewTrustScore" as const,
    },
    {
      name: "Recent Activity Count",
      value: recentVerificationEvents?.length?.toLocaleString() || 0,
      icon: Activity,
      permission: "canViewRecentActivity" as const,
    },
  ];

  // Filter stats based on role permissions
  const stats = allStats.filter((stat) => permissions[stat.permission]);

  return (
    <div className="space-y-6">
      {/* Header */}
      <div>
        <div className="flex items-center justify-between">
          <div>
            <h1 className="text-2xl font-bold text-gray-900 dark:text-white">
              Dashboard Overview
            </h1>
            <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">
              Monitor agent verification activities and system performance
              across all protocols.
            </p>
          </div>
          <TimezoneIndicator />
        </div>
      </div>

      {/* Stats */}
      <div className="grid grid-cols-1 gap-5 sm:grid-cols-2 lg:grid-cols-4">
        {stats.map((stat) => (
          <StatCard key={stat.name} stat={stat} />
        ))}
      </div>

      {/* Charts Row */}
      <div className="grid grid-cols-1 gap-6">{/* Trust Score Trend card removed (premium feature) */}

        {/* Agent Activity - All roles can see */}
        {permissions.canViewActivityChart && (
          <div className="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 shadow-sm p-6">
            <div className="flex items-center justify-between mb-4">
              <h3 className="text-lg font-medium text-gray-900 dark:text-gray-100">
                Agent Verification Activity
              </h3>
              <Activity className="h-5 w-5 text-gray-400" />
            </div>
            <div className="h-64">
              {activityLoading ? (
                <div className="space-y-3">
                  <div className="flex justify-between">
                    <div className="animate-pulse bg-gray-200 dark:bg-gray-700 h-4 w-16 rounded"></div>
                    <div className="animate-pulse bg-gray-200 dark:bg-gray-700 h-4 w-20 rounded"></div>
                  </div>
                  <div className="h-56 flex items-end justify-between gap-2">
                    {[
                      [90, 60],
                      [110, 80],
                      [70, 50],
                      [120, 90],
                      [100, 70],
                    ].map(([verifiedHeight, pendingHeight], i) => (
                      <div
                        key={i}
                        className="flex flex-col items-center gap-2 flex-1"
                      >
                        <div className="w-full flex gap-1">
                          <div
                            className="w-1/2 animate-pulse bg-gray-200 dark:bg-gray-700 rounded"
                            style={{ height: `${verifiedHeight}px` }}
                          />
                          <div
                            className="w-1/2 animate-pulse bg-gray-200 dark:bg-gray-700 rounded"
                            style={{ height: `${pendingHeight}px` }}
                          />
                        </div>
                        <div className="animate-pulse bg-gray-200 dark:bg-gray-700 h-3 w-12 rounded"></div>
                      </div>
                    ))}
                  </div>
                </div>
              ) : verificationActivity &&
                verificationActivity.activity &&
                verificationActivity.activity.length > 0 ? (
                <ResponsiveContainer width="100%" height="100%">
                  <BarChart
                    data={verificationActivity.activity.slice(-3)} // Show last 3 months
                  >
                    <CartesianGrid
                      strokeDasharray="3 3"
                      className="stroke-gray-200 dark:stroke-gray-700"
                    />
                    <XAxis
                      dataKey="month"
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
                      formatter={(value: number, name: string) => [
                        value,
                        name === "Verified"
                          ? "Verified Agents"
                          : "Pending Agents",
                      ]}
                      labelFormatter={(label) => `Month: ${label}`}
                    />
                    <Bar dataKey="verified" fill="#22c55e" name="Verified" />
                    <Bar dataKey="pending" fill="#eab308" name="Pending" />
                  </BarChart>
                </ResponsiveContainer>
              ) : (
                <div className="flex flex-col items-center justify-center h-full text-gray-500 dark:text-gray-400">
                  <AlertCircle className="h-12 w-12 mb-3 text-gray-300 dark:text-gray-600" />
                  <div className="text-center">
                    <p className="text-base font-medium mb-1">
                      No Activity Data
                    </p>
                    <p className="text-sm text-gray-400 dark:text-gray-500">
                      Verification activity will appear here once agents are
                      registered
                    </p>
                  </div>
                </div>
              )}
            </div>
            {/* Show activity stats */}
            {verificationActivity && verificationActivity.currentStats && (
              <div className="mt-2 text-xs text-gray-500 dark:text-gray-400 text-center">
                Total: {verificationActivity.currentStats.totalAgents} agents
                • Verified: {verificationActivity.currentStats.totalVerified}{" "}
                • Pending: {verificationActivity.currentStats.totalPending}
              </div>
            )}
          </div>
        )}
      </div>

      {/* Metrics Grid */}
      <div
        className={`grid grid-cols-1 gap-6 ${permissions.canViewSecurityMetrics ? "lg:grid-cols-3" : "lg:grid-cols-2"}`}
      >
        {/* Agent Metrics - All roles can see */}
        {permissions.canViewDetailedMetrics && (
          <div className="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 shadow-sm p-6">
            <h3 className="text-lg font-medium text-gray-900 dark:text-gray-100 mb-4 flex items-center gap-2">
              <Shield className="h-5 w-5 text-blue-500" />
              Agent Metrics
            </h3>
            <div className="space-y-3">
              <div className="flex justify-between items-center">
                <span className="text-sm text-gray-500 dark:text-gray-400">
                  Total Agents
                </span>
                <span className="text-sm font-medium text-gray-900 dark:text-gray-100">
                  {data?.totalAgents}
                </span>
              </div>
              <div className="flex justify-between items-center">
                <span className="text-sm text-gray-500 dark:text-gray-400">
                  Verified
                </span>
                <span className="text-sm font-medium text-green-600">
                  {data?.verifiedAgents}
                </span>
              </div>
              <div className="flex justify-between items-center">
                <span className="text-sm text-gray-500 dark:text-gray-400">
                  Pending
                </span>
                <span className="text-sm font-medium text-yellow-600">
                  {data?.pendingAgents}
                </span>
              </div>
              <div className="flex justify-between items-center pt-2 border-t border-gray-200 dark:border-gray-700">
                <span className="text-sm text-gray-500 dark:text-gray-400">
                  Verification Rate
                </span>
                <span className="text-sm font-medium text-gray-900 dark:text-gray-100">
                  {data?.verificationRate?.toFixed(1)}%
                </span>
              </div>
            </div>
          </div>
        )}

        {/* Security Metrics - Manager+ Only */}
        {permissions.canViewSecurityMetrics && (
          <div className="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 shadow-sm p-6">
            <h3 className="text-lg font-medium text-gray-900 dark:text-gray-100 mb-4 flex items-center gap-2">
              <AlertTriangle className="h-5 w-5 text-yellow-500" />
              Security Status
            </h3>
            <div className="space-y-3">
              <div className="flex justify-between items-center">
                <span className="text-sm text-gray-500 dark:text-gray-400">
                  Active Alerts
                </span>
                <span className="text-sm font-medium text-gray-900 dark:text-gray-100">
                  {data?.activeAlerts}
                </span>
              </div>
              <div className="flex justify-between items-center">
                <span className="text-sm text-gray-500 dark:text-gray-400">
                  Critical
                </span>
                <span className="text-sm font-medium text-red-600">
                  {data?.criticalAlerts}
                </span>
              </div>
              <div className="flex justify-between items-center">
                <span className="text-sm text-gray-500 dark:text-gray-400">
                  Incidents
                </span>
                <span className="text-sm font-medium text-gray-900 dark:text-gray-100">
                  {data?.securityIncidents}
                </span>
              </div>
              <div className="flex justify-between items-center pt-2 border-t border-gray-200 dark:border-gray-700">
                <span className="text-sm text-gray-500 dark:text-gray-400">
                  System Status
                </span>
                <div className="flex items-center gap-1">
                  <CheckCircle className="h-4 w-4 text-green-500" />
                  <span className="text-sm font-medium text-green-600">
                    Operational
                  </span>
                </div>
              </div>
            </div>
          </div>
        )}

        {/* Platform/MCP Metrics - All roles see this */}
        <div className="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 shadow-sm p-6">
          <h3 className="text-lg font-medium text-gray-900 dark:text-gray-100 mb-4 flex items-center gap-2">
            {permissions.canViewUserStats ? (
              <Users className="h-5 w-5 text-purple-500" />
            ) : (
              <Network className="h-5 w-5 text-purple-500" />
            )}
            {permissions.canViewUserStats ? "Platform Metrics" : "MCP Servers"}
          </h3>
          <div className="space-y-3">
            {permissions.canViewUserStats && (
              <>
                <div className="flex justify-between items-center">
                  <span className="text-sm text-gray-500 dark:text-gray-400">
                    Total Users
                  </span>
                  <span className="text-sm font-medium text-gray-900 dark:text-gray-100">
                    {data?.totalUsers}
                  </span>
                </div>
                <div className="flex justify-between items-center">
                  <span className="text-sm text-gray-500 dark:text-gray-400">
                    Active Users
                  </span>
                  <span className="text-sm font-medium text-green-600">
                    {data?.activeUsers}
                  </span>
                </div>
                <div className="flex justify-between items-center pt-2 border-t border-gray-200 dark:border-gray-700">
                  <span className="text-sm text-gray-500 dark:text-gray-400">
                    MCP Servers
                  </span>
                  <span className="text-sm font-medium text-gray-900 dark:text-gray-100">
                    {data?.totalMcpServers}
                  </span>
                </div>
              </>
            )}
            {!permissions.canViewUserStats && (
              <>
                <div className="flex justify-between items-center">
                  <span className="text-sm text-gray-500 dark:text-gray-400">
                    Total MCP Servers
                  </span>
                  <span className="text-sm font-medium text-gray-900 dark:text-gray-100">
                    {data.totalMcpServers}
                  </span>
                </div>
                <div className="flex justify-between items-center">
                  <span className="text-sm text-gray-500 dark:text-gray-400">
                    Active MCP Servers
                  </span>
                  <span className="text-sm font-medium text-green-600">
                    {data.activeMcpServers}
                  </span>
                </div>
                <div className="flex justify-between items-center pt-2 border-t border-gray-200 dark:border-gray-700">
                  <span className="text-sm text-gray-500 dark:text-gray-400">
                    Total Agents
                  </span>
                  <span className="text-sm font-medium text-gray-900 dark:text-gray-100">
                    {data?.totalAgents}
                  </span>
                </div>
              </>
            )}
            <div className="flex justify-between items-center">
              <span className="text-sm text-gray-500 dark:text-gray-400">
                {permissions.canViewUserStats
                  ? "Active MCPs"
                  : "Verified Agents"}
              </span>
              <span className="text-sm font-medium text-green-600">
                {permissions.canViewUserStats
                  ? data?.activeMcpServers
                  : data?.verifiedAgents}
              </span>
            </div>
          </div>
        </div>
      </div>

      {/* Recent Activity Table - All roles can see */}
      {permissions.canViewRecentActivity && (
        <div className="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 shadow-sm">
          <div className="p-6 border-b border-gray-200 dark:border-gray-700">
            <div className="flex items-center justify-between">
              <h3 className="text-lg font-medium text-gray-900 dark:text-gray-100">
                Recent Activity
              </h3>
              <Clock className="h-5 w-5 text-gray-400" />
            </div>
          </div>
          <div className="overflow-x-auto">
            <table className="min-w-full divide-y divide-gray-200 dark:divide-gray-700">
              <thead className="bg-gray-50 dark:bg-gray-800">
                <tr>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                    Event
                  </th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                    Initiated By
                  </th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                    Resource ID
                  </th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                    Type
                  </th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                    Status
                  </th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                    Timestamp
                  </th>
                </tr>
              </thead>
              <tbody className="bg-white dark:bg-gray-900 divide-y divide-gray-200 dark:divide-gray-700">
                {logsLoading ? (
                  [...Array(5)].map((_, i) => (
                    <tr key={i}>
                      <td className="px-6 py-4">
                        <div className="animate-pulse bg-gray-200 dark:bg-gray-700 h-4 w-32 rounded"></div>
                      </td>
                      <td className="px-6 py-4">
                        <div className="animate-pulse bg-gray-200 dark:bg-gray-700 h-4 w-24 rounded"></div>
                      </td>
                      <td className="px-6 py-4">
                        <div className="animate-pulse bg-gray-200 dark:bg-gray-700 h-4 w-28 rounded"></div>
                      </td>
                      <td className="px-6 py-4">
                        <div className="animate-pulse bg-gray-200 dark:bg-gray-700 h-6 w-20 rounded-full"></div>
                      </td>
                      <td className="px-6 py-4">
                        <div className="animate-pulse bg-gray-200 dark:bg-gray-700 h-6 w-16 rounded-full"></div>
                      </td>
                      <td className="px-6 py-4">
                        <div className="animate-pulse bg-gray-200 dark:bg-gray-700 h-4 w-20 rounded"></div>
                      </td>
                    </tr>
                  ))
                ) : auditLogs.length === 0 ? (
                  <tr>
                    <td colSpan={6} className="px-6 py-12 text-center">
                      <p className="text-sm text-gray-500 dark:text-gray-400">
                        No recent activity found
                      </p>
                    </td>
                  </tr>
                ) : (
                  auditLogs.map((log) => (
                    <tr
                      key={log.id}
                      className="hover:bg-gray-50 dark:hover:bg-gray-800 transition-colors"
                    >
                      <td className="px-6 py-4 whitespace-nowrap">
                        <div className="text-sm font-medium text-gray-900 dark:text-gray-100">
                          {formatEventName(log)}
                        </div>
                      </td>
                      <td className="px-6 py-4 whitespace-nowrap">
                        <div className="text-sm text-gray-700 dark:text-gray-300">
                          {getInitiatedBy(log)}
                        </div>
                      </td>
                      <td className="px-6 py-4">
                        <div className="text-xs font-mono text-gray-600 dark:text-gray-400 truncate max-w-[120px]" title={log.resourceId}>
                          {log.resourceId ? `${log.resourceId.substring(0, 8)}...` : 'N/A'}
                        </div>
                      </td>
                      <td className="px-6 py-4 whitespace-nowrap">
                        <span className="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-blue-100 dark:bg-blue-900/30 text-blue-800 dark:text-blue-300">
                          {getEventType(log)}
                        </span>
                      </td>
                      <td className="px-6 py-4 whitespace-nowrap">
                        <span className="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-green-100 dark:bg-green-900/30 text-green-800 dark:text-green-300">
                          ✓ success
                        </span>
                      </td>
                      <td className="px-6 py-4 whitespace-nowrap">
                        <div className="text-sm text-gray-500 dark:text-gray-400">
                          {formatRelativeTime(log.timestamp)}
                        </div>
                      </td>
                    </tr>
                  ))
                )}
              </tbody>
            </table>
          </div>
        </div>
      )}

      {/* Analytics Sections */}
      <div className="grid grid-cols-1 gap-6">
        <ActivityTimeline defaultLimit={20} />
      </div>
    </div>
  );
}

export default function DashboardPage() {
  return (
    <AuthGuard>
      <Suspense fallback={<DashboardSkeleton />}>
        <DashboardContent />
      </Suspense>
    </AuthGuard>
  );
}
