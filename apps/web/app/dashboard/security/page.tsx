"use client";

import { useState, useEffect, useMemo } from "react";
import {
  Shield,
  AlertTriangle,
  Activity,
  TrendingUp,
  Bot,
  Zap,
  Bell,
  CheckCircle,
  XCircle,
  Clock,
  ChevronRight,
  Filter,
  X,
  Eye,
  Server,
  FileWarning,
  Globe,
  Database,
  Mail,
  Folder,
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
  Area,
  ComposedChart,
} from "recharts";
import { api } from "@/lib/api";
import ThreatDetailModal from "@/components/modals/threat-detail-modal";
import { formatDateTime, formatRelativeTime } from "@/lib/date-utils";
import { getErrorMessage } from "@/lib/error-messages";
import { AuthGuard } from "@/components/auth-guard";

// ============================================
// TYPES
// ============================================

interface SecurityMetrics {
  securityScore: number;
  securityGrade: string;
  securityStatus: string;
  lastIncidentAt: string;
  actionsBlocked: number;
  actionsBlockedToday: number;
  agentsMonitored: number;
  agentsTrusted: number;
  trustPercentage: number;
  actionsToday: number;
  requiresAttention: number;
  averageTrustScore: number;
  // MCP Server metrics
  mcpServersTotal: number;
  mcpServersVerified: number;
  mcpTrustPercentage: number;
  protectionTimeline: Array<{ date: string; actions: number; blocked: number }>;
  riskByCategory: Array<{ category: string; blocked: number; riskLevel: string }>;
  recentBlockedActions: Array<{
    id: string;
    agentId: string;
    agentName: string;
    attemptedCapability: string;
    details: string;
    trustImpact: number;
    createdAt: string;
  }>;
  // Legacy fields
  threatTrend: Array<{ date: string; count: number }>;
  severityDistribution: Array<{ severity: string; count: number }>;
}

interface SecurityThreat {
  id: string;
  targetId: string;
  targetName?: string;
  threatType: string;
  severity: "low" | "medium" | "high" | "critical";
  description: string;
  isBlocked: boolean;
  createdAt: string;
  resolvedAt?: string;
  source?: string;
  targetType?: string;
  title?: string;
}

// ============================================
// COMPONENTS
// ============================================

function SecurityScoreGauge({ score, grade, status }: { score: number; grade: string; status: string }) {
  const circumference = 2 * Math.PI * 45;
  const strokeDashoffset = circumference - (score / 100) * circumference;

  const getScoreColor = () => {
    if (score >= 90) return { stroke: "#22c55e", bg: "bg-green-50 dark:bg-green-900/20", text: "text-green-600 dark:text-green-400" };
    if (score >= 80) return { stroke: "#84cc16", bg: "bg-lime-50 dark:bg-lime-900/20", text: "text-lime-600 dark:text-lime-400" };
    if (score >= 70) return { stroke: "#eab308", bg: "bg-yellow-50 dark:bg-yellow-900/20", text: "text-yellow-600 dark:text-yellow-400" };
    if (score >= 60) return { stroke: "#f97316", bg: "bg-orange-50 dark:bg-orange-900/20", text: "text-orange-600 dark:text-orange-400" };
    return { stroke: "#ef4444", bg: "bg-red-50 dark:bg-red-900/20", text: "text-red-600 dark:text-red-400" };
  };

  const colors = getScoreColor();

  return (
    <div className="relative flex items-center justify-center">
      <svg className="w-32 h-32 transform -rotate-90" viewBox="0 0 100 100">
        {/* Background circle */}
        <circle
          cx="50"
          cy="50"
          r="45"
          fill="none"
          stroke="currentColor"
          strokeWidth="8"
          className="text-gray-200 dark:text-gray-700"
        />
        {/* Progress circle */}
        <circle
          cx="50"
          cy="50"
          r="45"
          fill="none"
          stroke={colors.stroke}
          strokeWidth="8"
          strokeLinecap="round"
          strokeDasharray={circumference}
          strokeDashoffset={strokeDashoffset}
          className="transition-all duration-1000 ease-out"
        />
      </svg>
      <div className="absolute flex flex-col items-center">
        <span className={`text-3xl font-bold ${colors.text}`}>{score}</span>
        <span className="text-xs text-gray-500 dark:text-gray-400">/100</span>
      </div>
    </div>
  );
}

