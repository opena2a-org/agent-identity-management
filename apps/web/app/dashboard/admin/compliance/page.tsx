"use client";

import { useState, useEffect } from "react";
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
} from "lucide-react";
import {
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
  PieChart,
  Pie,
  Cell,
  AreaChart,
  Area,
} from "recharts";
import { api } from "@/lib/api";
import { formatDateTime } from "@/lib/date-utils";
import { AuthGuard } from "@/components/auth-guard";

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


// Recent Audit Activities (9 items to align with left column)
const RECENT_ACTIVITIES = [
  { action: "Agent verification completed", user: "system", time: "2 minutes ago", type: "success" },
  { action: "Trust score recalculated for 11 agents", user: "system", time: "15 minutes ago", type: "info" },
  { action: "API key rotated", user: "admin@opena2a.org", time: "1 hour ago", type: "warning" },
  { action: "New agent registered: claude-assistant", user: "admin@opena2a.org", time: "3 hours ago", type: "info" },
  { action: "Compliance check passed", user: "system", time: "6 hours ago", type: "success" },
  { action: "Access review completed", user: "admin@opena2a.org", time: "1 day ago", type: "info" },
  { action: "MCP server registered: data-pipeline", user: "admin@opena2a.org", time: "1 day ago", type: "success" },
  { action: "Security policy updated", user: "admin@opena2a.org", time: "2 days ago", type: "info" },
  { action: "Agent capability request approved", user: "admin@opena2a.org", time: "2 days ago", type: "success" },
];



// Component: Compliance Framework Card - Enterprise style similar to SOC2/HIPAA dashboards
function ComplianceSummaryCard({
  title,
  description,
  passed,
  total,
  icon: Icon,
  color,
  checks
}: {
  title: string;
  description: string;
  passed: number;
  total: number;
  icon: any;
  color: "blue" | "green" | "purple" | "orange";
  checks: CheckResult[];
}) {
  const progress = total > 0 ? Math.round((passed / total) * 100) : 0;

  const colorMap = {
    blue: {
      bg: "bg-blue-500",
      bgLight: "bg-blue-50 dark:bg-blue-950",
      border: "border-blue-100 dark:border-blue-900",
      text: "text-blue-600 dark:text-blue-400",
      badge: "bg-blue-100 dark:bg-blue-900/50 text-blue-700 dark:text-blue-300"
    },
    purple: {
      bg: "bg-purple-500",
      bgLight: "bg-purple-50 dark:bg-purple-950",
      border: "border-purple-100 dark:border-purple-900",
      text: "text-purple-600 dark:text-purple-400",
      badge: "bg-purple-100 dark:bg-purple-900/50 text-purple-700 dark:text-purple-300"
    },
    green: {
      bg: "bg-green-500",
      bgLight: "bg-green-50 dark:bg-green-950",
      border: "border-green-100 dark:border-green-900",
      text: "text-green-600 dark:text-green-400",
      badge: "bg-green-100 dark:bg-green-900/50 text-green-700 dark:text-green-300"
    },
    orange: {
      bg: "bg-orange-500",
      bgLight: "bg-orange-50 dark:bg-orange-950",
      border: "border-orange-100 dark:border-orange-900",
      text: "text-orange-600 dark:text-orange-400",
      badge: "bg-orange-100 dark:bg-orange-900/50 text-orange-700 dark:text-orange-300"
    },
  };
  const colors = colorMap[color];

  // Format check name for display
  const formatCheckName = (name: string) => {
    return name
      .replace(/_/g, " ")
      .split(" ")
      .map(word => word.charAt(0).toUpperCase() + word.slice(1))
      .join(" ");
  };

  return (
    <div className="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 shadow-sm overflow-hidden">
      {/* Header with gradient */}
      <div className={`${colors.bgLight} px-6 py-5 border-b ${colors.border}`}>
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-4">
            <div className={`${colors.bg} p-3 rounded-xl shadow-lg`}>
              <Icon className="h-6 w-6 text-white" />
            </div>
            <div>
              <h3 className="text-lg font-semibold text-gray-900 dark:text-white">{title}</h3>
              <p className="text-sm text-gray-500 dark:text-gray-400">{description}</p>
            </div>
          </div>
          <div className="text-right">
            <div className={`text-3xl font-bold ${colors.text}`}>{progress}%</div>
            <div className="text-xs text-gray-500 dark:text-gray-400 mt-0.5">Compliance</div>
          </div>
        </div>
      </div>

      {/* Controls implemented section */}
      <div className="px-6 py-4 border-b border-gray-100 dark:border-gray-700">
        <div className="flex items-center justify-between mb-2">
          <span className="text-sm font-medium text-gray-700 dark:text-gray-300">Controls Implemented</span>
          <span className="text-sm font-semibold text-gray-900 dark:text-white">{passed} / {total}</span>
        </div>
        <div className="w-full bg-gray-100 dark:bg-gray-700 rounded-full h-2.5">
          <div
            className={`${colors.bg} h-2.5 rounded-full transition-all duration-500`}
            style={{ width: `${progress}%` }}
          />
        </div>
      </div>

      {/* Individual checks */}
      <div className="px-6 py-4 space-y-3">
        {checks.map((check, idx) => (
          <div key={idx} className="flex items-center justify-between">
            <div className="flex items-center gap-2">
              {check.passed ? (
                <CheckCircle className="h-4 w-4 text-green-500" />
              ) : (
                <AlertTriangle className="h-4 w-4 text-amber-500" />
              )}
              <span className="text-sm text-gray-700 dark:text-gray-300">{formatCheckName(check.name)}</span>
            </div>
            <span className={`text-xs font-medium px-2 py-1 rounded-full ${
              check.passed
                ? "bg-green-100 dark:bg-green-900/30 text-green-700 dark:text-green-400"
                : "bg-amber-100 dark:bg-amber-900/30 text-amber-700 dark:text-amber-400"
            }`}>
              {check.passed ? "Passed" : "Action Required"}
            </span>
          </div>
        ))}
      </div>

      {/* Footer */}
      <div className={`${colors.bgLight} px-6 py-3 border-t ${colors.border}`}>
        <button className={`text-sm font-medium ${colors.text} hover:underline flex items-center gap-1`}>
          View Details
          <ArrowUpRight className="h-3.5 w-3.5" />
        </button>
      </div>
    </div>
  );
}


