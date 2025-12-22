"use client";

import { useState, useEffect } from "react";
import { useRouter } from "next/navigation";
import {
  Server,
  Shield,
  AlertTriangle,
  TrendingUp,
  Download,
  Activity,
  CheckCircle2,
  XCircle,
  Clock,
  Loader2,
  RefreshCw,
  Box,
  Link2,
  GitBranch,
  Eye,
  AlertCircle,
  ArrowUpRight,
  ArrowDownRight,
  BarChart3,
  Search,
  ChevronLeft,
  ChevronRight,
  Filter,
} from "lucide-react";
import {
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
  PieChart,
  Pie,
  Cell,
  AreaChart,
  Area,
  BarChart,
  Bar,
  Legend,
} from "recharts";
import { api } from "@/lib/api";
import { formatDateTime } from "@/lib/date-utils";
import { AuthGuard } from "@/components/auth-guard";

// Types
interface MCPServer {
  id: string;
  name: string;
  url: string;
  status: string;
  confidenceScore?: number;
  attestationCount?: number;
  lastAttestedAt?: string;
  lastVerifiedAt?: string;
  isVerified?: boolean;
  capabilities?: string[];
  toolCount?: number; // Actual count of tools from mcp_server_capabilities
  createdAt: string;
}

interface AgentMCPConnection {
  id: string;
  agentId: string;
  agentName: string;
  mcpServerId: string;
  mcpServerName: string;
  connectionType: string;
  attestationCount: number;
  lastAttestedAt?: string;
  firstConnectedAt: string;
  isActive: boolean;
}

interface SupplyChainStats {
  totalMCPServers: number;
  verifiedServers: number;
  pendingServers: number;
  totalConnections: number;
  activeConnections: number;
  avgConfidenceScore: number;
  attestationsLast24h: number;
  capabilityDriftAlerts: number;
}

// Colors for charts
const CHART_COLORS = {
  primary: "#3b82f6",
  success: "#22c55e",
  warning: "#f59e0b",
  danger: "#ef4444",
  info: "#06b6d4",
  purple: "#8b5cf6",
};

const CONFIDENCE_COLORS = [
  { range: "90-100%", color: "#22c55e", fill: "#dcfce7" },
  { range: "70-89%", color: "#3b82f6", fill: "#dbeafe" },
  { range: "50-69%", color: "#f59e0b", fill: "#fef3c7" },
  { range: "0-49%", color: "#ef4444", fill: "#fee2e2" },
];

function StatCard({
  icon: Icon,
  label,
  value,
  change,
  changeType,
  suffix,
}: {
  icon: any;
  label: string;
  value: number | string;
  change?: string;
  changeType?: "positive" | "negative" | "neutral";
  suffix?: string;
}) {
  return (
    <div className="bg-white dark:bg-gray-800 p-6 rounded-lg border border-gray-200 dark:border-gray-700 shadow-sm">
      <div className="flex items-center">
        <div className="flex-shrink-0">
          <Icon className="h-6 w-6 text-gray-400" />
        </div>
        <div className="ml-5 w-0 flex-1">
          <dl>
            <dt className="text-sm font-medium text-gray-500 dark:text-gray-400 truncate">
              {label}
            </dt>
            <dd className="flex items-baseline">
              <div className="text-2xl font-semibold text-gray-900 dark:text-gray-100">
                {value}
                {suffix && (
                  <span className="text-lg font-normal text-gray-500 ml-1">
                    {suffix}
                  </span>
                )}
              </div>
              {change && (
                <div
                  className={`ml-2 flex items-baseline text-sm font-semibold ${
                    changeType === "positive"
                      ? "text-green-600"
                      : changeType === "negative"
                      ? "text-red-600"
                      : "text-gray-500"
                  }`}
                >
                  {changeType === "positive" && (
                    <ArrowUpRight className="h-4 w-4 mr-0.5" />
                  )}
                  {changeType === "negative" && (
                    <ArrowDownRight className="h-4 w-4 mr-0.5" />
                  )}
                  {change}
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
      case "verified":
      case "active":
        return "bg-green-100 dark:bg-green-900/30 text-green-800 dark:text-green-300";
      case "pending":
        return "bg-yellow-100 dark:bg-yellow-900/30 text-yellow-800 dark:text-yellow-300";
      case "inactive":
      case "revoked":
        return "bg-red-100 dark:bg-red-900/30 text-red-800 dark:text-red-300";
      default:
        return "bg-gray-100 dark:bg-gray-900/30 text-gray-800 dark:text-gray-300";
    }
  };

  return (
    <span
      className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${getStatusStyles(
        status
      )}`}
    >
      {status}
    </span>
  );
}

function ConfidenceScoreBadge({ score }: { score: number }) {
  const getScoreStyles = (score: number) => {
    if (score >= 90)
      return "bg-green-100 dark:bg-green-900/30 text-green-800 dark:text-green-300";
    if (score >= 70)
      return "bg-blue-100 dark:bg-blue-900/30 text-blue-800 dark:text-blue-300";
    if (score >= 50)
      return "bg-yellow-100 dark:bg-yellow-900/30 text-yellow-800 dark:text-yellow-300";
    return "bg-red-100 dark:bg-red-900/30 text-red-800 dark:text-red-300";
  };

  return (
    <span
      className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${getScoreStyles(
        score
      )}`}
    >
      {score.toFixed(1)}%
    </span>
  );
}