function StatCard({
  icon: Icon,
  label,
  value,
  subValue,
  trend,
  variant = "default"
}: {
  icon: any;
  label: string;
  value: string | number;
  subValue?: string;
  trend?: string;
  variant?: "default" | "success" | "warning" | "danger";
}) {
  const variantStyles = {
    default: "from-gray-50 to-white dark:from-gray-800 dark:to-gray-800 border-gray-200 dark:border-gray-700",
    success: "from-green-50 to-white dark:from-green-900/20 dark:to-gray-800 border-green-200 dark:border-green-800",
    warning: "from-amber-50 to-white dark:from-amber-900/20 dark:to-gray-800 border-amber-200 dark:border-amber-800",
    danger: "from-red-50 to-white dark:from-red-900/20 dark:to-gray-800 border-red-200 dark:border-red-800",
  };

  const iconStyles = {
    default: "text-gray-600 dark:text-gray-400",
    success: "text-green-600 dark:text-green-400",
    warning: "text-amber-600 dark:text-amber-400",
    danger: "text-red-600 dark:text-red-400",
  };

  return (
    <div className={`bg-gradient-to-br ${variantStyles[variant]} rounded-xl border p-5 shadow-sm hover:shadow-md transition-shadow`}>
      <div className="flex items-start justify-between">
        <div>
          <p className="text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wide">{label}</p>
          <p className="text-2xl font-bold text-gray-900 dark:text-white mt-1">
            {typeof value === 'number' ? value.toLocaleString() : value}
          </p>
          {subValue && (
            <p className="text-xs text-gray-500 dark:text-gray-400 mt-1">{subValue}</p>
          )}
        </div>
        <div className={`p-2 rounded-lg bg-white/50 dark:bg-gray-900/50 ${iconStyles[variant]}`}>
          <Icon className="h-5 w-5" />
        </div>
      </div>
      {trend && (
        <div className="mt-2 flex items-center text-xs">
          <TrendingUp className="h-3 w-3 mr-1 text-green-500" />
          <span className="text-green-600 dark:text-green-400">{trend}</span>
        </div>
      )}
    </div>
  );
}

function BlockedActionCard({ action }: { action: any }) {
  const getCategoryIcon = (capability: string) => {
    const cap = capability.toLowerCase();
    if (cap.includes("file")) return Folder;
    if (cap.includes("api") || cap.includes("http")) return Globe;
    if (cap.includes("database") || cap.includes("db")) return Database;
    if (cap.includes("email") || cap.includes("mail")) return Mail;
    return FileWarning;
  };

  const CategoryIcon = getCategoryIcon(action.attemptedCapability);

  return (
    <div className="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-4 hover:shadow-md transition-all">
      <div className="flex items-start gap-3">
        <div className="p-2 rounded-lg bg-red-100 dark:bg-red-900/30">
          <XCircle className="h-5 w-5 text-red-600 dark:text-red-400" />
        </div>
        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-2 flex-wrap">
            <span className="text-xs font-semibold px-2 py-0.5 rounded-full bg-red-100 dark:bg-red-900/50 text-red-700 dark:text-red-300">
              BLOCKED
            </span>
            <span className="text-sm font-medium text-gray-900 dark:text-white truncate">
              {action.agentName}
            </span>
          </div>
          <div className="flex items-center gap-2 mt-1.5">
            <CategoryIcon className="h-4 w-4 text-gray-400" />
            <span className="text-sm text-gray-700 dark:text-gray-300 font-mono">
              {action.attemptedCapability}
            </span>
          </div>
          <div className="flex items-center justify-between mt-2">
            <div className="flex items-center gap-3 text-xs text-gray-500 dark:text-gray-400">
              <span className="flex items-center gap-1">
                <Clock className="h-3 w-3" />
                {formatRelativeTime(action.createdAt)}
              </span>
              {action.trustImpact !== 0 && (
                <span className="text-red-600 dark:text-red-400">
                  Trust {action.trustImpact > 0 ? "+" : ""}{action.trustImpact}%
                </span>
              )}
            </div>
            <Link
              href={`/dashboard/agents?id=${action.agentId}`}
              className="text-xs text-blue-600 dark:text-blue-400 hover:underline flex items-center gap-1"
            >
              Review Agent <ChevronRight className="h-3 w-3" />
            </Link>
          </div>
        </div>
      </div>
    </div>
  );
}

