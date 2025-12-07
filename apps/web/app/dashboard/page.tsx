"use client";

import { useState, useEffect, Suspense, useCallback } from "react";
import { useSearchParams, useRouter } from "next/navigation";
import Link from "next/link";
import { toast } from "sonner";
import {
  Shield,
  Users,
  Activity,
  TrendingUp,
  AlertTriangle,
  CheckCircle,
  Clock,
  Loader2,
  AlertCircle,
  ExternalLink,
  XCircle,
  Server,
  Bot,
  Key,
  FileCheck,
  ShieldCheck,
  ArrowRight,
  Eye,
} from "lucide-react";
import {
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
  BarChart,
  Bar,
  Legend,
  Cell,
  PieChart,
  Pie,
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
}

interface VerificationActivityData {
  period: string;
  activity: Array<{
    month: string;
    verified: number;
    pending: number;
    agentsVerified: number;
    agentsPending: number;
    mcpVerified: number;
    mcpPending: number;
    monthYear: string;
  }>;
  currentStats: {
    totalVerified: number;
    totalPending: number;
    totalAgents: number;
    totalMCP: number;
    agentsVerified: number;
    agentsPending: number;
    mcpVerified: number;
    mcpPending: number;
  };
}

interface MCPServerEntry {
  id: string;
  name: string;
  url: string;
  status: "active" | "inactive" | "pending" | "verified" | "suspended" | "revoked";
  isVerified?: boolean;
  confidenceScore?: number;
  attestationCount?: number;
  lastVerifiedAt?: string;
  createdAt: string;
}

// Compact stat card for key metrics
function KeyMetricCard({ label, value, icon: Icon, color, trend }: {
  label: string;
  value: string | number;
  icon: any;
  color: string;
  trend?: { value: string; positive: boolean };
}) {
  return (
    <div className="bg-white dark:bg-gray-800 p-4 rounded-lg border border-gray-200 dark:border-gray-700 shadow-sm">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <div className={`p-2 rounded-lg ${color}`}>
            <Icon className="h-5 w-5 text-white" />
          </div>
          <div>
            <p className="text-2xl font-bold text-gray-900 dark:text-white">{value}</p>
            <p className="text-xs text-gray-500 dark:text-gray-400">{label}</p>
          </div>
        </div>
        {trend && (
          <div className={`text-xs font-medium ${trend.positive ? "text-green-600" : "text-red-600"}`}>
            {trend.value}
          </div>
        )}
      </div>
    </div>
  );
}

