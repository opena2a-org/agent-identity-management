"use client";

import { useState, useEffect, useMemo } from "react";
import Link from "next/link";
import { useDebounce } from "@/hooks/use-debounce";
import { useRouter } from "next/navigation";
import {
  Shield,
  CheckCircle,
  AlertTriangle,
  TrendingUp,
  Download,
  Users,
  FileText,
  Loader2,
  XCircle,
  Lock,
  RefreshCw,
  BarChart3,
  FileCheck,
  ShieldCheck,
  Info,
  Activity,
  ArrowUpRight,
  ArrowDownRight,
  Minus,
  Clock,
  Filter,
  Search,
  Server,
  Bot,
  Key,
  Eye,
  Trash2,
  Edit3,
  UserPlus,
  LogIn,
  LogOut,
  ChevronDown,
  ChevronUp,
  Calendar,
  Database,
  ExternalLink,
  X,
} from "lucide-react";
import { DonutChart, TrendChart, chartSeries } from "@/components/charts";
import { api } from "@/lib/api";
import { formatDateTime } from "@/lib/date-utils";
import { AuthGuard } from "@/components/auth-guard";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";

// Types
interface ComplianceStatus {
  complianceLevel: string;
  totalAgents: number;
  verifiedAgents: number;
  verificationRate: number;
  averageTrustScore: number;
  recentAuditCount: number;
}

interface ComplianceMetrics {
  startDate: string;
  endDate: string;
  interval: string;
  metrics: {
    period: { start: string; end: string; interval: string };
    agentVerificationTrend: Array<{ date: string; verified: number }>;
    trustScoreTrend: Array<{ date: string; avgScore: number }>;
  };
}

interface AccessReviewUser {
  id: string;
  email: string;
  name: string;
  role: string;
  lastLogin: string;
  createdAt: string;
  status: string;
}

interface CheckResult {
  name: string;
  passed: boolean;
  details?: string;
  count?: number;
  actionUrl?: string;
  affectedItems?: Array<{
    id: string;
    name: string;
    score?: number;
    issue: string;
    severity?: string;
  }>;
}

// AIM-specific compliance categories (dynamically calculated)
interface ComplianceCategory {
  id: string;
  name: string;
  description: string;
  icon: any;
  color: string;
  items: { name: string; passed: boolean; details: string }[];
}

// Audit log entry from API
interface AuditLogEntry {
  id: string;
  action: string;
  resourceType: string;
  resourceId: string;
  userId: string;
  userEmail?: string;
  ipAddress: string;
  timestamp: string;
  metadata?: Record<string, any>;
}

// Alert entry from API
interface AlertEntry {
  id: string;
  title: string;
  description: string;
  severity: "info" | "low" | "medium" | "high" | "critical";
  alertType: string;
  isAcknowledged: boolean;
  createdAt: string;
  acknowledgedAt?: string;
  resourceType?: string;
  resourceId?: string;
}

// Agent entry for risk distribution and timeline
interface AgentEntry {
  id: string;
  displayName: string;
  name: string;
  trustScore: number;
  status: string;
  agentType?: string;
  createdAt: string;
}

// MCP server entry for risk distribution
interface MCPServerEntry {
  id: string;
  name: string;
  url: string;
  status: "active" | "inactive" | "pending" | "verified" | "suspended" | "revoked";
  isVerified?: boolean;
  lastVerifiedAt?: string;
  createdAt: string;
}

// Helper to format audit action for display
function formatAuditAction(log: AuditLogEntry): string {
  const actionMap: Record<string, string> = {
    login: "User logged in",
    logout: "User logged out",
    create: `Created ${log.resourceType}`,
    update: `Updated ${log.resourceType}`,
    delete: `Deleted ${log.resourceType}`,
    verify: `Verified ${log.resourceType}`,
    approve: `Approved ${log.resourceType}`,
    reject: `Rejected ${log.resourceType}`,
    view: `Viewed ${log.resourceType}`,
    export: `Exported ${log.resourceType}`,
  };
  return actionMap[log.action] || `${log.action} ${log.resourceType}`;
}

// Helper to determine activity type from audit action
function getActivityType(action: string): "success" | "warning" | "info" | "error" {
  const successActions = ["create", "verify", "approve", "login"];
  const warningActions = ["update", "export"];
  const errorActions = ["delete", "reject"];

  if (successActions.includes(action)) return "success";
  if (warningActions.includes(action)) return "warning";
  if (errorActions.includes(action)) return "error";
  return "info";
}

// Helper to format relative time
function formatRelativeTime(timestamp: string): string {
  const date = new Date(timestamp);
  const now = new Date();
  const diffMs = now.getTime() - date.getTime();
  const diffMins = Math.floor(diffMs / (1000 * 60));
  const diffHours = Math.floor(diffMs / (1000 * 60 * 60));
  const diffDays = Math.floor(diffMs / (1000 * 60 * 60 * 24));

  if (diffMins < 1) return "just now";
  if (diffMins < 60) return `${diffMins} minute${diffMins === 1 ? "" : "s"} ago`;
  if (diffHours < 24) return `${diffHours} hour${diffHours === 1 ? "" : "s"} ago`;
  if (diffDays < 7) return `${diffDays} day${diffDays === 1 ? "" : "s"} ago`;
  return date.toLocaleDateString();
}



