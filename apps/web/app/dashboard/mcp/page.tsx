"use client";

import { useState, useEffect, useMemo, useCallback } from "react";
import { useRouter } from "next/navigation";
import {
  Server,
  CheckCircle2,
  XCircle,
  Clock,
  Plus,
  Shield,
  Edit,
  Trash2,
  Loader2,
  AlertCircle,
  Globe,
  Eye,
  Search,
  Filter,
} from "lucide-react";
import { api } from "@/lib/api";
import { useDebounce } from "@/hooks/use-debounce";
import { RegisterMCPModal } from "@/components/modals/register-mcp-modal";
import { MCPDetailModal } from "@/components/modals/mcp-detail-modal";
import { formatDateTime } from "@/lib/date-utils";
import { getErrorMessage } from "@/lib/error-messages";
import { AuthGuard } from "@/components/auth-guard";
import { ConfirmDialog } from "@/components/modals/confirm-dialog";
interface MCPServer {
  id: string;
  name: string;
  url: string;
  description?: string;
  status:
    | "active"
    | "inactive"
    | "pending"
    | "verified"
    | "suspended"
    | "revoked";
  publicKey?: string;
  keyType?: string;
  lastVerifiedAt?: string;
  createdAt: string;
  trustScore?: number;
  capabilityCount?: number;
  capabilities?: Array<{
    id: string;
    mcpServerId: string;
    name: string;
    type: "tool" | "resource" | "prompt";
    description: string;
    schema: any;
    detectedAt: string;
    lastVerifiedAt?: string;
    isActive: boolean;
  }>;
  talksTo?: string[];
}

