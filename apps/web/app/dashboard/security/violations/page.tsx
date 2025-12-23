"use client";

import { useState, useEffect } from "react";
import {
  Shield,
  XCircle,
  CheckCircle,
  Clock,
  ChevronLeft,
  ChevronRight,
  Filter,
  Search,
  AlertTriangle,
  Bot,
  FileWarning,
  Globe,
  Database,
  Mail,
  Folder,
  Lock,
  CreditCard,
  Bell,
  Users,
  Server,
} from "lucide-react";
import Link from "next/link";
import { api } from "@/lib/api";
import { formatDateTime, formatRelativeTime } from "@/lib/date-utils";
import { getErrorMessage } from "@/lib/error-messages";
import { AuthGuard } from "@/components/auth-guard";

interface Violation {
  id: string;
  agentId: string;
  attemptedCapability: string;
  registeredCapabilities: string[];
  severity: string;
  trustScoreImpact: number;
  isBlocked: boolean;
  sourceIp: string;
  requestMetadata: Record<string, any>;
  createdAt: string;
  agentName?: string;
}

function getCategoryIcon(capability: string) {
  const cap = capability.toLowerCase();
  if (cap.startsWith("admin:")) return Server;
  if (cap.startsWith("file:")) return Folder;
  if (cap.startsWith("db:") || cap.startsWith("database:")) return Database;
  if (cap.startsWith("api:") || cap.startsWith("http:")) return Globe;
  if (cap.startsWith("secret:") || cap.startsWith("credential:")) return Lock;
  if (cap.startsWith("payment:") || cap.startsWith("financial:")) return CreditCard;
  if (cap.startsWith("notification:") || cap.startsWith("email:")) return Bell;
  if (cap.startsWith("user:")) return Users;
  if (cap.startsWith("network:")) return Globe;
  return FileWarning;
}

function getSeverityStyles(severity: string) {
  switch (severity.toLowerCase()) {
    case "critical":
      return {
        bg: "bg-red-100 dark:bg-red-900/30",
        text: "text-red-700 dark:text-red-300",
        border: "border-red-200 dark:border-red-800",
      };
    case "high":
      return {
        bg: "bg-orange-100 dark:bg-orange-900/30",
        text: "text-orange-700 dark:text-orange-300",
        border: "border-orange-200 dark:border-orange-800",
      };
    case "medium":
      return {
        bg: "bg-yellow-100 dark:bg-yellow-900/30",
        text: "text-yellow-700 dark:text-yellow-300",
        border: "border-yellow-200 dark:border-yellow-800",
      };
    default:
      return {
        bg: "bg-blue-100 dark:bg-blue-900/30",
        text: "text-blue-700 dark:text-blue-300",
        border: "border-blue-200 dark:border-blue-800",
      };
  }
}