// Component: Compliance Framework Card - Enterprise style similar to SOC2/HIPAA dashboards
function ComplianceSummaryCard({
  title,
  description,
  passed,
  total,
  icon: Icon,
  color,
  checks,
  onNavigate,
}: {
  title: string;
  description: string;
  passed: number;
  total: number;
  icon: any;
  color: "brand" | "success";
  checks: CheckResult[];
  onNavigate?: (url: string) => void;
}) {
  const progress = total > 0 ? Math.round((passed / total) * 100) : 0;

  // Theme tokens only: the icon tile is a tint with the icon in the matching text colour.
  const colorMap = {
    brand: { tile: "bg-brand-soft", icon: "text-brand-text", text: "text-brand-text", bar: "bg-brand" },
    success: { tile: "bg-success-fill", icon: "text-success-text", text: "text-success-text", bar: "bg-success" },
  };
  const colors = colorMap[color];

  // Comprehensive check metadata with labels, descriptions, and action URLs
  const checkMetadata: Record<string, { label: string; description: string; passedText: string; failedText: (count: number) => string; actionUrl: string }> = {
    // Security Compliance Checks
    "apiKeyRotationNeeded": {
      label: "Credential Rotation",
      description: "Agent credentials should be rotated every 90 days",
      passedText: "All credentials are current",
      failedText: (n) => `${n} agent${n > 1 ? "s" : ""} need${n === 1 ? "s" : ""} rotation`,
      actionUrl: "/dashboard/agents",
    },
    "trustScoreDegradation": {
      label: "Trust Score Health",
      description: "Agents should maintain trust scores above 60%",
      passedText: "All agents above threshold",
      failedText: (n) => `${n} agent${n > 1 ? "s" : ""} below 60%`,
      actionUrl: "/dashboard/agents",
    },
    "capabilityViolations": {
      label: "Capability Compliance",
      description: "Agents should not have capability violations",
      passedText: "No violations detected",
      failedText: (n) => `${n} violation${n > 1 ? "s" : ""} need review`,
      actionUrl: "/dashboard/admin/alerts",
    },
    "adminAccessReview": {
      label: "Admin Activity",
      description: "Admin users should log in at least every 30 days",
      passedText: "All admins recently active",
      failedText: (n) => `${n} admin${n > 1 ? "s" : ""} inactive 30+ days`,
      actionUrl: "/dashboard/admin/users",
    },
    "auditLogGaps": {
      label: "Audit Coverage",
      description: "Audit logs should be recorded daily",
      passedText: "Continuous logging confirmed",
      failedText: (n) => `${n} day${n > 1 ? "s" : ""} without logs`,
      actionUrl: "/dashboard/compliance",
    },
    // Operations Compliance Checks
    "inactiveAgents": {
      label: "Agent Activity",
      description: "Agents should be active within 30 days",
      passedText: "Activity within threshold",
      failedText: (n) => `${n} agent${n > 1 ? "s" : ""} inactive 30+ days`,
      actionUrl: "/dashboard/agents",
    },
    "unverifiedAgentBacklog": {
      label: "Agent Verification Queue",
      description: "Pending verifications should be processed promptly",
      passedText: "Queue is manageable",
      failedText: (n) => `${n} agent${n > 1 ? "s" : ""} awaiting verification`,
      actionUrl: "/dashboard/admin/jit-requests",
    },
    "orphanedResources": {
      label: "Resource Ownership",
      description: "All agents should have active owners and good trust scores",
      passedText: "All resources properly assigned",
      failedText: (n) => `${n} orphaned resource${n > 1 ? "s" : ""}`,
      actionUrl: "/dashboard/agents",
    },
    "inactiveMCPServers": {
      label: "MCP Server Activity",
      description: "MCP servers should be active within 30 days",
      passedText: "All MCPs recently active",
      failedText: (n) => `${n} MCP${n > 1 ? "s" : ""} inactive 30+ days`,
      actionUrl: "/dashboard/mcp",
    },
    "unverifiedMCPBacklog": {
      label: "MCP Verification Queue",
      description: "MCP servers should be verified promptly",
      passedText: "Queue is manageable",
      failedText: (n) => `${n} MCP${n > 1 ? "s" : ""} awaiting verification`,
      actionUrl: "/dashboard/mcp",
    },
  };

  const getCheckInfo = (check: CheckResult) => {
    const meta = checkMetadata[check.name];
    if (!meta) {
      return {
        label: check.name.replace(/([A-Z])/g, " $1").replace(/^./, str => str.toUpperCase()).trim(),
        description: check.details || "",
        statusText: check.passed ? "Passed" : `${check.count || 0} issues`,
        actionUrl: "/dashboard",
      };
    }
    return {
      label: meta.label,
      description: meta.description,
      statusText: check.passed ? meta.passedText : meta.failedText(check.count || 0),
      actionUrl: meta.actionUrl,
    };
  };

  return (
    <div className="glass overflow-hidden">
      {/* Header */}
      <div className="px-6 py-5 border-b border-divider">
        <div className="flex items-center justify-between gap-4">
          <div className="flex items-center gap-4 min-w-0">
            <div className={`${colors.tile} p-3 rounded-inset-sm flex-shrink-0`}>
              <Icon className={`h-6 w-6 ${colors.icon}`} />
            </div>
            <div className="min-w-0">
              <h3 className="text-lg font-semibold text-ink">{title}</h3>
              <p className="text-sm text-ink-secondary">{description}</p>
            </div>
          </div>
          <div className="text-right flex-shrink-0">
            <div className={`text-3xl font-bold ${progress === 100 ? "text-success-text" : progress >= 80 ? colors.text : "text-warning-text"}`}>
              {progress}%
            </div>
            <div className="text-xs text-ink-secondary mt-0.5">
              {progress === 100 ? "All Passed" : `${passed}/${total} Passed`}
            </div>
          </div>
        </div>
      </div>

      {/* Progress bar */}
      <div className="px-6 py-3 border-b border-divider bg-glass-inset-gray">
        <div className="w-full bg-track rounded-full h-2">
          <div
            className={`h-2 rounded-full transition-all duration-500 ${progress === 100 ? "bg-success" : progress >= 80 ? colors.bar : "bg-warning"}`}
            style={{ width: `${progress}%` }}
          />
        </div>
      </div>

      {/* Individual checks with details */}
      <div className="divide-y divide-divider">
        {checks.map((check, idx) => {
          const info = getCheckInfo(check);
          const hasIssues = !check.passed && (check.count || 0) > 0;

          return (
            <div
              key={idx}
              className={`px-6 py-3 ${hasIssues && onNavigate ? "cursor-pointer hover:bg-glass-inset-gray" : ""} transition-colors`}
              onClick={() => hasIssues && onNavigate && onNavigate(info.actionUrl)}
            >
              <div className="flex items-start justify-between gap-3">
                <div className="flex items-start gap-3 min-w-0 flex-1">
                  <div className="mt-0.5">
                    {check.passed ? (
                      <CheckCircle className="h-5 w-5 text-success" />
                    ) : (
                      <AlertTriangle className="h-5 w-5 text-warning" />
                    )}
                  </div>
                  <div className="min-w-0 flex-1">
                    <div className="flex items-center gap-2">
                      <span className="text-sm font-medium text-ink">{info.label}</span>
                      {hasIssues && onNavigate && (
                        <ArrowUpRight className="h-3 w-3 text-ink-tertiary" />
                      )}
                    </div>
                    <p className="text-xs text-ink-secondary mt-0.5">{info.description}</p>
                  </div>
                </div>
                <div className="flex-shrink-0">
                  <span className={`inline-flex items-center text-xs font-medium px-2.5 py-1 rounded-full ${
                    check.passed
                      ? "bg-success-fill text-success-text"
                      : "bg-warning-fill text-warning-text"
                  }`}>
                    {info.statusText}
                  </span>
                </div>
              </div>
              {/* Show affected items preview for failed checks */}
              {hasIssues && check.affectedItems && check.affectedItems.length > 0 && (
                <div className="mt-2 ml-8 flex flex-wrap gap-1">
                  {check.affectedItems.slice(0, 3).map((item, i) => (
                    <span key={i} className="inline-flex items-center text-xs px-2 py-0.5 rounded bg-glass-inset-gray text-ink-secondary">
                      {item.name}
                    </span>
                  ))}
                  {check.affectedItems.length > 3 && (
                    <span className="text-xs text-ink-secondary">+{check.affectedItems.length - 3} more</span>
                  )}
                </div>
              )}
            </div>
          );
        })}
      </div>
    </div>
  );
}


// Component: Alert Activity Item - for showing security alerts/violations
function AlertActivityItem({ alert }: { alert: AlertEntry }) {
  const severityStyles = {
    critical: { icon: XCircle, color: "text-danger", bg: "bg-danger-fill", badge: "bg-danger-fill text-danger-text" },
    high: { icon: AlertTriangle, color: "text-warning", bg: "bg-warning-fill", badge: "bg-warning-fill text-warning-text" },
    medium: { icon: AlertTriangle, color: "text-warning", bg: "bg-warning-fill", badge: "bg-warning-fill text-warning-text" },
    low: { icon: Info, color: "text-brand-text", bg: "bg-brand-soft", badge: "bg-brand-soft text-brand-text" },
    info: { icon: Info, color: "text-ink-tertiary", bg: "bg-glass-inset-gray", badge: "bg-glass-inset-gray text-ink-body" },
  };
  const style = severityStyles[alert.severity] || severityStyles.info;
  const Icon = style.icon;

  return (
    <div className="flex items-start gap-3 py-2.5 border-b border-divider last:border-0">
      <div className={`p-1.5 rounded-lg ${style.bg} flex-shrink-0`}>
        <Icon className={`h-4 w-4 ${style.color}`} />
      </div>
      <div className="flex-1 min-w-0">
        <div className="flex items-center gap-2">
          <p className="text-sm font-medium text-ink truncate">{alert.title}</p>
          <span className={`text-[10px] font-medium px-1.5 py-0.5 rounded-full uppercase ${style.badge}`}>
            {alert.severity}
          </span>
        </div>
        <p className="text-xs text-ink-secondary mt-0.5 truncate">{alert.description}</p>
        <div className="flex items-center gap-2 mt-1">
          <span className="text-xs text-ink-secondary">{formatRelativeTime(alert.createdAt)}</span>
          {alert.isAcknowledged && (
            <>
              <span className="text-xs text-ink-secondary" aria-hidden="true">•</span>
              <span className="text-xs text-success-text">Acknowledged</span>
            </>
          )}
        </div>
      </div>
    </div>
  );
}