function StatCard({ stat }: { stat: any }) {
  return (
    <div className="glass p-6">
      <div className="flex items-center">
        <div className="flex-shrink-0">
          <stat.icon className="h-6 w-6 text-ink-tertiary" />
        </div>
        <div className="ml-5 w-0 flex-1">
          <dl>
            <dt className="text-sm font-medium text-ink-secondary truncate">
              {stat.name}
            </dt>
            <dd className="flex items-baseline">
              <div className="text-2xl font-semibold text-ink">
                {stat.value}
              </div>
              {stat.change && (
                <div
                  className={`ml-2 flex items-baseline text-sm font-semibold ${
                    stat.changeType === "positive"
                      ? "text-success-text"
                      : "text-danger-text"
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
    switch (status) {
      case "verified":
        return "bg-success-fill text-success-text border border-success-border";
      case "pending":
        return "bg-warning-fill text-warning-text border border-warning-border";
      case "suspended":
      case "revoked":
        return "bg-danger-fill text-danger-text border border-danger-border";
      case "inactive":
        return "bg-glass-inset-gray text-ink-secondary border border-stroke";
      default:
        return "bg-glass-inset-gray text-ink-secondary border border-stroke";
    }
  };

  return (
    <span
      className={`inline-flex items-center px-2.5 py-0.5 rounded-pill text-xs font-medium capitalize ${getStatusStyles(status)}`}
    >
      {status}
    </span>
  );
}

function MCPServersTableSkeleton() {
  return (
    <div className="space-y-6">
      {/* Header Skeleton */}
      <div className="flex items-center justify-between">
        <div className="space-y-2">
          <div className="animate-pulse bg-track h-8 w-40 rounded"></div>
          <div className="animate-pulse bg-track h-4 w-96 rounded"></div>
        </div>
        <div className="animate-pulse bg-track h-10 w-32 rounded-lg"></div>
      </div>

      {/* Stats Cards Skeleton */}
      <div className="grid grid-cols-1 gap-5 sm:grid-cols-2 lg:grid-cols-4">
        {[...Array(4)].map((_, i) => (
          <div
            key={i}
            className="glass p-6"
          >
            <div className="flex items-center">
              <div className="flex-shrink-0">
                <div className="animate-pulse bg-track h-6 w-6 rounded"></div>
              </div>
              <div className="ml-5 w-0 flex-1">
                <div className="space-y-2">
                  <div className="animate-pulse bg-track h-4 w-24 rounded"></div>
                  <div className="flex items-baseline gap-2">
                    <div className="animate-pulse bg-track h-8 w-16 rounded"></div>
                    <div className="animate-pulse bg-track h-4 w-12 rounded"></div>
                  </div>
                </div>
              </div>
            </div>
          </div>
        ))}
      </div>

      {/* Filters Skeleton */}
      <div className="glass p-4">
        <div className="flex flex-col sm:flex-row gap-4">
          <div className="animate-pulse bg-track flex-1 h-10 rounded-lg"></div>
          <div className="animate-pulse bg-track h-10 w-40 rounded-lg"></div>
        </div>
      </div>

      {/* Table Skeleton */}
      <div className="glass overflow-hidden">
        <div className="overflow-x-auto">
          <table className="min-w-full divide-y divide-divider">
            <thead className="bg-glass-inset-gray">
              <tr>
                <th className="px-6 py-3">
                  <div className="animate-pulse bg-track h-4 w-24 rounded"></div>
                </th>
                <th className="px-6 py-3">
                  <div className="animate-pulse bg-track h-4 w-16 rounded"></div>
                </th>
                <th className="px-6 py-3">
                  <div className="animate-pulse bg-track h-4 w-16 rounded"></div>
                </th>
                <th className="px-6 py-3">
                  <div className="animate-pulse bg-track h-4 w-24 rounded"></div>
                </th>
                <th className="px-6 py-3">
                  <div className="animate-pulse bg-track h-4 w-16 rounded"></div>
                </th>
              </tr>
            </thead>
            <tbody className="divide-y divide-divider">
              {[...Array(5)].map((_, rowIndex) => (
                <tr key={rowIndex}>
                  <td className="px-6 py-4">
                    <div className="flex items-center">
                      <div className="animate-pulse bg-track h-10 w-10 rounded-lg"></div>
                      <div className="ml-4 space-y-1">
                        <div className="animate-pulse bg-track h-4 w-32 rounded"></div>
                        <div className="animate-pulse bg-track h-3 w-20 rounded"></div>
                      </div>
                    </div>
                  </td>
                  <td className="px-6 py-4">
                    <div className="animate-pulse bg-track h-6 w-20 rounded-full"></div>
                  </td>
                  <td className="px-6 py-4">
                    <div className="animate-pulse bg-track h-4 w-16 rounded"></div>
                  </td>
                  <td className="px-6 py-4">
                    <div className="animate-pulse bg-track h-4 w-20 rounded"></div>
                  </td>
                  <td className="px-6 py-4">
                    <div className="flex items-center gap-2">
                      <div className="animate-pulse bg-track h-6 w-6 rounded"></div>
                      <div className="animate-pulse bg-track h-6 w-6 rounded"></div>
                      <div className="animate-pulse bg-track h-6 w-6 rounded"></div>
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
  return (
    <div className="flex items-center justify-center min-h-[400px]">
      <div className="flex flex-col items-center gap-4 max-w-md text-center">
        <AlertCircle className="h-12 w-12 text-danger-text" />
        <h3 className="text-lg font-semibold text-ink">
          Failed to load MCP servers
        </h3>
        <p className="text-sm text-ink-secondary">{message}</p>
        <button
          onClick={onRetry}
          className="px-4 py-2 rounded-pill bg-brand text-white shadow-accent hover:bg-brand-hover transition-colors"
        >
          Retry
        </button>
      </div>
    </div>
  );
}

export default function MCPServersPage() {
  const router = useRouter();
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [mcpServers, setMcpServers] = useState<MCPServer[]>([]);

  // Modal state
  const [showRegisterModal, setShowRegisterModal] = useState(false);
  const [showDetailModal, setShowDetailModal] = useState(false);
  const [selectedMCP, setSelectedMCP] = useState<MCPServer | null>(null);
  const [editingMCP, setEditingMCP] = useState<MCPServer | null>(null);
  const [showDeleteConfirm, setShowDeleteConfirm] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<MCPServer | null>(null);
  const [deleteLoading, setDeleteLoading] = useState(false);

  // Role (for action permissions)
  const [userRole, setUserRole] = useState<
    "admin" | "manager" | "member" | "viewer"
  >("viewer");

  useEffect(() => {
    const token = api.getToken?.();
    if (!token) return;
    try {
      const payload = JSON.parse(atob(token.split(".")[1]));
      const role = (payload.role as any) || "viewer";
      setUserRole(role);
    } catch {}
  }, []);

  // Filter state
  const [searchTerm, setSearchTerm] = useState("");
  const [statusFilter, setStatusFilter] = useState<string>("all");

  // Pagination state
  const [currentPage, setCurrentPage] = useState(1);
  const PAGE_SIZE = 20;

  // Debounce search term for better performance
  const debouncedSearchTerm = useDebounce(searchTerm, 300);

  const fetchMCPServers = async () => {
    try {
      setLoading(true);
      setError(null);
      const data = await api.listMCPServers();
      setMcpServers(data.mcpServers || []);
    } catch (err) {
      console.error("Failed to fetch MCP servers:", err);
      const errorMessage = getErrorMessage(err, {
        resource: "MCP servers",
        action: "load",
      });
      setError(errorMessage);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchMCPServers();
  }, []);

  // Reset pagination when filters change
  useEffect(() => {
    setCurrentPage(1);
  }, [debouncedSearchTerm, statusFilter]);

  // Get most recent activity timestamp for an MCP server
  // Considers: registrations (createdAt), verifications (lastVerifiedAt), capability updates
  function getMostRecentActivity(server: MCPServer): Date | null {
    const timestamps: Date[] = [];

    // Registration time
    if (server.createdAt) {
      timestamps.push(new Date(server.createdAt));
    }

    // Last verification time
    if (server.lastVerifiedAt) {
      timestamps.push(new Date(server.lastVerifiedAt));
    }

    // Capability updates (if any capabilities have been detected/verified)
    if (server.capabilities && server.capabilities.length > 0) {
      server.capabilities.forEach(cap => {
        if (cap.detectedAt) {
          timestamps.push(new Date(cap.detectedAt));
        }
        if (cap.lastVerifiedAt) {
          timestamps.push(new Date(cap.lastVerifiedAt));
        }
      });
    }

    if (timestamps.length === 0) return null;

    // Return the most recent timestamp
    return timestamps.reduce((latest, current) =>
      current > latest ? current : latest
    );
  }

  // Calculate stats
  // Note: Backend counts "verified" status as "active" for dashboard metrics
  // We should also count both "active" and "verified" to be consistent
  const stats = {
    total: mcpServers.length,
    active: mcpServers.filter((s) => s.status === "active" || s.status === "verified").length,
    avgTrustScore:
      mcpServers.reduce((sum, s) => sum + (s.trustScore || 0), 0) /
      mcpServers.length,
    // Last Activity now considers: registrations, verifications, and capability updates
    lastActivity: (() => {
      const activities = mcpServers
        .map(s => getMostRecentActivity(s))
        .filter((d): d is Date => d !== null);

      if (activities.length === 0) return null;

      const mostRecent = activities.reduce((latest, current) =>
        current > latest ? current : latest
      );

      return mostRecent.toISOString();
    })(),
  };

  const statCards = [
    {
      name: "Total MCP servers",
      value: stats.total.toLocaleString(),
      // change: "+15.3%",
      // changeType: "positive",
      icon: Server,
    },
    {
      name: "Active servers",
      value: stats.active.toLocaleString(),
      // change: "+8.7%",
      // changeType: "positive",
      icon: CheckCircle2,
    },
    {
      name: "Avg trust score",
      // Trust scores are on the canonical [0,1] scale, so the average is
      // rendered as a percentage. It used to print the raw value, which read
      // as "75.0" only because the backend was writing the literal 75.0 into
      // a field the calculator defines on [0,1].
      value: mcpServers.length > 0
        ? `${(stats.avgTrustScore * 100).toFixed(1)}%`
        : "—",
      icon: Shield,
    },
    {
      name: "Last activity",
      value: stats.lastActivity
        ? formatRelativeTime(stats.lastActivity)
        : "N/A",
      icon: Clock,
    },
  ];

  function formatRelativeTime(dateString: string): string {
    const now = new Date();
    const date = new Date(dateString);
    const diffMs = now.getTime() - date.getTime();
    const diffMins = Math.floor(diffMs / 60000);
    const diffHours = Math.floor(diffMs / 3600000);
    const diffDays = Math.floor(diffMs / 86400000);

    if (diffMins < 60) return `${diffMins}m ago`;
    if (diffHours < 24) return `${diffHours}h ago`;
    return `${diffDays}d ago`;
  }

  // Filter MCP servers with useMemo for performance (using debounced search term)
  const filteredServers = useMemo(() => {
    return mcpServers.filter((server) => {
      const matchesSearch =
        debouncedSearchTerm === "" ||
        server.name.toLowerCase().includes(debouncedSearchTerm.toLowerCase()) ||
        server.url.toLowerCase().includes(debouncedSearchTerm.toLowerCase()) ||
        server.id.toLowerCase().includes(debouncedSearchTerm.toLowerCase());

      const matchesStatus =
        statusFilter === "all" || server.status === statusFilter;

      return matchesSearch && matchesStatus;
    });
  }, [mcpServers, debouncedSearchTerm, statusFilter]);

  // Pagination
  const paginatedServers = filteredServers.slice(0, currentPage * PAGE_SIZE);
  const hasMore = currentPage * PAGE_SIZE < filteredServers.length;

  // Handlers
  const handleServerCreated = (newServer: any) => {
    setMcpServers([newServer, ...mcpServers]);
    setShowRegisterModal(false);
  };

  const handleViewMCP = async (mcp: MCPServer) => {
    // Navigate to MCP server details page instead of opening modal
    router.push(`/dashboard/mcp/${mcp.id}`);
  };

  const handleEditMCP = (mcp: MCPServer) => {
    setEditingMCP(mcp);
    setShowDetailModal(false);
    setShowRegisterModal(true);
  };

  const requestDeleteMCP = (mcp: MCPServer) => {
    setDeleteTarget(mcp);
    setShowDeleteConfirm(true);
  };

  const handleDeleteMCP = async () => {
    if (!deleteTarget) return;

    setDeleteLoading(true);
    try {
      await api.deleteMCPServer(deleteTarget.id);
      setMcpServers((prev) => prev.filter((s) => s.id !== deleteTarget.id));
      if (selectedMCP?.id === deleteTarget.id) {
        setShowDetailModal(false);
        setSelectedMCP(null);
      }
      setShowDeleteConfirm(false);
      setDeleteTarget(null);
    } catch (err) {
      console.error("Failed to delete MCP server:", err);
      alert("Failed to delete MCP server");
    } finally {
      setDeleteLoading(false);
    }
  };

  if (loading) {
    return <MCPServersTableSkeleton />;
  }

  if (error && mcpServers.length === 0) {
    return <ErrorDisplay message={error} onRetry={fetchMCPServers} />;
  }

  return (
    <AuthGuard>
      <div className="space-y-6">
        {/* Header */}
        <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-ink">
            MCP servers
          </h1>
          <p className="mt-1 text-sm text-ink-secondary">
            Manage Model Context Protocol (MCP) servers and their cryptographic
            verification status.
          </p>
        </div>
        <button
          onClick={() => {
            setEditingMCP(null);
            setShowRegisterModal(true);
          }}
          className="flex items-center gap-2 px-4 py-2 rounded-pill bg-brand text-white shadow-accent hover:bg-brand-hover transition-colors"
        >
          <Plus className="h-4 w-4" />
          Register MCP server
        </button>
      </div>

      {/* Stats */}
      <div className="grid grid-cols-1 gap-5 sm:grid-cols-2 lg:grid-cols-4">
        {statCards.map((stat) => (
          <StatCard key={stat.name} stat={stat} />
        ))}
      </div>

      {/* Search and Filter */}
      <div className="glass p-4">
        <div className="flex flex-col sm:flex-row gap-4">
          <div className="flex-1 relative">
            <Search className="absolute left-3 top-1/2 transform -translate-y-1/2 h-4 w-4 text-ink-tertiary" />
            <input
              type="text"
              placeholder="Search by name, URL, or ID..."
              value={searchTerm}
              onChange={(e) => setSearchTerm(e.target.value)}
              className="w-full pl-10 pr-4 py-2 border border-stroke rounded-inset-sm bg-glass-inset text-ink placeholder:text-ink-tertiary focus:outline-none focus:ring-2 focus:ring-brand focus:border-transparent"
            />
          </div>
          <div className="flex items-center gap-2">
            <Filter className="h-4 w-4 text-ink-tertiary" />
            <select
              value={statusFilter}
              onChange={(e) => setStatusFilter(e.target.value)}
              className="px-4 py-2 border border-stroke rounded-inset-sm bg-glass-inset text-ink focus:outline-none focus:ring-2 focus:ring-brand focus:border-transparent"
            >
              <option value="all">All statuses</option>
              <option value="active">Active</option>
              <option value="inactive">Inactive</option>
              <option value="pending">Pending</option>
            </select>
          </div>
        </div>
        {searchTerm && (
          <div className="mt-2 text-sm text-ink-secondary">
            Found {filteredServers.length} of {mcpServers.length} servers
          </div>
        )}
      </div>

      {/* MCP Servers Table */}
      <div className="glass overflow-hidden">
        <div className="overflow-x-auto">
          <table className="min-w-full divide-y divide-divider">
            <thead className="bg-glass-inset-gray">
              <tr>
                <th className="px-4 py-3 text-left text-xs font-medium text-ink-secondary uppercase tracking-wider">
                  Name
                </th>
                <th className="px-4 py-3 text-left text-xs font-medium text-ink-secondary uppercase tracking-wider">
                  Endpoint
                </th>
                <th className="px-4 py-3 text-left text-xs font-medium text-ink-secondary uppercase tracking-wider">
                  Status
                </th>
                <th className="px-4 py-3 text-left text-xs font-medium text-ink-secondary uppercase tracking-wider">
                  Verified
                </th>
                <th className="px-4 py-3 text-left text-xs font-medium text-ink-secondary uppercase tracking-wider">
                  Actions
                </th>
              </tr>
            </thead>
            <tbody className="divide-y divide-divider">
              {paginatedServers.map((server) => (
                <tr
                  key={server?.id}
                  className="hover:bg-glass-inset-gray transition-colors cursor-pointer"
                  onClick={() => handleViewMCP(server)}
                >
                  <td className="px-4 py-3 whitespace-nowrap">
                    <div className="flex items-center">
                      <div className="flex-shrink-0 h-8 w-8 bg-brand-soft rounded-lg flex items-center justify-center">
                        <Server className="h-4 w-4 text-brand-text" />
                      </div>
                      <div className="ml-3">
                        <div className="text-sm font-medium text-ink">
                          {server?.name}
                        </div>
                        <div
                          className="text-xs text-ink-secondary"
                          title={server?.id}
                        >
                          {server?.id?.substring(0, 8)}...
                        </div>
                      </div>
                    </div>
                  </td>
                  <td className="px-4 py-3">
                    <div className="flex items-center text-sm text-ink">
                      <Globe className="h-3 w-3 mr-1 text-ink-tertiary flex-shrink-0" />
                      <a
                        href={server?.url}
                        target="_blank"
                        rel="noopener noreferrer"
                        className="truncate max-w-[200px] hover:text-brand-text hover:underline transition-colors text-xs"
                        title={server?.url}
                        onClick={(e) => e.stopPropagation()}
                      >
                        {server?.url}
                      </a>
                    </div>
                  </td>
                  <td className="px-4 py-3 whitespace-nowrap">
                    <StatusBadge status={server?.status} />
                  </td>
                  <td className="px-4 py-3 whitespace-nowrap">
                    {server?.lastVerifiedAt ? (
                      <div className="flex items-center gap-2">
                        <CheckCircle2 className="h-4 w-4 text-success-text" />
                        <span className="text-sm text-ink">
                          {formatRelativeTime(server.lastVerifiedAt)}
                        </span>
                      </div>
                    ) : server?.status === "verified" ? (
                      <div className="flex items-center gap-2">
                        <CheckCircle2 className="h-4 w-4 text-success-text" />
                        <span className="text-sm text-ink">
                          Via Attestation
                        </span>
                      </div>
                    ) : (
                      <div className="flex items-center gap-2">
                        <XCircle className="h-4 w-4 text-ink-tertiary" />
                        <span className="text-sm text-ink-secondary">
                          Not verified
                        </span>
                      </div>
                    )}
                  </td>
                  <td
                    className="px-4 py-3 whitespace-nowrap"
                    onClick={(e) => e.stopPropagation()}
                  >
                    <div className="flex items-center gap-2">
                      <button
                        onClick={() => handleViewMCP(server)}
                        className="p-1 text-ink-tertiary hover:text-brand-text transition-colors"
                        title="View details"
                      >
                        <Eye className="h-4 w-4" />
                      </button>
                      {(userRole === "admin" ||
                        userRole === "manager" ||
                        userRole === "member") && (
                        <button
                          onClick={() => handleEditMCP(server)}
                          className="p-1 text-ink-tertiary hover:text-warning-text transition-colors"
                          title="Edit"
                        >
                          <Edit className="h-4 w-4" />
                        </button>
                      )}
                      {(userRole === "admin" || userRole === "manager") && (
                        <button
                          onClick={() => requestDeleteMCP(server)}
                          className="p-1 text-ink-tertiary hover:text-danger-text transition-colors"
                          title="Delete"
                        >
                          <Trash2 className="h-4 w-4" />
                        </button>
                      )}
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
        {mcpServers.length === 0 && (
          <div className="text-center py-12 space-y-6">
            <Server className="mx-auto h-12 w-12 text-ink-tertiary" />
            <div>
              <h3 className="text-sm font-medium text-ink">
                No MCP servers registered
              </h3>
              <p className="mt-1 text-sm text-ink-secondary">
                Get started by registering your first MCP server.
              </p>
            </div>

            <div className="max-w-lg mx-auto space-y-3">
              <div className="glass-inset p-4 text-left">
                <p className="font-medium text-sm text-ink mb-1">Option 1: Register via SDK</p>
                <p className="text-xs text-ink-secondary">
                  Use the AIM SDK to automatically register MCP servers when your agent connects to them.
                </p>
                <code className="text-xs bg-glass-inset-gray px-2 py-1 rounded-md block mt-2 font-mono text-ink-body">
                  agent.attest_mcp("server-name", "https://mcp.example.com")
                </code>
              </div>
              <div className="glass-inset p-4 text-left">
                <p className="font-medium text-sm text-ink mb-1">Option 2: Manual registration</p>
                <p className="text-xs text-ink-secondary">
                  Click the button below to manually register an MCP server with its URL and details.
                </p>
              </div>
            </div>

            <button
              onClick={() => setShowRegisterModal(true)}
              className="inline-flex items-center gap-2 px-4 py-2 rounded-pill bg-brand text-white shadow-accent hover:bg-brand-hover transition-colors"
            >
              <Plus className="h-4 w-4" />
              Register MCP server
            </button>

            <p className="text-xs text-ink-secondary">
              See the <a href="https://opena2a.org/docs/integration/python" className="underline hover:text-ink">SDK documentation</a> for more details.
            </p>
          </div>
        )}
        {mcpServers.length > 0 && filteredServers.length === 0 && (
          <div className="text-center py-12">
            <Search className="mx-auto h-12 w-12 text-ink-tertiary" />
            <h3 className="mt-2 text-sm font-medium text-ink">
              No servers found
            </h3>
            <p className="mt-1 text-sm text-ink-secondary">
              Try adjusting your search or filter criteria.
            </p>
            <button
              onClick={() => {
                setSearchTerm("");
                setStatusFilter("all");
              }}
              className="mt-4 px-4 py-2 text-brand-text hover:underline"
            >
              Clear filters
            </button>
          </div>
        )}
        {/* Pagination Controls */}
        {filteredServers.length > 0 && (
          <div className="px-6 py-4 border-t border-divider flex items-center justify-between">
            <div className="text-sm text-ink-secondary">
              Showing {paginatedServers.length} of {filteredServers.length} servers
            </div>
            <div className="flex gap-2">
              {currentPage > 1 && (
                <button
                  onClick={() => setCurrentPage(1)}
                  className="px-4 py-2 text-sm text-ink-body hover:text-ink border border-stroke rounded-pill hover:bg-glass-inset-gray transition-colors"
                >
                  Show Less
                </button>
              )}
              {hasMore && (
                <button
                  onClick={() => setCurrentPage(currentPage + 1)}
                  className="px-4 py-2 text-sm rounded-pill bg-brand text-white shadow-accent hover:bg-brand-hover transition-colors"
                >
                  Load More
                </button>
              )}
            </div>
          </div>
        )}
      </div>

      {/* Info Card */}
      <div className="glass p-6">
        <div className="flex items-start gap-4">
          <div className="flex-shrink-0">
            <Shield className="h-6 w-6 text-brand-text" />
          </div>
          <div>
            <h3 className="text-sm font-medium text-ink">
              About MCP server verification
            </h3>
            <p className="mt-2 text-sm text-ink-body">
              Model Context Protocol (MCP) servers must be verified before they
              can interact with AI agents. Cryptographic verification uses
              public key infrastructure to ensure servers meet security
              standards and operate within defined boundaries. Regular
              re-verification is recommended to maintain trust scores.
            </p>
          </div>
        </div>
      </div>

      {/* Modals */}
      <RegisterMCPModal
        isOpen={showRegisterModal}
        onClose={() => {
          setShowRegisterModal(false);
          setEditingMCP(null);
        }}
        onSuccess={handleServerCreated}
        editMode={!!editingMCP}
        initialData={editingMCP}
      />

      <MCPDetailModal
        isOpen={showDetailModal}
        onClose={() => {
          setShowDetailModal(false);
          setSelectedMCP(null);
        }}
        mcp={selectedMCP}
        onEdit={handleEditMCP}
        onDelete={requestDeleteMCP}
      />

      <ConfirmDialog
        isOpen={showDeleteConfirm}
        title="Delete MCP server"
        message={`Are you sure you want to delete "${
          deleteTarget?.name ?? "this MCP server"
        }"? This action cannot be undone.`}
        confirmText="Delete"
        cancelText="Cancel"
        variant="danger"
        loading={deleteLoading}
        onConfirm={handleDeleteMCP}
        onCancel={() => {
          if (deleteLoading) return;
          setShowDeleteConfirm(false);
          setDeleteTarget(null);
        }}
      />
    </div>
    </AuthGuard>
  );
}
