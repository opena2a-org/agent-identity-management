"use client";

import { useState, useEffect, useMemo } from "react";
import { useRouter } from "next/navigation";
import {
  Link2,
  CheckCircle2,
  XCircle,
  Clock,
  Plus,
  Shield,
  RefreshCw,
  Trash2,
  Loader2,
  AlertCircle,
  Globe,
  Eye,
  Search,
  Filter,
  Users,
  FileCheck2,
  Activity,
  BadgeCheck,
  ExternalLink,
} from "lucide-react";
import { api, A2AAgentCard, A2ATrustScore, A2AConsent } from "@/lib/api";
import { useDebounce } from "@/hooks/use-debounce";
import { formatDateTime } from "@/lib/date-utils";
import { getErrorMessage } from "@/lib/error-messages";
import { AuthGuard } from "@/components/auth-guard";
import { ConfirmDialog } from "@/components/modals/confirm-dialog";

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

function StatusBadge({ verified }: { verified: boolean }) {
  if (verified) {
    return (
      <span className="inline-flex items-center gap-1 px-2.5 py-0.5 rounded-full text-xs font-medium bg-green-100 dark:bg-green-900/30 text-green-800 dark:text-green-300">
        <CheckCircle2 className="h-3 w-3" />
        Verified
      </span>
    );
  }
  return (
    <span className="inline-flex items-center gap-1 px-2.5 py-0.5 rounded-full text-xs font-medium bg-yellow-100 dark:bg-yellow-900/30 text-yellow-800 dark:text-yellow-300">
      <Clock className="h-3 w-3" />
      Pending
    </span>
  );
}