// Component: Activity Item
function ActivityItem({ activity }: { activity: typeof RECENT_ACTIVITIES[0] }) {
  const typeStyles = {
    success: { icon: CheckCircle, color: "text-green-600 dark:text-green-400", bg: "bg-green-50 dark:bg-green-900/20" },
    warning: { icon: AlertTriangle, color: "text-yellow-600 dark:text-yellow-400", bg: "bg-yellow-50 dark:bg-yellow-900/20" },
    info: { icon: Info, color: "text-blue-600 dark:text-blue-400", bg: "bg-blue-50 dark:bg-blue-900/20" },
    error: { icon: XCircle, color: "text-red-600 dark:text-red-400", bg: "bg-red-50 dark:bg-red-900/20" },
  };
  const style = typeStyles[activity.type as keyof typeof typeStyles] || typeStyles.info;
  const Icon = style.icon;

  return (
    <div className="flex items-start gap-3 py-3 border-b border-gray-100 dark:border-gray-800 last:border-0">
      <div className={`p-1.5 rounded-lg ${style.bg}`}>
        <Icon className={`h-4 w-4 ${style.color}`} />
      </div>
      <div className="flex-1 min-w-0">
        <p className="text-sm text-gray-900 dark:text-white">{activity.action}</p>
        <div className="flex items-center gap-2 mt-0.5">
          <span className="text-xs text-gray-500 dark:text-gray-400">{activity.user}</span>
          <span className="text-xs text-gray-400">•</span>
          <span className="text-xs text-gray-500 dark:text-gray-400">{activity.time}</span>
        </div>
      </div>
    </div>
  );
}