// Component: Stat Card
function StatCard({ stat }: { stat: { name: string; value: string; icon: any; iconColor?: string; change?: string; changeType?: string } }) {
  return (
    <div className="glass p-5">
      <div className="flex items-center justify-between">
        <div className="flex-shrink-0 p-2.5 rounded-lg bg-glass-inset-gray">
          <stat.icon className={`h-5 w-5 ${stat.iconColor || "text-ink-tertiary"}`} />
        </div>
        {stat.change && (
          <div className={`flex items-center gap-1 text-xs font-medium px-2 py-1 rounded-full ${
            stat.changeType === "positive"
              ? "text-success-text bg-success-fill"
              : stat.changeType === "negative"
              ? "text-danger-text bg-danger-fill"
              : "text-ink-body bg-glass-inset-gray"
          }`}>
            {stat.changeType === "positive" ? <ArrowUpRight className="h-3 w-3" /> :
             stat.changeType === "negative" ? <ArrowDownRight className="h-3 w-3" /> :
             <Minus className="h-3 w-3" />}
            {stat.change}
          </div>
        )}
      </div>
      <div className="mt-4">
        <p className="text-sm font-medium text-ink-secondary">{stat.name}</p>
        <p className="text-2xl font-bold text-ink mt-1">{stat.value}</p>
      </div>
    </div>
  );
}

// Component: Risk Distribution Chart (uses actual agent trust scores and MCP server status)
function RiskDistributionChart({ agents, mcpServers }: { agents: AgentEntry[]; mcpServers: MCPServerEntry[] }) {
  // Calculate risk distribution from actual agent trust scores and MCP server statuses
  // Agents: Trust score is stored as 0-1, Low Risk: >= 80%, Medium Risk: 60-79%, High Risk: 40-59%, Critical: < 40%
  // MCPs: verified/active = Low Risk, pending = Medium Risk, inactive = High Risk, suspended/revoked = Critical
  const calculateRiskDistribution = () => {
    const totalItems = agents.length + mcpServers.length;
    if (totalItems === 0) {
      return [
        { name: "No Resources", value: 100, color: chartSeries.muted, count: 0 },
      ];
    }

    let lowRisk = 0;
    let mediumRisk = 0;
    let highRisk = 0;
    let critical = 0;

    // Categorize agents by trust score
    for (const agent of agents) {
      const score = agent.trustScore * 100; // Convert 0-1 to 0-100
      if (score >= 80) {
        lowRisk++;
      } else if (score >= 60) {
        mediumRisk++;
      } else if (score >= 40) {
        highRisk++;
      } else {
        critical++;
      }
    }

    // Categorize MCP servers by status
    for (const mcp of mcpServers) {
      if (mcp.status === "verified" || mcp.status === "active") {
        lowRisk++;
      } else if (mcp.status === "pending") {
        mediumRisk++;
      } else if (mcp.status === "inactive") {
        highRisk++;
      } else {
        // suspended, revoked
        critical++;
      }
    }

    return [
      { name: "Low Risk", value: Math.round((lowRisk / totalItems) * 100), color: chartSeries.green, count: lowRisk },
      { name: "Medium Risk", value: Math.round((mediumRisk / totalItems) * 100), color: chartSeries.amber, count: mediumRisk },
      { name: "High Risk", value: Math.round((highRisk / totalItems) * 100), color: chartSeries.orange, count: highRisk },
      { name: "Critical", value: Math.round((critical / totalItems) * 100), color: chartSeries.red, count: critical },
    ].filter(d => d.count > 0);
  };

  const data = calculateRiskDistribution();
  const totalItems = agents.length + mcpServers.length;

  return (
    <div className="flex items-center gap-6">
      <div className="w-36 h-[188px]">
        <DonutChart
          title="Risk distribution, share of agents and MCP servers"
          data={data}
          formatValue={(v) => `${v}%`}
          columns={["Risk", "Share"]}
          tooltip={false}
          innerRadius={35}
          outerRadius={55}
          paddingAngle={3}
        />
      </div>
      <div className="space-y-2">
        {data.map((item) => (
          <div key={item.name} className="flex items-center gap-2">
            <div className="w-3 h-3 rounded-full" style={{ backgroundColor: item.color }} />
            <span className="text-sm text-ink-secondary">{item.name}</span>
            <span className="text-sm font-semibold text-ink">{item.value}%</span>
          </div>
        ))}
        {totalItems > 0 && (
          <div className="pt-2 border-t border-divider mt-2">
            <span className="text-xs text-ink-secondary">Based on {agents.length} agent{agents.length !== 1 ? "s" : ""} & {mcpServers.length} MCP{mcpServers.length !== 1 ? "s" : ""}</span>
          </div>
        )}
      </div>
    </div>
  );
}

// Loading Skeleton
function CompliancePageSkeleton() {
  return (
    <div className="space-y-6">
      <div className="space-y-2">
        <div className="animate-pulse bg-track h-8 w-64 max-w-full rounded" />
        <div className="animate-pulse bg-track h-4 w-96 max-w-full rounded" />
      </div>
      <div className="grid grid-cols-1 gap-5 sm:grid-cols-2 lg:grid-cols-5">
        {[...Array(5)].map((_, i) => (
          <div key={i} className="glass p-5">
            <div className="animate-pulse space-y-3">
              <div className="h-10 w-10 bg-track rounded-lg" />
              <div className="h-4 w-20 bg-track rounded" />
              <div className="h-8 w-16 bg-track rounded" />
            </div>
          </div>
        ))}
      </div>
      <div className="grid grid-cols-1 lg:grid-cols-2 xl:grid-cols-4 gap-5">
        {[...Array(4)].map((_, i) => (
          <div key={i} className="glass p-5 h-64">
            <div className="animate-pulse h-full bg-track rounded" />
          </div>
        ))}
      </div>
    </div>
  );
}

// Error Display
function ErrorDisplay({ message, onRetry }: { message: string; onRetry: () => void }) {
  const is403 = message.includes("403");

  return (
    <div className="flex items-center justify-center min-h-[400px]">
      <div className="flex flex-col items-center gap-4 max-w-md text-center px-4">
        <Shield className={`h-16 w-16 ${is403 ? "text-warning" : "text-danger"}`} />
        <div className="space-y-2">
          <h3 className="text-2xl font-bold text-ink">
            {is403 ? "Access Restricted" : "Failed to Load Compliance Data"}
          </h3>
          {is403 ? (
            <div className="space-y-3">
              <p className="text-base text-ink-body">
                Compliance monitoring is only available to <strong>Admin</strong> roles.
              </p>
              <p className="text-sm text-ink-secondary">
                Contact your organization administrator to access compliance features.
              </p>
            </div>
          ) : (
            <p className="text-sm text-ink-secondary">{message}</p>
          )}
        </div>
        {!is403 && (
          <button
            onClick={onRetry}
            className="px-4 py-2 bg-brand text-white rounded-pill shadow-glow hover:bg-brand-hover transition-colors"
          >
            Retry
          </button>
        )}
      </div>
    </div>
  );
}