function RiskCategoryBar({ category, blocked, riskLevel, maxBlocked }: {
  category: string;
  blocked: number;
  riskLevel: string;
  maxBlocked: number;
}) {
  const width = maxBlocked > 0 ? (blocked / maxBlocked) * 100 : 0;

  const riskColors = {
    high: "bg-red-500",
    medium: "bg-orange-500",
    low: "bg-yellow-500",
    secure: "bg-green-500",
  };

  const riskLabels = {
    high: { text: "High Risk", class: "text-red-600 dark:text-red-400" },
    medium: { text: "Medium", class: "text-orange-600 dark:text-orange-400" },
    low: { text: "Low", class: "text-yellow-600 dark:text-yellow-400" },
    secure: { text: "Secure", class: "text-green-600 dark:text-green-400" },
  };

  const label = riskLabels[riskLevel as keyof typeof riskLabels] || riskLabels.secure;

  return (
    <div className="space-y-1">
      <div className="flex items-center justify-between text-sm">
        <span className="text-gray-700 dark:text-gray-300">{category}</span>
        <div className="flex items-center gap-2">
          <span className="font-medium text-gray-900 dark:text-white">{blocked}</span>
          <span className={`text-xs ${label.class}`}>{label.text}</span>
        </div>
      </div>
      <div className="h-2 bg-gray-200 dark:bg-gray-700 rounded-full overflow-hidden">
        <div
          className={`h-full ${riskColors[riskLevel as keyof typeof riskColors] || riskColors.secure} rounded-full transition-all duration-500`}
          style={{ width: `${width}%` }}
        />
      </div>
    </div>
  );
}

