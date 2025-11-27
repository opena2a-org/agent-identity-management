"use client";

import { useState, useEffect } from "react";
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
    switch (status) {
      case "verified":
        return "bg-green-100 dark:bg-green-900/30 text-green-800 dark:text-green-300";
      case "pending":
        return "bg-yellow-100 dark:bg-yellow-900/30 text-yellow-800 dark:text-yellow-300";
      case "suspended":
      case "revoked":
        return "bg-red-100 dark:bg-red-900/30 text-red-800 dark:text-red-300";
      case "inactive":
        return "bg-gray-100 dark:bg-gray-700 text-gray-800 dark:text-gray-300";
      default:
        return "bg-gray-100 dark:bg-gray-700 text-gray-800 dark:text-gray-300";
    }
  };

  return (
    <span
      className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium capitalize ${getStatusStyles(status)}`}
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
          <div className="animate-pulse bg-gray-200 dark:bg-gray-700 h-8 w-40 rounded"></div>
          <div className="animate-pulse bg-gray-200 dark:bg-gray-700 h-4 w-96 rounded"></div>
        </div>
        <div className="animate-pulse bg-gray-200 dark:bg-gray-700 h-10 w-32 rounded-lg"></div>
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

      {/* Filters Skeleton */}
      <div className="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 shadow-sm p-4">
        <div className="flex flex-col sm:flex-row gap-4">
          <div className="animate-pulse bg-gray-200 dark:bg-gray-700 flex-1 h-10 rounded-lg"></div>
          <div className="animate-pulse bg-gray-200 dark:bg-gray-700 h-10 w-40 rounded-lg"></div>
        </div>
      </div>

      {/* Table Skeleton */}
      <div className="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 shadow-sm">
        <div className="overflow-x-auto">
          <table className="min-w-full divide-y divide-gray-200 dark:divide-gray-700">
            <thead className="bg-gray-50 dark:bg-gray-800">
              <tr>
                <th className="px-6 py-3">
                  <div className="animate-pulse bg-gray-200 dark:bg-gray-700 h-4 w-24 rounded"></div>
                </th>
                <th className="px-6 py-3">
                  <div className="animate-pulse bg-gray-200 dark:bg-gray-700 h-4 w-16 rounded"></div>
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
              {[...Array(5)].map((_, rowIndex) => (
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
                    <div className="animate-pulse bg-gray-200 dark:bg-gray-700 h-6 w-20 rounded-full"></div>
                  </td>
                  <td className="px-6 py-4">
                    <div className="animate-pulse bg-gray-200 dark:bg-gray-700 h-4 w-16 rounded"></div>
                  </td>
                  <td className="px-6 py-4">
                    <div className="animate-pulse bg-gray-200 dark:bg-gray-700 h-4 w-20 rounded"></div>
                  </td>
                  <td className="px-6 py-4">
                    <div className="flex items-center gap-2">
                      <div className="animate-pulse bg-gray-200 dark:bg-gray-700 h-6 w-6 rounded"></div>
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
  return (
    <div className="flex items-center justify-center min-h-[400px]">
      <div className="flex flex-col items-center gap-4 max-w-md text-center">
        <AlertCircle className="h-12 w-12 text-red-500" />
        <h3 className="text-lg font-semibold text-gray-900 dark:text-gray-100">
          Failed to Load MCP Servers
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

  // Calculate stats
  const stats = {
    total: mcpServers.length,
    active: mcpServers.filter((s) => s.status === "active").length,
    avgTrustScore:
      mcpServers.reduce((sum, s) => sum + (s.trustScore || 0), 0) /
      mcpServers.length,
    lastActivity: mcpServers
      .filter((s) => s.lastVerifiedAt)
      .sort(
        (a, b) =>
          new Date(b.lastVerifiedAt!).getTime() -
          new Date(a.lastVerifiedAt!).getTime()
      )[0]?.lastVerifiedAt,
  };

  const statCards = [
    {
      name: "Total MCP Servers",
      value: stats.total.toLocaleString(),
      // change: "+15.3%",
      // changeType: "positive",
      icon: Server,
    },
    {
      name: "Active Servers",
      value: stats.active.toLocaleString(),
      // change: "+8.7%",
      // changeType: "positive",
      icon: CheckCircle2,
    },
    {
      name: "Avg Trust Score",
      value: stats.avgTrustScore.toFixed(1),
      // change: stats.avgTrustScore >= 75 ? "+5.2%" : "-2.1%",
      // changeType: stats.avgTrustScore >= 75 ? "positive" : "negative",
      icon: Shield,
    },
    {
      name: "Last Activity",
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

  // Filter MCP servers based on search and status
  const filteredServers = mcpServers.filter((server) => {
    const matchesSearch =
      searchTerm === "" ||
      server.name.toLowerCase().includes(searchTerm.toLowerCase()) ||
      server.url.toLowerCase().includes(searchTerm.toLowerCase()) ||
      server.id.toLowerCase().includes(searchTerm.toLowerCase());

    const matchesStatus =
      statusFilter === "all" || server.status === statusFilter;

    return matchesSearch && matchesStatus;
  });

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
          <h1 className="text-2xl font-bold text-gray-900 dark:text-white">
            MCP Servers
          </h1>
          <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">
            Manage Model Context Protocol (MCP) servers and their cryptographic
            verification status.
          </p>
        </div>
        <button
          onClick={() => {
            setEditingMCP(null);
            setShowRegisterModal(true);
          }}
          className="flex items-center gap-2 px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 transition-colors"
        >
          <Plus className="h-4 w-4" />
          Register MCP Server
        </button>
      </div>

      {/* Stats */}
      <div className="grid grid-cols-1 gap-5 sm:grid-cols-2 lg:grid-cols-4">
        {statCards.map((stat) => (
          <StatCard key={stat.name} stat={stat} />
        ))}
      </div>

      {/* Search and Filter */}
      <div className="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 shadow-sm p-4">
        <div className="flex flex-col sm:flex-row gap-4">
          <div className="flex-1 relative">
            <Search className="absolute left-3 top-1/2 transform -translate-y-1/2 h-4 w-4 text-gray-400" />
            <input
              type="text"
              placeholder="Search by name, URL, or ID..."
              value={searchTerm}
              onChange={(e) => setSearchTerm(e.target.value)}
              className="w-full pl-10 pr-4 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-900 text-gray-900 dark:text-gray-100 placeholder-gray-400 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent"
            />
          </div>
          <div className="flex items-center gap-2">
            <Filter className="h-4 w-4 text-gray-400" />
            <select
              value={statusFilter}
              onChange={(e) => setStatusFilter(e.target.value)}
              className="px-4 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-900 text-gray-900 dark:text-gray-100 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent"
            >
              <option value="all">All Status</option>
              <option value="active">Active</option>
              <option value="inactive">Inactive</option>
              <option value="pending">Pending</option>
            </select>
          </div>
        </div>
        {searchTerm && (
          <div className="mt-2 text-sm text-gray-500 dark:text-gray-400">
            Found {filteredServers.length} of {mcpServers.length} servers
          </div>
        )}
      </div>

      {/* MCP Servers Table */}
      <div className="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 shadow-sm">
        <div className="overflow-x-auto">
          <table className="min-w-full divide-y divide-gray-200 dark:divide-gray-700">
            <thead className="bg-gray-50 dark:bg-gray-800">
              <tr>
                <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                  Name
                </th>
                <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                  Endpoint
                </th>
                <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                  Status
                </th>
                <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                  Verified
                </th>
                <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                  Actions
                </th>
              </tr>
            </thead>
            <tbody className="bg-white dark:bg-gray-900 divide-y divide-gray-200 dark:divide-gray-700">
              {filteredServers?.map((server) => (
                <tr
                  key={server?.id}
                  className="hover:bg-gray-50 dark:hover:bg-gray-800 transition-colors cursor-pointer"
                  onClick={() => handleViewMCP(server)}
                >
                  <td className="px-4 py-3 whitespace-nowrap">
                    <div className="flex items-center">
                      <div className="flex-shrink-0 h-8 w-8 bg-purple-100 dark:bg-purple-900/30 rounded-lg flex items-center justify-center">
                        <Server className="h-4 w-4 text-purple-600 dark:text-purple-400" />
                      </div>
                      <div className="ml-3">
                        <div className="text-sm font-medium text-gray-900 dark:text-gray-100">
                          {server?.name}
                        </div>
                        <div
                          className="text-xs text-gray-500 dark:text-gray-400"
                          title={server?.id}
                        >
                          {server?.id?.substring(0, 8)}...
                        </div>
                      </div>
                    </div>
                  </td>
                  <td className="px-4 py-3">
                    <div className="flex items-center text-sm text-gray-900 dark:text-gray-100">
                      <Globe className="h-3 w-3 mr-1 text-gray-400 flex-shrink-0" />
                      <a
                        href={server?.url}
                        target="_blank"
                        rel="noopener noreferrer"
                        className="truncate max-w-[200px] hover:text-blue-600 dark:hover:text-blue-400 hover:underline transition-colors text-xs"
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
                        <CheckCircle2 className="h-4 w-4 text-green-600 dark:text-green-400" />
                        <span className="text-sm text-gray-900 dark:text-gray-100">
                          {formatRelativeTime(server.lastVerifiedAt)}
                        </span>
                      </div>
                    ) : (
                      <div className="flex items-center gap-2">
                        <XCircle className="h-4 w-4 text-gray-400" />
                        <span className="text-sm text-gray-500 dark:text-gray-400">
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
                        className="p-1 text-gray-400 hover:text-blue-600 dark:hover:text-blue-400 transition-colors"
                        title="View details"
                      >
                        <Eye className="h-4 w-4" />
                      </button>
                      {(userRole === "admin" ||
                        userRole === "manager" ||
                        userRole === "member") && (
                        <button
                          onClick={() => handleEditMCP(server)}
                          className="p-1 text-gray-400 hover:text-yellow-600 dark:hover:text-yellow-400 transition-colors"
                          title="Edit"
                        >
                          <Edit className="h-4 w-4" />
                        </button>
                      )}
                      {(userRole === "admin" || userRole === "manager") && (
                        <button
                          onClick={() => requestDeleteMCP(server)}
                          className="p-1 text-gray-400 hover:text-red-600 dark:hover:text-red-400 transition-colors"
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
          <div className="text-center py-12">
            <Server className="mx-auto h-12 w-12 text-gray-400" />
            <h3 className="mt-2 text-sm font-medium text-gray-900 dark:text-gray-100">
              No MCP servers registered
            </h3>
            <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">
              Get started by registering your first MCP server.
            </p>
            <button
              onClick={() => setShowRegisterModal(true)}
              className="mt-4 inline-flex items-center gap-2 px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 transition-colors"
            >
              <Plus className="h-4 w-4" />
              Register MCP Server
            </button>
          </div>
        )}
        {mcpServers.length > 0 && filteredServers.length === 0 && (
          <div className="text-center py-12">
            <Search className="mx-auto h-12 w-12 text-gray-400" />
            <h3 className="mt-2 text-sm font-medium text-gray-900 dark:text-gray-100">
              No servers found
            </h3>
            <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">
              Try adjusting your search or filter criteria.
            </p>
            <button
              onClick={() => {
                setSearchTerm("");
                setStatusFilter("all");
              }}
              className="mt-4 px-4 py-2 text-blue-600 dark:text-blue-400 hover:underline"
            >
              Clear filters
            </button>
          </div>
        )}
      </div>

      {/* Info Card */}
      <div className="bg-blue-50 dark:bg-blue-900/20 border border-blue-200 dark:border-blue-800 rounded-lg p-6">
        <div className="flex items-start gap-4">
          <div className="flex-shrink-0">
            <Shield className="h-6 w-6 text-blue-600 dark:text-blue-400" />
          </div>
          <div>
            <h3 className="text-sm font-medium text-blue-900 dark:text-blue-100">
              About MCP Server Verification
            </h3>
            <p className="mt-2 text-sm text-blue-700 dark:text-blue-300">
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
        title="Delete MCP Server"
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