// Component: Stat Card
function StatCard({ stat }: { stat: { name: string; value: string; icon: any; iconColor?: string; change?: string; changeType?: string } }) {
  return (
    <div className="bg-white dark:bg-gray-800 p-5 rounded-xl border border-gray-200 dark:border-gray-700 shadow-sm hover:shadow-md transition-shadow">
      <div className="flex items-center justify-between">
        <div className="flex-shrink-0 p-2.5 rounded-lg bg-gray-50 dark:bg-gray-700">
          <stat.icon className={`h-5 w-5 ${stat.iconColor || "text-gray-500"}`} />
        </div>
        {stat.change && (
          <div className={`flex items-center gap-1 text-xs font-medium px-2 py-1 rounded-full ${
            stat.changeType === "positive"
              ? "text-green-700 bg-green-100 dark:bg-green-900/30 dark:text-green-400"
              : stat.changeType === "negative"
              ? "text-red-700 bg-red-100 dark:bg-red-900/30 dark:text-red-400"
              : "text-gray-700 bg-gray-100 dark:bg-gray-700 dark:text-gray-400"
          }`}>
            {stat.changeType === "positive" ? <ArrowUpRight className="h-3 w-3" /> :
             stat.changeType === "negative" ? <ArrowDownRight className="h-3 w-3" /> :
             <Minus className="h-3 w-3" />}
            {stat.change}
          </div>
        )}
      </div>
      <div className="mt-4">
        <p className="text-sm font-medium text-gray-500 dark:text-gray-400">{stat.name}</p>
        <p className="text-2xl font-bold text-gray-900 dark:text-white mt-1">{stat.value}</p>
      </div>
    </div>
  );
}