function TrustScoreBadge({ score }: { score: number }) {
  const getScoreColor = (score: number) => {
    if (score >= 0.8) return "bg-green-100 dark:bg-green-900/30 text-green-800 dark:text-green-300";
    if (score >= 0.5) return "bg-yellow-100 dark:bg-yellow-900/30 text-yellow-800 dark:text-yellow-300";
    return "bg-red-100 dark:bg-red-900/30 text-red-800 dark:text-red-300";
  };

  return (
    <span className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${getScoreColor(score)}`}>
      {(score * 100).toFixed(0)}%
    </span>
  );
}

function A2APageSkeleton() {
  return (
    <div className="space-y-6">
      {/* Header Skeleton */}
      <div className="flex items-center justify-between">
        <div className="space-y-2">
          <div className="animate-pulse bg-gray-200 dark:bg-gray-700 h-8 w-48 rounded"></div>
          <div className="animate-pulse bg-gray-200 dark:bg-gray-700 h-4 w-96 rounded"></div>
        </div>
        <div className="animate-pulse bg-gray-200 dark:bg-gray-700 h-10 w-40 rounded-lg"></div>
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
                  <div className="animate-pulse bg-gray-200 dark:bg-gray-700 h-8 w-16 rounded"></div>
                </div>
              </div>
            </div>
          </div>
        ))}
      </div>

      {/* Table Skeleton */}
      <div className="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 shadow-sm">
        <div className="overflow-x-auto">
          <table className="min-w-full divide-y divide-gray-200 dark:divide-gray-700">
            <thead className="bg-gray-50 dark:bg-gray-800">
              <tr>
                {[...Array(5)].map((_, i) => (
                  <th key={i} className="px-6 py-3">
                    <div className="animate-pulse bg-gray-200 dark:bg-gray-700 h-4 w-24 rounded"></div>
                  </th>
                ))}
              </tr>
            </thead>
            <tbody className="bg-white dark:bg-gray-900 divide-y divide-gray-200 dark:divide-gray-700">
              {[...Array(5)].map((_, rowIndex) => (
                <tr key={rowIndex}>
                  {[...Array(5)].map((_, colIndex) => (
                    <td key={colIndex} className="px-6 py-4">
                      <div className="animate-pulse bg-gray-200 dark:bg-gray-700 h-4 w-20 rounded"></div>
                    </td>
                  ))}
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
          Failed to Load A2A Data
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

export default function A2AProtocolPage() {
  const router = useRouter();
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [agentCards, setAgentCards] = useState<A2AAgentCard[]>([]);
  const [refreshingId, setRefreshingId] = useState<string | null>(null);

  // Modal state
  const [showDeleteConfirm, setShowDeleteConfirm] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<A2AAgentCard | null>(null);
  const [deleteLoading, setDeleteLoading] = useState(false);

  // Filter state
  const [searchTerm, setSearchTerm] = useState("");
  const [statusFilter, setStatusFilter] = useState<string>("all");

  // Pagination state
  const [currentPage, setCurrentPage] = useState(1);
  const PAGE_SIZE = 20;

  // Debounce search term
  const debouncedSearchTerm = useDebounce(searchTerm, 300);

  const fetchAgentCards = async () => {
    try {
      setLoading(true);
      setError(null);
      const data = await api.listA2AAgentCards();
      setAgentCards(data.cards || []);
    } catch (err) {
      console.error("Failed to fetch A2A agent cards:", err);
      const errorMessage = getErrorMessage(err, {
        resource: "A2A agent cards",
        action: "load",
      });
      setError(errorMessage);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchAgentCards();
  }, []);

  // Reset pagination when filters change
  useEffect(() => {
    setCurrentPage(1);
  }, [debouncedSearchTerm, statusFilter]);

  // Calculate stats
  const stats = {
    total: agentCards.length,
    verified: agentCards.filter((c) => c.verified).length,
    pending: agentCards.filter((c) => !c.verified).length,
    withAttestation: agentCards.filter((c) => c.aimAttestation).length,
  };

  const statCards = [
    {
      name: "Total Agent Cards",
      value: stats.total.toLocaleString(),
      icon: Link2,
    },
    {
      name: "Verified Agents",
      value: stats.verified.toLocaleString(),
      icon: BadgeCheck,
    },
    {
      name: "Pending Verification",
      value: stats.pending.toLocaleString(),
      icon: Clock,
    },
    {
      name: "AIM Attested",
      value: stats.withAttestation.toLocaleString(),
      icon: Shield,
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

  // Filter agent cards
  const filteredCards = useMemo(() => {
    return agentCards.filter((card) => {
      const matchesSearch =
        debouncedSearchTerm === "" ||
        card.name.toLowerCase().includes(debouncedSearchTerm.toLowerCase()) ||
        card.url.toLowerCase().includes(debouncedSearchTerm.toLowerCase()) ||
        card.agentId.toLowerCase().includes(debouncedSearchTerm.toLowerCase());

      const matchesStatus =
        statusFilter === "all" ||
        (statusFilter === "verified" && card.verified) ||
        (statusFilter === "pending" && !card.verified);

      return matchesSearch && matchesStatus;
    });
  }, [agentCards, debouncedSearchTerm, statusFilter]);

  // Pagination
  const paginatedCards = filteredCards.slice(0, currentPage * PAGE_SIZE);
  const hasMore = currentPage * PAGE_SIZE < filteredCards.length;

  // Handlers
  const handleRefreshAttestation = async (card: A2AAgentCard) => {
    setRefreshingId(card.agentId);
    try {
      const updatedCard = await api.refreshA2AAttestation(card.agentId);
      setAgentCards((prev) =>
        prev.map((c) => (c.agentId === card.agentId ? updatedCard : c))
      );
    } catch (err) {
      console.error("Failed to refresh attestation:", err);
      alert("Failed to refresh attestation");
    } finally {
      setRefreshingId(null);
    }
  };

  const requestDeleteCard = (card: A2AAgentCard) => {
    setDeleteTarget(card);
    setShowDeleteConfirm(true);
  };

  const handleDeleteCard = async () => {
    if (!deleteTarget) return;

    setDeleteLoading(true);
    try {
      await api.deleteA2AAgentCard(deleteTarget.agentId);
      setAgentCards((prev) => prev.filter((c) => c.agentId !== deleteTarget.agentId));
      setShowDeleteConfirm(false);
      setDeleteTarget(null);
    } catch (err) {
      console.error("Failed to delete agent card:", err);
      alert("Failed to delete agent card");
    } finally {
      setDeleteLoading(false);
    }
  };

  if (loading) {
    return <A2APageSkeleton />;
  }

  if (error && agentCards.length === 0) {
    return <ErrorDisplay message={error} onRetry={fetchAgentCards} />;
  }

  return (
    <AuthGuard>
      <div className="space-y-6">
        {/* Header */}
        <div className="flex items-center justify-between">
          <div>
            <h1 className="text-2xl font-bold text-gray-900 dark:text-white">
              A2A Protocol
            </h1>
            <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">
              Manage Agent-to-Agent protocol connections, trust relationships, and consent management.
            </p>
          </div>
          <a
            href="https://google.github.io/A2A/"
            target="_blank"
            rel="noopener noreferrer"
            className="flex items-center gap-2 px-4 py-2 text-blue-600 dark:text-blue-400 border border-blue-600 dark:border-blue-400 rounded-lg hover:bg-blue-50 dark:hover:bg-blue-900/20 transition-colors"
          >
            <ExternalLink className="h-4 w-4" />
            A2A Spec
          </a>
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
                placeholder="Search by name, URL, or agent ID..."
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
                <option value="verified">Verified</option>
                <option value="pending">Pending</option>
              </select>
            </div>
          </div>
          {searchTerm && (
            <div className="mt-2 text-sm text-gray-500 dark:text-gray-400">
              Found {filteredCards.length} of {agentCards.length} agent cards
            </div>
          )}
        </div>

        {/* Agent Cards Table */}
        <div className="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 shadow-sm">
          <div className="overflow-x-auto">
            <table className="min-w-full divide-y divide-gray-200 dark:divide-gray-700">
              <thead className="bg-gray-50 dark:bg-gray-800">
                <tr>
                  <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                    Agent
                  </th>
                  <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                    URL
                  </th>
                  <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                    Status
                  </th>
                  <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                    Skills
                  </th>
                  <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                    Actions
                  </th>
                </tr>
              </thead>
              <tbody className="bg-white dark:bg-gray-900 divide-y divide-gray-200 dark:divide-gray-700">
                {paginatedCards.map((card) => (
                  <tr
                    key={card.id}
                    className="hover:bg-gray-50 dark:hover:bg-gray-800 transition-colors"
                  >
                    <td className="px-4 py-3 whitespace-nowrap">
                      <div className="flex items-center">
                        <div className="flex-shrink-0 h-8 w-8 bg-blue-100 dark:bg-blue-900/30 rounded-lg flex items-center justify-center">
                          <Link2 className="h-4 w-4 text-blue-600 dark:text-blue-400" />
                        </div>
                        <div className="ml-3">
                          <div className="text-sm font-medium text-gray-900 dark:text-gray-100">
                            {card.name}
                          </div>
                          <div
                            className="text-xs text-gray-500 dark:text-gray-400"
                            title={card.agentId}
                          >
                            {card.agentId?.substring(0, 8)}...
                          </div>
                        </div>
                      </div>
                    </td>
                    <td className="px-4 py-3">
                      <div className="flex items-center text-sm text-gray-900 dark:text-gray-100">
                        <Globe className="h-3 w-3 mr-1 text-gray-400 flex-shrink-0" />
                        <a
                          href={card.url}
                          target="_blank"
                          rel="noopener noreferrer"
                          className="truncate max-w-[200px] hover:text-blue-600 dark:hover:text-blue-400 hover:underline transition-colors text-xs"
                          title={card.url}
                        >
                          {card.url}
                        </a>
                      </div>
                    </td>
                    <td className="px-4 py-3 whitespace-nowrap">
                      <StatusBadge verified={card.verified} />
                    </td>
                    <td className="px-4 py-3 whitespace-nowrap">
                      <span className="text-sm text-gray-900 dark:text-gray-100">
                        {card.skills?.length || 0} skills
                      </span>
                    </td>
                    <td className="px-4 py-3 whitespace-nowrap">
                      <div className="flex items-center gap-2">
                        <button
                          onClick={() => handleRefreshAttestation(card)}
                          disabled={refreshingId === card.agentId}
                          className="p-1 text-gray-400 hover:text-blue-600 dark:hover:text-blue-400 transition-colors disabled:opacity-50"
                          title="Refresh attestation"
                        >
                          {refreshingId === card.agentId ? (
                            <Loader2 className="h-4 w-4 animate-spin" />
                          ) : (
                            <RefreshCw className="h-4 w-4" />
                          )}
                        </button>
                        <button
                          onClick={() => requestDeleteCard(card)}
                          className="p-1 text-gray-400 hover:text-red-600 dark:hover:text-red-400 transition-colors"
                          title="Delete"
                        >
                          <Trash2 className="h-4 w-4" />
                        </button>
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          {agentCards.length === 0 && (
            <div className="text-center py-12 space-y-6">
              <Link2 className="mx-auto h-12 w-12 text-gray-400" />
              <div>
                <h3 className="text-sm font-medium text-gray-900 dark:text-gray-100">
                  No A2A agent cards registered
                </h3>
                <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">
                  Agent cards will appear here when agents register via the A2A protocol.
                </p>
              </div>
              <div className="max-w-lg mx-auto space-y-3">
                <div className="bg-gray-50 dark:bg-gray-800 rounded-md p-4 text-left">
                  <p className="font-medium text-sm text-gray-900 dark:text-gray-100 mb-1">
                    Register via SDK
                  </p>
                  <p className="text-xs text-gray-500 dark:text-gray-400">
                    Use the AIM SDK to register your agent's A2A card:
                  </p>
                  <code className="text-xs bg-gray-100 dark:bg-gray-700 px-2 py-1 rounded block mt-2 text-gray-700 dark:text-gray-300">
                    a2a.register_agent_card("https://myagent.example.com/.well-known/agent.json")
                  </code>
                </div>
              </div>
            </div>
          )}
          {agentCards.length > 0 && filteredCards.length === 0 && (
            <div className="text-center py-12">
              <Search className="mx-auto h-12 w-12 text-gray-400" />
              <h3 className="mt-2 text-sm font-medium text-gray-900 dark:text-gray-100">
                No agent cards found
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
          {/* Pagination Controls */}
          {filteredCards.length > 0 && (
            <div className="px-6 py-4 border-t border-gray-200 dark:border-gray-700 flex items-center justify-between">
              <div className="text-sm text-gray-500 dark:text-gray-400">
                Showing {paginatedCards.length} of {filteredCards.length} agent cards
              </div>
              <div className="flex gap-2">
                {currentPage > 1 && (
                  <button
                    onClick={() => setCurrentPage(1)}
                    className="px-4 py-2 text-sm text-gray-600 dark:text-gray-300 hover:text-gray-900 dark:hover:text-white border border-gray-300 dark:border-gray-600 rounded-lg hover:bg-gray-50 dark:hover:bg-gray-800 transition-colors"
                  >
                    Show Less
                  </button>
                )}
                {hasMore && (
                  <button
                    onClick={() => setCurrentPage(currentPage + 1)}
                    className="px-4 py-2 text-sm bg-blue-600 text-white rounded-lg hover:bg-blue-700 transition-colors"
                  >
                    Load More
                  </button>
                )}
              </div>
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
                About A2A Protocol
              </h3>
              <p className="mt-2 text-sm text-blue-700 dark:text-blue-300">
                The Agent-to-Agent (A2A) protocol enables secure communication between AI agents.
                Each agent publishes an agent card describing its capabilities, which can be
                cryptographically attested by AIM for enhanced trust. Trust scores track
                interaction history between agent pairs, and consent management ensures
                GDPR/PSD2 compliance for cross-agent data sharing.
              </p>
            </div>
          </div>
        </div>

        {/* Delete Confirmation Modal */}
        <ConfirmDialog
          isOpen={showDeleteConfirm}
          title="Delete Agent Card"
          message={`Are you sure you want to delete the agent card for "${
            deleteTarget?.name ?? "this agent"
          }"? This action cannot be undone.`}
          confirmText="Delete"
          cancelText="Cancel"
          variant="danger"
          loading={deleteLoading}
          onConfirm={handleDeleteCard}
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
