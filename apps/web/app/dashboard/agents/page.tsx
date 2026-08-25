"use client";

import { useState, useEffect, Suspense, useMemo, useCallback } from "react";
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
import { useDebounce } from "@/hooks/use-debounce";
import { RegisterAgentModal } from "@/components/modals/register-agent-modal";
import { AgentDetailModal } from "@/components/modals/agent-detail-modal";
import { ConfirmDialog } from "@/components/modals/confirm-dialog";
import { AgentsPageSkeleton } from "@/components/ui/content-loaders";
import { getAgentPermissions, UserRole } from "@/lib/permissions";
import { getErrorMessage } from "@/lib/error-messages";
import { AuthGuard } from "@/components/auth-guard";

// Agent type display configuration
const AGENT_TYPE_LABELS: Record<string, { label: string; color: string }> = {
  // LLM Providers
  claude: { label: "Claude", color: "border border-glass-inset-border bg-glass-inset-gray text-ink-body" },
  gpt: { label: "GPT", color: "border border-glass-inset-border bg-glass-inset-gray text-ink-body" },
  gemini: { label: "Gemini", color: "border border-glass-inset-border bg-glass-inset-gray text-ink-body" },
  llama: { label: "Llama", color: "border border-glass-inset-border bg-glass-inset-gray text-ink-body" },
  mistral: { label: "Mistral", color: "border border-glass-inset-border bg-glass-inset-gray text-ink-body" },
  cohere: { label: "Cohere", color: "border border-glass-inset-border bg-glass-inset-gray text-ink-body" },
  // Frameworks
  langchain: { label: "LangChain", color: "border border-glass-inset-border bg-glass-inset-gray text-ink-body" },
  llamaindex: { label: "LlamaIndex", color: "border border-glass-inset-border bg-glass-inset-gray text-ink-body" },
  langgraph: { label: "LangGraph", color: "border border-glass-inset-border bg-glass-inset-gray text-ink-body" },
  crewai: { label: "CrewAI", color: "border border-glass-inset-border bg-glass-inset-gray text-ink-body" },
  autogen: { label: "AutoGen", color: "border border-glass-inset-border bg-glass-inset-gray text-ink-body" },
  semantic_kernel: { label: "Semantic Kernel", color: "border border-glass-inset-border bg-glass-inset-gray text-ink-body" },
  haystack: { label: "Haystack", color: "border border-glass-inset-border bg-glass-inset-gray text-ink-body" },
  // Copilots & Assistants
  copilot: { label: "Copilot", color: "border border-glass-inset-border bg-glass-inset-gray text-ink-body" },
  assistant: { label: "Assistant", color: "border border-glass-inset-border bg-glass-inset-gray text-ink-body" },
  chatbot: { label: "Chatbot", color: "border border-glass-inset-border bg-glass-inset-gray text-ink-body" },
  // Autonomous Agents
  autogpt: { label: "AutoGPT", color: "border border-glass-inset-border bg-glass-inset-gray text-ink-body" },
  babyagi: { label: "BabyAGI", color: "border border-glass-inset-border bg-glass-inset-gray text-ink-body" },
  // Other
  custom: { label: "Custom", color: "border border-glass-inset-border bg-glass-inset-gray text-ink-body" },
  // Legacy
  ai_agent: { label: "AI Agent", color: "border border-glass-inset-border bg-glass-inset-gray text-ink-body" },
};

// Get display info for an agent type
function getAgentTypeDisplay(agentType: string | undefined): { label: string; color: string } {
  if (!agentType) return AGENT_TYPE_LABELS.custom;
  return AGENT_TYPE_LABELS[agentType] || { label: agentType, color: "border border-glass-inset-border bg-glass-inset-gray text-ink-body" };
}

