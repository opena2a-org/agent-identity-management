"use client";

import { useState, useEffect } from "react";
import {
  Shield,
  CheckCircle,
  AlertTriangle,
  TrendingUp,
  Download,
  Play,
  Users,
  FileText,
  Loader2,
  XCircle,
  Database,
  AlertCircle,
  Filter,
} from "lucide-react";
import {
  LineChart,
  Line,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
} from "recharts";
import { api } from "@/lib/api";
import { formatDateTime } from "@/lib/date-utils";
import { AuthGuard } from "@/components/auth-guard";
import { Button } from "@/components/ui/button";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Textarea } from "@/components/ui/textarea";
import { Label } from "@/components/ui/label";

// Backend response structure
interface ComplianceStatus {
  complianceLevel: string;
  totalAgents: number;
  verifiedAgents: number;
  verificationRate: number; // Already in percentage (0-100)
  averageTrustScore: number; // Already in percentage (0-100)
  recentAuditCount: number;
}

interface ComplianceMetrics {
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
      avgScore: number;
    }>;
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


function StatCard({ stat }: { stat: any }) {
  return (
    <div className="bg-white dark:bg-gray-800 p-6 rounded-lg border border-gray-200 dark:border-gray-700 shadow-sm">
      <div className="flex items-center">
        <div className="flex-shrink-0">
          <stat.icon
            className={`h-6 w-6 ${stat.iconColor || "text-gray-400"}`}
          />
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

function StatusBadge({ status }: { status: string }) {
  const getStatusStyles = (status: string) => {
    switch (status.toLowerCase()) {
      case "compliant":
      case "active":
        return "bg-green-100 dark:bg-green-900/30 text-green-800 dark:text-green-300";
      case "warning":
        return "bg-yellow-100 dark:bg-yellow-900/30 text-yellow-800 dark:text-yellow-300";
      case "non_compliant":
      case "inactive":
        return "bg-red-100 dark:bg-red-900/30 text-red-800 dark:text-red-300";
      default:
        return "bg-gray-100 dark:bg-gray-700 text-gray-800 dark:text-gray-300";
    }
  };

  return (
    <span
      className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium capitalize ${getStatusStyles(status)}`}
    >
      {status.replace("_", " ")}
    </span>
  );
}

function CompliancePageSkeleton() {
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

      {/* Compliance Records Table Skeleton */}
      <div className="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 shadow-sm">
        <div className="p-6 border-b border-gray-200 dark:border-gray-700">
          <div className="flex items-center justify-between">
            <div className="animate-pulse bg-gray-200 dark:bg-gray-700 h-6 w-48 rounded"></div>
            <div className="animate-pulse bg-gray-200 dark:bg-gray-700 h-10 w-32 rounded-lg"></div>
          </div>
        </div>
        <div className="overflow-x-auto">
          <table className="min-w-full divide-y divide-gray-200 dark:divide-gray-700">
            <thead className="bg-gray-50 dark:bg-gray-800">
              <tr>
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
                  <div className="animate-pulse bg-gray-200 dark:bg-gray-700 h-4 w-24 rounded"></div>
                </th>
                <th className="px-6 py-3">
                  <div className="animate-pulse bg-gray-200 dark:bg-gray-700 h-4 w-16 rounded"></div>
                </th>
              </tr>
            </thead>
            <tbody className="bg-white dark:bg-gray-900 divide-y divide-gray-200 dark:divide-gray-700">
              {[...Array(6)].map((_, rowIndex) => (
                <tr key={rowIndex}>
                  <td className="px-6 py-4">
                    <div className="flex items-center">
                      <div className="animate-pulse bg-gray-200 dark:bg-gray-700 h-10 w-10 rounded-lg"></div>
                      <div className="ml-4 space-y-1">
                        <div className="animate-pulse bg-gray-200 dark:bg-gray-700 h-4 w-32 rounded"></div>
                        <div className="animate-pulse bg-gray-200 dark:bg-gray-700 h-3 w-20 rounded"></div>
                      </div>
                    </div>
                  </td>
                  <td className="px-6 py-4">
                    <div className="animate-pulse bg-gray-200 dark:bg-gray-700 h-6 w-24 rounded-full"></div>
                  </td>
                  <td className="px-6 py-4">
                    <div className="animate-pulse bg-gray-200 dark:bg-gray-700 h-6 w-20 rounded-full"></div>
                  </td>
                  <td className="px-6 py-4">
                    <div className="animate-pulse bg-gray-200 dark:bg-gray-700 h-4 w-20 rounded"></div>
                  </td>
                  <td className="px-6 py-4">
                    <div className="flex items-center gap-2">
                      <div className="animate-pulse bg-gray-200 dark:bg-gray-700 h-6 w-6 rounded"></div>
                      <div className="animate-pulse bg-gray-200 dark:bg-gray-700 h-6 w-6 rounded"></div>
                    </div>
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
            {is403 ? "Access Restricted" : "Failed to Load Compliance Data"}
          </h3>
          {is403 ? (
            <div className="space-y-3">
              <p className="text-base text-gray-600 dark:text-gray-400">
                Compliance monitoring is only available to{" "}
                <strong>Admin</strong> roles.
              </p>
              <p className="text-sm text-gray-500 dark:text-gray-500">
                To view compliance status and audit logs, please contact your
                organization administrator.
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

export default function CompliancePage() {
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [status, setStatus] = useState<ComplianceStatus | null>(null);
  const [metrics, setMetrics] = useState<ComplianceMetrics | null>(null);
  const [accessReview, setAccessReview] = useState<AccessReviewUser[]>([]);
  const [checkResults, setCheckResults] = useState<CheckResult[] | null>(null);
  const [runningCheck, setRunningCheck] = useState(false);
  const [exporting, setExporting] = useState(false);

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
      setError(
        err instanceof Error ? err.message : "An unknown error occurred"
      );
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchComplianceData();
  }, []);

  const handleRunComplianceCheck = async () => {
    try {
      setRunningCheck(true);
      const result = await api.runComplianceCheck();
      setCheckResults(result.checks);
    } catch (err) {
      console.error("Failed to run compliance check:", err);
      alert(
        "Failed to run compliance check: " +
          (err instanceof Error ? err.message : "Unknown error")
      );
    } finally {
      setRunningCheck(false);
    }
  };

  if (loading) {
    return <CompliancePageSkeleton />;
  }

  if (error && !status) {
    return <ErrorDisplay message={error} onRetry={fetchComplianceData} />;
  }

  // Compliance score (backend returns 0-100 scale already)
  const complianceScore = status?.averageTrustScore
    ? Math.round(status.averageTrustScore)
    : 0;

  const stats = [
    {
      name: "System Health Score",
      value: `${complianceScore}%`,
      iconColor: complianceScore >= 80 ? "text-green-500" : "text-yellow-500",
      icon: Shield,
    },
    {
      name: "Audit Logs",
      value: status?.recentAuditCount?.toLocaleString() || "0",
      icon: FileText,
    },
    {
      name: "Verified Agents",
      value: `${status?.verifiedAgents || 0} / ${status?.totalAgents || 0}`,
      icon: CheckCircle,
    },
    {
      name: "Verification Rate",
      value: status?.verificationRate
        ? `${Math.round(status.verificationRate)}%`
        : "0%",
      icon: TrendingUp,
    },
  ];

  return (
    <AuthGuard>
      <div className="space-y-6">
        {/* Header */}
        <div className="space-y-4">
          <div className="flex items-center justify-between">
            <h1 className="text-2xl font-bold text-gray-900 dark:text-white">
              Compliance Dashboard
            </h1>
          <div className="flex gap-2">
            <button
              onClick={handleRunComplianceCheck}
              disabled={runningCheck}
              className="inline-flex items-center gap-2 px-4 py-2 bg-green-600 text-white rounded-lg hover:bg-green-700 disabled:bg-green-400 transition-colors text-sm"
            >
              {runningCheck ? (
                <>
                  <Loader2 className="h-4 w-4 animate-spin" />
                  Running...
                </>
              ) : (
                <>
                  <Play className="h-4 w-4" />
                  Run Check
                </>
              )}
            </button>
          </div>
        </div>
      </div>

      {/* Stats */}
      <div className="grid grid-cols-1 gap-5 sm:grid-cols-2 lg:grid-cols-4">
        {stats.map((stat) => (
          <StatCard key={stat.name} stat={stat} />
        ))}
      </div>

      {/* Compliance Trend */}
      <div className="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 shadow-sm p-6">
        <div className="flex items-center justify-between mb-4">
          <h3 className="text-lg font-medium text-gray-900 dark:text-gray-100">
            Trust Score Trend (30 Days)
          </h3>
          <TrendingUp className="h-5 w-5 text-gray-400" />
        </div>
        <div className="h-64">
          <ResponsiveContainer width="100%" height="100%">
            <LineChart
              data={
                metrics?.metrics?.trustScoreTrend?.map((d) => ({
                  date: d.date,
                  score: Math.round(d.avgScore * 100), // Convert 0-1 to 0-100
                })) || []
              }
            >
              <CartesianGrid
                strokeDasharray="3 3"
                className="stroke-gray-200 dark:stroke-gray-700"
              />
              <XAxis
                dataKey="date"
                className="text-xs text-gray-500 dark:text-gray-400"
                stroke="#9CA3AF"
              />
              <YAxis
                className="text-xs text-gray-500 dark:text-gray-400"
                stroke="#9CA3AF"
                domain={[0, 100]}
              />
              <Tooltip
                contentStyle={{
                  backgroundColor: "#fff",
                  border: "1px solid #e5e7eb",
                  borderRadius: "0.5rem",
                  boxShadow: "0 1px 3px 0 rgb(0 0 0 / 0.1)",
                }}
              />
              <Line
                type="monotone"
                dataKey="score"
                stroke="#10b981"
                strokeWidth={2}
                name="Trust Score (%)"
              />
            </LineChart>
          </ResponsiveContainer>
        </div>
      </div>

      {/* Check Results */}
      {checkResults && (
        <div className="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 shadow-sm">
          <div className="p-6 border-b border-gray-200 dark:border-gray-700">
            <h3 className="text-lg font-medium text-gray-900 dark:text-gray-100">
              Compliance Check Results
            </h3>
          </div>
          <div className="p-6 space-y-3">
            {checkResults.map((result, idx) => (
              <div
                key={idx}
                className={`flex items-start gap-3 p-4 rounded-lg ${
                  result.passed
                    ? "bg-green-50 dark:bg-green-900/20"
                    : "bg-red-50 dark:bg-red-900/20"
                }`}
              >
                {result.passed ? (
                  <CheckCircle className="h-5 w-5 text-green-600 dark:text-green-400 flex-shrink-0 mt-0.5" />
                ) : (
                  <XCircle className="h-5 w-5 text-red-600 dark:text-red-400 flex-shrink-0 mt-0.5" />
                )}
                <div className="flex-1">
                  <p
                    className={`text-sm font-medium ${
                      result.passed
                        ? "text-green-900 dark:text-green-100"
                        : "text-red-900 dark:text-red-100"
                    }`}
                  >
                    {result.name
                      .replace(/_/g, " ")
                      .replace(/\b\w/g, (l) => l.toUpperCase())}
                  </p>
                  <p
                    className={`text-sm mt-1 ${
                      result.passed
                        ? "text-green-700 dark:text-green-300"
                        : "text-red-700 dark:text-red-300"
                    }`}
                  >
                    {result.details ||
                      (result.passed
                        ? "Check passed successfully"
                        : "Check failed - requires attention")}
                  </p>
                  {result.actionUrl && !result.passed && (
                    <a
                      href={result.actionUrl}
                      className="inline-flex items-center gap-1 mt-2 text-sm font-medium text-blue-600 dark:text-blue-400 hover:text-blue-700 dark:hover:text-blue-300"
                    >
                      View affected items →
                    </a>
                  )}
                </div>
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Access Review */}
      <div className="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 shadow-sm">
        <div className="p-6 border-b border-gray-200 dark:border-gray-700">
          <div className="flex items-center justify-between">
            <h3 className="text-lg font-medium text-gray-900 dark:text-gray-100">
              Access Review
            </h3>
            <Users className="h-5 w-5 text-gray-400" />
          </div>
        </div>
        <div className="overflow-x-auto">
          <table className="w-full divide-y divide-gray-200 dark:divide-gray-700">
            <thead className="bg-gray-50 dark:bg-gray-800">
              <tr>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                  User
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                  Email
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                  Role
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                  Status
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                  Last Login
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                  Created
                </th>
              </tr>
            </thead>
            <tbody className="bg-white dark:bg-gray-900 divide-y divide-gray-200 dark:divide-gray-700">
              {accessReview.map((user) => (
                <tr
                  key={user.id}
                  className="hover:bg-gray-50 dark:hover:bg-gray-800 transition-colors"
                >
                  <td className="px-6 py-4 whitespace-nowrap text-sm font-medium text-gray-900 dark:text-gray-100">
                    {user.name}
                  </td>
                  <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500 dark:text-gray-400">
                    {user.email}
                  </td>
                  <td className="px-6 py-4 whitespace-nowrap">
                    <span className="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium capitalize bg-blue-100 dark:bg-blue-900/30 text-blue-800 dark:text-blue-300">
                      {user.role}
                    </span>
                  </td>
                  <td className="px-6 py-4 whitespace-nowrap">
                    <StatusBadge status={user.status} />
                  </td>
                  <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500 dark:text-gray-400">
                    {user.lastLogin
                      ? formatDateTime(user.lastLogin)
                      : "Never"}
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

      {/* Data Retention Policies section removed */}
      {/* Compliance Violations section removed */}
      {/* Remediation Dialog removed */}

    </div>
    </AuthGuard>
  );
}