interface CapabilityDriftAlert {
  id: string;
  mcpServerId: string;
  mcpServerName: string;
  driftType: "added" | "removed" | "modified";
  severity: "low" | "medium" | "high";
  capabilityName: string;
  capabilityType: string;
  description: string;
  detectedAt: string;
  previousVerifiedAt?: string;
  isAcknowledged: boolean;
}

function SupplyChainPage() {
  const router = useRouter();
  const [loading, setLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);
  const [stats, setStats] = useState<SupplyChainStats | null>(null);
  const [mcpServers, setMcpServers] = useState<MCPServer[]>([]);
  const [connections, setConnections] = useState<AgentMCPConnection[]>([]);
  const [attestationTrend, setAttestationTrend] = useState<any[]>([]);
  const [confidenceDistribution, setConfidenceDistribution] = useState<any[]>(
    []
  );
  const [driftAlerts, setDriftAlerts] = useState<CapabilityDriftAlert[]>([]);
  const [driftAlertCount, setDriftAlertCount] = useState(0);
  const [unmappedMcpCount, setUnmappedMcpCount] = useState(0);

  // Filters and pagination
  const [mcpSearchFilter, setMcpSearchFilter] = useState("");
  const [mcpStatusFilter, setMcpStatusFilter] = useState<string>("all");
  const [mcpPageSize, setMcpPageSize] = useState(10);
  const [mcpPage, setMcpPage] = useState(1);

  const [connSearchFilter, setConnSearchFilter] = useState("");
  const [connStatusFilter, setConnStatusFilter] = useState<string>("all");
  const [connPageSize, setConnPageSize] = useState(10);
  const [connPage, setConnPage] = useState(1);

  // Drift alerts pagination
  const [driftPageSize, setDriftPageSize] = useState(10);
  const [driftPage, setDriftPage] = useState(1);

  const fetchData = async (showRefreshing = false) => {
    if (showRefreshing) setRefreshing(true);
    try {
      // Fetch MCP servers (for server table and confidence distribution)
      const serversResponse = await api.listMCPServers();
      const servers = serversResponse.mcpServers || [];
      setMcpServers(servers);

      // Fetch supply chain analytics from dedicated backend endpoint (REAL data)
      const analyticsResponse = await api.getSupplyChainAnalytics(7);

      // Set connections from backend (includes agent/MCP names, real timestamps)
      const connectionsList: AgentMCPConnection[] = analyticsResponse.connections.map((conn) => ({
        id: conn.id,
        agentId: conn.agentId,
        agentName: conn.agentName,
        mcpServerId: conn.mcpServerId,
        mcpServerName: conn.mcpServerName,
        connectionType: conn.connectionType,
        attestationCount: conn.attestationCount,
        lastAttestedAt: conn.lastAttestedAt || undefined,
        firstConnectedAt: conn.firstConnectedAt,
        isActive: conn.isActive,
      }));
      setConnections(connectionsList);

      // Calculate statistics from backend stats + server data
      const verifiedCount = servers.filter(
        (s: MCPServer) => s.status === "verified"
      ).length;
      const pendingCount = servers.filter(
        (s: MCPServer) => s.status === "pending"
      ).length;

      // Calculate average confidence score from servers
      const scoresWithData = servers.filter(
        (s: MCPServer) => (s.confidenceScore || 0) > 0
      );
      const avgScore =
        scoresWithData.length > 0
          ? scoresWithData.reduce(
              (sum: number, s: MCPServer) => sum + (s.confidenceScore || 0),
              0
            ) / scoresWithData.length
          : 0;

      // Merge backend stats with server-derived stats (drift alerts set later)
      setStats({
        totalMCPServers: servers.length,
        verifiedServers: verifiedCount,
        pendingServers: pendingCount,
        totalConnections: analyticsResponse.stats.totalConnections,
        activeConnections: analyticsResponse.stats.activeConnections,
        avgConfidenceScore: avgScore,
        attestationsLast24h: analyticsResponse.stats.attestationsLast24h,
        capabilityDriftAlerts: 0, // Updated below after drift alerts fetch
      });

      // Calculate confidence distribution from servers
      const distribution = [
        {
          name: "High (90-100%)",
          value: servers.filter((s: MCPServer) => (s.confidenceScore || 0) >= 90)
            .length,
          color: CHART_COLORS.success,
        },
        {
          name: "Good (70-89%)",
          value: servers.filter(
            (s: MCPServer) =>
              (s.confidenceScore || 0) >= 70 && (s.confidenceScore || 0) < 90
          ).length,
          color: CHART_COLORS.primary,
        },
        {
          name: "Medium (50-69%)",
          value: servers.filter(
            (s: MCPServer) =>
              (s.confidenceScore || 0) >= 50 && (s.confidenceScore || 0) < 70
          ).length,
          color: CHART_COLORS.warning,
        },
        {
          name: "Low (0-49%)",
          value: servers.filter(
            (s: MCPServer) =>
              (s.confidenceScore || 0) < 50
          ).length,
          color: CHART_COLORS.danger,
        },
      ];
      setConfidenceDistribution(distribution);

      // Set attestation trend from backend (REAL data, not mock!)
      const trendData = analyticsResponse.attestationTrend.map((entry) => ({
        date: entry.date,
        attestations: entry.attestationCount,
        newServers: entry.newConnections,
      }));
      setAttestationTrend(trendData);

      // Fetch capability drift alerts
      let driftCount = 0;
      try {
        const driftResponse = await api.getCapabilityDriftAlerts(7);
        setDriftAlerts(driftResponse.alerts || []);
        driftCount = driftResponse.stats?.totalAlerts || 0;
        setDriftAlertCount(driftCount);
      } catch (driftError) {
        console.error("Failed to fetch drift alerts:", driftError);
        setDriftAlerts([]);
        setDriftAlertCount(0);
      }

      // Update stats with drift alert count
      setStats((prev) => prev ? { ...prev, capabilityDriftAlerts: driftCount } : null);

      // Fetch unmapped MCP count from discovery endpoint
      try {
        const discoveryResponse = await api.getDiscoveredMCPs();
        setUnmappedMcpCount(discoveryResponse.totalUnmapped || 0);
      } catch (discoveryError) {
        console.error("Failed to fetch discovered MCPs:", discoveryError);
        setUnmappedMcpCount(0);
      }
    } catch (error) {
      console.error("Failed to fetch supply chain data:", error);
    } finally {
      setLoading(false);
      setRefreshing(false);
    }
  };

  useEffect(() => {
    fetchData();
  }, []);

  // Filtered and paginated MCP servers
  const filteredMcpServers = mcpServers.filter((server) => {
    const matchesSearch = mcpSearchFilter === "" ||
      server.name.toLowerCase().includes(mcpSearchFilter.toLowerCase()) ||
      server.url.toLowerCase().includes(mcpSearchFilter.toLowerCase());
    const matchesStatus = mcpStatusFilter === "all" || server.status === mcpStatusFilter;
    return matchesSearch && matchesStatus;
  });
  const totalMcpPages = Math.ceil(filteredMcpServers.length / mcpPageSize);
  const paginatedMcpServers = filteredMcpServers.slice(
    (mcpPage - 1) * mcpPageSize,
    mcpPage * mcpPageSize
  );

  // Filtered and paginated connections
  const filteredConnections = connections.filter((conn) => {
    const matchesSearch = connSearchFilter === "" ||
      conn.agentName.toLowerCase().includes(connSearchFilter.toLowerCase()) ||
      conn.mcpServerName.toLowerCase().includes(connSearchFilter.toLowerCase());
    const matchesStatus = connStatusFilter === "all" ||
      (connStatusFilter === "active" && conn.isActive) ||
      (connStatusFilter === "inactive" && !conn.isActive);
    return matchesSearch && matchesStatus;
  });
  const totalConnPages = Math.ceil(filteredConnections.length / connPageSize);
  const paginatedConnections = filteredConnections.slice(
    (connPage - 1) * connPageSize,
    connPage * connPageSize
  );

  // Paginated drift alerts
  const totalDriftPages = Math.ceil(driftAlerts.length / driftPageSize);
  const paginatedDriftAlerts = driftAlerts.slice(
    (driftPage - 1) * driftPageSize,
    driftPage * driftPageSize
  );

  const handleExport = () => {
    // Generate CSV export
    const csvData = mcpServers.map((server) => ({
      Name: server.name,
      URL: server.url,
      Status: server.status,
      "Confidence Score": server.confidenceScore,
      "Attestation Count": server.attestationCount,
      "Last Attested": server.lastAttestedAt || "N/A",
    }));

    const headers = Object.keys(csvData[0] || {}).join(",");
    const rows = csvData.map((row) => Object.values(row).join(",")).join("\n");
    const csv = `${headers}\n${rows}`;

    const blob = new Blob([csv], { type: "text/csv" });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = `mcp-supply-chain-${new Date().toISOString().split("T")[0]}.csv`;
    a.click();
    URL.revokeObjectURL(url);
  };

  if (loading) {
    return (
      <div className="min-h-screen flex items-center justify-center">
        <div className="text-center">
          <Loader2 className="h-8 w-8 animate-spin text-blue-500 mx-auto mb-4" />
          <p className="text-gray-500 dark:text-gray-400">
            Loading supply chain data...
          </p>
        </div>
      </div>
    );
  }

  return (
    <div className="min-h-screen bg-gray-50 dark:bg-gray-900 p-6">
      {/* Header */}
      <div className="mb-8">
        <div className="flex items-center justify-between">
          <div>
            <h1 className="text-3xl font-bold text-gray-900 dark:text-white flex items-center gap-3">
              <GitBranch className="h-8 w-8 text-blue-500" />
              MCP Supply Chain Analytics
            </h1>
            <p className="mt-2 text-gray-600 dark:text-gray-400">
              Monitor MCP server dependencies, attestation health, and supply
              chain security across your organization.
            </p>
          </div>
          <div className="flex gap-3">
            <button
              onClick={() => fetchData(true)}
              disabled={refreshing}
              className="inline-flex items-center px-4 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm text-sm font-medium text-gray-700 dark:text-gray-200 bg-white dark:bg-gray-800 hover:bg-gray-50 dark:hover:bg-gray-700 disabled:opacity-50"
            >
              <RefreshCw
                className={`h-4 w-4 mr-2 ${refreshing ? "animate-spin" : ""}`}
              />
              Refresh
            </button>
            <button
              onClick={handleExport}
              className="inline-flex items-center px-4 py-2 border border-transparent rounded-md shadow-sm text-sm font-medium text-white bg-blue-600 hover:bg-blue-700"
            >
              <Download className="h-4 w-4 mr-2" />
              Export Report
            </button>
          </div>
        </div>
      </div>

      {/* Stats Grid */}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4 mb-8">
        <StatCard
          icon={Server}
          label="Total MCP Servers"
          value={stats?.totalMCPServers || 0}
        />
        <StatCard
          icon={Shield}
          label="Verified Servers"
          value={stats?.verifiedServers || 0}
          change={
            stats && stats.totalMCPServers > 0
              ? `${((stats.verifiedServers / stats.totalMCPServers) * 100).toFixed(0)}%`
              : undefined
          }
          changeType="positive"
        />
        <StatCard
          icon={Link2}
          label="Active Connections"
          value={stats?.activeConnections || 0}
          suffix={`/ ${stats?.totalConnections || 0}`}
        />
        <StatCard
          icon={AlertCircle}
          label="Unmapped MCPs"
          value={unmappedMcpCount}
          changeType={
            unmappedMcpCount > 0 ? "negative" : "positive"
          }
        />
      </div>

      {/* Secondary Stats */}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4 mb-8">
        <StatCard
          icon={Clock}
          label="Pending Verification"
          value={stats?.pendingServers || 0}
          changeType={
            (stats?.pendingServers || 0) > 0 ? "negative" : "positive"
          }
        />
        <StatCard
          icon={Activity}
          label="Attestations (24h)"
          value={stats?.attestationsLast24h || 0}
        />
        <StatCard
          icon={AlertTriangle}
          label="Capability Drift Alerts"
          value={stats?.capabilityDriftAlerts || 0}
          changeType={
            (stats?.capabilityDriftAlerts || 0) > 0 ? "negative" : "positive"
          }
        />
        <StatCard
          icon={Box}
          label="Unique Capabilities"
          value={mcpServers.reduce(
            (sum, s) => sum + (s.toolCount || 0),
            0
          )}
        />
      </div>

      {/* Charts Row */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6 mb-8">
        {/* Attestation Trend Chart */}
        <div className="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-6">
          <h3 className="text-lg font-semibold text-gray-900 dark:text-white mb-4 flex items-center gap-2">
            <BarChart3 className="h-5 w-5 text-blue-500" />
            Attestation Activity (7 Days)
          </h3>
          <div className="h-64">
            <ResponsiveContainer width="100%" height="100%">
              <AreaChart data={attestationTrend}>
                <defs>
                  <linearGradient
                    id="attestationGradient"
                    x1="0"
                    y1="0"
                    x2="0"
                    y2="1"
                  >
                    <stop
                      offset="5%"
                      stopColor={CHART_COLORS.primary}
                      stopOpacity={0.3}
                    />
                    <stop
                      offset="95%"
                      stopColor={CHART_COLORS.primary}
                      stopOpacity={0}
                    />
                  </linearGradient>
                </defs>
                <CartesianGrid
                  strokeDasharray="3 3"
                  className="stroke-gray-200 dark:stroke-gray-700"
                />
                <XAxis
                  dataKey="date"
                  tick={{ fill: "#9ca3af", fontSize: 12 }}
                />
                <YAxis tick={{ fill: "#9ca3af", fontSize: 12 }} />
                <Tooltip
                  contentStyle={{
                    backgroundColor: "#1f2937",
                    border: "none",
                    borderRadius: "8px",
                    color: "#fff",
                  }}
                />
                <Area
                  type="monotone"
                  dataKey="attestations"
                  stroke={CHART_COLORS.primary}
                  fill="url(#attestationGradient)"
                  strokeWidth={2}
                  name="Attestations"
                />
              </AreaChart>
            </ResponsiveContainer>
          </div>
        </div>

        {/* Confidence Distribution Chart */}
        <div className="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-6">
          <h3 className="text-lg font-semibold text-gray-900 dark:text-white mb-4 flex items-center gap-2">
            <Shield className="h-5 w-5 text-green-500" />
            Confidence Score Distribution
          </h3>
          <div className="h-64">
            <ResponsiveContainer width="100%" height="100%">
              <PieChart>
                <Pie
                  data={confidenceDistribution}
                  cx="50%"
                  cy="50%"
                  innerRadius={60}
                  outerRadius={80}
                  paddingAngle={5}
                  dataKey="value"
                  label={({ name, value }) => (value > 0 ? `${value}` : "")}
                >
                  {confidenceDistribution.map((entry, index) => (
                    <Cell key={`cell-${index}`} fill={entry.color} />
                  ))}
                </Pie>
                <Tooltip
                  contentStyle={{
                    backgroundColor: "#1f2937",
                    border: "none",
                    borderRadius: "8px",
                    color: "#fff",
                  }}
                />
                <Legend
                  wrapperStyle={{ paddingTop: "20px" }}
                  formatter={(value) => (
                    <span className="text-gray-600 dark:text-gray-300 text-sm">
                      {value}
                    </span>
                  )}
                />
              </PieChart>
            </ResponsiveContainer>
          </div>
        </div>
      </div>

      {/* MCP Servers Table */}
      <div className="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 overflow-hidden mb-8">
        <div className="px-6 py-4 border-b border-gray-200 dark:border-gray-700">
          <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4">
            <div>
              <h3 className="text-lg font-semibold text-gray-900 dark:text-white flex items-center gap-2">
                <Server className="h-5 w-5 text-purple-500" />
                MCP Server Dependencies
                <span className="ml-2 px-2 py-0.5 text-xs font-medium bg-gray-100 dark:bg-gray-700 text-gray-600 dark:text-gray-300 rounded-full">
                  {filteredMcpServers.length}
                </span>
              </h3>
              <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">
                All MCP servers in your supply chain with their attestation status
              </p>
            </div>
            <div className="flex flex-col sm:flex-row gap-2">
              <div className="relative">
                <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-gray-400" />
                <input
                  type="text"
                  placeholder="Search servers..."
                  value={mcpSearchFilter}
                  onChange={(e) => {
                    setMcpSearchFilter(e.target.value);
                    setMcpPage(1);
                  }}
                  className="pl-9 pr-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-white placeholder-gray-400 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent w-full sm:w-48"
                />
              </div>
              <select
                value={mcpStatusFilter}
                onChange={(e) => {
                  setMcpStatusFilter(e.target.value);
                  setMcpPage(1);
                }}
                className="px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-white focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent"
              >
                <option value="all">All Status</option>
                <option value="verified">Verified</option>
                <option value="pending">Pending</option>
                <option value="active">Active</option>
                <option value="inactive">Inactive</option>
              </select>
            </div>
          </div>
        </div>
        <div className="overflow-x-auto">
          <table className="min-w-full divide-y divide-gray-200 dark:divide-gray-700">
            <thead className="bg-gray-50 dark:bg-gray-900">
              <tr>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                  Server
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                  Status
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                  Attestations
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                  Capabilities
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                  Last Attested
                </th>
                <th className="px-6 py-3 text-right text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                  Actions
                </th>
              </tr>
            </thead>
            <tbody className="bg-white dark:bg-gray-800 divide-y divide-gray-200 dark:divide-gray-700">
              {paginatedMcpServers.length === 0 ? (
                <tr>
                  <td
                    colSpan={6}
                    className="px-6 py-12 text-center text-gray-500 dark:text-gray-400"
                  >
                    <Server className="h-12 w-12 mx-auto mb-4 opacity-50" />
                    <p className="text-lg font-medium">
                      {mcpSearchFilter || mcpStatusFilter !== "all"
                        ? "No servers match your filters"
                        : "No MCP servers found"}
                    </p>
                    <p className="text-sm mt-1">
                      {mcpSearchFilter || mcpStatusFilter !== "all"
                        ? "Try adjusting your search or filter criteria"
                        : "Register MCP servers to start building your supply chain"}
                    </p>
                  </td>
                </tr>
              ) : (
                paginatedMcpServers.map((server) => (
                  <tr
                    key={server.id}
                    className="hover:bg-gray-50 dark:hover:bg-gray-700/50"
                  >
                    <td className="px-6 py-4 whitespace-nowrap">
                      <div className="flex items-center">
                        <Server className="h-5 w-5 text-gray-400 mr-3" />
                        <div>
                          <div className="text-sm font-medium text-gray-900 dark:text-white">
                            {server.name}
                          </div>
                          <div className="text-sm text-gray-500 dark:text-gray-400 truncate max-w-xs">
                            {server.url}
                          </div>
                        </div>
                      </div>
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap">
                      <StatusBadge status={server.status} />
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-900 dark:text-white">
                      {server.attestationCount || 0}
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500 dark:text-gray-400">
                      {server.toolCount || 0} tools
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500 dark:text-gray-400">
                      {server.lastAttestedAt || server.lastVerifiedAt
                        ? formatDateTime(server.lastAttestedAt || server.lastVerifiedAt || "")
                        : "Never"}
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap text-right text-sm font-medium">
                      <button
                        onClick={() =>
                          router.push(`/dashboard/mcp/${server.id}`)
                        }
                        className="text-blue-600 dark:text-blue-400 hover:text-blue-900 dark:hover:text-blue-300"
                      >
                        <Eye className="h-4 w-4" />
                      </button>
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
        {/* MCP Servers Pagination */}
        {filteredMcpServers.length > 0 && (
          <div className="px-6 py-4 border-t border-gray-200 dark:border-gray-700 flex flex-col sm:flex-row items-center justify-between gap-4">
            <div className="flex items-center gap-2 text-sm text-gray-500 dark:text-gray-400">
              <span>Show</span>
              <select
                value={mcpPageSize}
                onChange={(e) => {
                  setMcpPageSize(Number(e.target.value));
                  setMcpPage(1);
                }}
                className="px-2 py-1 border border-gray-300 dark:border-gray-600 rounded bg-white dark:bg-gray-700 text-gray-900 dark:text-white text-sm"
              >
                <option value={5}>5</option>
                <option value={10}>10</option>
                <option value={25}>25</option>
                <option value={50}>50</option>
              </select>
              <span>of {filteredMcpServers.length} servers</span>
            </div>
            <div className="flex items-center gap-2">
              <button
                onClick={() => setMcpPage((p) => Math.max(1, p - 1))}
                disabled={mcpPage === 1}
                className="p-2 border border-gray-300 dark:border-gray-600 rounded bg-white dark:bg-gray-700 text-gray-700 dark:text-gray-200 hover:bg-gray-50 dark:hover:bg-gray-600 disabled:opacity-50 disabled:cursor-not-allowed"
              >
                <ChevronLeft className="h-4 w-4" />
              </button>
              <span className="text-sm text-gray-700 dark:text-gray-300">
                Page {mcpPage} of {totalMcpPages || 1}
              </span>
              <button
                onClick={() => setMcpPage((p) => Math.min(totalMcpPages, p + 1))}
                disabled={mcpPage >= totalMcpPages}
                className="p-2 border border-gray-300 dark:border-gray-600 rounded bg-white dark:bg-gray-700 text-gray-700 dark:text-gray-200 hover:bg-gray-50 dark:hover:bg-gray-600 disabled:opacity-50 disabled:cursor-not-allowed"
              >
                <ChevronRight className="h-4 w-4" />
              </button>
            </div>
          </div>
        )}
      </div>

      {/* Agent Connections Table */}
      <div className="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 overflow-hidden">
        <div className="px-6 py-4 border-b border-gray-200 dark:border-gray-700">
          <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4">
            <div>
              <h3 className="text-lg font-semibold text-gray-900 dark:text-white flex items-center gap-2">
                <Link2 className="h-5 w-5 text-cyan-500" />
                Agent-MCP Connections
                <span className="ml-2 px-2 py-0.5 text-xs font-medium bg-gray-100 dark:bg-gray-700 text-gray-600 dark:text-gray-300 rounded-full">
                  {filteredConnections.length}
                </span>
              </h3>
              <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">
                Active connections between agents and MCP servers
              </p>
            </div>
            <div className="flex flex-col sm:flex-row gap-2">
              <div className="relative">
                <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-gray-400" />
                <input
                  type="text"
                  placeholder="Search agents or servers..."
                  value={connSearchFilter}
                  onChange={(e) => {
                    setConnSearchFilter(e.target.value);
                    setConnPage(1);
                  }}
                  className="pl-9 pr-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-white placeholder-gray-400 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent w-full sm:w-56"
                />
              </div>
              <select
                value={connStatusFilter}
                onChange={(e) => {
                  setConnStatusFilter(e.target.value);
                  setConnPage(1);
                }}
                className="px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-white focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent"
              >
                <option value="all">All Status</option>
                <option value="active">Active</option>
                <option value="inactive">Inactive</option>
              </select>
            </div>
          </div>
        </div>
        <div className="overflow-x-auto">
          <table className="min-w-full divide-y divide-gray-200 dark:divide-gray-700">
            <thead className="bg-gray-50 dark:bg-gray-900">
              <tr>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                  Agent
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                  MCP Server
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                  Connection Type
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                  Attestations
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                  First Connected
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                  Last Attested
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                  Status
                </th>
              </tr>
            </thead>
            <tbody className="bg-white dark:bg-gray-800 divide-y divide-gray-200 dark:divide-gray-700">
              {paginatedConnections.length === 0 ? (
                <tr>
                  <td
                    colSpan={7}
                    className="px-6 py-12 text-center text-gray-500 dark:text-gray-400"
                  >
                    <Link2 className="h-12 w-12 mx-auto mb-4 opacity-50" />
                    <p className="text-lg font-medium">
                      {connSearchFilter || connStatusFilter !== "all"
                        ? "No connections match your filters"
                        : "No connections found"}
                    </p>
                    <p className="text-sm mt-1">
                      {connSearchFilter || connStatusFilter !== "all"
                        ? "Try adjusting your search or filter criteria"
                        : "Agent-MCP connections will appear here when agents start using MCP tools"}
                    </p>
                  </td>
                </tr>
              ) : (
                paginatedConnections.map((conn) => (
                  <tr
                    key={conn.id}
                    className="hover:bg-gray-50 dark:hover:bg-gray-700/50"
                  >
                    <td className="px-6 py-4 whitespace-nowrap">
                      <div className="text-sm font-medium text-gray-900 dark:text-white">
                        {conn.agentName || conn.agentId.slice(0, 8)}
                      </div>
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap">
                      <div className="text-sm text-gray-900 dark:text-white">
                        {conn.mcpServerName || conn.mcpServerId.slice(0, 8)}
                      </div>
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap">
                      <span
                        className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${
                          conn.connectionType === "attested"
                            ? "bg-green-100 dark:bg-green-900/30 text-green-800 dark:text-green-300"
                            : "bg-gray-100 dark:bg-gray-900/30 text-gray-800 dark:text-gray-300"
                        }`}
                      >
                        {conn.connectionType}
                      </span>
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-900 dark:text-white">
                      {conn.attestationCount}
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500 dark:text-gray-400">
                      {formatDateTime(conn.firstConnectedAt)}
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500 dark:text-gray-400">
                      {conn.lastAttestedAt
                        ? formatDateTime(conn.lastAttestedAt)
                        : "Never"}
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap">
                      {conn.isActive ? (
                        <span className="inline-flex items-center text-green-600 dark:text-green-400">
                          <CheckCircle2 className="h-4 w-4 mr-1" />
                          Active
                        </span>
                      ) : (
                        <span className="inline-flex items-center text-gray-500 dark:text-gray-400">
                          <XCircle className="h-4 w-4 mr-1" />
                          Inactive
                        </span>
                      )}
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
        {/* Connections Pagination */}
        {filteredConnections.length > 0 && (
          <div className="px-6 py-4 border-t border-gray-200 dark:border-gray-700 flex flex-col sm:flex-row items-center justify-between gap-4">
            <div className="flex items-center gap-2 text-sm text-gray-500 dark:text-gray-400">
              <span>Show</span>
              <select
                value={connPageSize}
                onChange={(e) => {
                  setConnPageSize(Number(e.target.value));
                  setConnPage(1);
                }}
                className="px-2 py-1 border border-gray-300 dark:border-gray-600 rounded bg-white dark:bg-gray-700 text-gray-900 dark:text-white text-sm"
              >
                <option value={5}>5</option>
                <option value={10}>10</option>
                <option value={25}>25</option>
                <option value={50}>50</option>
              </select>
              <span>of {filteredConnections.length} connections</span>
            </div>
            <div className="flex items-center gap-2">
              <button
                onClick={() => setConnPage((p) => Math.max(1, p - 1))}
                disabled={connPage === 1}
                className="p-2 border border-gray-300 dark:border-gray-600 rounded bg-white dark:bg-gray-700 text-gray-700 dark:text-gray-200 hover:bg-gray-50 dark:hover:bg-gray-600 disabled:opacity-50 disabled:cursor-not-allowed"
              >
                <ChevronLeft className="h-4 w-4" />
              </button>
              <span className="text-sm text-gray-700 dark:text-gray-300">
                Page {connPage} of {totalConnPages || 1}
              </span>
              <button
                onClick={() => setConnPage((p) => Math.min(totalConnPages, p + 1))}
                disabled={connPage >= totalConnPages}
                className="p-2 border border-gray-300 dark:border-gray-600 rounded bg-white dark:bg-gray-700 text-gray-700 dark:text-gray-200 hover:bg-gray-50 dark:hover:bg-gray-600 disabled:opacity-50 disabled:cursor-not-allowed"
              >
                <ChevronRight className="h-4 w-4" />
              </button>
            </div>
          </div>
        )}
      </div>

      {/* Capability Drift Alerts */}
      {driftAlerts.length > 0 && (
        <div className="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 overflow-hidden mt-8">
          <div className="px-6 py-4 border-b border-gray-200 dark:border-gray-700">
            <h3 className="text-lg font-semibold text-gray-900 dark:text-white flex items-center gap-2">
              <AlertTriangle className="h-5 w-5 text-amber-500" />
              Capability Drift Alerts
              <span className="ml-2 px-2 py-0.5 text-xs font-medium bg-amber-100 dark:bg-amber-900/30 text-amber-800 dark:text-amber-300 rounded-full">
                {driftAlertCount}
              </span>
            </h3>
            <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">
              Detected changes in MCP server capabilities that may require attention
            </p>
          </div>
          <div className="overflow-x-auto">
            <table className="min-w-full divide-y divide-gray-200 dark:divide-gray-700">
              <thead className="bg-gray-50 dark:bg-gray-900">
                <tr>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                    Severity
                  </th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                    MCP Server
                  </th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                    Capability
                  </th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                    Change Type
                  </th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                    Description
                  </th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                    Detected
                  </th>
                </tr>
              </thead>
              <tbody className="bg-white dark:bg-gray-800 divide-y divide-gray-200 dark:divide-gray-700">
                {paginatedDriftAlerts.map((alert) => (
                  <tr
                    key={alert.id}
                    className="hover:bg-gray-50 dark:hover:bg-gray-700/50"
                  >
                    <td className="px-6 py-4 whitespace-nowrap">
                      <span
                        className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${
                          alert.severity === "high"
                            ? "bg-red-100 dark:bg-red-900/30 text-red-800 dark:text-red-300"
                            : alert.severity === "medium"
                            ? "bg-yellow-100 dark:bg-yellow-900/30 text-yellow-800 dark:text-yellow-300"
                            : "bg-blue-100 dark:bg-blue-900/30 text-blue-800 dark:text-blue-300"
                        }`}
                      >
                        {alert.severity === "high" && (
                          <AlertCircle className="h-3 w-3 mr-1" />
                        )}
                        {alert.severity}
                      </span>
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap">
                      <div className="text-sm font-medium text-gray-900 dark:text-white">
                        {alert.mcpServerName}
                      </div>
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap">
                      <div className="text-sm text-gray-900 dark:text-white">
                        {alert.capabilityName}
                      </div>
                      <div className="text-xs text-gray-500 dark:text-gray-400">
                        {alert.capabilityType}
                      </div>
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap">
                      <span
                        className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${
                          alert.driftType === "added"
                            ? "bg-green-100 dark:bg-green-900/30 text-green-800 dark:text-green-300"
                            : alert.driftType === "removed"
                            ? "bg-red-100 dark:bg-red-900/30 text-red-800 dark:text-red-300"
                            : "bg-yellow-100 dark:bg-yellow-900/30 text-yellow-800 dark:text-yellow-300"
                        }`}
                      >
                        {alert.driftType}
                      </span>
                    </td>
                    <td className="px-6 py-4">
                      <div
                        className="text-sm text-gray-500 dark:text-gray-400 max-w-md truncate"
                        title={alert.description}
                      >
                        {alert.description}
                      </div>
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500 dark:text-gray-400">
                      {formatDateTime(alert.detectedAt)}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          {/* Drift Alerts Pagination */}
          {driftAlerts.length > 0 && (
            <div className="px-6 py-4 border-t border-gray-200 dark:border-gray-700 flex flex-col sm:flex-row items-center justify-between gap-4">
              <div className="flex items-center gap-2 text-sm text-gray-500 dark:text-gray-400">
                <span>Show</span>
                <select
                  value={driftPageSize}
                  onChange={(e) => {
                    setDriftPageSize(Number(e.target.value));
                    setDriftPage(1);
                  }}
                  className="px-2 py-1 border border-gray-300 dark:border-gray-600 rounded bg-white dark:bg-gray-700 text-gray-900 dark:text-white text-sm"
                >
                  <option value={5}>5</option>
                  <option value={10}>10</option>
                  <option value={25}>25</option>
                  <option value={50}>50</option>
                </select>
                <span>of {driftAlerts.length} alerts</span>
              </div>
              <div className="flex items-center gap-2">
                <button
                  onClick={() => setDriftPage((p) => Math.max(1, p - 1))}
                  disabled={driftPage === 1}
                  className="p-2 border border-gray-300 dark:border-gray-600 rounded bg-white dark:bg-gray-700 text-gray-700 dark:text-gray-200 hover:bg-gray-50 dark:hover:bg-gray-600 disabled:opacity-50 disabled:cursor-not-allowed"
                >
                  <ChevronLeft className="h-4 w-4" />
                </button>
                <span className="text-sm text-gray-700 dark:text-gray-300">
                  Page {driftPage} of {totalDriftPages || 1}
                </span>
                <button
                  onClick={() => setDriftPage((p) => Math.min(totalDriftPages, p + 1))}
                  disabled={driftPage >= totalDriftPages}
                  className="p-2 border border-gray-300 dark:border-gray-600 rounded bg-white dark:bg-gray-700 text-gray-700 dark:text-gray-200 hover:bg-gray-50 dark:hover:bg-gray-600 disabled:opacity-50 disabled:cursor-not-allowed"
                >
                  <ChevronRight className="h-4 w-4" />
                </button>
              </div>
            </div>
          )}
        </div>
      )}
    </div>
  );
}

export default function SupplyChainPageWrapper() {
  return (
    <AuthGuard>
      <SupplyChainPage />
    </AuthGuard>
  );
}