interface AgentStats {
  total: number;
  verified: number;
  pending: number;
  avgTrustScore: number;
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
        return "border border-success-border bg-success-fill text-success-text";
      case "pending":
        return "border border-warning-border bg-warning-fill text-warning-text";
      case "suspended":
      case "revoked":
        return "border border-danger-border bg-danger-fill text-danger-text";
      default:
        return "border border-glass-inset-border bg-glass-inset-gray text-ink-body";
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

function TrustScoreBar({ score }: { score: number }) {
  // Convert decimal (0-1) to percentage (0-100) if needed
  const normalizedScore =
    score <= 1 ? Math.round(score * 100) : Math.round(score);

  const getScoreColor = (score: number) => {
    if (score >= 80) return "bg-success";
    if (score >= 60) return "bg-warning";
    return "bg-danger";
  };

  const getScoreBadgeColor = (score: number) => {
    if (score >= 80)
      return "border border-success-border bg-success-fill text-success-text";
    if (score >= 60)
      return "border border-warning-border bg-warning-fill text-warning-text";
    return "border border-danger-border bg-danger-fill text-danger-text";
  };

  return (
    <div className="flex items-center gap-3">
      <div className="flex-1">
        <div className="w-full bg-track rounded-full h-2">
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
        <Loader2 className="h-12 w-12 text-brand-text animate-spin" />
        <p className="text-sm text-ink-secondary">
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
        <AlertCircle className="h-12 w-12 text-danger-text" />
        <h3 className="text-lg font-semibold text-ink">
          Failed to load agents
        </h3>
        <p className="text-sm text-ink-secondary">{message}</p>
        <button
          onClick={onRetry}
          className="rounded-pill bg-brand px-4 py-2 text-white shadow-glow hover:bg-brand-hover transition-colors"
        >
          Retry
        </button>
      </div>
    </div>
  );
}

function AgentsPageContent() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [agents, setAgents] = useState<Agent[]>([]);
  const [searchTerm, setSearchTerm] = useState("");
  const [statusFilter, setStatusFilter] = useState<string>("all");
  const [userRole, setUserRole] = useState<UserRole>("viewer");
  const [currentPage, setCurrentPage] = useState(1);
  const PAGE_SIZE = 20;

  // Debounce search term for better performance
  const debouncedSearchTerm = useDebounce(searchTerm, 300);

  // Get filter parameter from URL (e.g., ?filter=low_trust)
  const urlFilter = searchParams.get("filter");

  // Modal states
  const [showRegisterModal, setShowRegisterModal] = useState(false);
  const [showDetailModal, setShowDetailModal] = useState(false);
  const [showEditModal, setShowEditModal] = useState(false);
  const [showDeleteConfirm, setShowDeleteConfirm] = useState(false);
  const [selectedAgent, setSelectedAgent] = useState<Agent | null>(null);
  const [deleteLoading, setDeleteLoading] = useState(false);

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

  // /dashboard/agents?register=1 (home CTA, mobile tab bar) opens the registration form for
  // roles that may register; ?filter=pending (the Executive lens) preselects the status filter.
  useEffect(() => {
    if (searchParams.get("register") === "1" && permissions.canCreateAgent) setShowRegisterModal(true);
    if (urlFilter === "pending") setStatusFilter("pending");
  }, [searchParams, urlFilter, permissions.canCreateAgent]);

  const fetchAgents = async () => {
    try {
      setLoading(true);
      setError(null);
      const data = await api.listAgents();
      setAgents(data.agents ?? []);
    } catch (err) {
      console.error("Failed to fetch agents:", err);
      const errorMessage = getErrorMessage(err, {
        resource: "agents",
        action: "load",
      });
      setError(errorMessage);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchAgents();
  }, []);

  // Reset pagination when filters change
  useEffect(() => {
    setCurrentPage(1);
  }, [debouncedSearchTerm, statusFilter, urlFilter]);

  // Calculate stats (with null check)
  const stats: AgentStats = {
    total: agents?.length || 0,
    verified: agents?.filter((a) => a.status === "verified").length || 0,
    pending: agents?.filter((a) => a.status === "pending").length || 0,
    avgTrustScore:
      agents && agents.length > 0
        ? Math.round(
            (agents.reduce((sum, a) => sum + a.trustScore, 0) / agents.length) * 100
          )
        : 0,
  };

  const statCards = [
    {
      name: "Total agents",
      value: stats.total.toLocaleString(),
      changeType: "positive",
      icon: Users,
    },
    {
      name: "Verified agents",
      value: stats.verified.toLocaleString(),
      changeType: "positive",
      icon: CheckCircle2,
    },
    {
      name: "Pending review",
      value: stats.pending.toLocaleString(),
      icon: Clock,
    },
    {
      name: "Avg trust score",
      value: `${stats.avgTrustScore}%`,
      changeType: "positive",
      icon: Shield,
    },
  ];

