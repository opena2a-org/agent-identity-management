"use client";

import { useEffect, useState, useCallback, useMemo } from "react";
import { useDebounce } from "@/hooks/use-debounce";
import { api } from "@/lib/api";
import {
  Loader2,
  AlertCircle,
  Server,
  Bot,
  Check,
  X,
  Plus,
  Eye,
  RefreshCw,
  Search,
  AlertTriangle,
  Clock,
} from "lucide-react";
import Link from "next/link";

interface DiscoveredMCP {
  name: string;
  url: string;
  detectedBy: string[];
  detectedByCount: number;
  detectionMethod: string;
  firstDetectedAt: string;
  lastDetectedAt: string;
  isRegistered: boolean;
  matchingServerId?: string;
}

interface DiscoveryData {
  discovered: DiscoveredMCP[];
  totalUnmapped: number;
  totalAgents: number;
  registeredServers: number;
}

export default function MCPDiscoveryPage() {
  const [data, setData] = useState<DiscoveryData | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [filter, setFilter] = useState<"all" | "unmapped" | "mapped">("all");
  const [searchQuery, setSearchQuery] = useState("");

  // Debounce search input for better performance
  const debouncedSearchQuery = useDebounce(searchQuery, 300);
  const [currentPage, setCurrentPage] = useState(1);
  const PAGE_SIZE = 20;

  const fetchDiscoveryData = useCallback(async () => {
    try {
      setLoading(true);
      setError(null);
      const result = await api.getDiscoveredMCPs();
      setData(result);
    } catch (err) {
      console.error("Failed to fetch discovery data:", err);
      setError("Failed to load MCP discovery data");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchDiscoveryData();
  }, [fetchDiscoveryData]);

  // Memoized filtered MCPs for better performance
  const filteredMCPs = useMemo(() => {
    return (data?.discovered || []).filter((mcp) => {
      // Apply status filter
      if (filter === "unmapped" && mcp.isRegistered) return false;
      if (filter === "mapped" && !mcp.isRegistered) return false;

      // Apply search filter (using debounced value)
      if (debouncedSearchQuery) {
        const query = debouncedSearchQuery.toLowerCase();
        return (
          mcp.name.toLowerCase().includes(query) ||
          mcp.url?.toLowerCase().includes(query) ||
          mcp.detectedBy.some((agent) => agent.toLowerCase().includes(query))
        );
      }

      return true;
    });
  }, [data?.discovered, filter, debouncedSearchQuery]);

  // Paginated results
  const paginatedMCPs = filteredMCPs.slice(0, currentPage * PAGE_SIZE);
  const hasMore = currentPage * PAGE_SIZE < filteredMCPs.length;

  // Reset page when filters change (use debounced search to prevent premature reset)
  useEffect(() => {
    setCurrentPage(1);
  }, [filter, debouncedSearchQuery]);

  const formatDate = (dateStr: string) => {
    const date = new Date(dateStr);
    return date.toLocaleDateString("en-US", {
      month: "short",
      day: "numeric",
      year: "numeric",
    });
  };

  if (loading) {
    return (
      <div className="flex items-center justify-center min-h-[400px]">
        <div className="flex flex-col items-center gap-2">
          <Loader2 className="h-8 w-8 animate-spin text-brand" />
          <span className="text-sm text-ink-secondary">
            Scanning for discovered MCP servers...
          </span>
        </div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="flex items-center justify-center min-h-[400px]">
        <div className="flex flex-col items-center gap-2">
          <AlertCircle className="h-8 w-8 text-danger" />
          <span className="text-sm text-ink-secondary">
            {error}
          </span>
          <button
            onClick={fetchDiscoveryData}
            className="mt-2 px-3 py-1 text-sm bg-brand text-white rounded-pill shadow-accent hover:bg-brand-hover flex items-center gap-1 transition-colors"
          >
            <RefreshCw className="h-3 w-3" />
            Retry
          </button>
        </div>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold tracking-[-0.03em] text-ink">
            MCP discovery
          </h1>
          <p className="text-sm text-ink-secondary mt-1">
            MCP servers detected through agent connections that may need registration
          </p>
        </div>
        <button
          onClick={fetchDiscoveryData}
          className="flex items-center gap-2 px-4 py-2 text-sm bg-glass-inset-gray text-ink-body rounded-pill hover:bg-glass-inset transition-colors"
        >
          <RefreshCw className="h-4 w-4" />
          Refresh
        </button>
      </div>

      {/* Stats Cards */}
      <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
        <div className="glass p-4">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-2">
              <div className="p-2 bg-warning-fill rounded-inset-sm">
                <AlertTriangle className="h-5 w-5 text-warning-text" />
              </div>
              <span className="text-sm font-medium text-ink-secondary">
                Unmapped MCPs
              </span>
            </div>
            <span className="text-2xl font-bold text-warning-text">
              {data?.totalUnmapped || 0}
            </span>
          </div>
        </div>

        <div className="glass p-4">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-2">
              <div className="p-2 bg-success-fill rounded-inset-sm">
                <Server className="h-5 w-5 text-success-text" />
              </div>
              <span className="text-sm font-medium text-ink-secondary">
                Registered
              </span>
            </div>
            <span className="text-2xl font-bold text-success-text">
              {data?.registeredServers || 0}
            </span>
          </div>
        </div>

        <div className="glass p-4">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-2">
              <div className="p-2 bg-brand-soft rounded-inset-sm">
                <Bot className="h-5 w-5 text-brand-text" />
              </div>
              <span className="text-sm font-medium text-ink-secondary">
                Agents scanned
              </span>
            </div>
            <span className="text-2xl font-bold text-brand-text">
              {data?.totalAgents || 0}
            </span>
          </div>
        </div>

        <div className="glass p-4">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-2">
              <div className="p-2 bg-glass-inset-gray rounded-inset-sm">
                <Search className="h-5 w-5 text-ink-secondary" />
              </div>
              <span className="text-sm font-medium text-ink-secondary">
                Total discovered
              </span>
            </div>
            <span className="text-2xl font-bold text-ink">
              {data?.discovered?.length || 0}
            </span>
          </div>
        </div>
      </div>

      {/* Filters */}
      <div className="flex items-center gap-4 glass p-4">
        <div className="flex items-center gap-2">
          <Search className="h-4 w-4 text-ink-tertiary" />
          <input
            type="text"
            placeholder="Search by name, URL, or agent..."
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            className="bg-transparent border-none outline-none text-sm text-ink placeholder:text-ink-tertiary w-64"
          />
        </div>

        <div className="h-6 w-px bg-stroke" />

        <div className="flex items-center gap-2">
          <span className="text-sm text-ink-secondary">Filter:</span>
          <div className="flex items-center gap-1">
            <button
              onClick={() => setFilter("all")}
              className={`px-3 py-1 text-sm rounded-pill transition-colors ${
                filter === "all"
                  ? "bg-brand text-white shadow-accent"
                  : "bg-glass-inset-gray text-ink-body hover:bg-glass-inset"
              }`}
            >
              All
            </button>
            <button
              onClick={() => setFilter("unmapped")}
              className={`px-3 py-1 text-sm rounded-pill transition-colors ${
                filter === "unmapped"
                  ? "bg-warning-fill text-warning-text"
                  : "bg-glass-inset-gray text-ink-body hover:bg-glass-inset"
              }`}
            >
              Unmapped
            </button>
            <button
              onClick={() => setFilter("mapped")}
              className={`px-3 py-1 text-sm rounded-pill transition-colors ${
                filter === "mapped"
                  ? "bg-success-fill text-success-text"
                  : "bg-glass-inset-gray text-ink-body hover:bg-glass-inset"
              }`}
            >
              Mapped
            </button>
          </div>
        </div>
      </div>

      {/* Discovery Table */}
      <div className="glass overflow-hidden">
        {!paginatedMCPs || filteredMCPs.length === 0 ? (
          <div className="flex flex-col items-center justify-center py-12">
            <Server className="h-12 w-12 text-ink-tertiary mb-4" />
            <p className="text-ink-secondary text-sm font-medium">
              {searchQuery || filter !== "all"
                ? "No MCP servers match your filters"
                : "No MCP servers discovered yet"}
            </p>
            <p className="text-ink-tertiary text-xs mt-1">
              {searchQuery || filter !== "all"
                ? "Try adjusting your search or filter criteria"
                : "MCP references in agent talks_to fields will appear here"}
            </p>
          </div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full">
              <thead className="bg-glass-inset-gray border-b border-divider">
                <tr>
                  <th className="px-4 py-3 text-left text-xs font-medium text-ink-tertiary uppercase tracking-wider">
                    MCP name / URL
                  </th>
                  <th className="px-4 py-3 text-left text-xs font-medium text-ink-tertiary uppercase tracking-wider">
                    Status
                  </th>
                  <th className="px-4 py-3 text-left text-xs font-medium text-ink-tertiary uppercase tracking-wider">
                    Detected by
                  </th>
                  <th className="px-4 py-3 text-left text-xs font-medium text-ink-tertiary uppercase tracking-wider">
                    Detection method
                  </th>
                  <th className="px-4 py-3 text-left text-xs font-medium text-ink-tertiary uppercase tracking-wider">
                    First seen
                  </th>
                  <th className="px-4 py-3 text-right text-xs font-medium text-ink-tertiary uppercase tracking-wider">
                    Actions
                  </th>
                </tr>
              </thead>
              <tbody className="divide-y divide-divider">
                {paginatedMCPs.map((mcp, index) => (
                  <tr
                    key={index}
                    className="hover:bg-glass-inset-gray transition-colors"
                  >
                    <td className="px-4 py-4">
                      <div className="flex items-center gap-3">
                        <div
                          className={`p-2 rounded-inset-sm ${
                            mcp.isRegistered
                              ? "bg-success-fill"
                              : "bg-warning-fill"
                          }`}
                        >
                          <Server
                            className={`h-4 w-4 ${
                              mcp.isRegistered
                                ? "text-success-text"
                                : "text-warning-text"
                            }`}
                          />
                        </div>
                        <div>
                          <p className="text-sm font-medium text-ink">
                            {mcp.name}
                          </p>
                          {mcp.url && (
                            <p className="text-xs text-ink-tertiary truncate max-w-xs">
                              {mcp.url}
                            </p>
                          )}
                        </div>
                      </div>
                    </td>
                    <td className="px-4 py-4">
                      {mcp.isRegistered ? (
                        <span className="inline-flex items-center gap-1 px-2 py-1 rounded-pill text-xs font-medium border border-success-border bg-success-fill text-success-text">
                          <Check className="h-3 w-3" />
                          Registered
                        </span>
                      ) : (
                        <span className="inline-flex items-center gap-1 px-2 py-1 rounded-pill text-xs font-medium border border-warning-border bg-warning-fill text-warning-text">
                          <AlertTriangle className="h-3 w-3" />
                          Unmapped
                        </span>
                      )}
                    </td>
                    <td className="px-4 py-4">
                      <div className="flex items-center gap-2">
                        <div className="flex -space-x-2">
                          {mcp.detectedBy.slice(0, 3).map((agent, i) => (
                            <div
                              key={i}
                              className="w-6 h-6 rounded-full bg-brand flex items-center justify-center text-[10px] text-white font-medium border-2 border-glass-border"
                              title={agent}
                            >
                              {agent.charAt(0).toUpperCase()}
                            </div>
                          ))}
                          {mcp.detectedByCount > 3 && (
                            <div className="w-6 h-6 rounded-full bg-track flex items-center justify-center text-[10px] text-ink-body font-medium border-2 border-glass-border">
                              +{mcp.detectedByCount - 3}
                            </div>
                          )}
                        </div>
                        <span className="text-xs text-ink-tertiary">
                          {mcp.detectedByCount} agent{mcp.detectedByCount !== 1 ? "s" : ""}
                        </span>
                      </div>
                    </td>
                    <td className="px-4 py-4">
                      <span className="text-xs text-ink-body bg-glass-inset-gray px-2 py-1 rounded-pill">
                        {mcp.detectionMethod}
                      </span>
                    </td>
                    <td className="px-4 py-4">
                      <div className="flex items-center gap-1 text-xs text-ink-tertiary">
                        <Clock className="h-3 w-3" />
                        {formatDate(mcp.firstDetectedAt)}
                      </div>
                    </td>
                    <td className="px-4 py-4 text-right">
                      <div className="flex items-center justify-end gap-2">
                        {mcp.isRegistered && mcp.matchingServerId ? (
                          <Link
                            href={`/dashboard/mcp/${mcp.matchingServerId}`}
                            className="inline-flex items-center gap-1 px-3 py-1 text-xs text-brand-text hover:bg-brand-soft rounded-pill transition-colors"
                          >
                            <Eye className="h-3 w-3" />
                            View
                          </Link>
                        ) : (
                          <Link
                            href={`/dashboard/mcp?register=${encodeURIComponent(mcp.name)}`}
                            className="inline-flex items-center gap-1 px-3 py-1 text-xs bg-brand text-white shadow-accent hover:bg-brand-hover rounded-pill transition-colors"
                          >
                            <Plus className="h-3 w-3" />
                            Register
                          </Link>
                        )}
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>

            {/* Pagination Controls */}
            {filteredMCPs.length > 0 && (
              <div className="flex items-center justify-between px-4 py-3 border-t border-divider">
                <div className="text-sm text-ink-secondary">
                  Showing {Math.min(currentPage * PAGE_SIZE, filteredMCPs.length)} of {filteredMCPs.length} servers
                </div>
                <div className="flex items-center gap-2">
                  {currentPage > 1 && (
                    <button
                      onClick={() => setCurrentPage(1)}
                      className="px-3 py-1 text-sm bg-glass-inset-gray text-ink-body rounded-pill hover:bg-glass-inset transition-colors"
                    >
                      Show less
                    </button>
                  )}
                  {hasMore && (
                    <button
                      onClick={() => setCurrentPage(currentPage + 1)}
                      className="px-3 py-1 text-sm bg-brand text-white shadow-accent rounded-pill hover:bg-brand-hover transition-colors"
                    >
                      Load more ({Math.min(PAGE_SIZE, filteredMCPs.length - currentPage * PAGE_SIZE)} more)
                    </button>
                  )}
                </div>
              </div>
            )}
          </div>
        )}
      </div>

      {/* Help Text */}
      <div className="bg-brand-soft rounded-card border border-stroke p-4">
        <div className="flex items-start gap-3">
          <div className="p-2 bg-glass-inset rounded-inset-sm">
            <Server className="h-5 w-5 text-brand-text" />
          </div>
          <div>
            <h3 className="text-sm font-medium text-ink">
              How MCP discovery works
            </h3>
            <p className="text-xs text-ink-body mt-1">
              AIM automatically detects MCP servers referenced in agent <code className="bg-glass-inset text-ink px-1 rounded">talks_to</code> fields.
              Unmapped MCPs represent servers that agents communicate with but haven't been formally registered in your organization.
              Registering these servers enables proper governance, trust scoring, and attestation workflows.
            </p>
          </div>
        </div>
      </div>
    </div>
  );
}