function ErrorDisplay({ message, onRetry }: { message: string; onRetry: () => void }) {
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

function DashboardContent() {
  const searchParams = useSearchParams();
  const router = useRouter();
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [dashboardData, setDashboardData] = useState<DashboardStats | null>(null);
  const [verificationActivity, setVerificationActivity] = useState<VerificationActivityData | null>(null);
  const [activityLoading, setActivityLoading] = useState(true);
  const [userRole, setUserRole] = useState<UserRole>("viewer");
  const [mcpServers, setMcpServers] = useState<MCPServerEntry[]>([]);
  const [mcpLoading, setMcpLoading] = useState(true);

  useEffect(() => {
    const token = api.getToken();
    if (token) {
      try {
        const payload = JSON.parse(atob(token.split(".")[1]));
        setUserRole((payload.role as UserRole) || "viewer");
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
      const errorMessage = getErrorMessage(err, { resource: "dashboard data", action: "load" });
      setError(errorMessage);
    } finally {
      setLoading(false);
    }
  };

  const fetchVerificationActivity = async () => {
    try {
      setActivityLoading(true);
      const data = await api.getVerificationActivity(6);
      if (data && data.activity && data.activity.length > 0) {
        setVerificationActivity(data);
      } else {
        setVerificationActivity(null);
      }
    } catch (err) {
      console.error("Failed to fetch verification activity:", err);
      setVerificationActivity(null);
    } finally {
      setActivityLoading(false);
    }
  };

  const fetchMcpServers = async () => {
    try {
      setMcpLoading(true);
      const data = await api.listMCPServers(100, 0);
      setMcpServers(data.mcpServers || []);
    } catch (err) {
      console.error("Failed to fetch MCP servers:", err);
    } finally {
      setMcpLoading(false);
    }
  };

  useEffect(() => {
    const token = searchParams.get("token");
    if (token) {
      api.setToken(token);
      window.history.replaceState({}, "", "/dashboard");
    }
    fetchDashboardData();
    fetchVerificationActivity();
    fetchMcpServers();
  }, [searchParams]);

  if (loading) {
    return <DashboardSkeleton />;
  }

  if (error && !dashboardData) {
    return <ErrorDisplay message={error} onRetry={fetchDashboardData} />;
  }

  const data = dashboardData!;
  const permissions = getDashboardPermissions(userRole);

  // Calculate derived metrics
  const verifiedMcps = mcpServers.filter((m) => m.isVerified || m.status === "verified").length;
  const pendingMcps = mcpServers.filter((m) => m.status === "pending").length;
  const totalAttestations = mcpServers.reduce((sum, m) => sum + (m.attestationCount || 0), 0);

  // Verification status data for pie charts
  const agentStatusData = [
    { name: "Verified", value: data?.verifiedAgents || 0, color: "#22c55e" },
    { name: "Pending", value: data?.pendingAgents || 0, color: "#f59e0b" },
    { name: "Other", value: Math.max(0, (data?.totalAgents || 0) - (data?.verifiedAgents || 0) - (data?.pendingAgents || 0)), color: "#6b7280" },
  ].filter((d) => d.value > 0);

  const mcpStatusData = [
    { name: "Verified", value: verifiedMcps, color: "#22c55e" },
    { name: "Active", value: mcpServers.filter((m) => m.status === "active" && !m.isVerified).length, color: "#3b82f6" },
    { name: "Pending", value: pendingMcps, color: "#f59e0b" },
    { name: "Inactive", value: mcpServers.filter((m) => m.status === "inactive" || m.status === "suspended").length, color: "#ef4444" },
  ].filter((d) => d.value > 0);

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-gray-900 dark:text-white">
            Executive Dashboard
          </h1>
          <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">
            Security posture overview for agents and MCP servers
          </p>
        </div>
        <TimezoneIndicator />
      </div>

      {/* Security Alert Banner (if critical) */}
      {data?.criticalAlerts > 0 && (
        <div className="bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded-lg p-4">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-3">
              <AlertTriangle className="h-6 w-6 text-red-600" />
              <div>
                <p className="font-medium text-red-800 dark:text-red-200">
                  {data.criticalAlerts} Critical Security Alert{data.criticalAlerts > 1 ? "s" : ""}
                </p>
                <p className="text-sm text-red-600 dark:text-red-300">
                  Immediate attention required
                </p>
              </div>
            </div>
            <Link
              href="/dashboard/security"
              className="px-4 py-2 bg-red-600 text-white rounded-lg hover:bg-red-700 text-sm font-medium"
            >
              View Alerts
            </Link>
          </div>
        </div>
      )}

      {/* Key Metrics Row - 6 cards balanced between agents, MCPs, and security */}
      <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-6 gap-4">
        <KeyMetricCard
          label="Total Agents"
          value={data?.totalAgents || 0}
          icon={Bot}
          color="bg-blue-500"
        />
        <KeyMetricCard
          label="Verified Agents"
          value={data?.verifiedAgents || 0}
          icon={CheckCircle}
          color="bg-green-500"
        />
        <KeyMetricCard
          label="Avg Trust Score"
          value={data?.avgTrustScore ? `${(data.avgTrustScore * 100).toFixed(0)}%` : "0%"}
          icon={TrendingUp}
          color="bg-purple-500"
        />
        <KeyMetricCard
          label="MCP Servers"
          value={data?.totalMcpServers || 0}
          icon={Server}
          color="bg-indigo-500"
        />
        <KeyMetricCard
          label="Verified MCPs"
          value={verifiedMcps}
          icon={ShieldCheck}
          color="bg-emerald-500"
        />
        <KeyMetricCard
          label="Active Alerts"
          value={data?.activeAlerts || 0}
          icon={AlertTriangle}
          color={data?.activeAlerts > 0 ? "bg-yellow-500" : "bg-gray-400"}
        />
      </div>

      {/* Main Content Grid - Side by Side Agent vs MCP */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        {/* Agent Section */}
        <div className="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 shadow-sm">
          <div className="p-4 border-b border-gray-200 dark:border-gray-700">
            <div className="flex items-center justify-between">
              <h3 className="text-lg font-semibold text-gray-900 dark:text-white flex items-center gap-2">
                <Bot className="h-5 w-5 text-blue-500" />
                Agent Verification Status
              </h3>
              <Link href="/dashboard/agents" className="text-sm text-blue-600 hover:text-blue-700 flex items-center gap-1">
                View All <ArrowRight className="h-3 w-3" />
              </Link>
            </div>
          </div>
          <div className="p-4">
            <div className="flex items-center gap-6">
              {/* Pie Chart */}
              <div className="w-32 h-32 flex-shrink-0">
                {agentStatusData.length > 0 ? (
                  <ResponsiveContainer width="100%" height="100%">
                    <PieChart>
                      <Pie
                        data={agentStatusData}
                        cx="50%"
                        cy="50%"
                        innerRadius={30}
                        outerRadius={50}
                        paddingAngle={2}
                        dataKey="value"
                      >
                        {agentStatusData.map((entry, index) => (
                          <Cell key={`cell-${index}`} fill={entry.color} />
                        ))}
                      </Pie>
                      <Tooltip />
                    </PieChart>
                  </ResponsiveContainer>
                ) : (
                  <div className="w-full h-full flex items-center justify-center">
                    <Bot className="h-12 w-12 text-gray-300" />
                  </div>
                )}
              </div>
              {/* Stats */}
              <div className="flex-1 space-y-3">
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-2">
                    <div className="w-3 h-3 rounded-full bg-green-500"></div>
                    <span className="text-sm text-gray-600 dark:text-gray-400">Verified</span>
                  </div>
                  <span className="font-semibold text-gray-900 dark:text-white">{data?.verifiedAgents || 0}</span>
                </div>
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-2">
                    <div className="w-3 h-3 rounded-full bg-yellow-500"></div>
                    <span className="text-sm text-gray-600 dark:text-gray-400">Pending</span>
                  </div>
                  <span className="font-semibold text-gray-900 dark:text-white">{data?.pendingAgents || 0}</span>
                </div>
                <div className="flex items-center justify-between pt-2 border-t border-gray-200 dark:border-gray-700">
                  <span className="text-sm text-gray-600 dark:text-gray-400">Verification Rate</span>
                  <span className="font-semibold text-gray-900 dark:text-white">{data?.verificationRate?.toFixed(1) || 0}%</span>
                </div>
              </div>
            </div>
          </div>
        </div>

        {/* MCP Section */}
        <div className="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 shadow-sm">
          <div className="p-4 border-b border-gray-200 dark:border-gray-700">
            <div className="flex items-center justify-between">
              <h3 className="text-lg font-semibold text-gray-900 dark:text-white flex items-center gap-2">
                <Server className="h-5 w-5 text-indigo-500" />
                MCP Server Status
              </h3>
              <Link href="/dashboard/mcp" className="text-sm text-blue-600 hover:text-blue-700 flex items-center gap-1">
                View All <ArrowRight className="h-3 w-3" />
              </Link>
            </div>
          </div>
          <div className="p-4">
            <div className="flex items-center gap-6">
              {/* Pie Chart */}
              <div className="w-32 h-32 flex-shrink-0">
                {mcpLoading ? (
                  <div className="w-full h-full flex items-center justify-center">
                    <Loader2 className="h-8 w-8 animate-spin text-gray-400" />
                  </div>
                ) : mcpStatusData.length > 0 ? (
                  <ResponsiveContainer width="100%" height="100%">
                    <PieChart>
                      <Pie
                        data={mcpStatusData}
                        cx="50%"
                        cy="50%"
                        innerRadius={30}
                        outerRadius={50}
                        paddingAngle={2}
                        dataKey="value"
                      >
                        {mcpStatusData.map((entry, index) => (
                          <Cell key={`cell-${index}`} fill={entry.color} />
                        ))}
                      </Pie>
                      <Tooltip />
                    </PieChart>
                  </ResponsiveContainer>
                ) : (
                  <div className="w-full h-full flex items-center justify-center">
                    <Server className="h-12 w-12 text-gray-300" />
                  </div>
                )}
              </div>
              {/* Stats */}
              <div className="flex-1 space-y-3">
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-2">
                    <div className="w-3 h-3 rounded-full bg-green-500"></div>
                    <span className="text-sm text-gray-600 dark:text-gray-400">Verified</span>
                  </div>
                  <span className="font-semibold text-gray-900 dark:text-white">{verifiedMcps}</span>
                </div>
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-2">
                    <div className="w-3 h-3 rounded-full bg-blue-500"></div>
                    <span className="text-sm text-gray-600 dark:text-gray-400">Active</span>
                  </div>
                  <span className="font-semibold text-gray-900 dark:text-white">{data?.activeMcpServers || 0}</span>
                </div>
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-2">
                    <div className="w-3 h-3 rounded-full bg-yellow-500"></div>
                    <span className="text-sm text-gray-600 dark:text-gray-400">Pending</span>
                  </div>
                  <span className="font-semibold text-gray-900 dark:text-white">{pendingMcps}</span>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>

      {/* Verification Activity - Historical trend (Agents + MCP Servers) */}
      {permissions.canViewActivityChart && (
        <div className="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 shadow-sm p-6">
          <div className="flex items-center justify-between mb-4">
            <div>
              <h3 className="text-lg font-semibold text-gray-900 dark:text-white flex items-center gap-2">
                <FileCheck className="h-5 w-5 text-blue-500" />
                Verification Activity Trend
              </h3>
              <p className="text-xs text-gray-500 dark:text-gray-400 mt-1">
                Agents & MCP Servers Combined
              </p>
            </div>
            <div className="flex items-center gap-3">
              {verificationActivity?.currentStats && (
                <div className="flex items-center gap-4 text-xs">
                  <div className="flex items-center gap-1">
                    <Bot className="h-3 w-3 text-emerald-500" />
                    <span className="text-gray-600 dark:text-gray-400">
                      {verificationActivity.currentStats.agentsVerified || 0} agents
                    </span>
                  </div>
                  <div className="flex items-center gap-1">
                    <Server className="h-3 w-3 text-blue-500" />
                    <span className="text-gray-600 dark:text-gray-400">
                      {verificationActivity.currentStats.mcpVerified || 0} MCP
                    </span>
                  </div>
                </div>
              )}
              <span className="text-xs px-2 py-1 rounded-full bg-gray-100 dark:bg-gray-700 text-gray-600 dark:text-gray-400">
                Last 6 Months
              </span>
            </div>
          </div>
          <div className="h-64">
            {activityLoading ? (
              <ChartSkeleton />
            ) : verificationActivity && verificationActivity.activity.length > 0 ? (
              <ResponsiveContainer width="100%" height="100%">
                <BarChart data={verificationActivity.activity.slice(-6)} margin={{ top: 10, right: 10, left: 0, bottom: 5 }}>
                  <defs>
                    <linearGradient id="agentsVerifiedGradient" x1="0" y1="0" x2="0" y2="1">
                      <stop offset="0%" stopColor="#10b981" stopOpacity={1} />
                      <stop offset="100%" stopColor="#059669" stopOpacity={0.8} />
                    </linearGradient>
                    <linearGradient id="mcpVerifiedGradient" x1="0" y1="0" x2="0" y2="1">
                      <stop offset="0%" stopColor="#3b82f6" stopOpacity={1} />
                      <stop offset="100%" stopColor="#2563eb" stopOpacity={0.8} />
                    </linearGradient>
                    <linearGradient id="agentsPendingGradient" x1="0" y1="0" x2="0" y2="1">
                      <stop offset="0%" stopColor="#f59e0b" stopOpacity={1} />
                      <stop offset="100%" stopColor="#d97706" stopOpacity={0.8} />
                    </linearGradient>
                    <linearGradient id="mcpPendingGradient" x1="0" y1="0" x2="0" y2="1">
                      <stop offset="0%" stopColor="#8b5cf6" stopOpacity={1} />
                      <stop offset="100%" stopColor="#7c3aed" stopOpacity={0.8} />
                    </linearGradient>
                  </defs>
                  <CartesianGrid strokeDasharray="3 3" stroke="#e5e7eb" vertical={false} />
                  <XAxis dataKey="month" tick={{ fontSize: 12, fill: "#6b7280" }} axisLine={{ stroke: "#e5e7eb" }} tickLine={false} />
                  <YAxis tick={{ fontSize: 12, fill: "#6b7280" }} axisLine={false} tickLine={false} width={40} />
                  <Tooltip
                    cursor={{ fill: "rgba(0, 0, 0, 0.04)" }}
                    contentStyle={{
                      backgroundColor: "#fff",
                      border: "none",
                      borderRadius: "0.5rem",
                      boxShadow: "0 4px 6px -1px rgb(0 0 0 / 0.1)",
                    }}
                    formatter={(value: number, name: string) => {
                      const labels: Record<string, string> = {
                        agentsVerified: "Agents Verified",
                        mcpVerified: "MCP Verified",
                        agentsPending: "Agents Pending",
                        mcpPending: "MCP Pending",
                      };
                      return [value, labels[name] || name];
                    }}
                  />
                  <Legend
                    verticalAlign="top"
                    align="right"
                    wrapperStyle={{ paddingBottom: "10px" }}
                    formatter={(value: string) => {
                      const labels: Record<string, string> = {
                        agentsVerified: "Agents ✓",
                        mcpVerified: "MCP ✓",
                        agentsPending: "Agents ⏳",
                        mcpPending: "MCP ⏳",
                      };
                      return labels[value] || value;
                    }}
                  />
                  <Bar dataKey="agentsVerified" stackId="verified" fill="url(#agentsVerifiedGradient)" name="agentsVerified" radius={[0, 0, 0, 0]} maxBarSize={35} />
                  <Bar dataKey="mcpVerified" stackId="verified" fill="url(#mcpVerifiedGradient)" name="mcpVerified" radius={[4, 4, 0, 0]} maxBarSize={35} />
                  <Bar dataKey="agentsPending" stackId="pending" fill="url(#agentsPendingGradient)" name="agentsPending" radius={[0, 0, 0, 0]} maxBarSize={35} />
                  <Bar dataKey="mcpPending" stackId="pending" fill="url(#mcpPendingGradient)" name="mcpPending" radius={[4, 4, 0, 0]} maxBarSize={35} />
                </BarChart>
              </ResponsiveContainer>
            ) : (
              <div className="flex flex-col items-center justify-center h-full text-gray-500">
                <AlertCircle className="h-10 w-10 mb-2 text-gray-300" />
                <p className="text-sm">No verification activity data available</p>
              </div>
            )}
          </div>
        </div>
      )}

      {/* Security Overview and Quick Links */}
      <div className="grid grid-cols-1 lg:grid-cols-2 xl:grid-cols-4 gap-6">
        {/* Security Status */}
        {permissions.canViewSecurityMetrics && (
          <div className="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 shadow-sm">
            <div className="p-4 border-b border-gray-200 dark:border-gray-700">
              <h3 className="text-lg font-semibold text-gray-900 dark:text-white flex items-center gap-2">
                <Shield className="h-5 w-5 text-yellow-500" />
                Security Status
              </h3>
            </div>
            <div className="p-4 space-y-4">
              <div className="flex items-center justify-between p-3 rounded-lg bg-gray-50 dark:bg-gray-700/50">
                <span className="text-sm text-gray-600 dark:text-gray-400">System Status</span>
                {data?.criticalAlerts > 0 ? (
                  <span className="flex items-center gap-1 text-red-600 font-medium">
                    <XCircle className="h-4 w-4" /> Critical
                  </span>
                ) : data?.activeAlerts > 0 ? (
                  <span className="flex items-center gap-1 text-yellow-600 font-medium">
                    <AlertTriangle className="h-4 w-4" /> Warning
                  </span>
                ) : (
                  <span className="flex items-center gap-1 text-green-600 font-medium">
                    <CheckCircle className="h-4 w-4" /> Healthy
                  </span>
                )}
              </div>
              <div className="grid grid-cols-2 gap-3">
                <div className="text-center p-3 rounded-lg bg-gray-50 dark:bg-gray-700/50">
                  <p className="text-2xl font-bold text-gray-900 dark:text-white">{data?.activeAlerts || 0}</p>
                  <p className="text-xs text-gray-500">Active Alerts</p>
                </div>
                <div className="text-center p-3 rounded-lg bg-gray-50 dark:bg-gray-700/50">
                  <p className="text-2xl font-bold text-red-600">{data?.criticalAlerts || 0}</p>
                  <p className="text-xs text-gray-500">Critical</p>
                </div>
              </div>
              <Link
                href="/dashboard/security"
                className="block w-full py-2 text-center text-sm font-medium bg-blue-600 text-white rounded-lg hover:bg-blue-700"
              >
                View Security Dashboard
              </Link>
            </div>
          </div>
        )}

        {/* MCP Attestations */}
        <div className="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 shadow-sm">
          <div className="p-4 border-b border-gray-200 dark:border-gray-700">
            <h3 className="text-lg font-semibold text-gray-900 dark:text-white flex items-center gap-2">
              <FileCheck className="h-5 w-5 text-cyan-500" />
              MCP Attestations
            </h3>
          </div>
          <div className="p-4 space-y-4">
            <div className="flex items-center justify-between p-3 rounded-lg bg-gray-50 dark:bg-gray-700/50">
              <span className="text-sm text-gray-600 dark:text-gray-400">Attestation Status</span>
              {mcpServers.length > 0 ? (
                mcpServers.filter(m => (m.attestationCount || 0) > 0).length === mcpServers.length ? (
                  <span className="flex items-center gap-1 text-green-600 font-medium">
                    <CheckCircle className="h-4 w-4" /> All Attested
                  </span>
                ) : mcpServers.filter(m => (m.attestationCount || 0) > 0).length > 0 ? (
                  <span className="flex items-center gap-1 text-yellow-600 font-medium">
                    <Clock className="h-4 w-4" /> Partial
                  </span>
                ) : (
                  <span className="flex items-center gap-1 text-gray-500 font-medium">
                    <AlertCircle className="h-4 w-4" /> None
                  </span>
                )
              ) : (
                <span className="flex items-center gap-1 text-gray-500 font-medium">
                  <AlertCircle className="h-4 w-4" /> No MCPs
                </span>
              )}
            </div>
            <div className="grid grid-cols-2 gap-3">
              <div className="text-center p-3 rounded-lg bg-gray-50 dark:bg-gray-700/50">
                <p className="text-2xl font-bold text-cyan-600">{totalAttestations}</p>
                <p className="text-xs text-gray-500">Total Attestations</p>
              </div>
              <div className="text-center p-3 rounded-lg bg-gray-50 dark:bg-gray-700/50">
                <p className="text-2xl font-bold text-gray-900 dark:text-white">
                  {mcpServers.filter(m => (m.attestationCount || 0) > 0).length}/{mcpServers.length}
                </p>
                <p className="text-xs text-gray-500">MCPs Attested</p>
              </div>
            </div>
            <Link
              href="/dashboard/mcp"
              className="block w-full py-2 text-center text-sm font-medium bg-cyan-600 text-white rounded-lg hover:bg-cyan-700"
            >
              View MCP Servers
            </Link>
          </div>
        </div>

        {/* Quick Actions */}
        <div className="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 shadow-sm">
          <div className="p-4 border-b border-gray-200 dark:border-gray-700">
            <h3 className="text-lg font-semibold text-gray-900 dark:text-white flex items-center gap-2">
              <Activity className="h-5 w-5 text-blue-500" />
              Quick Actions
            </h3>
          </div>
          <div className="p-4 space-y-2">
            <Link
              href="/dashboard/agents"
              className="flex items-center justify-between p-3 rounded-lg hover:bg-gray-50 dark:hover:bg-gray-700 transition-colors"
            >
              <div className="flex items-center gap-3">
                <Bot className="h-5 w-5 text-blue-500" />
                <span className="text-sm font-medium text-gray-700 dark:text-gray-300">Manage Agents</span>
              </div>
              <ArrowRight className="h-4 w-4 text-gray-400" />
            </Link>
            <Link
              href="/dashboard/mcp"
              className="flex items-center justify-between p-3 rounded-lg hover:bg-gray-50 dark:hover:bg-gray-700 transition-colors"
            >
              <div className="flex items-center gap-3">
                <Server className="h-5 w-5 text-indigo-500" />
                <span className="text-sm font-medium text-gray-700 dark:text-gray-300">Manage MCP Servers</span>
              </div>
              <ArrowRight className="h-4 w-4 text-gray-400" />
            </Link>
            <Link
              href="/dashboard/admin/verifications"
              className="flex items-center justify-between p-3 rounded-lg hover:bg-gray-50 dark:hover:bg-gray-700 transition-colors"
            >
              <div className="flex items-center gap-3">
                <Eye className="h-5 w-5 text-green-500" />
                <span className="text-sm font-medium text-gray-700 dark:text-gray-300">Review Requests</span>
              </div>
              <ArrowRight className="h-4 w-4 text-gray-400" />
            </Link>
            <Link
              href="/dashboard/compliance"
              className="flex items-center justify-between p-3 rounded-lg hover:bg-gray-50 dark:hover:bg-gray-700 transition-colors"
            >
              <div className="flex items-center gap-3">
                <ShieldCheck className="h-5 w-5 text-purple-500" />
                <span className="text-sm font-medium text-gray-700 dark:text-gray-300">Compliance Reports</span>
              </div>
              <ArrowRight className="h-4 w-4 text-gray-400" />
            </Link>
          </div>
        </div>

        {/* Platform Summary */}
        <div className="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 shadow-sm">
          <div className="p-4 border-b border-gray-200 dark:border-gray-700">
            <h3 className="text-lg font-semibold text-gray-900 dark:text-white flex items-center gap-2">
              <Users className="h-5 w-5 text-green-500" />
              Platform Summary
            </h3>
          </div>
          <div className="p-4 space-y-3">
            <div className="flex items-center justify-between py-2 border-b border-gray-100 dark:border-gray-700">
              <span className="text-sm text-gray-600 dark:text-gray-400">Total Users</span>
              <span className="font-semibold text-gray-900 dark:text-white">{data?.totalUsers || 0}</span>
            </div>
            <div className="flex items-center justify-between py-2 border-b border-gray-100 dark:border-gray-700">
              <span className="text-sm text-gray-600 dark:text-gray-400">Active Users</span>
              <span className="font-semibold text-green-600">{data?.activeUsers || 0}</span>
            </div>
            <div className="flex items-center justify-between py-2 border-b border-gray-100 dark:border-gray-700">
              <span className="text-sm text-gray-600 dark:text-gray-400">Total Agents</span>
              <span className="font-semibold text-gray-900 dark:text-white">{data?.totalAgents || 0}</span>
            </div>
            <div className="flex items-center justify-between py-2 border-b border-gray-100 dark:border-gray-700">
              <span className="text-sm text-gray-600 dark:text-gray-400">Total MCP Servers</span>
              <span className="font-semibold text-gray-900 dark:text-white">{data?.totalMcpServers || 0}</span>
            </div>
            <div className="flex items-center justify-between py-2">
              <span className="text-sm text-gray-600 dark:text-gray-400">Security Incidents</span>
              <span className={`font-semibold ${data?.securityIncidents > 0 ? "text-red-600" : "text-green-600"}`}>
                {data?.securityIncidents || 0}
              </span>
            </div>
          </div>
        </div>
      </div>

      {/* Activity Timeline */}
      <ActivityTimeline defaultLimit={10} />
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