  // Filter agents with useMemo for performance (using debounced search term)
  const filteredAgents = useMemo(() => {
    return agents?.filter((agent) => {
      const matchesSearch =
        agent.name.toLowerCase().includes(debouncedSearchTerm.toLowerCase()) ||
        agent.displayName.toLowerCase().includes(debouncedSearchTerm.toLowerCase());
      const matchesStatus =
        statusFilter === "all" || agent.status === statusFilter;

      // Apply URL filter (e.g., ?filter=low_trust shows only agents with trust_score < 60)
      let matchesUrlFilter = true;
      if (urlFilter === "low_trust") {
        // Normalize trust score: convert decimal (0-1) to percentage (0-100) if needed
        const normalizedScore =
          agent.trustScore <= 1 ? agent.trustScore * 100 : agent.trustScore;
        matchesUrlFilter = normalizedScore < 60;
      }

      return matchesSearch && matchesStatus && matchesUrlFilter;
    }) || [];
  }, [agents, debouncedSearchTerm, statusFilter, urlFilter]);

  // Pagination
  const paginatedAgents = filteredAgents.slice(0, currentPage * PAGE_SIZE);
  const hasMore = currentPage * PAGE_SIZE < filteredAgents.length;

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
    // Add the new agent to the list without closing the modal
    // The modal will close itself when user clicks "Done" or downloads SDK
    setAgents([newAgent, ...agents]);
    // Don't close modal here - let the modal handle it after user sees SDK download
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

