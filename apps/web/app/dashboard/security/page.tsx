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
    if (score >= 90) return { stroke: "var(--green)", bg: "bg-success-fill", text: "text-success-text" };
    if (score >= 80) return { stroke: "var(--green)", bg: "bg-success-fill", text: "text-success-text" };
    if (score >= 70) return { stroke: "var(--amber)", bg: "bg-warning-fill", text: "text-warning-text" };
    if (score >= 60) return { stroke: "var(--amber)", bg: "bg-warning-fill", text: "text-warning-text" };
    return { stroke: "var(--brand)", bg: "bg-brand-soft", text: "text-brand-text" };
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
          stroke="var(--track)"
          strokeWidth="8"
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
        <span className="text-xs text-ink-secondary">/100</span>
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
    default: "glass",
    success: "glass border-success-border",
    warning: "glass border-warning-border",
    danger: "glass-alert",
  };

  const iconStyles = {
    default: "text-ink-secondary",
    success: "text-success-text",
    warning: "text-warning-text",
    danger: "text-danger-text",
  };

  return (
    <div className={`${variantStyles[variant]} p-5 transition-shadow`}>
      <div className="flex items-start justify-between">
        <div>
          <p className="text-xs font-medium text-ink-secondary uppercase tracking-wide">{label}</p>
          <p className="text-2xl font-bold text-ink mt-1">
            {typeof value === 'number' ? value.toLocaleString() : value}
          </p>
          {subValue && (
            <p className="text-xs text-ink-secondary mt-1">{subValue}</p>
          )}
        </div>
        <div className={`p-2 rounded-lg bg-glass-inset-gray ${iconStyles[variant]}`}>
          <Icon className="h-5 w-5" />
        </div>
      </div>
      {trend && (
        <div className="mt-2 flex items-center text-xs">
          <TrendingUp className="h-3 w-3 mr-1 text-success-text" />
          <span className="text-success-text">{trend}</span>
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
    <div className="glass-inset border border-divider p-4 transition-all">
      <div className="flex items-start gap-3">
        <div className="p-2 rounded-lg bg-danger-fill">
          <XCircle className="h-5 w-5 text-danger-text" />
        </div>
        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-2 flex-wrap">
            <span className="text-xs font-semibold px-2 py-0.5 rounded-full bg-danger-fill text-danger-text">
              Blocked
            </span>
            <span className="text-sm font-medium text-ink truncate">
              {action.agentName}
            </span>
          </div>
          <div className="flex items-center gap-2 mt-1.5">
            <CategoryIcon className="h-4 w-4 text-ink-tertiary" />
            <span className="text-sm text-ink-body font-mono">
              {action.attemptedCapability}
            </span>
          </div>
          <div className="flex items-center justify-between mt-2">
            <div className="flex items-center gap-3 text-xs text-ink-secondary">
              <span className="flex items-center gap-1">
                <Clock className="h-3 w-3" />
                {formatRelativeTime(action.createdAt)}
              </span>
              {action.trustImpact !== 0 && (
                <span className="text-danger-text">
                  Trust {action.trustImpact > 0 ? "+" : ""}{action.trustImpact}%
                </span>
              )}
            </div>
            <Link
              href={`/dashboard/agents?id=${action.agentId}`}
              className="text-xs text-brand-text hover:underline flex items-center gap-1"
            >
              Review agent <ChevronRight className="h-3 w-3" />
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
    high: "bg-danger",
    medium: "bg-warning",
    low: "bg-track",
    secure: "bg-success",
  };

  const riskLabels = {
    high: { text: "High risk", class: "text-danger-text" },
    medium: { text: "Medium", class: "text-warning-text" },
    low: { text: "Low", class: "text-ink-secondary" },
    secure: { text: "Secure", class: "text-success-text" },
  };

  const label = riskLabels[riskLevel as keyof typeof riskLabels] || riskLabels.secure;

  return (
    <div className="space-y-1">
      <div className="flex items-center justify-between text-sm">
        <span className="text-ink-body">{category}</span>
        <div className="flex items-center gap-2">
          <span className="font-medium text-ink">{blocked}</span>
          <span className={`text-xs ${label.class}`}>{label.text}</span>
        </div>
      </div>
      <div className="h-2 bg-track rounded-full overflow-hidden">
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
      <div className="glass p-6">
        <div className="flex items-center gap-8">
          <div className="animate-pulse bg-track h-32 w-32 rounded-full"></div>
          <div className="flex-1 space-y-3">
            <div className="animate-pulse bg-track h-6 w-48 rounded"></div>
            <div className="animate-pulse bg-track h-4 w-96 rounded"></div>
          </div>
        </div>
      </div>

      {/* Stats Skeleton */}
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
        {[...Array(4)].map((_, i) => (
          <div key={i} className="glass p-5">
            <div className="animate-pulse space-y-3">
              <div className="bg-track h-4 w-24 rounded"></div>
              <div className="bg-track h-8 w-16 rounded"></div>
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
        <Shield className={`h-16 w-16 ${is403 ? "text-warning" : "text-danger"}`} />
        <div className="space-y-2">
          <h3 className="text-2xl font-bold text-ink">
            {is403 ? "Access restricted" : "Failed to load security data"}
          </h3>
          {is403 ? (
            <p className="text-base text-ink-secondary">
              Security monitoring is only available to Admin and Manager roles.
            </p>
          ) : (
            <p className="text-sm text-ink-secondary">{message}</p>
          )}
        </div>
        {!is403 && (
          <button
            onClick={onRetry}
            className="px-4 py-2 rounded-pill bg-brand text-white shadow-glow hover:bg-brand-hover transition-colors"
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
    // Both numbers come from the same 30-day timeline so the ratio stays within 0-100.
    const totalActions = metrics.protectionTimeline?.reduce((sum, d) => sum + d.actions, 0) || 0;
    const totalBlocked = metrics.protectionTimeline?.reduce((sum, d) => sum + d.blocked, 0) || 0;
    if (totalActions === 0) return "No agent actions recorded in the last 30 days.";
    const authorizedRate = Math.min(100, Math.max(0, ((totalActions - totalBlocked) / totalActions) * 100)).toFixed(1);
    return `${authorizedRate}% of agent actions in the last 30 days were authorized. AIM denied ${totalBlocked} of them.`;
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
    ? { color: "bg-success", text: "All systems operational" }
    : metrics?.securityStatus === "Needs Attention"
    ? { color: "bg-warning", text: "Needs attention" }
    : { color: "bg-danger", text: "Action required" };

  return (
    <AuthGuard>
      <div className="space-y-6">
        {/* ============================================ */}
        {/* HERO SECTION - Security Command Center */}
        {/* ============================================ */}
        <div className="glass overflow-hidden">
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
                  <h1 className="text-2xl font-bold text-ink">
                    Your AI fleet
                  </h1>
                  <div className="flex items-center gap-2">
                    <span className={`w-2 h-2 rounded-full ${statusIndicator.color} animate-pulse`}></span>
                    <span className="text-sm text-ink-secondary">{statusIndicator.text}</span>
                  </div>
                </div>
                <p className="text-ink-secondary">
                  <span className="font-medium text-ink">{metrics?.agentsMonitored || 0}</span> agents
                  <span className="mx-1">+</span>
                  <span className="font-medium text-ink">{metrics?.mcpServersTotal || 0}</span> MCP servers monitored
                  <span className="mx-2">•</span>
                  <span className="font-medium text-ink">{metrics?.actionsBlocked || 0}</span> actions denied
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
                      className="inline-flex items-center gap-2 px-4 py-2 bg-warning-fill text-warning-text rounded-pill hover:bg-warning-border transition-colors text-sm font-medium"
                    >
                      <Bell className="h-4 w-4" />
                      {metrics?.requiresAttention} items need attention
                      <ChevronRight className="h-4 w-4" />
                    </Link>
                  )}
                  <Link
                    href="/dashboard/agents"
                    className="inline-flex items-center gap-2 px-4 py-2 bg-glass-inset-gray text-ink-body rounded-pill hover:bg-track transition-colors text-sm font-medium"
                  >
                    <Bot className="h-4 w-4" />
                    View all agents
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
            label="Actions blocked"
            value={metrics?.actionsBlocked || 0}
            subValue={`${metrics?.actionsBlockedToday || 0} today`}
            variant="success"
          />
          <StatCard
            icon={Bot}
            label="Agents monitored"
            value={metrics?.agentsMonitored || 0}
            subValue={`${metrics?.trustPercentage || 0}% trusted`}
            variant="default"
          />
          <StatCard
            icon={Server}
            label="MCP servers"
            value={metrics?.mcpServersTotal || 0}
            subValue={`${metrics?.mcpTrustPercentage || 0}% verified`}
            variant="default"
          />
          <StatCard
            icon={Zap}
            label="Actions today"
            value={metrics?.actionsToday || 0}
            subValue="processed by agents"
            variant="default"
          />
          <StatCard
            icon={Bell}
            label="Requires attention"
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
          <div className="glass p-6">
            <div className="flex items-center justify-between mb-4">
              <div>
                <h3 className="text-lg font-semibold text-ink">Protection timeline</h3>
                <p className="text-sm text-ink-secondary">Last 30 days</p>
              </div>
              <Activity className="h-5 w-5 text-ink-tertiary" />
            </div>
            <div className="h-64">
              <ResponsiveContainer width="100%" height="100%">
                <ComposedChart data={metrics?.protectionTimeline || []}>
                  <defs>
                    <linearGradient id="actionsGradient" x1="0" y1="0" x2="0" y2="1">
                      <stop offset="5%" stopColor="var(--brand)" stopOpacity={0.2}/>
                      <stop offset="95%" stopColor="var(--brand)" stopOpacity={0}/>
                    </linearGradient>
                  </defs>
                  <CartesianGrid strokeDasharray="3 3" className="stroke-divider" />
                  <XAxis dataKey="date" stroke="var(--text-tertiary)" tick={{ fontSize: 11 }} interval="preserveStartEnd" />
                  <YAxis stroke="var(--text-tertiary)" tick={{ fontSize: 11 }} />
                  <Tooltip
                    contentStyle={{
                      backgroundColor: "var(--glass-fill)",
                      border: "1px solid var(--glass-border)",
                      borderRadius: "12px",
                      boxShadow: "var(--shadow-card)",
                      color: "var(--text-primary)",
                    }}
                  />
                  <Area
                    type="monotone"
                    dataKey="actions"
                    fill="url(#actionsGradient)"
                    stroke="var(--brand)"
                    strokeWidth={2}
                    name="Actions"
                  />
                  <Line
                    type="monotone"
                    dataKey="blocked"
                    stroke="var(--red)"
                    strokeWidth={2}
                    name="Blocked"
                    dot={{ fill: "var(--red)", strokeWidth: 2, r: 3 }}
                  />
                </ComposedChart>
              </ResponsiveContainer>
            </div>
            {/* Insight Box */}
            {protectionInsight && (
              <div className="mt-4 p-3 bg-brand-soft rounded-inset border border-stroke">
                <p className="text-sm text-brand-text">
                  <span className="font-medium">Insight:</span> {protectionInsight}
                </p>
              </div>
            )}
          </div>

          {/* Risk by Category */}
          <div className="glass p-6">
            <div className="flex items-center justify-between mb-4">
              <div>
                <h3 className="text-lg font-semibold text-ink">Risk by category</h3>
                <p className="text-sm text-ink-secondary">Blocked actions by type</p>
              </div>
              <Shield className="h-5 w-5 text-ink-tertiary" />
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
                  <CheckCircle className="h-12 w-12 text-success mb-3" />
                  <p className="text-sm text-ink-secondary">No blocked actions detected</p>
                  <p className="text-xs text-ink-tertiary mt-1">All agent actions are within authorized capabilities</p>
                </div>
              )}
            </div>
            {/* Risk Insight */}
            {metrics?.riskByCategory && metrics.riskByCategory.length > 0 && (
              <div className="mt-4 p-3 bg-warning-fill rounded-inset border border-warning-border">
                <p className="text-sm text-warning-text">
                  <span className="font-medium">Recommendation:</span> Review agents attempting{" "}
                  <span className="font-medium">{metrics.riskByCategory[0]?.category}</span> actions.
                </p>
              </div>
            )}
          </div>
        </div>

        {/* ============================================ */}
        {/* BLOCKED ACTIONS */}
        {/* ============================================ */}
        <div className="glass">
          <div className="p-6 border-b border-divider">
            <div className="flex items-center justify-between">
              <div>
                <h3 className="text-lg font-semibold text-ink flex items-center gap-2">
                  <Shield className="h-5 w-5 text-success-text" />
                  Denied actions
                </h3>
                <p className="text-sm text-ink-secondary mt-1">
                  Actions AIM denied. Whether the agent ran them anyway is self-reported; AIM does not observe execution.
                </p>
              </div>
              {(metrics?.actionsBlocked || 0) > 10 && (
                <Link
                  href="/dashboard/security/violations"
                  className="text-sm text-brand-text hover:underline flex items-center gap-1"
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
                <div className="p-4 rounded-full bg-success-fill mb-4">
                  <CheckCircle className="h-8 w-8 text-success-text" />
                </div>
                <h4 className="text-lg font-medium text-ink">All clear</h4>
                <p className="text-sm text-ink-secondary mt-1">
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
          <div className="glass">
            <div className="p-6 border-b border-divider">
              <div className="flex items-center justify-between">
                <div>
                  <h3 className="text-lg font-semibold text-ink flex items-center gap-2">
                    <Bell className="h-5 w-5 text-warning" />
                    Requires your attention
                  </h3>
                  <p className="text-sm text-ink-secondary mt-1">
                    {metrics?.requiresAttention || 0} pending items
                  </p>
                </div>
                <Link
                  href="/dashboard/admin/alerts"
                  className="text-sm text-brand-text hover:underline flex items-center gap-1"
                >
                  View all <ChevronRight className="h-4 w-4" />
                </Link>
              </div>
            </div>
            <div className="divide-y divide-divider max-h-[400px] overflow-y-auto">
              {/* Capability Requests */}
              {capabilityRequests.slice(0, 3).map((req) => (
                <div key={req.id} className="p-4 hover:bg-glass-inset-gray transition-colors">
                  <div className="flex items-start gap-3">
                    <div className="p-1.5 rounded-full bg-warning-fill">
                      <Clock className="h-4 w-4 text-warning-text" />
                    </div>
                    <div className="flex-1 min-w-0">
                      <p className="text-sm font-medium text-ink">
                        Capability request
                      </p>
                      <p className="text-xs text-ink-secondary truncate">
                        {req.agentName || "Agent"} wants <span className="font-mono">{req.requestedCapability}</span>
                      </p>
                      <div className="flex gap-2 mt-2">
                        <Link
                          href={`/dashboard/admin/capability-requests?id=${req.id}`}
                          className="px-2 py-1 text-xs font-medium rounded bg-glass-inset-gray text-ink-body hover:bg-track transition-colors"
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
                <div key={alert.id} className="p-4 hover:bg-glass-inset-gray transition-colors">
                  <div className="flex items-start gap-3">
                    <div className={`p-1.5 rounded-full ${
                      alert.severity === 'critical' || alert.severity === 'high'
                        ? 'bg-danger-fill'
                        : 'bg-warning-fill'
                    }`}>
                      <AlertTriangle className={`h-4 w-4 ${
                        alert.severity === 'critical' || alert.severity === 'high'
                          ? 'text-danger-text'
                          : 'text-warning-text'
                      }`} />
                    </div>
                    <div className="flex-1 min-w-0">
                      <p className="text-sm font-medium text-ink truncate">
                        {alert.title}
                      </p>
                      <p className="text-xs text-ink-secondary">
                        {formatRelativeTime(alert.createdAt)}
                      </p>
                      <Link
                        href={`/dashboard/admin/alerts?selected=${alert.id}`}
                        className="mt-1 text-xs text-brand-text hover:underline"
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
                  <CheckCircle className="h-12 w-12 mx-auto mb-3 text-success" />
                  <p className="text-sm text-ink-secondary">All caught up</p>
                  <p className="text-xs text-ink-tertiary mt-1">No pending items require your attention</p>
                </div>
              )}
            </div>
          </div>

          {/* Security Alerts (for legacy compatibility) */}
          <div className="glass">
            <div className="p-6 border-b border-divider">
              <div className="flex items-center justify-between">
                <div>
                  <h3 className="text-lg font-semibold text-ink flex items-center gap-2">
                    <AlertTriangle className="h-5 w-5 text-danger" />
                    Security alerts
                  </h3>
                  <p className="text-sm text-ink-secondary mt-1">
                    {alertCounts.unacknowledged} unacknowledged of {alertCounts.all} total
                  </p>
                </div>
                <Link
                  href="/dashboard/admin/alerts"
                  className="text-sm text-brand-text hover:underline flex items-center gap-1"
                >
                  View all <ChevronRight className="h-4 w-4" />
                </Link>
              </div>
            </div>
            <div className="divide-y divide-divider max-h-[400px] overflow-y-auto">
              {alerts.length > 0 ? (
                alerts.map((alert) => (
                  <div key={alert.id} className="p-4 hover:bg-glass-inset-gray transition-colors">
                    <div className="flex items-start gap-3">
                      <div className={`p-1.5 rounded-full ${
                        alert.severity === 'critical' ? 'bg-danger-strong' :
                        alert.severity === 'high' ? 'bg-danger-fill' :
                        alert.severity === 'medium' ? 'bg-warning-fill' :
                        'bg-brand-soft'
                      }`}>
                        <AlertTriangle className={`h-4 w-4 ${
                          alert.severity === 'critical' ? 'text-white' :
                          alert.severity === 'high' ? 'text-danger-text' :
                          alert.severity === 'medium' ? 'text-warning-text' :
                          'text-brand-text'
                        }`} />
                      </div>
                      <div className="flex-1 min-w-0">
                        <div className="flex items-center gap-2">
                          <p className="text-sm font-medium text-ink truncate">
                            {alert.title}
                          </p>
                          <span className={`text-xs px-2 py-0.5 rounded-full capitalize ${
                            alert.severity === 'critical' ? 'bg-danger-fill text-danger-text border border-danger-strong font-bold' :
                            alert.severity === 'high' ? 'bg-danger-fill text-danger-text' :
                            alert.severity === 'medium' ? 'bg-warning-fill text-warning-text' :
                            'bg-brand-soft text-brand-text'
                          }`}>
                            {alert.severity}
                          </span>
                        </div>
                        <p className="text-xs text-ink-secondary truncate mt-0.5">
                          {alert.description}
                        </p>
                        <div className="flex items-center gap-2 mt-1">
                          <Clock className="h-3 w-3 text-ink-tertiary" />
                          <span className="text-xs text-ink-tertiary">{formatDateTime(alert.createdAt)}</span>
                          {!alert.isAcknowledged && (
                            <span className="text-xs px-1.5 py-0.5 bg-warning-fill text-warning-text rounded">
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
                  <CheckCircle className="h-12 w-12 mx-auto mb-3 text-success" />
                  <p className="text-sm text-ink-secondary">No security alerts</p>
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