function SecurityPageSkeleton() {
  return (
    <div className="space-y-6">
      {/* Hero Skeleton */}
      <div className="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-6">
        <div className="flex items-center gap-8">
          <div className="animate-pulse bg-gray-200 dark:bg-gray-700 h-32 w-32 rounded-full"></div>
          <div className="flex-1 space-y-3">
            <div className="animate-pulse bg-gray-200 dark:bg-gray-700 h-6 w-48 rounded"></div>
            <div className="animate-pulse bg-gray-200 dark:bg-gray-700 h-4 w-96 rounded"></div>
          </div>
        </div>
      </div>

      {/* Stats Skeleton */}
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
        {[...Array(4)].map((_, i) => (
          <div key={i} className="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-5">
            <div className="animate-pulse space-y-3">
              <div className="bg-gray-200 dark:bg-gray-700 h-4 w-24 rounded"></div>
              <div className="bg-gray-200 dark:bg-gray-700 h-8 w-16 rounded"></div>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}

function ErrorDisplay({ message, onRetry }: { message: string; onRetry: () => void }) {
  const is403 = message.includes("403");

  return (
    <div className="flex items-center justify-center min-h-[400px]">
      <div className="flex flex-col items-center gap-4 max-w-md text-center px-4">
        <Shield className={`h-16 w-16 ${is403 ? "text-amber-500" : "text-red-500"}`} />
        <div className="space-y-2">
          <h3 className="text-2xl font-bold text-gray-900 dark:text-gray-100">
            {is403 ? "Access Restricted" : "Failed to Load Security Data"}
          </h3>
          {is403 ? (
            <p className="text-base text-gray-600 dark:text-gray-400">
              Security monitoring is only available to Admin and Manager roles.
            </p>
          ) : (
            <p className="text-sm text-gray-500 dark:text-gray-400">{message}</p>
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

// ============================================
// MAIN PAGE
// ============================================

export default function SecurityPage() {
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [metrics, setMetrics] = useState<SecurityMetrics | null>(null);
  const [threats, setThreats] = useState<SecurityThreat[]>([]);
  const [alerts, setAlerts] = useState<any[]>([]);
  const [alertCounts, setAlertCounts] = useState({ all: 0, acknowledged: 0, unacknowledged: 0 });
  const [capabilityRequests, setCapabilityRequests] = useState<any[]>([]);

  // Modal states
  const [selectedThreat, setSelectedThreat] = useState<SecurityThreat | null>(null);
  const [showThreatModal, setShowThreatModal] = useState(false);

  const fetchSecurityData = async () => {
    try {
      setLoading(true);
      setError(null);
      const [metricsData, threatsData, alertsData, requestsData] = await Promise.all([
        api.getSecurityMetrics(),
        api.getSecurityThreats().catch(() => ({ threats: [] })),
        api.getAlerts(5, 0).catch(() => ({ alerts: [], allCount: 0, acknowledgedCount: 0, unacknowledgedCount: 0 })),
        api.getCapabilityRequests({ status: "pending", limit: 5, offset: 0 }).catch(() => []),
      ]);
      setMetrics(metricsData);
      setThreats(threatsData.threats || []);
      setAlerts(alertsData.alerts || []);
      setAlertCounts({
        all: alertsData.allCount || 0,
        acknowledged: alertsData.acknowledgedCount || 0,
        unacknowledged: alertsData.unacknowledgedCount || 0,
      });
      setCapabilityRequests(requestsData || []);
    } catch (err) {
      console.error("Failed to fetch security data:", err);
      setError(getErrorMessage(err, { resource: "security data", action: "load" }));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchSecurityData();
  }, []);

  // Calculate insight text for protection
  const protectionInsight = useMemo(() => {
    if (!metrics) return null;
    const totalActions = metrics.protectionTimeline?.reduce((sum, d) => sum + d.actions, 0) || 0;
    const totalBlocked = metrics.actionsBlocked || 0;
    if (totalActions === 0) return "No agent actions recorded yet.";
    const authorizedRate = ((totalActions - totalBlocked) / totalActions * 100).toFixed(1);
    return `${authorizedRate}% of agent actions were authorized. AIM blocked ${totalBlocked} unauthorized attempt${totalBlocked !== 1 ? 's' : ''}.`;
  }, [metrics]);

  // Get max blocked for category chart scaling
  const maxBlocked = useMemo(() => {
    if (!metrics?.riskByCategory?.length) return 1;
    return Math.max(...metrics.riskByCategory.map(c => c.blocked), 1);
  }, [metrics?.riskByCategory]);

  if (loading) {
    return <SecurityPageSkeleton />;
  }

  if (error && !metrics) {
    return <ErrorDisplay message={error} onRetry={fetchSecurityData} />;
  }

  const statusIndicator = metrics?.securityStatus === "Secure" || metrics?.securityStatus === "Good"
    ? { color: "bg-green-500", text: "All Systems Operational" }
    : metrics?.securityStatus === "Needs Attention"
    ? { color: "bg-yellow-500", text: "Needs Attention" }
    : { color: "bg-red-500", text: "Action Required" };

  return (
    <AuthGuard>
      <div className="space-y-6">
        {/* ============================================ */}
        {/* HERO SECTION - Security Command Center */}
        {/* ============================================ */}
        <div className="bg-gradient-to-br from-slate-50 to-white dark:from-gray-800 dark:to-gray-900 rounded-xl border border-gray-200 dark:border-gray-700 shadow-sm overflow-hidden">
          <div className="p-6 md:p-8">
            <div className="flex flex-col md:flex-row md:items-center gap-6">
              {/* Security Score Gauge */}
              <div className="flex-shrink-0">
                <SecurityScoreGauge
                  score={metrics?.securityScore || 0}
                  grade={metrics?.securityGrade || "?"}
                  status={metrics?.securityStatus || "Unknown"}
                />
              </div>

              {/* Status Content */}
              <div className="flex-1">
                <div className="flex items-center gap-3 mb-2">
                  <h1 className="text-2xl font-bold text-gray-900 dark:text-white">
                    Your AI Fleet
                  </h1>
                  <div className="flex items-center gap-2">
                    <span className={`w-2 h-2 rounded-full ${statusIndicator.color} animate-pulse`}></span>
                    <span className="text-sm text-gray-600 dark:text-gray-400">{statusIndicator.text}</span>
                  </div>
                </div>
                <p className="text-gray-600 dark:text-gray-400">
                  <span className="font-medium text-gray-900 dark:text-white">{metrics?.agentsMonitored || 0}</span> agents
                  <span className="mx-1">+</span>
                  <span className="font-medium text-gray-900 dark:text-white">{metrics?.mcpServersTotal || 0}</span> MCP servers monitored
                  <span className="mx-2">•</span>
                  <span className="font-medium text-gray-900 dark:text-white">{metrics?.actionsBlocked || 0}</span> threats blocked
                  {metrics?.lastIncidentAt && (
                    <>
                      <span className="mx-2">•</span>
                      Last incident: <span className="font-medium">{formatRelativeTime(metrics.lastIncidentAt)}</span>
                    </>
                  )}
                </p>

                {/* Quick Actions */}
                <div className="flex flex-wrap gap-3 mt-4">
                  {(metrics?.requiresAttention || 0) > 0 && (
                    <Link
                      href="/dashboard/admin/alerts"
                      className="inline-flex items-center gap-2 px-4 py-2 bg-amber-100 dark:bg-amber-900/30 text-amber-800 dark:text-amber-300 rounded-lg hover:bg-amber-200 dark:hover:bg-amber-900/50 transition-colors text-sm font-medium"
                    >
                      <Bell className="h-4 w-4" />
                      {metrics?.requiresAttention} items need attention
                      <ChevronRight className="h-4 w-4" />
                    </Link>
                  )}
                  <Link
                    href="/dashboard/agents"
                    className="inline-flex items-center gap-2 px-4 py-2 bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300 rounded-lg hover:bg-gray-200 dark:hover:bg-gray-600 transition-colors text-sm font-medium"
                  >
                    <Bot className="h-4 w-4" />
                    View All Agents
                    <ChevronRight className="h-4 w-4" />
                  </Link>
                </div>
              </div>
            </div>
          </div>
        </div>

        {/* ============================================ */}
        {/* STAT CARDS */}
        {/* ============================================ */}
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-5">
          <StatCard
            icon={Shield}
            label="Actions Blocked"
            value={metrics?.actionsBlocked || 0}
            subValue={`${metrics?.actionsBlockedToday || 0} today`}
            variant="success"
          />
          <StatCard
            icon={Bot}
            label="Agents Monitored"
            value={metrics?.agentsMonitored || 0}
            subValue={`${metrics?.trustPercentage || 0}% trusted`}
            variant="default"
          />
          <StatCard
            icon={Server}
            label="MCP Servers"
            value={metrics?.mcpServersTotal || 0}
            subValue={`${metrics?.mcpTrustPercentage || 0}% verified`}
            variant="default"
          />
          <StatCard
            icon={Zap}
            label="Actions Today"
            value={metrics?.actionsToday || 0}
            subValue="processed by agents"
            variant="default"
          />
          <StatCard
            icon={Bell}
            label="Requires Attention"
            value={metrics?.requiresAttention || 0}
            subValue="pending items"
            variant={(metrics?.requiresAttention || 0) > 10 ? "warning" : "default"}
          />
        </div>

        {/* ============================================ */}
        {/* CHARTS ROW */}
        {/* ============================================ */}
        <div className="grid grid-cols-1 gap-6 lg:grid-cols-2">
          {/* Protection Timeline */}
          <div className="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 shadow-sm p-6">
            <div className="flex items-center justify-between mb-4">
              <div>
                <h3 className="text-lg font-semibold text-gray-900 dark:text-white">Protection Timeline</h3>
                <p className="text-sm text-gray-500 dark:text-gray-400">Last 30 days</p>
              </div>
              <Activity className="h-5 w-5 text-gray-400" />
            </div>
            <div className="h-64">
              <ResponsiveContainer width="100%" height="100%">
                <ComposedChart data={metrics?.protectionTimeline || []}>
                  <defs>
                    <linearGradient id="actionsGradient" x1="0" y1="0" x2="0" y2="1">
                      <stop offset="5%" stopColor="#3b82f6" stopOpacity={0.2}/>
                      <stop offset="95%" stopColor="#3b82f6" stopOpacity={0}/>
                    </linearGradient>
                  </defs>
                  <CartesianGrid strokeDasharray="3 3" className="stroke-gray-200 dark:stroke-gray-700" />
                  <XAxis dataKey="date" stroke="#9CA3AF" tick={{ fontSize: 11 }} interval="preserveStartEnd" />
                  <YAxis stroke="#9CA3AF" tick={{ fontSize: 11 }} />
                  <Tooltip
                    contentStyle={{
                      backgroundColor: "white",
                      border: "1px solid #e5e7eb",
                      borderRadius: "0.5rem",
                      boxShadow: "0 4px 6px -1px rgb(0 0 0 / 0.1)",
                    }}
                  />
                  <Area
                    type="monotone"
                    dataKey="actions"
                    fill="url(#actionsGradient)"
                    stroke="#3b82f6"
                    strokeWidth={2}
                    name="Actions"
                  />
                  <Line
                    type="monotone"
                    dataKey="blocked"
                    stroke="#ef4444"
                    strokeWidth={2}
                    name="Blocked"
                    dot={{ fill: "#ef4444", strokeWidth: 2, r: 3 }}
                  />
                </ComposedChart>
              </ResponsiveContainer>
            </div>
            {/* Insight Box */}
            {protectionInsight && (
              <div className="mt-4 p-3 bg-blue-50 dark:bg-blue-900/20 rounded-lg border border-blue-200 dark:border-blue-800">
                <p className="text-sm text-blue-800 dark:text-blue-300">
                  <span className="font-medium">💡 Insight:</span> {protectionInsight}
                </p>
              </div>
            )}
          </div>

          {/* Risk by Category */}
          <div className="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 shadow-sm p-6">
            <div className="flex items-center justify-between mb-4">
              <div>
                <h3 className="text-lg font-semibold text-gray-900 dark:text-white">Risk by Category</h3>
                <p className="text-sm text-gray-500 dark:text-gray-400">Blocked actions by type</p>
              </div>
              <Shield className="h-5 w-5 text-gray-400" />
            </div>
            <div className="space-y-4">
              {metrics?.riskByCategory && metrics.riskByCategory.length > 0 ? (
                metrics.riskByCategory.map((cat) => (
                  <RiskCategoryBar
                    key={cat.category}
                    category={cat.category}
                    blocked={cat.blocked}
                    riskLevel={cat.riskLevel}
                    maxBlocked={maxBlocked}
                  />
                ))
              ) : (
                <div className="flex flex-col items-center justify-center py-8 text-center">
                  <CheckCircle className="h-12 w-12 text-green-500 mb-3" />
                  <p className="text-sm text-gray-600 dark:text-gray-400">No blocked actions detected</p>
                  <p className="text-xs text-gray-500 dark:text-gray-500 mt-1">All agent actions are within authorized capabilities</p>
                </div>
              )}
            </div>
            {/* Risk Insight */}
            {metrics?.riskByCategory && metrics.riskByCategory.length > 0 && (
              <div className="mt-4 p-3 bg-amber-50 dark:bg-amber-900/20 rounded-lg border border-amber-200 dark:border-amber-800">
                <p className="text-sm text-amber-800 dark:text-amber-300">
                  <span className="font-medium">⚠️ Recommendation:</span> Review agents attempting{" "}
                  <span className="font-medium">{metrics.riskByCategory[0]?.category}</span> actions.
                </p>
              </div>
            )}
          </div>
        </div>

        {/* ============================================ */}
        {/* BLOCKED ACTIONS */}
        {/* ============================================ */}
        <div className="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 shadow-sm">
          <div className="p-6 border-b border-gray-200 dark:border-gray-700">
            <div className="flex items-center justify-between">
              <div>
                <h3 className="text-lg font-semibold text-gray-900 dark:text-white flex items-center gap-2">
                  <Shield className="h-5 w-5 text-green-600" />
                  Blocked Actions
                </h3>
                <p className="text-sm text-gray-500 dark:text-gray-400 mt-1">
                  AIM prevented these unauthorized attempts
                </p>
              </div>
              {(metrics?.actionsBlocked || 0) > 10 && (
                <Link
                  href="/dashboard/security/violations"
                  className="text-sm text-blue-600 dark:text-blue-400 hover:underline flex items-center gap-1"
                >
                  View all <ChevronRight className="h-4 w-4" />
                </Link>
              )}
            </div>
          </div>
          <div className="p-6">
            {metrics?.recentBlockedActions && metrics.recentBlockedActions.length > 0 ? (
              <div className="space-y-3">
                {metrics.recentBlockedActions.slice(0, 5).map((action) => (
                  <BlockedActionCard key={action.id} action={action} />
                ))}
              </div>
            ) : (
              <div className="flex flex-col items-center justify-center py-12 text-center">
                <div className="p-4 rounded-full bg-green-100 dark:bg-green-900/30 mb-4">
                  <CheckCircle className="h-8 w-8 text-green-600 dark:text-green-400" />
                </div>
                <h4 className="text-lg font-medium text-gray-900 dark:text-white">All Clear</h4>
                <p className="text-sm text-gray-500 dark:text-gray-400 mt-1">
                  No unauthorized actions have been blocked recently
                </p>
              </div>
            )}
          </div>
        </div>

        {/* ============================================ */}
        {/* TWO COLUMN: ATTENTION QUEUE + ACTIVITY */}
        {/* ============================================ */}
        <div className="grid grid-cols-1 gap-6 lg:grid-cols-2">
          {/* Requires Attention */}
          <div className="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 shadow-sm">
            <div className="p-6 border-b border-gray-200 dark:border-gray-700">
              <div className="flex items-center justify-between">
                <div>
                  <h3 className="text-lg font-semibold text-gray-900 dark:text-white flex items-center gap-2">
                    <Bell className="h-5 w-5 text-amber-500" />
                    Requires Your Attention
                  </h3>
                  <p className="text-sm text-gray-500 dark:text-gray-400 mt-1">
                    {metrics?.requiresAttention || 0} pending items
                  </p>
                </div>
                <Link
                  href="/dashboard/admin/alerts"
                  className="text-sm text-blue-600 dark:text-blue-400 hover:underline flex items-center gap-1"
                >
                  View all <ChevronRight className="h-4 w-4" />
                </Link>
              </div>
            </div>
            <div className="divide-y divide-gray-200 dark:divide-gray-700 max-h-[400px] overflow-y-auto">
              {/* Capability Requests */}
              {capabilityRequests.slice(0, 3).map((req) => (
                <div key={req.id} className="p-4 hover:bg-gray-50 dark:hover:bg-gray-800/50 transition-colors">
                  <div className="flex items-start gap-3">
                    <div className="p-1.5 rounded-full bg-amber-100 dark:bg-amber-900/30">
                      <Clock className="h-4 w-4 text-amber-600 dark:text-amber-400" />
                    </div>
                    <div className="flex-1 min-w-0">
                      <p className="text-sm font-medium text-gray-900 dark:text-white">
                        Capability Request
                      </p>
                      <p className="text-xs text-gray-500 dark:text-gray-400 truncate">
                        {req.agentName || "Agent"} wants <span className="font-mono">{req.requestedCapability}</span>
                      </p>
                      <div className="flex gap-2 mt-2">
                        <button className="px-2 py-1 text-xs font-medium rounded bg-green-100 dark:bg-green-900/30 text-green-700 dark:text-green-400 hover:bg-green-200 dark:hover:bg-green-900/50 transition-colors">
                          Approve
                        </button>
                        <button className="px-2 py-1 text-xs font-medium rounded bg-red-100 dark:bg-red-900/30 text-red-700 dark:text-red-400 hover:bg-red-200 dark:hover:bg-red-900/50 transition-colors">
                          Deny
                        </button>
                        <Link
                          href={`/dashboard/admin/capability-requests?id=${req.id}`}
                          className="px-2 py-1 text-xs font-medium rounded bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300 hover:bg-gray-200 dark:hover:bg-gray-600 transition-colors"
                        >
                          Review
                        </Link>
                      </div>
                    </div>
                  </div>
                </div>
              ))}

              {/* Unacknowledged Alerts */}
              {alerts.filter(a => !a.isAcknowledged).slice(0, 3).map((alert) => (
                <div key={alert.id} className="p-4 hover:bg-gray-50 dark:hover:bg-gray-800/50 transition-colors">
                  <div className="flex items-start gap-3">
                    <div className={`p-1.5 rounded-full ${
                      alert.severity === 'critical' || alert.severity === 'high'
                        ? 'bg-red-100 dark:bg-red-900/30'
                        : 'bg-amber-100 dark:bg-amber-900/30'
                    }`}>
                      <AlertTriangle className={`h-4 w-4 ${
                        alert.severity === 'critical' || alert.severity === 'high'
                          ? 'text-red-600 dark:text-red-400'
                          : 'text-amber-600 dark:text-amber-400'
                      }`} />
                    </div>
                    <div className="flex-1 min-w-0">
                      <p className="text-sm font-medium text-gray-900 dark:text-white truncate">
                        {alert.title}
                      </p>
                      <p className="text-xs text-gray-500 dark:text-gray-400">
                        {formatRelativeTime(alert.createdAt)}
                      </p>
                      <Link
                        href={`/dashboard/admin/alerts?id=${alert.id}`}
                        className="mt-1 text-xs text-blue-600 dark:text-blue-400 hover:underline"
                      >
                        Investigate →
                      </Link>
                    </div>
                  </div>
                </div>
              ))}

              {/* Empty State */}
              {capabilityRequests.length === 0 && alerts.filter(a => !a.isAcknowledged).length === 0 && (
                <div className="p-8 text-center">
                  <CheckCircle className="h-12 w-12 mx-auto mb-3 text-green-500" />
                  <p className="text-sm text-gray-500 dark:text-gray-400">All caught up!</p>
                  <p className="text-xs text-gray-400 dark:text-gray-500 mt-1">No pending items require your attention</p>
                </div>
              )}
            </div>
          </div>

          {/* Security Alerts (for legacy compatibility) */}
          <div className="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 shadow-sm">
            <div className="p-6 border-b border-gray-200 dark:border-gray-700">
              <div className="flex items-center justify-between">
                <div>
                  <h3 className="text-lg font-semibold text-gray-900 dark:text-white flex items-center gap-2">
                    <AlertTriangle className="h-5 w-5 text-red-500" />
                    Security Alerts
                  </h3>
                  <p className="text-sm text-gray-500 dark:text-gray-400 mt-1">
                    {alertCounts.unacknowledged} unacknowledged of {alertCounts.all} total
                  </p>
                </div>
                <Link
                  href="/dashboard/admin/alerts"
                  className="text-sm text-blue-600 dark:text-blue-400 hover:underline flex items-center gap-1"
                >
                  View all <ChevronRight className="h-4 w-4" />
                </Link>
              </div>
            </div>
            <div className="divide-y divide-gray-200 dark:divide-gray-700 max-h-[400px] overflow-y-auto">
              {alerts.length > 0 ? (
                alerts.map((alert) => (
                  <div key={alert.id} className="p-4 hover:bg-gray-50 dark:hover:bg-gray-800/50 transition-colors">
                    <div className="flex items-start gap-3">
                      <div className={`p-1.5 rounded-full ${
                        alert.severity === 'critical' ? 'bg-red-100 dark:bg-red-900/30' :
                        alert.severity === 'high' ? 'bg-orange-100 dark:bg-orange-900/30' :
                        alert.severity === 'medium' ? 'bg-yellow-100 dark:bg-yellow-900/30' :
                        'bg-blue-100 dark:bg-blue-900/30'
                      }`}>
                        <AlertTriangle className={`h-4 w-4 ${
                          alert.severity === 'critical' ? 'text-red-600 dark:text-red-400' :
                          alert.severity === 'high' ? 'text-orange-600 dark:text-orange-400' :
                          alert.severity === 'medium' ? 'text-yellow-600 dark:text-yellow-400' :
                          'text-blue-600 dark:text-blue-400'
                        }`} />
                      </div>
                      <div className="flex-1 min-w-0">
                        <div className="flex items-center gap-2">
                          <p className="text-sm font-medium text-gray-900 dark:text-white truncate">
                            {alert.title}
                          </p>
                          <span className={`text-xs px-2 py-0.5 rounded-full capitalize ${
                            alert.severity === 'critical' ? 'bg-red-100 dark:bg-red-900/30 text-red-700 dark:text-red-300' :
                            alert.severity === 'high' ? 'bg-orange-100 dark:bg-orange-900/30 text-orange-700 dark:text-orange-300' :
                            alert.severity === 'medium' ? 'bg-yellow-100 dark:bg-yellow-900/30 text-yellow-700 dark:text-yellow-300' :
                            'bg-blue-100 dark:bg-blue-900/30 text-blue-700 dark:text-blue-300'
                          }`}>
                            {alert.severity}
                          </span>
                        </div>
                        <p className="text-xs text-gray-500 dark:text-gray-400 truncate mt-0.5">
                          {alert.description}
                        </p>
                        <div className="flex items-center gap-2 mt-1">
                          <Clock className="h-3 w-3 text-gray-400" />
                          <span className="text-xs text-gray-400">{formatDateTime(alert.createdAt)}</span>
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
              ) : (
                <div className="p-8 text-center">
                  <CheckCircle className="h-12 w-12 mx-auto mb-3 text-green-500" />
                  <p className="text-sm text-gray-500 dark:text-gray-400">No security alerts</p>
                </div>
              )}
            </div>
          </div>
        </div>

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