    setDeleteLoading(true);
    try {
      await api.deleteAgent(selectedAgent.id);
      setAgents(agents.filter((a) => a.id !== selectedAgent.id));
    } catch (err) {
      console.error("Failed to delete agent:", err);
      setError(err instanceof Error ? err.message : "Failed to delete agent");
    } finally {
      setDeleteLoading(false);
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
          <h1 className="text-2xl font-bold text-ink">
            Agent registry
          </h1>
          <p className="mt-1 text-sm text-ink-secondary">
            Manage and monitor all registered AI agents and MCP servers in your
            organization.
          </p>
        </div>
        {permissions.canCreateAgent && (
          <button
            onClick={() => setShowRegisterModal(true)}
            className="flex items-center gap-2 rounded-pill bg-brand px-4 py-2 text-white shadow-glow hover:bg-brand-hover transition-colors"
          >
            <Plus className="h-4 w-4" />
            Create agent
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
      <div className="glass p-4">
        <div className="flex flex-col sm:flex-row gap-4">
          <div className="flex-1 relative">
            <Search className="absolute left-3 top-1/2 transform -translate-y-1/2 h-5 w-5 text-ink-tertiary" />
            <input
              type="text"
              placeholder="Search agents by name..."
              value={searchTerm}
              onChange={(e) => setSearchTerm(e.target.value)}
              className="w-full pl-10 pr-4 py-2 bg-glass-inset border border-stroke rounded-inset focus:outline-none focus:ring-2 focus:ring-brand text-ink placeholder:text-ink-tertiary"
            />
          </div>
          <div className="relative">
            <Filter className="absolute left-3 top-1/2 transform -translate-y-1/2 h-5 w-5 text-ink-tertiary" />
            <select
              value={statusFilter}
              onChange={(e) => setStatusFilter(e.target.value)}
              className="pl-10 pr-8 py-2 bg-glass-inset border border-stroke rounded-inset focus:outline-none focus:ring-2 focus:ring-brand text-ink"
            >
              <option value="all">All statuses</option>
              <option value="verified">Verified</option>
              <option value="pending">Pending</option>
              <option value="suspended">Suspended</option>
              <option value="revoked">Revoked</option>
            </select>
          </div>
        </div>
      </div>

      {/* Agents Table */}
      <div className="glass overflow-hidden">
        <div className="overflow-x-auto">
          <table className="min-w-full divide-y divide-divider">
            <thead className="bg-glass-inset-gray">
              <tr>
                <th className="px-6 py-3 text-left text-xs font-medium text-ink-tertiary uppercase tracking-wider">
                  Agent name
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium text-ink-tertiary uppercase tracking-wider">
                  Type
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium text-ink-tertiary uppercase tracking-wider">
                  Version
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium text-ink-tertiary uppercase tracking-wider">
                  Status
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium text-ink-tertiary uppercase tracking-wider">
                  Trust score
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium text-ink-tertiary uppercase tracking-wider">
                  Last updated
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium text-ink-tertiary uppercase tracking-wider">
                  Actions
                </th>
              </tr>
            </thead>
            <tbody className="divide-y divide-divider">
              {paginatedAgents.map((agent) => (
                <tr
                  key={agent?.id}
                  className="hover:bg-glass-inset-gray transition-colors cursor-pointer"
                  onClick={() => handleViewAgent(agent)}
                >
                  <td className="px-6 py-4 whitespace-nowrap">
                    <div className="flex items-center">
                      <div className="flex-shrink-0 h-10 w-10 bg-brand-soft rounded-avatar flex items-center justify-center">
                        <Users className="h-5 w-5 text-brand-text" />
                      </div>
                      <div className="ml-4">
                        <div className="text-sm font-medium text-ink">
                          {agent?.displayName}
                        </div>
                        <div className="text-xs text-ink-secondary">
                          {agent?.name}
                        </div>
                      </div>
                    </div>
                  </td>
                  <td className="px-6 py-4 whitespace-nowrap">
                    <span
                      className={`inline-flex items-center px-2.5 py-0.5 rounded-pill text-xs font-medium ${getAgentTypeDisplay(agent?.agentType).color}`}
                    >
                      {getAgentTypeDisplay(agent?.agentType).label}
                    </span>
                  </td>
                  <td className="px-6 py-4 whitespace-nowrap">
                    <div className="text-sm text-ink">
                      {agent?.version || (
                        <span className="text-ink-tertiary">—</span>
                      )}
                    </div>
                  </td>
                  <td className="px-6 py-4 whitespace-nowrap">
                    <StatusBadge status={agent?.status} />
                  </td>
                  <td className="px-6 py-4 whitespace-nowrap">
                    <div className="w-40">
                      <TrustScoreBar score={agent?.trustScore} />
                    </div>
                  </td>
                  <td className="px-6 py-4 whitespace-nowrap">
                    <div className="text-sm text-ink-secondary">
                      {agent?.updatedAt && formatDate(agent.updatedAt)}
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
                          className="p-1 text-ink-tertiary hover:text-brand-text transition-colors"
                          title="View details"
                        >
                          <Eye className="h-4 w-4" />
                        </button>
                      )}
                      {permissions.canEditAgent && (
                        <button
                          onClick={() => handleEditAgent(agent)}
                          className="p-1 text-ink-tertiary hover:text-warning-text transition-colors"
                          title="Edit agent"
                        >
                          <Edit className="h-4 w-4" />
                        </button>
                      )}
                      {permissions.canDeleteAgent && (
                        <button
                          onClick={() => handleDeleteAgent(agent)}
                          className="p-1 text-ink-tertiary hover:text-danger-text transition-colors"
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
            <Users className="mx-auto h-12 w-12 text-ink-tertiary" />
            <h3 className="mt-2 text-sm font-medium text-ink">
              No agents found
            </h3>
            <p className="mt-1 text-sm text-ink-secondary">
              {searchTerm || statusFilter !== "all"
                ? "Try adjusting your search or filters."
                : "Get started by registering your first agent."}
            </p>
          </div>
        )}
        {/* Pagination Controls */}
        {filteredAgents.length > 0 && (
          <div className="px-6 py-4 border-t border-divider flex items-center justify-between">
            <div className="text-sm text-ink-secondary">
              Showing {paginatedAgents.length} of {filteredAgents.length} agents
            </div>
            <div className="flex gap-2">
              {currentPage > 1 && (
                <button
                  onClick={() => setCurrentPage(1)}
                  className="rounded-pill border border-stroke px-4 py-2 text-sm text-ink-body hover:text-ink hover:bg-glass-inset-gray transition-colors"
                >
                  Show less
                </button>
              )}
              {hasMore && (
                <button
                  onClick={() => setCurrentPage(currentPage + 1)}
                  className="rounded-pill bg-brand px-4 py-2 text-sm text-white shadow-glow hover:bg-brand-hover transition-colors"
                >
                  Load more
                </button>
              )}
            </div>
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
        title="Delete agent"
        message={`Are you sure you want to delete "${selectedAgent?.displayName}"? This action cannot be undone.`}
        confirmText="Delete"
        cancelText="Cancel"
        variant="danger"
        loading={deleteLoading}
        onConfirm={confirmDelete}
        onCancel={() => {
          if (!deleteLoading) {
            setShowDeleteConfirm(false);
            setSelectedAgent(null);
          }
        }}
      />
    </div>
  );
}

export default function AgentsPage() {
  return (
    <AuthGuard>
      <Suspense fallback={<AgentsPageSkeleton />}>
        <AgentsPageContent />
      </Suspense>
    </AuthGuard>
  );
}
