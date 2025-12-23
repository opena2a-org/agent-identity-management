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
          <Loader2 className="h-8 w-8 animate-spin text-blue-500" />
          <span className="text-sm text-gray-500 dark:text-gray-400">
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
          <AlertCircle className="h-8 w-8 text-red-500" />
          <span className="text-sm text-gray-500 dark:text-gray-400">
            {error}
          </span>
          <button
            onClick={fetchDiscoveryData}
            className="mt-2 px-3 py-1 text-sm bg-blue-600 text-white rounded hover:bg-blue-700 flex items-center gap-1"
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
          <h1 className="text-2xl font-bold text-gray-900 dark:text-gray-100">
            MCP Discovery
          </h1>
          <p className="text-sm text-gray-500 dark:text-gray-400 mt-1">
            MCP servers detected through agent connections that may need registration
          </p>
        </div>
        <button
          onClick={fetchDiscoveryData}
          className="flex items-center gap-2 px-4 py-2 text-sm bg-gray-100 dark:bg-gray-800 text-gray-700 dark:text-gray-300 rounded-lg hover:bg-gray-200 dark:hover:bg-gray-700 transition-colors"
        >
          <RefreshCw className="h-4 w-4" />
          Refresh
        </button>
      </div>

      {/* Stats Cards */}
      <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
        <div className="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-4">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-2">
              <div className="p-2 bg-orange-100 dark:bg-orange-900/30 rounded-lg">
                <AlertTriangle className="h-5 w-5 text-orange-600 dark:text-orange-400" />
              </div>
              <span className="text-sm font-medium text-gray-600 dark:text-gray-400">
                Unmapped MCPs
              </span>
            </div>
            <span className="text-2xl font-bold text-orange-600 dark:text-orange-400">
              {data?.totalUnmapped || 0}
            </span>
          </div>
        </div>

        <div className="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-4">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-2">
              <div className="p-2 bg-green-100 dark:bg-green-900/30 rounded-lg">
                <Server className="h-5 w-5 text-green-600 dark:text-green-400" />
              </div>
              <span className="text-sm font-medium text-gray-600 dark:text-gray-400">
                Registered
              </span>
            </div>
            <span className="text-2xl font-bold text-green-600 dark:text-green-400">
              {data?.registeredServers || 0}
            </span>
          </div>
        </div>

        <div className="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-4">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-2">
              <div className="p-2 bg-blue-100 dark:bg-blue-900/30 rounded-lg">
                <Bot className="h-5 w-5 text-blue-600 dark:text-blue-400" />
              </div>
              <span className="text-sm font-medium text-gray-600 dark:text-gray-400">
                Agents Scanned
              </span>
            </div>
            <span className="text-2xl font-bold text-blue-600 dark:text-blue-400">
              {data?.totalAgents || 0}
            </span>
          </div>
        </div>

        <div className="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-4">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-2">
              <div className="p-2 bg-purple-100 dark:bg-purple-900/30 rounded-lg">
                <Search className="h-5 w-5 text-purple-600 dark:text-purple-400" />
              </div>
              <span className="text-sm font-medium text-gray-600 dark:text-gray-400">
                Total Discovered
              </span>
            </div>
            <span className="text-2xl font-bold text-purple-600 dark:text-purple-400">
              {data?.discovered?.length || 0}
            </span>
          </div>
        </div>
      </div>

      {/* Filters */}
      <div className="flex items-center gap-4 bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-4">
        <div className="flex items-center gap-2">
          <Search className="h-4 w-4 text-gray-400" />
          <input
            type="text"
            placeholder="Search by name, URL, or agent..."
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            className="bg-transparent border-none outline-none text-sm text-gray-900 dark:text-gray-100 placeholder-gray-500 w-64"
          />
        </div>

        <div className="h-6 w-px bg-gray-200 dark:bg-gray-700" />

        <div className="flex items-center gap-2">
          <span className="text-sm text-gray-500 dark:text-gray-400">Filter:</span>
          <div className="flex items-center gap-1">
            <button
              onClick={() => setFilter("all")}
              className={`px-3 py-1 text-sm rounded-md transition-colors ${
                filter === "all"
                  ? "bg-blue-600 text-white"
                  : "bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300 hover:bg-gray-200 dark:hover:bg-gray-600"
              }`}
            >
              All
            </button>
            <button
              onClick={() => setFilter("unmapped")}
              className={`px-3 py-1 text-sm rounded-md transition-colors ${
                filter === "unmapped"
                  ? "bg-orange-600 text-white"
                  : "bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300 hover:bg-gray-200 dark:hover:bg-gray-600"
              }`}
            >
              Unmapped
            </button>
            <button
              onClick={() => setFilter("mapped")}
              className={`px-3 py-1 text-sm rounded-md transition-colors ${
                filter === "mapped"
                  ? "bg-green-600 text-white"
                  : "bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300 hover:bg-gray-200 dark:hover:bg-gray-600"
              }`}
            >
              Mapped
            </button>
          </div>
        </div>
      </div>

      {/* Discovery Table */}
      <div className="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 overflow-hidden">
        {!paginatedMCPs || filteredMCPs.length === 0 ? (
          <div className="flex flex-col items-center justify-center py-12">
            <Server className="h-12 w-12 text-gray-300 dark:text-gray-600 mb-4" />
            <p className="text-gray-500 dark:text-gray-400 text-sm font-medium">
              {searchQuery || filter !== "all"
                ? "No MCP servers match your filters"
                : "No MCP servers discovered yet"}
            </p>
            <p className="text-gray-400 dark:text-gray-500 text-xs mt-1">
              {searchQuery || filter !== "all"
                ? "Try adjusting your search or filter criteria"
                : "MCP references in agent talks_to fields will appear here"}
            </p>
          </div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full">
              <thead className="bg-gray-50 dark:bg-gray-900/50 border-b border-gray-200 dark:border-gray-700">
                <tr>
                  <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                    MCP Name / URL
                  </th>
                  <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                    Status
                  </th>
                  <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                    Detected By
                  </th>
                  <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                    Detection Method
                  </th>
                  <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                    First Seen
                  </th>
                  <th className="px-4 py-3 text-right text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                    Actions
                  </th>
                </tr>
              </thead>
              <tbody className="divide-y divide-gray-200 dark:divide-gray-700">
                {paginatedMCPs.map((mcp, index) => (
                  <tr
                    key={index}
                    className="hover:bg-gray-50 dark:hover:bg-gray-900/30 transition-colors"
                  >
                    <td className="px-4 py-4">
                      <div className="flex items-center gap-3">
                        <div
                          className={`p-2 rounded-lg ${
                            mcp.isRegistered
                              ? "bg-green-100 dark:bg-green-900/30"
                              : "bg-orange-100 dark:bg-orange-900/30"
                          }`}
                        >
                          <Server
                            className={`h-4 w-4 ${
                              mcp.isRegistered
                                ? "text-green-600 dark:text-green-400"
                                : "text-orange-600 dark:text-orange-400"
                            }`}
                          />
                        </div>
                        <div>
                          <p className="text-sm font-medium text-gray-900 dark:text-gray-100">
                            {mcp.name}
                          </p>
                          {mcp.url && (
                            <p className="text-xs text-gray-500 dark:text-gray-400 truncate max-w-xs">
                              {mcp.url}
                            </p>
                          )}
                        </div>
                      </div>
                    </td>
                    <td className="px-4 py-4">
                      {mcp.isRegistered ? (
                        <span className="inline-flex items-center gap-1 px-2 py-1 rounded-full text-xs font-medium bg-green-100 dark:bg-green-900/30 text-green-700 dark:text-green-300">
                          <Check className="h-3 w-3" />
                          Registered
                        </span>
                      ) : (
                        <span className="inline-flex items-center gap-1 px-2 py-1 rounded-full text-xs font-medium bg-orange-100 dark:bg-orange-900/30 text-orange-700 dark:text-orange-300">
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
                              className="w-6 h-6 rounded-full bg-blue-500 flex items-center justify-center text-[10px] text-white font-medium border-2 border-white dark:border-gray-800"
                              title={agent}
                            >
                              {agent.charAt(0).toUpperCase()}
                            </div>
                          ))}
                          {mcp.detectedByCount > 3 && (
                            <div className="w-6 h-6 rounded-full bg-gray-300 dark:bg-gray-600 flex items-center justify-center text-[10px] text-gray-700 dark:text-gray-300 font-medium border-2 border-white dark:border-gray-800">
                              +{mcp.detectedByCount - 3}
                            </div>
                          )}
                        </div>
                        <span className="text-xs text-gray-500 dark:text-gray-400">
                          {mcp.detectedByCount} agent{mcp.detectedByCount !== 1 ? "s" : ""}
                        </span>
                      </div>
                    </td>
                    <td className="px-4 py-4">
                      <span className="text-xs text-gray-600 dark:text-gray-400 bg-gray-100 dark:bg-gray-700 px-2 py-1 rounded">
                        {mcp.detectionMethod}
                      </span>
                    </td>
                    <td className="px-4 py-4">
                      <div className="flex items-center gap-1 text-xs text-gray-500 dark:text-gray-400">
                        <Clock className="h-3 w-3" />
                        {formatDate(mcp.firstDetectedAt)}
                      </div>
                    </td>
                    <td className="px-4 py-4 text-right">
                      <div className="flex items-center justify-end gap-2">
                        {mcp.isRegistered && mcp.matchingServerId ? (
                          <Link
                            href={`/dashboard/mcp/${mcp.matchingServerId}`}
                            className="inline-flex items-center gap-1 px-3 py-1 text-xs text-blue-600 dark:text-blue-400 hover:bg-blue-50 dark:hover:bg-blue-900/30 rounded-md transition-colors"
                          >
                            <Eye className="h-3 w-3" />
                            View
                          </Link>
                        ) : (
                          <Link
                            href={`/dashboard/mcp?register=${encodeURIComponent(mcp.name)}`}
                            className="inline-flex items-center gap-1 px-3 py-1 text-xs bg-blue-600 text-white hover:bg-blue-700 rounded-md transition-colors"
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
              <div className="flex items-center justify-between px-4 py-3 border-t border-gray-200 dark:border-gray-700">
                <div className="text-sm text-gray-500 dark:text-gray-400">
                  Showing {Math.min(currentPage * PAGE_SIZE, filteredMCPs.length)} of {filteredMCPs.length} servers
                </div>
                <div className="flex items-center gap-2">
                  {currentPage > 1 && (
                    <button
                      onClick={() => setCurrentPage(1)}
                      className="px-3 py-1 text-sm bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300 rounded-md hover:bg-gray-200 dark:hover:bg-gray-600 transition-colors"
                    >
                      Show Less
                    </button>
                  )}
                  {hasMore && (
                    <button
                      onClick={() => setCurrentPage(currentPage + 1)}
                      className="px-3 py-1 text-sm bg-blue-600 text-white rounded-md hover:bg-blue-700 transition-colors"
                    >
                      Load More ({Math.min(PAGE_SIZE, filteredMCPs.length - currentPage * PAGE_SIZE)} more)
                    </button>
                  )}
                </div>
              </div>
            )}
          </div>
        )}
      </div>

      {/* Help Text */}
      <div className="bg-blue-50 dark:bg-blue-900/20 rounded-lg border border-blue-200 dark:border-blue-800 p-4">
        <div className="flex items-start gap-3">
          <div className="p-2 bg-blue-100 dark:bg-blue-900/50 rounded-lg">
            <Server className="h-5 w-5 text-blue-600 dark:text-blue-400" />
          </div>
          <div>
            <h3 className="text-sm font-medium text-blue-900 dark:text-blue-100">
              How MCP Discovery Works
            </h3>
            <p className="text-xs text-blue-700 dark:text-blue-300 mt-1">
              AIM automatically detects MCP servers referenced in agent <code className="bg-blue-100 dark:bg-blue-800 px-1 rounded">talks_to</code> fields.
              Unmapped MCPs represent servers that agents communicate with but haven't been formally registered in your organization.
              Registering these servers enables proper governance, trust scoring, and attestation workflows.
            </p>
          </div>
        </div>
      </div>
    </div>
  );
}