function ViolationCard({ violation }: { violation: Violation }) {
  const CategoryIcon = getCategoryIcon(violation.attemptedCapability);
  const severityStyles = getSeverityStyles(violation.severity);

  return (
    <div className="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-4 hover:shadow-md transition-all">
      <div className="flex items-start gap-4">
        {/* Status Icon */}
        <div className={`p-2 rounded-lg ${violation.isBlocked ? "bg-red-100 dark:bg-red-900/30" : "bg-yellow-100 dark:bg-yellow-900/30"}`}>
          {violation.isBlocked ? (
            <XCircle className="h-5 w-5 text-red-600 dark:text-red-400" />
          ) : (
            <AlertTriangle className="h-5 w-5 text-yellow-600 dark:text-yellow-400" />
          )}
        </div>

        {/* Content */}
        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-2 flex-wrap mb-2">
            <span className={`text-xs font-semibold px-2 py-0.5 rounded-full ${
              violation.isBlocked
                ? "bg-red-100 dark:bg-red-900/50 text-red-700 dark:text-red-300"
                : "bg-yellow-100 dark:bg-yellow-900/50 text-yellow-700 dark:text-yellow-300"
            }`}>
              {violation.isBlocked ? "BLOCKED" : "ALLOWED"}
            </span>
            <span className={`text-xs font-medium px-2 py-0.5 rounded-full ${severityStyles.bg} ${severityStyles.text}`}>
              {violation.severity.toUpperCase()}
            </span>
            <Link
              href={`/dashboard/agents/${violation.agentId}`}
              className="text-sm font-medium text-gray-900 dark:text-white hover:text-blue-600 dark:hover:text-blue-400 truncate"
            >
              {violation.agentName || violation.agentId.slice(0, 8)}
            </Link>
          </div>

          <div className="flex items-center gap-2 mb-2">
            <CategoryIcon className="h-4 w-4 text-gray-400" />
            <span className="text-sm font-mono text-gray-700 dark:text-gray-300">
              {violation.attemptedCapability}
            </span>
          </div>

          <div className="flex items-center justify-between mt-3">
            <div className="flex items-center gap-4 text-xs text-gray-500 dark:text-gray-400">
              <span className="flex items-center gap-1">
                <Clock className="h-3 w-3" />
                {formatRelativeTime(violation.createdAt)}
              </span>
              {violation.sourceIp && (
                <span className="flex items-center gap-1">
                  <Globe className="h-3 w-3" />
                  {violation.sourceIp}
                </span>
              )}
              {violation.trustScoreImpact !== 0 && (
                <span className={violation.trustScoreImpact < 0 ? "text-red-600 dark:text-red-400" : "text-green-600 dark:text-green-400"}>
                  Trust {violation.trustScoreImpact > 0 ? "+" : ""}{violation.trustScoreImpact}%
                </span>
              )}
            </div>
            <Link
              href={`/dashboard/agents/${violation.agentId}`}
              className="text-xs text-blue-600 dark:text-blue-400 hover:underline flex items-center gap-1"
            >
              View Agent <ChevronRight className="h-3 w-3" />
            </Link>
          </div>
        </div>
      </div>
    </div>
  );
}

function ViolationsPageSkeleton() {
  return (
    <div className="space-y-4">
      {[...Array(5)].map((_, i) => (
        <div key={i} className="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-4">
          <div className="flex items-start gap-4">
            <div className="animate-pulse bg-gray-200 dark:bg-gray-700 h-10 w-10 rounded-lg"></div>
            <div className="flex-1 space-y-3">
              <div className="animate-pulse bg-gray-200 dark:bg-gray-700 h-4 w-48 rounded"></div>
              <div className="animate-pulse bg-gray-200 dark:bg-gray-700 h-4 w-64 rounded"></div>
              <div className="animate-pulse bg-gray-200 dark:bg-gray-700 h-3 w-32 rounded"></div>
            </div>
          </div>
        </div>
      ))}
    </div>
  );
}