// Main Page Component
export default function CompliancePage() {
  const router = useRouter();
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [status, setStatus] = useState<ComplianceStatus | null>(null);
  const [metrics, setMetrics] = useState<ComplianceMetrics | null>(null);
  const [accessReview, setAccessReview] = useState<AccessReviewUser[]>([]);
  const [checkResults, setCheckResults] = useState<CheckResult[] | null>(null);
  const [agents, setAgents] = useState<AgentEntry[]>([]);
  const [mcpServers, setMcpServers] = useState<MCPServerEntry[]>([]);
  const [alerts, setAlerts] = useState<AlertEntry[]>([]);
  const [activitySummary, setActivitySummary] = useState<{
    attestationCount: number;
    verificationCount: number;
  } | null>(null);
  const [runningCheck, setRunningCheck] = useState(false);
  const [exportingReport, setExportingReport] = useState(false);
  // A failed check or export, shown inline under the header with the step that gets past it.
  const [actionError, setActionError] = useState<{ title: string; detail: string; nextStep: string } | null>(null);
  // Framework for exports only (not for display)
  const exportFramework = "aim";

  // Compliance Activity Timeline state
  const [auditLogs, setAuditLogs] = useState<AuditLogEntry[]>([]);
  const [verificationEvents, setVerificationEvents] = useState<any[]>([]);
  const [auditLogFilter, setAuditLogFilter] = useState<{
    eventType: string;
    search: string;
  }>({
    eventType: "all",
    search: "",
  });

  // Debounce search input for better performance
  const debouncedAuditSearch = useDebounce(auditLogFilter.search, 300);
  const [auditTrailExpanded, setAuditTrailExpanded] = useState(true);
  const [exportingAuditLogs, setExportingAuditLogs] = useState(false);
  const [auditTrailPage, setAuditTrailPage] = useState(1);
  const AUDIT_TRAIL_PAGE_SIZE = 25;

  const fetchComplianceData = async () => {
    try {
      setLoading(true);
      setError(null);
      const [statusData, metricsData, accessData, agentsData, mcpData, alertsData, activityData, auditData, verificationData] = await Promise.all([
        api.getComplianceStatus(),
        api.getComplianceMetrics(),
        api.getAccessReview(),
        api.listAgents(),
        api.listMCPServers(100, 0), // Get MCP servers for compliance tracking
        api.getAlerts(50, 0), // Get 50 most recent alerts (violations)
        api.getActivitySummary(30), // Get last 30 days activity including attestations
        api.getAuditLogs(200, 0), // Get 200 most recent audit logs
        api.getRecentVerificationEvents(10080), // Get last 7 days of verification events (capability usage)
      ]);
      setStatus(statusData);
      setMetrics(metricsData);
      setAccessReview(accessData.users || []);
      setAgents(agentsData.agents || []);
      setMcpServers(mcpData.mcpServers || []);
      setAlerts(alertsData.alerts || []);
      setActivitySummary(activityData.summary || null);
      setAuditLogs(auditData || []);
      setVerificationEvents(verificationData?.events || []);
    } catch (err) {
      console.error("Failed to fetch compliance data:", err);
      setError(err instanceof Error ? err.message : "An unknown error occurred");
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchComplianceData();
    // Auto-run compliance check on page load
    const runInitialCheck = async () => {
      try {
        setRunningCheck(true);
        const result = await api.runComplianceCheck();
        setCheckResults(result.checks);
      } catch (err) {
        console.error("Failed to run initial compliance check:", err);
      } finally {
        setRunningCheck(false);
      }
    };
    runInitialCheck();
  }, []);

  const handleRunComplianceCheck = async () => {
    try {
      setActionError(null);
      setRunningCheck(true);
      const result = await api.runComplianceCheck();
      setCheckResults(result.checks);
    } catch (err) {
      console.error("Failed to run compliance check:", err);
      setActionError({
        title: "The compliance check did not run",
        detail: err instanceof Error ? err.message : "Unknown error",
        nextStep: "Reload the page to run the checks again. If they fail again, check that this browser can reach the AIM server.",
      });
    } finally {
      setRunningCheck(false);
    }
  };

  const handleExportReport = async (format: "csv" | "json") => {
    try {
      setActionError(null);
      setExportingReport(true);
      const result = await api.exportComplianceReport(format, exportFramework);

      if (format === "csv" && result instanceof Blob) {
        const url = window.URL.createObjectURL(result);
        const a = document.createElement("a");
        a.href = url;
        a.download = `compliance-report-aim-${new Date().toISOString().split("T")[0]}.csv`;
        document.body.appendChild(a);
        a.click();
        window.URL.revokeObjectURL(url);
        document.body.removeChild(a);
      } else {
        // JSON export - download as file
        const blob = new Blob([JSON.stringify(result, null, 2)], { type: "application/json" });
        const url = window.URL.createObjectURL(blob);
        const a = document.createElement("a");
        a.href = url;
        a.download = `compliance-report-aim-${new Date().toISOString().split("T")[0]}.json`;
        document.body.appendChild(a);
        a.click();
        window.URL.revokeObjectURL(url);
        document.body.removeChild(a);
      }
    } catch (err) {
      console.error("Failed to export report:", err);
      setActionError({
        title: "The compliance report did not export",
        detail: err instanceof Error ? err.message : "Unknown error",
        nextStep: "Choose Export Report again. If it fails again, select Refresh to reload the compliance data, then export.",
      });
    } finally {
      setExportingReport(false);
    }
  };

  // Unified compliance event type
  interface ComplianceEvent {
    id: string;
    type: "violation" | "capability" | "attestation" | "registration" | "verification";
    severity: "info" | "low" | "medium" | "high" | "critical";
    title: string;
    description: string;
    resourceType: string;
    resourceName?: string;
    agentName?: string;
    timestamp: string;
    metadata?: Record<string, any>;
  }

  // Build unified compliance timeline from multiple sources (memoized for performance)
  const unifiedTimeline = useMemo((): ComplianceEvent[] => {
    const events: ComplianceEvent[] = [];

    // 1. Add violations from alerts
    for (const alert of alerts) {
      events.push({
        id: `alert-${alert.id}`,
        type: "violation",
        severity: alert.severity,
        title: alert.title,
        description: alert.description,
        resourceType: alert.resourceType || "security",
        timestamp: alert.createdAt,
        metadata: { alertType: alert.alertType, isAcknowledged: alert.isAcknowledged },
      });
    }

    // 2. Add capability usage from verification events
    for (const event of verificationEvents) {
      const isSuccess = event.status === "success" || event.status === "allowed";
      const isFailed = event.status === "failed" || event.status === "denied";
      events.push({
        id: `verification-${event.id}`,
        type: "capability",
        severity: isFailed ? "high" : isSuccess ? "low" : "medium",
        title: `Capability ${isSuccess ? "Allowed" : isFailed ? "Denied" : "Checked"}: ${event.action || event.verificationType || "Unknown"}`,
        description: event.resource ? `Resource: ${event.resource}` : `Agent verification ${event.status}`,
        resourceType: "agent",
        agentName: event.agentName,
        timestamp: event.createdAt,
        metadata: {
          trustScore: event.trustScore,
          durationMs: event.durationMs,
          status: event.status,
          reason: event.reason || event.errorReason,
        },
      });
    }

    // 3. Add agent registrations from agents list (created in last 30 days)
    const thirtyDaysAgo = new Date();
    thirtyDaysAgo.setDate(thirtyDaysAgo.getDate() - 30);
    for (const agent of agents) {
      const createdAt = new Date(agent.createdAt);
      if (createdAt >= thirtyDaysAgo) {
        events.push({
          id: `agent-reg-${agent.id}`,
          type: "registration",
          severity: "info",
          title: `Agent Registered: ${agent.displayName || agent.name}`,
          description: `New ${agent.agentType === "ai_agent" ? "AI Agent" : "Agent"} registered with status: ${agent.status}`,
          resourceType: "agent",
          resourceName: agent.displayName || agent.name,
          timestamp: agent.createdAt,
          metadata: { trustScore: agent.trustScore, status: agent.status },
        });
      }
    }

    // 4. Add MCP server registrations from mcpServers list (created in last 30 days)
    for (const mcp of mcpServers) {
      const createdAt = new Date(mcp.createdAt);
      if (createdAt >= thirtyDaysAgo) {
        events.push({
          id: `mcp-reg-${mcp.id}`,
          type: "registration",
          severity: "info",
          title: `MCP Server Registered: ${mcp.name}`,
          description: `New MCP server registered: ${mcp.url}`,
          resourceType: "mcp_server",
          resourceName: mcp.name,
          timestamp: mcp.createdAt,
          metadata: { status: mcp.status, isVerified: mcp.isVerified },
        });
      }
    }

    // Sort by timestamp descending (newest first)
    events.sort((a, b) => new Date(b.timestamp).getTime() - new Date(a.timestamp).getTime());

    // Apply filters (using debounced search for better performance)
    return events.filter(event => {
      if (auditLogFilter.eventType !== "all" && event.type !== auditLogFilter.eventType) {
        return false;
      }
      if (debouncedAuditSearch) {
        const searchLower = debouncedAuditSearch.toLowerCase();
        const matchesTitle = event.title.toLowerCase().includes(searchLower);
        const matchesDescription = event.description.toLowerCase().includes(searchLower);
        const matchesAgent = event.agentName?.toLowerCase().includes(searchLower);
        const matchesResource = event.resourceName?.toLowerCase().includes(searchLower);
        if (!matchesTitle && !matchesDescription && !matchesAgent && !matchesResource) {
          return false;
        }
      }
      return true;
    });
  }, [alerts, verificationEvents, agents, mcpServers, auditLogFilter.eventType, debouncedAuditSearch]);

  // Export compliance timeline
  const handleExportAuditLogs = async (format: "csv" | "json") => {
    try {
      setActionError(null);
      setExportingAuditLogs(true);
      const timeline = unifiedTimeline;

      if (format === "csv") {
        const headers = ["ID", "Timestamp", "Type", "Severity", "Title", "Description", "Resource Type", "Agent/Resource Name"];
        const rows = timeline.map(event => [
          event.id,
          event.timestamp,
          event.type,
          event.severity,
          `"${event.title.replace(/"/g, '""')}"`,
          `"${event.description.replace(/"/g, '""')}"`,
          event.resourceType,
          event.agentName || event.resourceName || "N/A",
        ]);
        const csvContent = [headers.join(","), ...rows.map(r => r.join(","))].join("\n");
        const blob = new Blob([csvContent], { type: "text/csv" });
        const url = window.URL.createObjectURL(blob);
        const a = document.createElement("a");
        a.href = url;
        a.download = `compliance-timeline-${new Date().toISOString().split("T")[0]}.csv`;
        document.body.appendChild(a);
        a.click();
        window.URL.revokeObjectURL(url);
        document.body.removeChild(a);
      } else {
        const blob = new Blob([JSON.stringify(timeline, null, 2)], { type: "application/json" });
        const url = window.URL.createObjectURL(blob);
        const a = document.createElement("a");
        a.href = url;
        a.download = `compliance-timeline-${new Date().toISOString().split("T")[0]}.json`;
        document.body.appendChild(a);
        a.click();
        window.URL.revokeObjectURL(url);
        document.body.removeChild(a);
      }
    } catch (err) {
      console.error("Failed to export timeline:", err);
      setActionError({
        title: "The audit trail did not export",
        detail: err instanceof Error ? err.message : "Unknown error",
        nextStep: "Choose Export Logs again. If it fails again, narrow the audit trail with the search or event filter and export the smaller set.",
      });
    } finally {
      setExportingAuditLogs(false);
    }
  };

  // Get icon and color for compliance event
  const getEventStyle = (event: ComplianceEvent) => {
    // By type
    const danger = { color: "text-danger", bg: "bg-danger-fill", badge: "bg-danger-fill text-danger-text" };
    const warning = { color: "text-warning", bg: "bg-warning-fill", badge: "bg-warning-fill text-warning-text" };
    const success = { color: "text-success-text", bg: "bg-success-fill", badge: "bg-success-fill text-success-text" };
    const brand = { color: "text-brand-text", bg: "bg-brand-soft", badge: "bg-brand-soft text-brand-text" };
    if (event.type === "violation") {
      if (event.severity === "critical") return { icon: XCircle, ...danger };
      return { icon: AlertTriangle, ...warning };
    }
    if (event.type === "capability") {
      if (event.severity === "high") return { icon: Shield, ...danger };
      return { icon: Shield, ...brand };
    }
    if (event.type === "attestation") {
      return { icon: FileCheck, ...success };
    }
    if (event.type === "registration") {
      return { icon: event.resourceType === "agent" ? Bot : Server, ...brand };
    }
    return { icon: Activity, color: "text-ink-tertiary", bg: "bg-glass-inset-gray", badge: "bg-glass-inset-gray text-ink-body" };
  };

  if (loading) return <CompliancePageSkeleton />;
  if (error && !status) return <ErrorDisplay message={error} onRetry={fetchComplianceData} />;

  // Calculate compliance summary from check results
  const passedChecks = checkResults?.filter(c => c.passed).length || 0;
  const totalChecks = checkResults?.length || 0;
  const complianceRate = totalChecks > 0 ? Math.round((passedChecks / totalChecks) * 100) : 0;

  // Show audit logs count with "+" if at limit (100)
  const auditLogCount = status?.recentAuditCount || 0;
  const auditLogDisplay = auditLogCount >= 100 ? "100+" : auditLogCount.toString();

  // Calculate MCP verification stats
  const verifiedMcps = mcpServers.filter(m => m.isVerified || m.status === "verified").length;
  const totalMcps = mcpServers.length;

  // Get attestation count from activity summary
  const attestationCount = activitySummary?.attestationCount || 0;

  const stats = [
    { name: "Avg Trust Score", value: `${Math.round(status?.averageTrustScore || 0)}%`, icon: Shield, iconColor: (status?.averageTrustScore || 0) >= 80 ? "text-success" : (status?.averageTrustScore || 0) >= 60 ? "text-warning" : "text-danger" },
    { name: "Verified Agents", value: `${status?.verifiedAgents || 0} / ${status?.totalAgents || 0}`, icon: CheckCircle, iconColor: "text-success" },
    { name: "Verified MCPs", value: `${verifiedMcps} / ${totalMcps}`, icon: ShieldCheck, iconColor: verifiedMcps === totalMcps && totalMcps > 0 ? "text-success" : verifiedMcps > 0 ? "text-warning" : "text-ink-tertiary" },
    { name: "MCP Attestations", value: `${attestationCount}`, icon: FileCheck, iconColor: attestationCount > 0 ? "text-brand-text" : "text-ink-tertiary" },
  ];

  return (
    <AuthGuard>
      <div className="space-y-6">
        {/* Header */}
        <div className="flex flex-col lg:flex-row lg:items-center lg:justify-between gap-4">
          <div>
            <h1 className="text-2xl font-bold text-ink flex items-center gap-3">
              <Shield className="h-7 w-7 text-brand-text" />
              Compliance Dashboard
            </h1>
            <p className="mt-1 text-sm text-ink-secondary">
              Monitor agent and MCP server verification, trust scores, and security compliance for your organization.
            </p>
          </div>
          <div className="flex flex-wrap gap-2 items-center">
            <button
              onClick={() => fetchComplianceData()}
              className="inline-flex items-center gap-2 px-3 py-2 text-sm font-medium text-ink bg-glass border border-stroke rounded-pill hover:bg-glass-inset transition-colors"
            >
              <RefreshCw className="h-4 w-4" />
              Refresh
            </button>

            {/* Export Dropdown */}
            <div className="relative group">
              <button
                disabled={exportingReport}
                className="inline-flex items-center gap-2 px-3 py-2 text-sm font-medium text-ink bg-glass border border-stroke rounded-pill hover:bg-glass-inset transition-colors disabled:opacity-50"
              >
                {exportingReport ? <Loader2 className="h-4 w-4 animate-spin" /> : <Download className="h-4 w-4" />}
                Export Report
              </button>
              <div className="absolute right-0 mt-1 w-40 overlay-surface rounded-inset-sm overflow-hidden opacity-0 invisible group-hover:opacity-100 group-hover:visible transition-all z-10">
                <button
                  onClick={() => handleExportReport("json")}
                  className="w-full px-4 py-2 text-sm text-left text-ink-body hover:bg-glass-inset-gray"
                >
                  Export as JSON
                </button>
                <button
                  onClick={() => handleExportReport("csv")}
                  className="w-full px-4 py-2 text-sm text-left text-ink-body hover:bg-glass-inset-gray"
                >
                  Export as CSV
                </button>
              </div>
            </div>

          </div>
        </div>

        {actionError && (
          <Alert variant="destructive">
            <button
              type="button"
              onClick={() => setActionError(null)}
              aria-label="Dismiss"
              className="absolute right-3 top-3 rounded p-1 hover:bg-destructive/10 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-destructive"
            >
              <X className="h-4 w-4" aria-hidden="true" />
            </button>
            <AlertTriangle className="h-4 w-4" aria-hidden="true" />
            <AlertTitle className="pr-8">{actionError.title}</AlertTitle>
            <AlertDescription className="pr-8">
              <p>{actionError.detail}</p>
              <p className="mt-1">{actionError.nextStep}</p>
            </AlertDescription>
          </Alert>
        )}

        {/* Key Metrics */}
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
          {stats.map((stat) => (
            <StatCard key={stat.name} stat={stat} />
          ))}
        </div>

        {/* ABOM Summary Card */}
        <div className="glass p-4">
          <div className="flex flex-wrap items-center justify-between gap-4">
            <div className="flex items-center gap-4 min-w-0">
              <div className="p-3 bg-brand-soft rounded-lg flex-shrink-0">
                <Database className="h-6 w-6 text-brand-text" />
              </div>
              <div className="min-w-0">
                <h3 className="text-sm font-semibold text-ink">Agent Bill of Materials (ABOM)</h3>
                <p className="text-xs text-ink-secondary">
                  Complete inventory of all agents, MCP servers, tools, and data access patterns observed by AIM
                </p>
              </div>
            </div>
            <Link
              href="/dashboard/mcp/supply-chain?tab=abom"
              className="inline-flex items-center gap-2 px-4 py-2 text-sm font-medium text-brand-text bg-brand-soft rounded-pill hover:brightness-95 transition-colors"
            >
              View ABOM
              <ExternalLink className="h-4 w-4" />
            </Link>
          </div>
        </div>

        {/* Compliance Categories - Based on Actual Check Results */}
        {checkResults && checkResults.length > 0 && (
          <div>
            <div className="mb-4">
              <h2 className="text-lg font-semibold text-ink">Compliance Status</h2>
              <p className="text-sm text-ink-secondary">Real-time security and operational health checks</p>
            </div>
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
              {/* Security Checks - focuses on security posture */}
              <ComplianceSummaryCard
                title="Security Compliance"
                description="Credential hygiene, trust scores, violations, and audit coverage"
                passed={checkResults.filter(c => ["apiKeyRotationNeeded", "trustScoreDegradation", "capabilityViolations", "adminAccessReview", "auditLogGaps"].includes(c.name) && c.passed).length}
                total={checkResults.filter(c => ["apiKeyRotationNeeded", "trustScoreDegradation", "capabilityViolations", "adminAccessReview", "auditLogGaps"].includes(c.name)).length}
                icon={Lock}
                color="brand"
                checks={checkResults.filter(c => ["apiKeyRotationNeeded", "trustScoreDegradation", "capabilityViolations", "adminAccessReview", "auditLogGaps"].includes(c.name))}
                onNavigate={(url) => router.push(url)}
              />

              {/* Operations Checks - focuses on operational health */}
              <ComplianceSummaryCard
                title="Operations Compliance"
                description="Agent and MCP server activity, verification queues, and resource ownership"
                passed={checkResults.filter(c => ["inactiveAgents", "unverifiedAgentBacklog", "orphanedResources", "inactiveMCPServers", "unverifiedMCPBacklog"].includes(c.name) && c.passed).length}
                total={checkResults.filter(c => ["inactiveAgents", "unverifiedAgentBacklog", "orphanedResources", "inactiveMCPServers", "unverifiedMCPBacklog"].includes(c.name)).length}
                icon={Activity}
                color="success"
                checks={checkResults.filter(c => ["inactiveAgents", "unverifiedAgentBacklog", "orphanedResources", "inactiveMCPServers", "unverifiedMCPBacklog"].includes(c.name))}
                onNavigate={(url) => router.push(url)}
              />
            </div>
          </div>
        )}

        {/* Main Content Grid */}
        <div className="grid grid-cols-1 xl:grid-cols-3 gap-6">
          {/* Left Column - Charts & Risk */}
          <div className="xl:col-span-2 space-y-6">
            {/* Trust Score Trend */}
            <div className="glass p-6">
              <div className="flex items-center justify-between mb-4">
                <div>
                  <h3 className="text-lg font-semibold text-ink">Trust Score Trend</h3>
                  <p className="text-sm text-ink-secondary">30-day compliance score history</p>
                </div>
                <TrendingUp className="h-5 w-5 text-ink-tertiary" />
              </div>
              <div className="h-64">
                <TrendChart
                  title="Average trust score, last 30 days"
                  data={metrics?.metrics?.trustScoreTrend?.map((d) => ({ date: d.date, score: Math.round(d.avgScore * 100) })) || []}
                  xKey="date"
                  series={[{ key: "score", name: "Trust Score (%)", color: chartSeries.green }]}
                  yDomain={[0, 100]}
                  formatValue={(v) => `${v}%`}
                />
              </div>
            </div>

            {/* Risk Assessment & Compliance Check Results */}
            <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
              {/* Risk Distribution */}
              <div className="glass p-6">
                <div className="flex items-center justify-between mb-4">
                  <h3 className="text-lg font-semibold text-ink">Risk Distribution</h3>
                  <BarChart3 className="h-5 w-5 text-ink-tertiary" />
                </div>
                <RiskDistributionChart agents={agents} mcpServers={mcpServers} />
              </div>

              {/* Compliance Checks */}
              <div className="glass p-6">
                <div className="flex items-center justify-between mb-4">
                  <h3 className="text-lg font-semibold text-ink">Compliance Checks</h3>
                  {runningCheck ? (
                    <Loader2 className="h-5 w-5 text-brand animate-spin" />
                  ) : (
                    <FileCheck className="h-5 w-5 text-ink-tertiary" />
                  )}
                </div>
                {runningCheck ? (
                  <div className="flex flex-col items-center justify-center h-40 text-center">
                    <Loader2 className="h-8 w-8 text-brand animate-spin mb-2" />
                    <p className="text-sm text-ink-secondary">Running compliance checks...</p>
                  </div>
                ) : checkResults && checkResults.length > 0 ? (
                  <div className="space-y-2 max-h-48 overflow-y-auto">
                    {checkResults.slice(0, 6).map((result, idx) => (
                      <div key={idx} className={`flex items-center gap-2 p-2 rounded-lg ${result.passed ? "bg-success-fill" : "bg-danger-fill"}`}>
                        {result.passed ? (
                          <CheckCircle className="h-4 w-4 text-success-text flex-shrink-0" />
                        ) : (
                          <XCircle className="h-4 w-4 text-danger-text flex-shrink-0" />
                        )}
                        <span className="text-sm text-ink-body truncate">
                          {result.name.replace(/_/g, " ")}
                        </span>
                      </div>
                    ))}
                  </div>
                ) : (
                  <div className="flex flex-col items-center justify-center h-40 text-center">
                    <CheckCircle className="h-10 w-10 text-ink-tertiary mb-2" />
                    <p className="text-sm text-ink-secondary">No compliance issues detected</p>
                  </div>
                )}
              </div>
            </div>

          </div>

          {/* Right Column - Security Alerts */}
          <div className="flex flex-col">
            {/* Security Alerts & Violations */}
            <div className="glass p-6 flex-1 flex flex-col">
              <div className="flex items-center justify-between mb-4">
                <div>
                  <h3 className="font-semibold text-ink">Security Alerts</h3>
                  <p className="text-xs text-ink-secondary">Recent violations and warnings</p>
                </div>
                <AlertTriangle className="h-5 w-5 text-ink-tertiary" />
              </div>
              <div className="space-y-0 flex-1">
                {alerts.length > 0 ? (
                  alerts.slice(0, 8).map((alert) => (
                    <AlertActivityItem key={alert.id} alert={alert} />
                  ))
                ) : (
                  <div className="flex flex-col items-center justify-center h-32 text-center">
                    <CheckCircle className="h-8 w-8 text-success mb-2" />
                    <p className="text-sm font-medium text-ink-body">All Clear</p>
                    <p className="text-xs text-ink-secondary">No security alerts</p>
                  </div>
                )}
              </div>
              {alerts.length > 8 && (
                <button
                  onClick={() => router.push("/dashboard/admin/alerts")}
                  className="mt-3 min-h-6 text-sm text-brand-text hover:underline text-center"
                >
                  View all {alerts.length} alerts →
                </button>
              )}
            </div>
          </div>
        </div>

        {/* Access Review Table */}
        <div className="glass overflow-hidden">
          <div className="p-6 border-b border-divider">
            <div className="flex items-center justify-between">
              <div>
                <h3 className="text-lg font-semibold text-ink">Access Review</h3>
                <p className="text-sm text-ink-secondary">Review user access patterns and permissions</p>
              </div>
              <Users className="h-5 w-5 text-ink-tertiary" />
            </div>
          </div>
          <div className="overflow-x-auto">
            <table className="w-full divide-y divide-divider">
              <thead className="bg-glass-inset-gray">
                <tr>
                  <th className="px-6 py-3 text-left text-xs font-medium text-ink-secondary uppercase tracking-wider">User</th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-ink-secondary uppercase tracking-wider">Email</th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-ink-secondary uppercase tracking-wider">Role</th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-ink-secondary uppercase tracking-wider">Status</th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-ink-secondary uppercase tracking-wider">Last Login</th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-ink-secondary uppercase tracking-wider">Created</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-divider">
                {accessReview.map((user) => (
                  <tr key={user.id} className="hover:bg-glass-inset-gray transition-colors">
                    <td className="px-6 py-4 whitespace-nowrap text-sm font-medium text-ink">{user.name}</td>
                    <td className="px-6 py-4 whitespace-nowrap text-sm text-ink-secondary">{user.email}</td>
                    <td className="px-6 py-4 whitespace-nowrap">
                      <span className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium capitalize ${
                        user.role === "admin"
                          ? "bg-brand-soft text-brand-text"
                          : "bg-glass-inset-gray text-ink-body"
                      }`}>
                        {user.role}
                      </span>
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap">
                      <span className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium capitalize ${
                        user.status === "active"
                          ? "bg-success-fill text-success-text"
                          : "bg-glass-inset-gray text-ink-body"
                      }`}>
                        {user.status}
                      </span>
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap text-sm text-ink-secondary">
                      {user.lastLogin ? formatDateTime(user.lastLogin) : "Never"}
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap text-sm text-ink-secondary">
                      {formatDateTime(user.createdAt)}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>

        {/* Comprehensive Audit Trail Section */}
        <div className="glass">
          <div className="p-6 border-b border-divider">
            <div className="flex items-center justify-between gap-3">
              <div className="flex items-center gap-3 min-w-0">
                <button
                  onClick={() => setAuditTrailExpanded(!auditTrailExpanded)}
                  className="p-1 hover:bg-glass-inset-gray rounded transition-colors"
                >
                  {auditTrailExpanded ? (
                    <ChevronUp className="h-5 w-5 text-ink-tertiary" />
                  ) : (
                    <ChevronDown className="h-5 w-5 text-ink-tertiary" />
                  )}
                </button>
                <div className="min-w-0">
                  <h3 className="text-lg font-semibold text-ink flex items-center gap-2">
                    <Clock className="h-5 w-5 text-brand-text" />
                    Audit Trail
                  </h3>
                  <p className="text-sm text-ink-secondary">
                    Control-plane lifecycle events (agent create/update, MCP changes, user actions). For runtime verification activity on a specific agent, open that agent and use the <span className="font-medium">Recent Activity</span> timeline.
                  </p>
                </div>
              </div>
              <div className="flex items-center gap-2">
                {/* Export Dropdown for Audit Logs */}
                <div className="relative group">
                  <button
                    disabled={exportingAuditLogs}
                    className="inline-flex items-center gap-2 px-3 py-2 text-sm font-medium text-ink bg-glass border border-stroke rounded-pill hover:bg-glass-inset transition-colors disabled:opacity-50 whitespace-nowrap"
                  >
                    {exportingAuditLogs ? <Loader2 className="h-4 w-4 animate-spin" /> : <Download className="h-4 w-4" />}
                    Export Logs
                  </button>
                  <div className="absolute right-0 mt-1 w-40 overlay-surface rounded-inset-sm overflow-hidden opacity-0 invisible group-hover:opacity-100 group-hover:visible transition-all z-10">
                    <button
                      onClick={() => handleExportAuditLogs("json")}
                      className="w-full px-4 py-2 text-sm text-left text-ink-body hover:bg-glass-inset-gray"
                    >
                      Export as JSON
                    </button>
                    <button
                      onClick={() => handleExportAuditLogs("csv")}
                      className="w-full px-4 py-2 text-sm text-left text-ink-body hover:bg-glass-inset-gray"
                    >
                      Export as CSV
                    </button>
                  </div>
                </div>
              </div>
            </div>
          </div>

          {auditTrailExpanded && (
            <>
              {/* Filters */}
              <div className="p-4 border-b border-divider bg-glass-inset-gray">
                <div className="flex flex-wrap items-center gap-3">
                  {/* Search */}
                  <div className="relative flex-1 min-w-[200px] max-w-xs">
                    <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-ink-tertiary" />
                    <input
                      type="text"
                      placeholder="Search events, agents, resources..."
                      value={auditLogFilter.search}
                      onChange={(e) => setAuditLogFilter(prev => ({ ...prev, search: e.target.value }))}
                      className="w-full pl-9 pr-3 py-2 text-sm border border-stroke rounded-inset bg-glass-inset text-ink placeholder:text-ink-tertiary focus:outline-none focus:ring-2 focus:ring-brand"
                    />
                  </div>

                  {/* Event Type Filter */}
                  <div className="flex items-center gap-2">
                    <Filter className="h-4 w-4 text-ink-tertiary" />
                    <select
                      value={auditLogFilter.eventType}
                      onChange={(e) => setAuditLogFilter(prev => ({ ...prev, eventType: e.target.value }))}
                      className="text-sm border border-stroke rounded-inset px-3 py-2 bg-glass-inset text-ink focus:outline-none focus:ring-2 focus:ring-brand"
                    >
                      <option value="all">All Events</option>
                      <option value="violation">Violations</option>
                      <option value="capability">Capability Usage</option>
                      <option value="attestation">Attestations</option>
                      <option value="registration">Registrations</option>
                    </select>
                  </div>

                  {/* Results count */}
                  <span className="text-sm text-ink-secondary">
                    {unifiedTimeline.length} events
                  </span>

                  {/* Reset page on filter change */}
                  {(auditLogFilter.search || auditLogFilter.eventType !== "all") && auditTrailPage > 1 && (
                    <button
                      onClick={() => setAuditTrailPage(1)}
                      className="min-h-6 text-xs text-brand-text hover:underline"
                    >
                      Reset page
                    </button>
                  )}
                </div>
              </div>

              {/* Compliance Timeline */}
              <div className="p-6">
                {unifiedTimeline.length > 0 ? (
                  <div className="relative">
                    {/* Vertical line */}
                    <div className="absolute left-4 top-0 bottom-0 w-0.5 bg-track" />

                    <div className="space-y-4">
                      {unifiedTimeline.slice(0, auditTrailPage * AUDIT_TRAIL_PAGE_SIZE).map((event) => {
                        const { icon: Icon, color, bg, badge } = getEventStyle(event);
                        return (
                          <div key={event.id} className="relative flex items-start gap-4 pl-10">
                            {/* Timeline dot: an opaque disc under the tint, so the line stops at the dot */}
                            <div className="absolute left-2 w-5 h-5 rounded-full bg-page">
                              <div className={`w-full h-full rounded-full ${bg} flex items-center justify-center`}>
                                <Icon className={`h-3 w-3 ${color}`} />
                              </div>
                            </div>

                            {/* Content. min-w-0 lets the box shrink to the row, so a long description is cut with an
                                ellipsis. The box is outlined, not filled: a badge's tint over a second tint loses contrast. */}
                            <div className="flex-1 min-w-0 border border-divider rounded-inset-sm p-4">
                              <div className="flex items-start justify-between gap-4">
                                <div className="min-w-0 flex-1">
                                  <p className="text-sm font-medium text-ink break-words">
                                    {event.title}
                                  </p>
                                  <p className="text-xs text-ink-secondary mt-1 truncate">
                                    {event.description}
                                  </p>
                                  <div className="flex items-center gap-2 mt-2 flex-wrap">
                                    {/* Event type badge */}
                                    <span className={`inline-flex items-center px-2 py-0.5 rounded text-xs font-medium capitalize ${badge}`}>
                                      {event.type}
                                    </span>
                                    {/* Severity badge for violations/high severity events */}
                                    {(event.type === "violation" || event.severity === "high" || event.severity === "critical") && (
                                      <span className={`inline-flex items-center px-2 py-0.5 rounded text-xs font-medium uppercase ${
                                        event.severity === "critical" ? "bg-danger-fill text-danger-text" :
                                        event.severity === "high" || event.severity === "medium" ? "bg-warning-fill text-warning-text" :
                                        "bg-brand-soft text-brand-text"
                                      }`}>
                                        {event.severity}
                                      </span>
                                    )}
                                    {/* Agent/Resource name */}
                                    {(event.agentName || event.resourceName) && (
                                      <span className="text-xs text-ink-secondary break-all">
                                        {event.agentName || event.resourceName}
                                      </span>
                                    )}
                                    {/* Trust score for capability events */}
                                    {event.metadata?.trustScore !== undefined && (
                                      <span className="text-xs text-ink-secondary">
                                        Trust: {Math.round(event.metadata.trustScore * 100)}%
                                      </span>
                                    )}
                                  </div>
                                </div>
                                <div className="text-right flex-shrink-0">
                                  <p className="text-xs text-ink-secondary">
                                    {formatRelativeTime(event.timestamp)}
                                  </p>
                                  {/* Status indicator */}
                                  {event.metadata?.status && (
                                    <p className={`text-xs mt-0.5 ${
                                      event.metadata.status === "success" || event.metadata.status === "allowed" ? "text-success-text" :
                                      event.metadata.status === "failed" || event.metadata.status === "denied" ? "text-danger-text" :
                                      "text-ink-secondary"
                                    }`}>
                                      {event.metadata.status}
                                    </p>
                                  )}
                                  {/* Acknowledged indicator for violations */}
                                  {event.metadata?.isAcknowledged && (
                                    <p className="text-xs text-success-text mt-0.5">acknowledged</p>
                                  )}
                                </div>
                              </div>
                            </div>
                          </div>
                        );
                      })}
                    </div>

                    {/* Pagination Controls */}
                    {unifiedTimeline.length > 0 && (
                      <div className="mt-6 flex flex-wrap items-center justify-between gap-3 pt-4 border-t border-divider">
                        <p className="text-sm text-ink-secondary">
                          Showing {Math.min(auditTrailPage * AUDIT_TRAIL_PAGE_SIZE, unifiedTimeline.length)} of {unifiedTimeline.length} events
                        </p>
                        <div className="flex items-center gap-2">
                          {auditTrailPage > 1 && (
                            <button
                              onClick={() => setAuditTrailPage(1)}
                              className="px-3 py-1.5 text-sm font-medium text-ink-body bg-glass-inset-gray rounded-pill hover:bg-glass-inset transition-colors"
                            >
                              Show Less
                            </button>
                          )}
                          {auditTrailPage * AUDIT_TRAIL_PAGE_SIZE < unifiedTimeline.length && (
                            <button
                              onClick={() => setAuditTrailPage(auditTrailPage + 1)}
                              className="px-3 py-1.5 text-sm font-medium text-white bg-brand rounded-pill hover:bg-brand-hover transition-colors"
                            >
                              Load More ({Math.min(AUDIT_TRAIL_PAGE_SIZE, unifiedTimeline.length - auditTrailPage * AUDIT_TRAIL_PAGE_SIZE)} more)
                            </button>
                          )}
                        </div>
                      </div>
                    )}
                  </div>
                ) : (
                  <div className="text-center py-12">
                    <Clock className="h-12 w-12 text-ink-tertiary mx-auto mb-3" />
                    <p className="text-ink-secondary">No compliance events found</p>
                    <p className="text-sm text-ink-secondary mt-1">
                      {auditLogFilter.search || auditLogFilter.eventType !== "all"
                        ? "Try adjusting your filters"
                        : "Violations, capability usage, and registrations will appear here"}
                    </p>
                  </div>
                )}
              </div>
            </>
          )}
        </div>
      </div>
    </AuthGuard>
  );
}
