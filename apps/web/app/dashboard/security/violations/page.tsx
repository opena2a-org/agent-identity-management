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
        bg: "bg-danger-fill",
        text: "text-danger-text",
        border: "border-danger-border",
      };
    case "high":
      return {
        bg: "bg-danger-fill",
        text: "text-danger-text",
        border: "border-danger-border",
      };
    case "medium":
      return {
        bg: "bg-warning-fill",
        text: "text-warning-text",
        border: "border-warning-border",
      };
    default:
      return {
        bg: "bg-brand-soft",
        text: "text-brand-text",
        border: "border-stroke",
      };
  }
}

function ViolationCard({ violation }: { violation: Violation }) {
  const CategoryIcon = getCategoryIcon(violation.attemptedCapability);
  const severityStyles = getSeverityStyles(violation.severity);

  return (
    <div className="glass p-4 hover:-translate-y-0.5 transition-transform">
      <div className="flex items-start gap-4">
        {/* Status Icon */}
        <div className={`p-2 rounded-inset-sm ${violation.isBlocked ? "bg-danger-fill" : "bg-warning-fill"}`}>
          {violation.isBlocked ? (
            <XCircle className="h-5 w-5 text-danger-text" />
          ) : (
            <AlertTriangle className="h-5 w-5 text-warning-text" />
          )}
        </div>

        {/* Content */}
        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-2 flex-wrap mb-2">
            <span className={`text-xs font-semibold px-2 py-0.5 rounded-pill border ${
              violation.isBlocked
                ? "bg-danger-fill border-danger-border text-danger-text"
                : "bg-warning-fill border-warning-border text-warning-text"
            }`}>
              {violation.isBlocked ? "Blocked" : "Allowed"}
            </span>
            <span className={`text-xs font-medium px-2 py-0.5 rounded-pill border ${severityStyles.bg} ${severityStyles.text} ${severityStyles.border}`}>
              {violation.severity.charAt(0).toUpperCase() + violation.severity.slice(1).toLowerCase()}
            </span>
            <Link
              href={`/dashboard/agents/${violation.agentId}`}
              className="text-sm font-medium text-ink hover:text-brand-text truncate"
            >
              {violation.agentName || violation.agentId.slice(0, 8)}
            </Link>
          </div>

          <div className="flex items-center gap-2 mb-2">
            <CategoryIcon className="h-4 w-4 text-ink-tertiary" />
            <span className="text-sm font-mono text-ink-body">
              {violation.attemptedCapability}
            </span>
          </div>

          <div className="flex items-center justify-between mt-3">
            <div className="flex items-center gap-4 text-xs text-ink-secondary">
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
                <span className={violation.trustScoreImpact < 0 ? "text-danger-text" : "text-success-text"}>
                  Trust {violation.trustScoreImpact > 0 ? "+" : ""}{violation.trustScoreImpact}%
                </span>
              )}
            </div>
            <Link
              href={`/dashboard/agents/${violation.agentId}`}
              className="text-xs text-brand-text hover:underline flex items-center gap-1"
            >
              View agent <ChevronRight className="h-3 w-3" />
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
        <div key={i} className="glass p-4">
          <div className="flex items-start gap-4">
            <div className="animate-pulse bg-track h-10 w-10 rounded-inset-sm"></div>
            <div className="flex-1 space-y-3">
              <div className="animate-pulse bg-track h-4 w-48 rounded"></div>
              <div className="animate-pulse bg-track h-4 w-64 rounded"></div>
              <div className="animate-pulse bg-track h-3 w-32 rounded"></div>
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
              className="p-2 rounded-nav hover:bg-nav-active transition-colors"
            >
              <ChevronLeft className="h-5 w-5 text-ink-secondary" />
            </Link>
            <div>
              <h1 className="text-2xl font-bold tracking-[-0.02em] text-ink flex items-center gap-2">
                <Shield className="h-6 w-6 text-success-text" />
                Capability violations
              </h1>
              <p className="text-sm text-ink-secondary mt-1">
                All blocked and flagged agent actions
              </p>
            </div>
          </div>
          <div className="text-right">
            <p className="text-2xl font-bold tracking-[-0.02em] text-ink">{total}</p>
            <p className="text-sm text-ink-secondary">total violations</p>
          </div>
        </div>

        {/* Filters */}
        <div className="glass p-4">
          <div className="flex flex-col sm:flex-row gap-4">
            {/* Search */}
            <div className="flex-1 relative">
              <Search className="absolute left-3 top-1/2 transform -translate-y-1/2 h-4 w-4 text-ink-tertiary" />
              <input
                type="text"
                placeholder="Search by capability or agent..."
                value={searchQuery}
                onChange={(e) => setSearchQuery(e.target.value)}
                className="w-full pl-10 pr-4 py-2 border border-stroke rounded-inset bg-glass-inset text-sm text-ink placeholder:text-ink-tertiary focus:outline-none focus:ring-2 focus:ring-ring focus:ring-offset-0"
              />
            </div>

            {/* Status Filter */}
            <div className="flex items-center gap-2">
              <Filter className="h-4 w-4 text-ink-tertiary" />
              <select
                value={filterBlocked}
                onChange={(e) => setFilterBlocked(e.target.value as any)}
                className="px-3 py-2 border border-stroke rounded-inset bg-glass-inset text-sm text-ink focus:outline-none focus:ring-2 focus:ring-ring focus:ring-offset-0"
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
          <div className="glass-alert p-8 text-center">
            <AlertTriangle className="h-12 w-12 mx-auto text-danger-text mb-4" />
            <p className="text-ink-body">{error}</p>
            <button
              onClick={fetchViolations}
              className="mt-4 px-4 py-2 rounded-pill bg-brand text-white shadow-glow hover:bg-brand-hover transition-colors"
            >
              Retry
            </button>
          </div>
        ) : filteredViolations.length === 0 ? (
          <div className="glass p-12 text-center">
            <div className="p-4 rounded-pill bg-success-fill inline-block mb-4">
              <CheckCircle className="h-8 w-8 text-success-text" />
            </div>
            <h3 className="text-lg font-medium text-ink">No violations found</h3>
            <p className="text-sm text-ink-secondary mt-1">
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
          <div className="glass flex items-center justify-between px-4 py-3">
            <p className="text-sm text-ink-secondary">
              Showing {(page - 1) * pageSize + 1} to {Math.min(page * pageSize, total)} of {total} violations
            </p>
            <div className="flex items-center gap-2">
              <button
                onClick={() => setPage((p) => Math.max(1, p - 1))}
                disabled={page === 1}
                className="p-2 rounded-nav text-ink-secondary hover:bg-nav-active disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
              >
                <ChevronLeft className="h-5 w-5" />
              </button>
              <span className="text-sm text-ink-body">
                Page {page} of {totalPages}
              </span>
              <button
                onClick={() => setPage((p) => Math.min(totalPages, p + 1))}
                disabled={page === totalPages}
                className="p-2 rounded-nav text-ink-secondary hover:bg-nav-active disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
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