export default function ViolationsPage() {
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [violations, setViolations] = useState<Violation[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [filterBlocked, setFilterBlocked] = useState<"all" | "blocked" | "allowed">("all");
  const [searchQuery, setSearchQuery] = useState("");
  const pageSize = 20;

  const fetchViolations = async () => {
    try {
      setLoading(true);
      setError(null);
      const offset = (page - 1) * pageSize;
      const data = await api.getSecurityViolations(pageSize, offset);
      setViolations(data.violations || []);
      setTotal(data.total || 0);
    } catch (err) {
      console.error("Failed to fetch violations:", err);
      setError(getErrorMessage(err, { resource: "violations", action: "load" }));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchViolations();
  }, [page]);

  // Filter violations locally
  const filteredViolations = violations.filter((v) => {
    if (filterBlocked === "blocked" && !v.isBlocked) return false;
    if (filterBlocked === "allowed" && v.isBlocked) return false;
    if (searchQuery) {
      const query = searchQuery.toLowerCase();
      return (
        v.attemptedCapability.toLowerCase().includes(query) ||
        v.agentName?.toLowerCase().includes(query) ||
        v.agentId.toLowerCase().includes(query)
      );
    }
    return true;
  });

  const totalPages = Math.ceil(total / pageSize);
  const blockedCount = violations.filter((v) => v.isBlocked).length;
  const allowedCount = violations.filter((v) => !v.isBlocked).length;

  return (
    <AuthGuard>
      <div className="space-y-6">
        {/* Header */}
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-4">
            <Link
              href="/dashboard/security"
              className="p-2 rounded-lg hover:bg-gray-100 dark:hover:bg-gray-800 transition-colors"
            >
              <ChevronLeft className="h-5 w-5 text-gray-500" />
            </Link>
            <div>
              <h1 className="text-2xl font-bold text-gray-900 dark:text-white flex items-center gap-2">
                <Shield className="h-6 w-6 text-green-600" />
                Capability Violations
              </h1>
              <p className="text-sm text-gray-500 dark:text-gray-400 mt-1">
                All blocked and flagged agent actions
              </p>
            </div>
          </div>
          <div className="text-right">
            <p className="text-2xl font-bold text-gray-900 dark:text-white">{total}</p>
            <p className="text-sm text-gray-500 dark:text-gray-400">total violations</p>
          </div>
        </div>

        {/* Filters */}
        <div className="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-4">
          <div className="flex flex-col sm:flex-row gap-4">
            {/* Search */}
            <div className="flex-1 relative">
              <Search className="absolute left-3 top-1/2 transform -translate-y-1/2 h-4 w-4 text-gray-400" />
              <input
                type="text"
                placeholder="Search by capability or agent..."
                value={searchQuery}
                onChange={(e) => setSearchQuery(e.target.value)}
                className="w-full pl-10 pr-4 py-2 border border-gray-200 dark:border-gray-700 rounded-lg bg-white dark:bg-gray-900 text-gray-900 dark:text-white placeholder-gray-500 focus:outline-none focus:ring-2 focus:ring-blue-500"
              />
            </div>

            {/* Status Filter */}
            <div className="flex items-center gap-2">
              <Filter className="h-4 w-4 text-gray-400" />
              <select
                value={filterBlocked}
                onChange={(e) => setFilterBlocked(e.target.value as any)}
                className="px-3 py-2 border border-gray-200 dark:border-gray-700 rounded-lg bg-white dark:bg-gray-900 text-gray-900 dark:text-white focus:outline-none focus:ring-2 focus:ring-blue-500"
              >
                <option value="all">All ({violations.length})</option>
                <option value="blocked">Blocked ({blockedCount})</option>
                <option value="allowed">Allowed ({allowedCount})</option>
              </select>
            </div>
          </div>
        </div>

        {/* Content */}
        {loading ? (
          <ViolationsPageSkeleton />
        ) : error ? (
          <div className="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-8 text-center">
            <AlertTriangle className="h-12 w-12 mx-auto text-red-500 mb-4" />
            <p className="text-gray-600 dark:text-gray-400">{error}</p>
            <button
              onClick={fetchViolations}
              className="mt-4 px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 transition-colors"
            >
              Retry
            </button>
          </div>
        ) : filteredViolations.length === 0 ? (
          <div className="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-12 text-center">
            <div className="p-4 rounded-full bg-green-100 dark:bg-green-900/30 inline-block mb-4">
              <CheckCircle className="h-8 w-8 text-green-600 dark:text-green-400" />
            </div>
            <h3 className="text-lg font-medium text-gray-900 dark:text-white">No Violations Found</h3>
            <p className="text-sm text-gray-500 dark:text-gray-400 mt-1">
              {searchQuery || filterBlocked !== "all"
                ? "No violations match your filters"
                : "All agent actions are within authorized capabilities"}
            </p>
          </div>
        ) : (
          <div className="space-y-3">
            {filteredViolations.map((violation) => (
              <ViolationCard key={violation.id} violation={violation} />
            ))}
          </div>
        )}

        {/* Pagination */}
        {totalPages > 1 && (
          <div className="flex items-center justify-between bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 px-4 py-3">
            <p className="text-sm text-gray-500 dark:text-gray-400">
              Showing {(page - 1) * pageSize + 1} to {Math.min(page * pageSize, total)} of {total} violations
            </p>
            <div className="flex items-center gap-2">
              <button
                onClick={() => setPage((p) => Math.max(1, p - 1))}
                disabled={page === 1}
                className="p-2 rounded-lg hover:bg-gray-100 dark:hover:bg-gray-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
              >
                <ChevronLeft className="h-5 w-5" />
              </button>
              <span className="text-sm text-gray-700 dark:text-gray-300">
                Page {page} of {totalPages}
              </span>
              <button
                onClick={() => setPage((p) => Math.min(totalPages, p + 1))}
                disabled={page === totalPages}
                className="p-2 rounded-lg hover:bg-gray-100 dark:hover:bg-gray-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
              >
                <ChevronRight className="h-5 w-5" />
              </button>
            </div>
          </div>
        )}
      </div>
    </AuthGuard>
  );
}
