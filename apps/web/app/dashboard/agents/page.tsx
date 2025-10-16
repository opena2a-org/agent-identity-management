"use client";

import { useState, useEffect } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import {
  Users,
  Shield,
  Clock,
  TrendingUp,
  Search,
  Filter,
  Eye,
  Edit,
  Trash2,
  Plus,
  Loader2,
  AlertCircle,
  CheckCircle2,
  XCircle,
} from "lucide-react";
import { api, Agent } from "@/lib/api";
import { RegisterAgentModal } from "@/components/modals/register-agent-modal";
import { AgentDetailModal } from "@/components/modals/agent-detail-modal";
import { ConfirmDialog } from "@/components/modals/confirm-dialog";
import { AgentsPageSkeleton } from "@/components/ui/content-loaders";
import { getAgentPermissions, UserRole } from "@/lib/permissions";

interface AgentStats {
  total: number;
  verified: number;
  pending: number;
  avgTrustScore: number;
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

function TrustScoreBar({ score }: { score: number }) {
  // Convert decimal (0-1) to percentage (0-100) if needed
  const normalizedScore =
    score <= 1 ? Math.round(score * 100) : Math.round(score);

  const getScoreColor = (score: number) => {
    if (score >= 80) return "bg-green-500";
    if (score >= 60) return "bg-yellow-500";
    return "bg-red-500";
  };

  const getScoreBadgeColor = (score: number) => {
    if (score >= 80)
      return "bg-green-100 dark:bg-green-900/30 text-green-800 dark:text-green-300";
    if (score >= 60)
      return "bg-yellow-100 dark:bg-yellow-900/30 text-yellow-800 dark:text-yellow-300";
    return "bg-red-100 dark:bg-red-900/30 text-red-800 dark:text-red-300";
  };

  return (
    <div className="flex items-center gap-3">
      <div className="flex-1">
        <div className="w-full bg-gray-200 dark:bg-gray-700 rounded-full h-2">
          <div
            className={`${getScoreColor(normalizedScore)} h-2 rounded-full transition-all duration-300`}
            style={{ width: `${normalizedScore}%` }}
          />
        </div>
      </div>
      <span
        className={`inline-flex items-center px-2 py-1 rounded-md text-xs font-medium ${getScoreBadgeColor(normalizedScore)}`}
      >
        {normalizedScore}%
      </span>
    </div>
  );
}

function LoadingSpinner() {
  return (
    <div className="flex items-center justify-center min-h-[400px]">
      <div className="flex flex-col items-center gap-4">
        <Loader2 className="h-12 w-12 text-blue-500 animate-spin" />
        <p className="text-sm text-gray-500 dark:text-gray-400">
          Loading agents...
        </p>
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
          Failed to Load Agents
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

export default function AgentsPage() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [agents, setAgents] = useState<Agent[]>([]);
  const [searchTerm, setSearchTerm] = useState("");
  const [statusFilter, setStatusFilter] = useState<string>("all");
  const [userRole, setUserRole] = useState<UserRole>("viewer");

  // Get filter parameter from URL (e.g., ?filter=low_trust)
  const urlFilter = searchParams.get("filter");

  // Modal states
  const [showRegisterModal, setShowRegisterModal] = useState(false);
  const [showDetailModal, setShowDetailModal] = useState(false);
  const [showEditModal, setShowEditModal] = useState(false);
  const [showDeleteConfirm, setShowDeleteConfirm] = useState(false);
  const [selectedAgent, setSelectedAgent] = useState<Agent | null>(null);

  // Extract user role from JWT token
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

  // Get role-based permissions
  const permissions = getAgentPermissions(userRole);

  const fetchAgents = async () => {
    try {
      setLoading(true);
      setError(null);
      const data = await api.listAgents();
      setAgents(data.agents);
    } catch (err) {
      console.error("Failed to fetch agents:", err);
      setError(
        err instanceof Error ? err.message : "An unknown error occurred"
      );
      // For development, use mock data as fallback
      setAgents(getMockAgents());
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchAgents();
  }, []);

  // Mock data fallback for development
  const getMockAgents = (): Agent[] => [
    {
      id: "agt_001",
      organization_id: "org_123",
      name: "claude-assistant",
      display_name: "Claude AI Assistant",
      description: "Advanced AI assistant for code analysis and support",
      agent_type: "ai_agent",
      status: "verified",
      version: "2.1.0",
      trust_score: 95,
      created_at: "2025-01-15T10:30:00Z",
      updated_at: "2025-01-20T14:22:00Z",
    },
    {
      id: "agt_002",
      organization_id: "org_123",
      name: "data-processor",
      display_name: "Data Processing Agent",
      description: "Automated data transformation and analysis",
      agent_type: "ai_agent",
      status: "verified",
      version: "1.5.2",
      trust_score: 88,
      created_at: "2025-01-10T09:15:00Z",
      updated_at: "2025-01-19T11:45:00Z",
    },
    {
      id: "agt_003",
      organization_id: "org_123",
      name: "security-monitor",
      display_name: "Security Monitoring Agent",
      description: "Real-time security threat detection and response",
      agent_type: "ai_agent",
      status: "verified",
      version: "3.0.1",
      trust_score: 92,
      created_at: "2025-01-12T13:20:00Z",
      updated_at: "2025-01-20T16:30:00Z",
    },
    {
      id: "agt_004",
      organization_id: "org_123",
      name: "report-generator",
      display_name: "Report Generator",
      description: "Automated report creation and distribution",
      agent_type: "ai_agent",
      status: "pending",
      version: "1.0.0",
      trust_score: 65,
      created_at: "2025-01-18T08:00:00Z",
      updated_at: "2025-01-18T08:00:00Z",
    },
    {
      id: "agt_005",
      organization_id: "org_123",
      name: "workflow-automation",
      display_name: "Workflow Automation Agent",
      description: "Business process automation and orchestration",
      agent_type: "ai_agent",
      status: "verified",
      version: "2.3.0",
      trust_score: 90,
      created_at: "2025-01-14T11:00:00Z",
      updated_at: "2025-01-20T10:15:00Z",
    },
    {
      id: "mcp_001",
      organization_id: "org_123",
      name: "file-server",
      display_name: "File Server MCP",
      description: "MCP server for file operations and management",
      agent_type: "mcp_server",
      status: "verified",
      version: "1.2.0",
      trust_score: 85,
      created_at: "2025-01-16T14:30:00Z",
      updated_at: "2025-01-20T09:00:00Z",
    },
    {
      id: "mcp_002",
      organization_id: "org_123",
      name: "database-connector",
      display_name: "Database Connector MCP",
      description: "MCP server for database queries and operations",
      agent_type: "mcp_server",
      status: "pending",
      version: "0.9.0",
      trust_score: 58,
      created_at: "2025-01-19T16:00:00Z",
      updated_at: "2025-01-19T16:00:00Z",
    },
    {
      id: "agt_006",
      organization_id: "org_123",
      name: "analytics-engine",
      display_name: "Analytics Engine",
      description: "Advanced analytics and insights generation",
      agent_type: "ai_agent",
      status: "verified",
      version: "2.0.5",
      trust_score: 87,
      created_at: "2025-01-11T12:00:00Z",
      updated_at: "2025-01-20T15:00:00Z",
    },
  ];

  // Calculate stats (with null check)
  const stats: AgentStats = {
    total: agents?.length || 0,
    verified: agents?.filter((a) => a.status === "verified").length || 0,
    pending: agents?.filter((a) => a.status === "pending").length || 0,
    avgTrustScore:
      agents && agents.length > 0
        ? Math.round(
            agents.reduce((sum, a) => sum + a.trust_score, 0) / agents.length
          )
        : 0,
  };

  const statCards = [
    {
      name: "Total Agents",
      value: stats.total.toLocaleString(),
      change: "+12.5%",
      changeType: "positive",
      icon: Users,
    },
    {
      name: "Verified Agents",
      value: stats.verified.toLocaleString(),
      change: "+8.2%",
      changeType: "positive",
      icon: CheckCircle2,
    },
    {
      name: "Pending Review",
      value: stats.pending.toLocaleString(),
      icon: Clock,
    },
    {
      name: "Avg Trust Score",
      value: `${stats.avgTrustScore}%`,
      change: "+2.1%",
      changeType: "positive",
      icon: Shield,
    },
  ];

  // Filter agents (with null check)
  const filteredAgents =
    agents?.filter((agent) => {
      const matchesSearch =
        agent.name.toLowerCase().includes(searchTerm.toLowerCase()) ||
        agent.display_name.toLowerCase().includes(searchTerm.toLowerCase());
      const matchesStatus =
        statusFilter === "all" || agent.status === statusFilter;

      // Apply URL filter (e.g., ?filter=low_trust shows only agents with trust_score < 60)
      let matchesUrlFilter = true;
      if (urlFilter === "low_trust") {
        // Normalize trust score: convert decimal (0-1) to percentage (0-100) if needed
        const normalizedScore =
          agent.trust_score <= 1 ? agent.trust_score * 100 : agent.trust_score;
        matchesUrlFilter = normalizedScore < 60;
      }

      return matchesSearch && matchesStatus && matchesUrlFilter;
    }) || [];

  const formatDate = (dateString: string) => {
    const date = new Date(dateString);
    return date.toLocaleDateString("en-US", {
      month: "short",
      day: "numeric",
      year: "numeric",
    });
  };

  // Handler functions
  const handleAgentCreated = (newAgent: Agent) => {
    setAgents([newAgent, ...agents]);
    setShowRegisterModal(false);
  };

  const handleAgentUpdated = (updatedAgent: Agent) => {
    setAgents(agents.map((a) => (a.id === updatedAgent.id ? updatedAgent : a)));
    setShowEditModal(false);
    setSelectedAgent(null);
  };

  const handleViewAgent = (agent: Agent) => {
    // Navigate to agent details page instead of opening modal
    router.push(`/dashboard/agents/${agent.id}`);
  };

  const handleEditAgent = (agent: Agent) => {
    setSelectedAgent(agent);
    setShowDetailModal(false);
    setShowEditModal(true);
  };

  const handleDeleteAgent = (agent: Agent) => {
    setSelectedAgent(agent);
    setShowDeleteConfirm(true);
  };

  const confirmDelete = async () => {
    if (!selectedAgent) return;

    try {
      await api.deleteAgent(selectedAgent.id);
      setAgents(agents.filter((a) => a.id !== selectedAgent.id));
    } catch (err) {
      console.error("Failed to delete agent:", err);
      // Mock delete for development
      setAgents(agents.filter((a) => a.id !== selectedAgent.id));
    } finally {
      setShowDeleteConfirm(false);
      setShowDetailModal(false);
      setSelectedAgent(null);
    }
  };

  if (loading) {
    return <AgentsPageSkeleton />;
  }

  if (error && agents.length === 0) {
    return <ErrorDisplay message={error} onRetry={fetchAgents} />;
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-gray-900 dark:text-white">
            Agent Registry
          </h1>
          <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">
            Manage and monitor all registered AI agents and MCP servers in your
            organization.
          </p>
          {error && (
            <div className="mt-2 p-3 bg-yellow-50 dark:bg-yellow-900/20 border border-yellow-200 dark:border-yellow-800 rounded-lg">
              <p className="text-sm text-yellow-800 dark:text-yellow-300">
                ⚠️ Using mock data - API connection failed: {error}
              </p>
            </div>
          )}
        </div>
        {permissions.canCreateAgent && (
          <button
            onClick={() => setShowRegisterModal(true)}
            className="flex items-center gap-2 px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 transition-colors"
          >
            <Plus className="h-4 w-4" />
            Register Agent
          </button>
        )}
      </div>

      {/* Stats */}
      <div className="grid grid-cols-1 gap-5 sm:grid-cols-2 lg:grid-cols-4">
        {statCards.map((stat) => (
          <StatCard key={stat.name} stat={stat} />
        ))}
      </div>

      {/* Filters */}
      <div className="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 shadow-sm p-4">
        <div className="flex flex-col sm:flex-row gap-4">
          <div className="flex-1 relative">
            <Search className="absolute left-3 top-1/2 transform -translate-y-1/2 h-5 w-5 text-gray-400" />
            <input
              type="text"
              placeholder="Search agents by name..."
              value={searchTerm}
              onChange={(e) => setSearchTerm(e.target.value)}
              className="w-full pl-10 pr-4 py-2 bg-gray-50 dark:bg-gray-900 border border-gray-200 dark:border-gray-700 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 text-gray-900 dark:text-gray-100"
            />
          </div>
          <div className="relative">
            <Filter className="absolute left-3 top-1/2 transform -translate-y-1/2 h-5 w-5 text-gray-400" />
            <select
              value={statusFilter}
              onChange={(e) => setStatusFilter(e.target.value)}
              className="pl-10 pr-8 py-2 bg-gray-50 dark:bg-gray-900 border border-gray-200 dark:border-gray-700 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 text-gray-900 dark:text-gray-100"
            >
              <option value="all">All Status</option>
              <option value="verified">Verified</option>
              <option value="pending">Pending</option>
              <option value="suspended">Suspended</option>
              <option value="revoked">Revoked</option>
            </select>
          </div>
        </div>
      </div>

      {/* Agents Table */}
      <div className="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 shadow-sm">
        <div className="overflow-x-auto">
          <table className="min-w-full divide-y divide-gray-200 dark:divide-gray-700">
            <thead className="bg-gray-50 dark:bg-gray-800">
              <tr>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                  Agent Name
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                  Type
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                  Version
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                  Status
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                  Trust Score
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                  Last Updated
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                  Actions
                </th>
              </tr>
            </thead>
            <tbody className="bg-white dark:bg-gray-900 divide-y divide-gray-200 dark:divide-gray-700">
              {filteredAgents.map((agent) => (
                <tr
                  key={agent.id}
                  className="hover:bg-gray-50 dark:hover:bg-gray-800 transition-colors cursor-pointer"
                  onClick={() => handleViewAgent(agent)}
                >
                  <td className="px-6 py-4 whitespace-nowrap">
                    <div className="flex items-center">
                      <div className="flex-shrink-0 h-10 w-10 bg-blue-100 dark:bg-blue-900/30 rounded-lg flex items-center justify-center">
                        {agent.agent_type === "ai_agent" ? (
                          <Users className="h-5 w-5 text-blue-600 dark:text-blue-400" />
                        ) : (
                          <Shield className="h-5 w-5 text-purple-600 dark:text-purple-400" />
                        )}
                      </div>
                      <div className="ml-4">
                        <div className="text-sm font-medium text-gray-900 dark:text-gray-100">
                          {agent.display_name}
                        </div>
                        <div className="text-xs text-gray-500 dark:text-gray-400">
                          {agent.name}
                        </div>
                      </div>
                    </div>
                  </td>
                  <td className="px-6 py-4 whitespace-nowrap">
                    <span
                      className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${
                        agent.agent_type === "ai_agent"
                          ? "bg-blue-100 dark:bg-blue-900/30 text-blue-800 dark:text-blue-300"
                          : "bg-purple-100 dark:bg-purple-900/30 text-purple-800 dark:text-purple-300"
                      }`}
                    >
                      {agent.agent_type === "ai_agent"
                        ? "AI Agent"
                        : "MCP Server"}
                    </span>
                  </td>
                  <td className="px-6 py-4 whitespace-nowrap">
                    <div className="text-sm text-gray-900 dark:text-gray-100">
                      {agent.version}
                    </div>
                  </td>
                  <td className="px-6 py-4 whitespace-nowrap">
                    <StatusBadge status={agent.status} />
                  </td>
                  <td className="px-6 py-4 whitespace-nowrap">
                    <div className="w-40">
                      <TrustScoreBar score={agent.trust_score} />
                    </div>
                  </td>
                  <td className="px-6 py-4 whitespace-nowrap">
                    <div className="text-sm text-gray-500 dark:text-gray-400">
                      {formatDate(agent.updated_at)}
                    </div>
                  </td>
                  <td
                    className="px-6 py-4 whitespace-nowrap"
                    onClick={(e) => e.stopPropagation()}
                  >
                    <div className="flex items-center gap-2">
                      {permissions.canViewAgent && (
                        <button
                          onClick={() => handleViewAgent(agent)}
                          className="p-1 text-gray-400 hover:text-blue-600 dark:hover:text-blue-400 transition-colors"
                          title="View details"
                        >
                          <Eye className="h-4 w-4" />
                        </button>
                      )}
                      {permissions.canEditAgent && (
                        <button
                          onClick={() => handleEditAgent(agent)}
                          className="p-1 text-gray-400 hover:text-yellow-600 dark:hover:text-yellow-400 transition-colors"
                          title="Edit agent"
                        >
                          <Edit className="h-4 w-4" />
                        </button>
                      )}
                      {permissions.canDeleteAgent && (
                        <button
                          onClick={() => handleDeleteAgent(agent)}
                          className="p-1 text-gray-400 hover:text-red-600 dark:hover:text-red-400 transition-colors"
                          title="Delete agent"
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
        {filteredAgents.length === 0 && (
          <div className="text-center py-12">
            <Users className="mx-auto h-12 w-12 text-gray-400" />
            <h3 className="mt-2 text-sm font-medium text-gray-900 dark:text-gray-100">
              No agents found
            </h3>
            <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">
              {searchTerm || statusFilter !== "all"
                ? "Try adjusting your search or filters."
                : "Get started by registering your first agent."}
            </p>
          </div>
        )}
      </div>

      {/* Modals */}
      <RegisterAgentModal
        isOpen={showRegisterModal}
        onClose={() => setShowRegisterModal(false)}
        onSuccess={handleAgentCreated}
      />

      <RegisterAgentModal
        isOpen={showEditModal}
        onClose={() => {
          setShowEditModal(false);
          setSelectedAgent(null);
        }}
        onSuccess={handleAgentUpdated}
        editMode={true}
        initialData={selectedAgent || undefined}
      />

      <AgentDetailModal
        isOpen={showDetailModal}
        onClose={() => {
          setShowDetailModal(false);
          setSelectedAgent(null);
        }}
        agent={selectedAgent}
        onEdit={permissions.canEditAgent ? handleEditAgent : undefined}
        onDelete={permissions.canDeleteAgent ? handleDeleteAgent : undefined}
      />

      <ConfirmDialog
        isOpen={showDeleteConfirm}
        title="Delete Agent"
        message={`Are you sure you want to delete "${selectedAgent?.display_name}"? This action cannot be undone.`}
        confirmText="Delete"
        cancelText="Cancel"
        variant="danger"
        onConfirm={confirmDelete}
        onCancel={() => {
          setShowDeleteConfirm(false);
          setSelectedAgent(null);
        }}
      />
    </div>
  );
}