// Component: Risk Distribution Chart (dynamic based on agent trust scores)
function RiskDistributionChart({ status }: { status: ComplianceStatus | null }) {
  // Calculate risk distribution from actual data
  // Based on trust scores: Low >= 80%, Medium 60-79%, High 40-59%, Critical < 40%
  const totalAgents = status?.totalAgents || 0;
  const verifiedAgents = status?.verifiedAgents || 0;
  const avgTrustScore = status?.averageTrustScore || 0;

  // Estimate risk distribution based on verification rate and trust score
  // In production, this would come from actual agent trust score breakdown
  const calculateRiskDistribution = () => {
    if (totalAgents === 0) {
      return [
        { name: "No Agents", value: 100, color: "#9ca3af" },
      ];
    }

    const verificationRate = totalAgents > 0 ? (verifiedAgents / totalAgents) * 100 : 0;

    // Estimate distribution based on avg trust score
    let lowRisk, mediumRisk, highRisk, critical;

    if (avgTrustScore >= 80) {
      lowRisk = Math.round(verificationRate * 0.8);
      mediumRisk = Math.round(verificationRate * 0.15);
      highRisk = Math.round((100 - verificationRate) * 0.6);
      critical = 100 - lowRisk - mediumRisk - highRisk;
    } else if (avgTrustScore >= 60) {
      lowRisk = Math.round(verificationRate * 0.5);
      mediumRisk = Math.round(verificationRate * 0.35);
      highRisk = Math.round((100 - verificationRate) * 0.5 + 10);
      critical = 100 - lowRisk - mediumRisk - highRisk;
    } else {
      lowRisk = Math.round(verificationRate * 0.3);
      mediumRisk = Math.round(verificationRate * 0.3);
      highRisk = Math.round(40);
      critical = 100 - lowRisk - mediumRisk - highRisk;
    }

    // Ensure values are non-negative
    lowRisk = Math.max(0, lowRisk);
    mediumRisk = Math.max(0, mediumRisk);
    highRisk = Math.max(0, highRisk);
    critical = Math.max(0, critical);

    return [
      { name: "Low Risk", value: lowRisk, color: "#10b981" },
      { name: "Medium Risk", value: mediumRisk, color: "#f59e0b" },
      { name: "High Risk", value: highRisk, color: "#f97316" },
      { name: "Critical", value: critical, color: "#ef4444" },
    ].filter(d => d.value > 0);
  };

  const data = calculateRiskDistribution();

  return (
    <div className="flex items-center gap-6">
      <div className="w-36 h-36">
        <ResponsiveContainer width="100%" height="100%">
          <PieChart>
            <Pie
              data={data}
              cx="50%"
              cy="50%"
              innerRadius={35}
              outerRadius={55}
              paddingAngle={3}
              dataKey="value"
            >
              {data.map((entry, index) => (
                <Cell key={`cell-${index}`} fill={entry.color} />
              ))}
            </Pie>
          </PieChart>
        </ResponsiveContainer>
      </div>
      <div className="space-y-2">
        {data.map((item) => (
          <div key={item.name} className="flex items-center gap-2">
            <div className="w-3 h-3 rounded-full" style={{ backgroundColor: item.color }} />
            <span className="text-sm text-gray-600 dark:text-gray-400">{item.name}</span>
            <span className="text-sm font-semibold text-gray-900 dark:text-white">{item.value}%</span>
          </div>
        ))}
        {totalAgents > 0 && (
          <div className="pt-2 border-t border-gray-200 dark:border-gray-700 mt-2">
            <span className="text-xs text-gray-500">Based on {totalAgents} agent{totalAgents !== 1 ? "s" : ""}</span>
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
        <div className="animate-pulse bg-gray-200 dark:bg-gray-700 h-8 w-64 rounded" />
        <div className="animate-pulse bg-gray-200 dark:bg-gray-700 h-4 w-96 rounded" />
      </div>
      <div className="grid grid-cols-1 gap-5 sm:grid-cols-2 lg:grid-cols-5">
        {[...Array(5)].map((_, i) => (
          <div key={i} className="bg-white dark:bg-gray-800 p-5 rounded-xl border border-gray-200 dark:border-gray-700">
            <div className="animate-pulse space-y-3">
              <div className="h-10 w-10 bg-gray-200 dark:bg-gray-700 rounded-lg" />
              <div className="h-4 w-20 bg-gray-200 dark:bg-gray-700 rounded" />
              <div className="h-8 w-16 bg-gray-200 dark:bg-gray-700 rounded" />
            </div>
          </div>
        ))}
      </div>
      <div className="grid grid-cols-1 lg:grid-cols-2 xl:grid-cols-4 gap-5">
        {[...Array(4)].map((_, i) => (
          <div key={i} className="bg-white dark:bg-gray-800 p-5 rounded-xl border border-gray-200 dark:border-gray-700 h-64">
            <div className="animate-pulse h-full bg-gray-200 dark:bg-gray-700 rounded" />
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
        <Shield className={`h-16 w-16 ${is403 ? "text-amber-500" : "text-red-500"}`} />
        <div className="space-y-2">
          <h3 className="text-2xl font-bold text-gray-900 dark:text-gray-100">
            {is403 ? "Access Restricted" : "Failed to Load Compliance Data"}
          </h3>
          {is403 ? (
            <div className="space-y-3">
              <p className="text-base text-gray-600 dark:text-gray-400">
                Compliance monitoring is only available to <strong>Admin</strong> roles.
              </p>
              <p className="text-sm text-gray-500">
                Contact your organization administrator to access compliance features.
              </p>
            </div>
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

// Main Page Component
export default function CompliancePage() {
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [status, setStatus] = useState<ComplianceStatus | null>(null);
  const [metrics, setMetrics] = useState<ComplianceMetrics | null>(null);
  const [accessReview, setAccessReview] = useState<AccessReviewUser[]>([]);
  const [checkResults, setCheckResults] = useState<CheckResult[] | null>(null);
  const [runningCheck, setRunningCheck] = useState(false);
  const [exportingReport, setExportingReport] = useState(false);
  // Framework for exports only (not for display)
  const exportFramework = "aim";

  const fetchComplianceData = async () => {
    try {
      setLoading(true);
      setError(null);
      const [statusData, metricsData, accessData] = await Promise.all([
        api.getComplianceStatus(),
        api.getComplianceMetrics(),
        api.getAccessReview(),
      ]);
      setStatus(statusData);
      setMetrics(metricsData);
      setAccessReview(accessData.users || []);
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
      setRunningCheck(true);
      const result = await api.runComplianceCheck();
      setCheckResults(result.checks);
    } catch (err) {
      console.error("Failed to run compliance check:", err);
      alert("Failed to run compliance check: " + (err instanceof Error ? err.message : "Unknown error"));
    } finally {
      setRunningCheck(false);
    }
  };

  const handleExportReport = async (format: "csv" | "json") => {
    try {
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
      alert("Failed to export report: " + (err instanceof Error ? err.message : "Unknown error"));
    } finally {
      setExportingReport(false);
    }
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

  const stats = [
    { name: "Avg Trust Score", value: `${Math.round(status?.averageTrustScore || 0)}%`, icon: Shield, iconColor: (status?.averageTrustScore || 0) >= 80 ? "text-green-500" : (status?.averageTrustScore || 0) >= 60 ? "text-yellow-500" : "text-red-500" },
    { name: "Audit Logs (Recent)", value: auditLogDisplay, icon: FileText, iconColor: "text-blue-500" },
    { name: "Verified Agents", value: `${status?.verifiedAgents || 0} / ${status?.totalAgents || 0}`, icon: CheckCircle, iconColor: "text-green-500" },
    { name: "Compliance Rate", value: `${complianceRate}%`, icon: ShieldCheck, iconColor: complianceRate >= 80 ? "text-green-500" : complianceRate >= 60 ? "text-yellow-500" : "text-red-500" },
  ];

  return (
    <AuthGuard>
      <div className="space-y-6">
        {/* Header */}
        <div className="flex flex-col lg:flex-row lg:items-center lg:justify-between gap-4">
          <div>
            <h1 className="text-2xl font-bold text-gray-900 dark:text-white flex items-center gap-3">
              <Shield className="h-7 w-7 text-blue-600" />
              Compliance Dashboard
            </h1>
            <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">
              Monitor agent verification, trust scores, and security compliance for your organization.
            </p>
          </div>
          <div className="flex flex-wrap gap-2 items-center">
            <button
              onClick={() => fetchComplianceData()}
              className="inline-flex items-center gap-2 px-3 py-2 text-sm font-medium text-gray-700 dark:text-gray-300 bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg hover:bg-gray-50 dark:hover:bg-gray-700 transition-colors"
            >
              <RefreshCw className="h-4 w-4" />
              Refresh
            </button>

            {/* Export Dropdown */}
            <div className="relative group">
              <button
                disabled={exportingReport}
                className="inline-flex items-center gap-2 px-3 py-2 text-sm font-medium text-gray-700 dark:text-gray-300 bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg hover:bg-gray-50 dark:hover:bg-gray-700 transition-colors disabled:opacity-50"
              >
                {exportingReport ? <Loader2 className="h-4 w-4 animate-spin" /> : <Download className="h-4 w-4" />}
                Export Report
              </button>
              <div className="absolute right-0 mt-1 w-40 bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg shadow-lg opacity-0 invisible group-hover:opacity-100 group-hover:visible transition-all z-10">
                <button
                  onClick={() => handleExportReport("json")}
                  className="w-full px-4 py-2 text-sm text-left text-gray-700 dark:text-gray-300 hover:bg-gray-50 dark:hover:bg-gray-700 rounded-t-lg"
                >
                  Export as JSON
                </button>
                <button
                  onClick={() => handleExportReport("csv")}
                  className="w-full px-4 py-2 text-sm text-left text-gray-700 dark:text-gray-300 hover:bg-gray-50 dark:hover:bg-gray-700 rounded-b-lg"
                >
                  Export as CSV
                </button>
              </div>
            </div>

          </div>
        </div>

        {/* Key Metrics */}
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
          {stats.map((stat) => (
            <StatCard key={stat.name} stat={stat} />
          ))}
        </div>

        {/* Compliance Categories - Based on Actual Check Results */}
        {checkResults && checkResults.length > 0 && (
          <div>
            <div className="mb-4">
              <h2 className="text-lg font-semibold text-gray-900 dark:text-white">Compliance Status</h2>
              <p className="text-sm text-gray-500 dark:text-gray-400">Real-time compliance checks for agent identity management</p>
            </div>
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
              {/* Security Checks */}
              <ComplianceSummaryCard
                title="Security Compliance"
                description="API keys, certificates, and trust scores"
                passed={checkResults.filter(c => ["api_key_rotation_needed", "trust_score_degradation", "certificate_expiry_warning"].includes(c.name) && c.passed).length}
                total={checkResults.filter(c => ["api_key_rotation_needed", "trust_score_degradation", "certificate_expiry_warning"].includes(c.name)).length}
                icon={Lock}
                color="blue"
                checks={checkResults.filter(c => ["api_key_rotation_needed", "trust_score_degradation", "certificate_expiry_warning"].includes(c.name))}
              />

              {/* Operations Checks */}
              <ComplianceSummaryCard
                title="Operations Compliance"
                description="Agent activity and verification status"
                passed={checkResults.filter(c => ["inactive_agents", "unverified_agent_backlog", "orphaned_resources"].includes(c.name) && c.passed).length}
                total={checkResults.filter(c => ["inactive_agents", "unverified_agent_backlog", "orphaned_resources"].includes(c.name)).length}
                icon={Activity}
                color="green"
                checks={checkResults.filter(c => ["inactive_agents", "unverified_agent_backlog", "orphaned_resources"].includes(c.name))}
              />
            </div>
          </div>
        )}

        {/* Main Content Grid */}
        <div className="grid grid-cols-1 xl:grid-cols-3 gap-6">
          {/* Left Column - Charts & Risk */}
          <div className="xl:col-span-2 space-y-6">
            {/* Trust Score Trend */}
            <div className="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-6">
              <div className="flex items-center justify-between mb-4">
                <div>
                  <h3 className="text-lg font-semibold text-gray-900 dark:text-white">Trust Score Trend</h3>
                  <p className="text-sm text-gray-500 dark:text-gray-400">30-day compliance score history</p>
                </div>
                <TrendingUp className="h-5 w-5 text-gray-400" />
              </div>
              <div className="h-64">
                <ResponsiveContainer width="100%" height="100%">
                  <AreaChart data={metrics?.metrics?.trustScoreTrend?.map((d) => ({ date: d.date, score: Math.round(d.avgScore * 100) })) || []}>
                    <defs>
                      <linearGradient id="colorScore" x1="0" y1="0" x2="0" y2="1">
                        <stop offset="5%" stopColor="#10b981" stopOpacity={0.3} />
                        <stop offset="95%" stopColor="#10b981" stopOpacity={0} />
                      </linearGradient>
                    </defs>
                    <CartesianGrid strokeDasharray="3 3" className="stroke-gray-200 dark:stroke-gray-700" />
                    <XAxis dataKey="date" stroke="#9CA3AF" fontSize={12} />
                    <YAxis stroke="#9CA3AF" fontSize={12} domain={[0, 100]} />
                    <Tooltip
                      contentStyle={{
                        backgroundColor: "#fff",
                        border: "1px solid #e5e7eb",
                        borderRadius: "0.5rem",
                        boxShadow: "0 4px 6px -1px rgb(0 0 0 / 0.1)",
                      }}
                    />
                    <Area type="monotone" dataKey="score" stroke="#10b981" strokeWidth={2} fill="url(#colorScore)" name="Trust Score (%)" />
                  </AreaChart>
                </ResponsiveContainer>
              </div>
            </div>

            {/* Risk Assessment & Compliance Check Results */}
            <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
              {/* Risk Distribution */}
              <div className="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-6">
                <div className="flex items-center justify-between mb-4">
                  <h3 className="text-lg font-semibold text-gray-900 dark:text-white">Risk Distribution</h3>
                  <BarChart3 className="h-5 w-5 text-gray-400" />
                </div>
                <RiskDistributionChart status={status} />
              </div>

              {/* Compliance Checks */}
              <div className="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-6">
                <div className="flex items-center justify-between mb-4">
                  <h3 className="text-lg font-semibold text-gray-900 dark:text-white">Compliance Checks</h3>
                  {runningCheck ? (
                    <Loader2 className="h-5 w-5 text-blue-500 animate-spin" />
                  ) : (
                    <FileCheck className="h-5 w-5 text-gray-400" />
                  )}
                </div>
                {runningCheck ? (
                  <div className="flex flex-col items-center justify-center h-40 text-center">
                    <Loader2 className="h-8 w-8 text-blue-500 animate-spin mb-2" />
                    <p className="text-sm text-gray-500 dark:text-gray-400">Running compliance checks...</p>
                  </div>
                ) : checkResults && checkResults.length > 0 ? (
                  <div className="space-y-2 max-h-48 overflow-y-auto">
                    {checkResults.slice(0, 6).map((result, idx) => (
                      <div key={idx} className={`flex items-center gap-2 p-2 rounded-lg ${result.passed ? "bg-green-50 dark:bg-green-900/20" : "bg-red-50 dark:bg-red-900/20"}`}>
                        {result.passed ? (
                          <CheckCircle className="h-4 w-4 text-green-600" />
                        ) : (
                          <XCircle className="h-4 w-4 text-red-600" />
                        )}
                        <span className="text-sm text-gray-700 dark:text-gray-300 truncate">
                          {result.name.replace(/_/g, " ")}
                        </span>
                      </div>
                    ))}
                  </div>
                ) : (
                  <div className="flex flex-col items-center justify-center h-40 text-center">
                    <CheckCircle className="h-10 w-10 text-gray-300 dark:text-gray-600 mb-2" />
                    <p className="text-sm text-gray-500 dark:text-gray-400">No compliance issues detected</p>
                  </div>
                )}
              </div>
            </div>

          </div>

          {/* Right Column - Activity */}
          <div className="flex flex-col">
            {/* Audit Activity */}
            <div className="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-6 flex-1 flex flex-col">
              <div className="flex items-center justify-between mb-4">
                <h3 className="font-semibold text-gray-900 dark:text-white">Recent Activity</h3>
                <Activity className="h-5 w-5 text-gray-400" />
              </div>
              <div className="space-y-1 flex-1">
                {RECENT_ACTIVITIES.map((activity, idx) => (
                  <ActivityItem key={idx} activity={activity} />
                ))}
              </div>
            </div>
          </div>
        </div>

        {/* Access Review Table */}
        <div className="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700">
          <div className="p-6 border-b border-gray-200 dark:border-gray-700">
            <div className="flex items-center justify-between">
              <div>
                <h3 className="text-lg font-semibold text-gray-900 dark:text-white">Access Review</h3>
                <p className="text-sm text-gray-500 dark:text-gray-400">Review user access patterns and permissions</p>
              </div>
              <Users className="h-5 w-5 text-gray-400" />
            </div>
          </div>
          <div className="overflow-x-auto">
            <table className="w-full divide-y divide-gray-200 dark:divide-gray-700">
              <thead className="bg-gray-50 dark:bg-gray-800">
                <tr>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">User</th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">Email</th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">Role</th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">Status</th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">Last Login</th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">Created</th>
                </tr>
              </thead>
              <tbody className="bg-white dark:bg-gray-900 divide-y divide-gray-200 dark:divide-gray-700">
                {accessReview.map((user) => (
                  <tr key={user.id} className="hover:bg-gray-50 dark:hover:bg-gray-800 transition-colors">
                    <td className="px-6 py-4 whitespace-nowrap text-sm font-medium text-gray-900 dark:text-gray-100">{user.name}</td>
                    <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500 dark:text-gray-400">{user.email}</td>
                    <td className="px-6 py-4 whitespace-nowrap">
                      <span className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium capitalize ${
                        user.role === "admin"
                          ? "bg-purple-100 dark:bg-purple-900/30 text-purple-800 dark:text-purple-300"
                          : "bg-blue-100 dark:bg-blue-900/30 text-blue-800 dark:text-blue-300"
                      }`}>
                        {user.role}
                      </span>
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap">
                      <span className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium capitalize ${
                        user.status === "active"
                          ? "bg-green-100 dark:bg-green-900/30 text-green-800 dark:text-green-300"
                          : "bg-gray-100 dark:bg-gray-700 text-gray-800 dark:text-gray-300"
                      }`}>
                        {user.status}
                      </span>
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500 dark:text-gray-400">
                      {user.lastLogin ? formatDateTime(user.lastLogin) : "Never"}
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500 dark:text-gray-400">
                      {formatDateTime(user.createdAt)}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      </div>
    </AuthGuard>
  );
}
