"use client";

import { useState, useEffect, useMemo, Suspense } from "react";
import { useRouter, useSearchParams } from "next/navigation";
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
  FileText,
  Bot,
  Wrench,
  Database,
  Calendar,
  FileJson,
  FileCode,
  Printer,
  Package,
  Layers,
  Lock,
  Unlock,
  ArrowRight,
  Cpu,
  X,
  User,
  Code,
  Key,
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
  iconBg = "bg-gray-100 dark:bg-gray-700",
  iconColor = "text-gray-500 dark:text-gray-400",
  valueColor = "text-gray-900 dark:text-gray-100",
}: {
  icon: any;
  label: string;
  value: number | string;
  change?: string;
  changeType?: "positive" | "negative" | "neutral";
  suffix?: string;
  iconBg?: string;
  iconColor?: string;
  valueColor?: string;
}) {
  return (
    <div className="bg-white dark:bg-gray-800 p-6 rounded-lg border border-gray-200 dark:border-gray-700 shadow-sm">
      <div className="flex items-center">
        <div className={`flex-shrink-0 p-2 rounded-lg ${iconBg}`}>
          <Icon className={`h-5 w-5 ${iconColor}`} />
        </div>
        <div className="ml-4 w-0 flex-1">
          <dl>
            <dt className="text-sm font-medium text-gray-500 dark:text-gray-400 truncate">
              {label}
            </dt>
            <dd className="flex items-baseline">
              <div className={`text-2xl font-semibold ${valueColor}`}>
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

// ABOM Types
interface Agent {
  id: string;
  name: string;
  status: string;
  trustScore: number;
  capabilities: string[];
  dataAccess?: string[];
  lastActiveAt?: string;
  createdAt: string;
  createdByName?: string;
  createdByEmail?: string;
  createdBySdkTokenId?: string;
  createdByApiKeyId?: string;
}

interface MCPCapability {
  id: string;
  mcpServerId: string;
  capabilityName: string;
  capabilityType: string;
  description?: string;
  inputSchema?: Record<string, any>;
  createdAt: string;
}

interface ABOMData {
  generatedAt: string;
  version: string;
  summary: {
    totalAgents: number;
    totalMcpServers: number;
    totalTools: number;
    totalConnections: number;
    totalCapabilities: number;
    dataCategories: string[];
  };
  agents: Agent[];
  mcpServers: MCPServer[];
  connections: AgentMCPConnection[];
  capabilities: MCPCapability[];
}

function SupplyChainPage() {
  const router = useRouter();
  const searchParams = useSearchParams();
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

  // Tab state - initialize from URL param
  const tabParam = searchParams.get("tab");
  const [activeTab, setActiveTab] = useState<"analytics" | "abom">(
    tabParam === "abom" ? "abom" : "analytics"
  );

  // ABOM state
  const [agents, setAgents] = useState<Agent[]>([]);
  const [selectedAgent, setSelectedAgent] = useState<Agent | null>(null);
  const [mcpCapabilities, setMcpCapabilities] = useState<MCPCapability[]>([]);

  // ABOM pagination
  const [abomAgentPage, setAbomAgentPage] = useState(1);
  const [abomAgentPageSize, setAbomAgentPageSize] = useState(10);
  const [abomMcpPage, setAbomMcpPage] = useState(1);
  const [abomMcpPageSize, setAbomMcpPageSize] = useState(10);
  const [abomConnPage, setAbomConnPage] = useState(1);
  const [abomConnPageSize, setAbomConnPageSize] = useState(12);

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
      const servers: MCPServer[] = serversResponse.mcpServers || [];
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

      // Fetch agents for ABOM
      try {
        const agentsResponse = await api.listAgents();
        const agentsList = (agentsResponse.agents || []).map((a: any) => ({
          id: a.id,
          name: a.name,
          status: a.status,
          trustScore: a.trustScore || 0,
          capabilities: a.capabilities || [],
          dataAccess: a.dataAccess || [],
          lastActiveAt: a.lastActiveAt,
          createdAt: a.createdAt,
          createdByName: a.createdByName,
          createdByEmail: a.createdByEmail,
          createdBySdkTokenId: a.createdBySdkTokenId,
          createdByApiKeyId: a.createdByApiKeyId,
        }));
        setAgents(agentsList);
      } catch (agentsError) {
        console.error("Failed to fetch agents:", agentsError);
        setAgents([]);
      }

      // Fetch MCP capabilities for ABOM
      try {
        const capabilitiesData: MCPCapability[] = [];
        for (const server of servers) {
          if (server.capabilities) {
            server.capabilities.forEach((cap: string, idx: number) => {
              capabilitiesData.push({
                id: `${server.id}-${idx}`,
                mcpServerId: server.id,
                capabilityName: cap,
                capabilityType: "tool",
                createdAt: server.createdAt,
              });
            });
          }
        }
        setMcpCapabilities(capabilitiesData);
      } catch (capError) {
        console.error("Failed to process capabilities:", capError);
        setMcpCapabilities([]);
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

  // ABOM pagination calculations
  const totalAbomAgentPages = Math.ceil(agents.length / abomAgentPageSize);
  const paginatedAbomAgents = agents.slice(
    (abomAgentPage - 1) * abomAgentPageSize,
    abomAgentPage * abomAgentPageSize
  );

  const totalAbomMcpPages = Math.ceil(mcpServers.length / abomMcpPageSize);
  const paginatedAbomMcpServers = mcpServers.slice(
    (abomMcpPage - 1) * abomMcpPageSize,
    abomMcpPage * abomMcpPageSize
  );

  const totalAbomConnPages = Math.ceil(connections.length / abomConnPageSize);
  const paginatedAbomConnections = connections.slice(
    (abomConnPage - 1) * abomConnPageSize,
    abomConnPage * abomConnPageSize
  );

  // Generate ABOM data
  const abomData = useMemo<ABOMData>(() => {
    const totalTools = mcpServers.reduce((sum, s) => sum + (s.toolCount || 0), 0);
    const dataCategories = new Set<string>();
    agents.forEach((a) => {
      (a.dataAccess || []).forEach((d) => dataCategories.add(d));
    });

    return {
      generatedAt: new Date().toISOString(),
      version: "1.0.0",
      summary: {
        totalAgents: agents.length,
        totalMcpServers: mcpServers.length,
        totalTools,
        totalConnections: connections.length,
        totalCapabilities: mcpCapabilities.length,
        dataCategories: Array.from(dataCategories),
      },
      agents,
      mcpServers,
      connections,
      capabilities: mcpCapabilities,
    };
  }, [agents, mcpServers, connections, mcpCapabilities]);

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

  const handleExportABOM = (format: "json" | "yaml" | "pdf") => {
    const filename = `abom-${new Date().toISOString().split("T")[0]}`;

    if (format === "json") {
      const jsonStr = JSON.stringify(abomData, null, 2);
      const blob = new Blob([jsonStr], { type: "application/json" });
      const url = URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = url;
      a.download = `${filename}.json`;
      a.click();
      URL.revokeObjectURL(url);
    } else if (format === "yaml") {
      // Simple YAML generation
      const yamlStr = generateYAML(abomData);
      const blob = new Blob([yamlStr], { type: "text/yaml" });
      const url = URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = url;
      a.download = `${filename}.yaml`;
      a.click();
      URL.revokeObjectURL(url);
    } else if (format === "pdf") {
      // For PDF, we'll generate a printable HTML and open print dialog
      const printWindow = window.open("", "_blank");
      if (printWindow) {
        printWindow.document.write(generateABOMPrintHTML(abomData));
        printWindow.document.close();
        printWindow.print();
      }
    }
  };

  // Simple YAML generator
  const generateYAML = (data: any, indent = 0): string => {
    const spaces = "  ".repeat(indent);
    let yaml = "";

    if (Array.isArray(data)) {
      data.forEach((item) => {
        if (typeof item === "object" && item !== null) {
          yaml += `${spaces}-\n${generateYAML(item, indent + 1)}`;
        } else {
          yaml += `${spaces}- ${item}\n`;
        }
      });
    } else if (typeof data === "object" && data !== null) {
      Object.entries(data).forEach(([key, value]) => {
        if (typeof value === "object" && value !== null) {
          yaml += `${spaces}${key}:\n${generateYAML(value, indent + 1)}`;
        } else {
          yaml += `${spaces}${key}: ${value}\n`;
        }
      });
    }

    return yaml;
  };

  // Generate printable HTML for PDF
  const generateABOMPrintHTML = (data: ABOMData): string => {
    return `
      <!DOCTYPE html>
      <html>
      <head>
        <title>Agent Bill of Materials (ABOM)</title>
        <style>
          body { font-family: system-ui, sans-serif; padding: 40px; max-width: 1000px; margin: 0 auto; }
          h1 { color: #1e40af; border-bottom: 2px solid #3b82f6; padding-bottom: 10px; }
          h2 { color: #1e3a8a; margin-top: 30px; }
          h3 { color: #1e40af; margin-top: 20px; }
          .meta { color: #6b7280; font-size: 14px; margin-bottom: 30px; }
          .summary-grid { display: grid; grid-template-columns: repeat(3, 1fr); gap: 20px; margin: 20px 0; }
          .summary-card { background: #f3f4f6; padding: 15px; border-radius: 8px; }
          .summary-card h4 { margin: 0 0 5px 0; color: #374151; font-size: 14px; }
          .summary-card p { margin: 0; font-size: 24px; font-weight: 600; color: #1e40af; }
          table { width: 100%; border-collapse: collapse; margin: 15px 0; }
          th { background: #e5e7eb; text-align: left; padding: 10px; font-size: 12px; text-transform: uppercase; }
          td { padding: 10px; border-bottom: 1px solid #e5e7eb; font-size: 14px; }
          .badge { display: inline-block; padding: 2px 8px; border-radius: 4px; font-size: 12px; margin: 2px; }
          .badge-green { background: #dcfce7; color: #166534; }
          .badge-blue { background: #dbeafe; color: #1e40af; }
          .badge-yellow { background: #fef3c7; color: #92400e; }
          .footer { margin-top: 50px; padding-top: 20px; border-top: 1px solid #e5e7eb; color: #6b7280; font-size: 12px; }
          @media print { body { padding: 20px; } }
        </style>
      </head>
      <body>
        <h1>Agent Bill of Materials (ABOM)</h1>
        <div class="meta">
          <p>Generated: ${new Date(data.generatedAt).toLocaleString()}</p>
          <p>Version: ${data.version}</p>
        </div>

        <h2>Summary</h2>
        <div class="summary-grid">
          <div class="summary-card"><h4>Total Agents</h4><p>${data.summary.totalAgents}</p></div>
          <div class="summary-card"><h4>MCP Servers</h4><p>${data.summary.totalMcpServers}</p></div>
          <div class="summary-card"><h4>Total Tools</h4><p>${data.summary.totalTools}</p></div>
          <div class="summary-card"><h4>Connections</h4><p>${data.summary.totalConnections}</p></div>
          <div class="summary-card"><h4>Capabilities</h4><p>${data.summary.totalCapabilities}</p></div>
          <div class="summary-card"><h4>Data Categories</h4><p>${data.summary.dataCategories.length}</p></div>
        </div>

        <h2>Agents Inventory</h2>
        <table>
          <tr><th>Name</th><th>Status</th><th>Trust Score</th><th>Capabilities</th><th>Data Access</th></tr>
          ${data.agents.map((a) => `
            <tr>
              <td>${a.name}</td>
              <td><span class="badge badge-${a.status === "active" ? "green" : "yellow"}">${a.status}</span></td>
              <td>${a.trustScore}%</td>
              <td>${(a.capabilities || []).map((c) => `<span class="badge badge-blue">${c}</span>`).join(" ")}</td>
              <td>${(a.dataAccess || []).map((d) => `<span class="badge badge-yellow">${d}</span>`).join(" ")}</td>
            </tr>
          `).join("")}
        </table>

        <h2>MCP Servers</h2>
        <table>
          <tr><th>Name</th><th>URL</th><th>Status</th><th>Tools</th><th>Attestations</th></tr>
          ${data.mcpServers.map((s) => `
            <tr>
              <td>${s.name}</td>
              <td style="font-size:12px;color:#6b7280;">${s.url}</td>
              <td><span class="badge badge-${s.status === "verified" ? "green" : "yellow"}">${s.status}</span></td>
              <td>${s.toolCount || 0}</td>
              <td>${s.attestationCount || 0}</td>
            </tr>
          `).join("")}
        </table>

        <h2>Agent-MCP Connections</h2>
        <table>
          <tr><th>Agent</th><th>MCP Server</th><th>Type</th><th>Attestations</th><th>Status</th></tr>
          ${data.connections.map((c) => `
            <tr>
              <td>${c.agentName}</td>
              <td>${c.mcpServerName}</td>
              <td>${c.connectionType}</td>
              <td>${c.attestationCount}</td>
              <td><span class="badge badge-${c.isActive ? "green" : "yellow"}">${c.isActive ? "Active" : "Inactive"}</span></td>
            </tr>
          `).join("")}
        </table>

        <div class="footer">
          <p>This Agent Bill of Materials (ABOM) is automatically generated by AIM (Agent Identity Management).</p>
          <p>For security compliance and audit purposes. Read-only observational data.</p>
        </div>
      </body>
      </html>
    `;
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
      <div className="mb-6">
        <div className="flex items-center justify-between">
          <div>
            <h1 className="text-3xl font-bold text-gray-900 dark:text-white flex items-center gap-3">
              <GitBranch className="h-8 w-8 text-blue-500" />
              Supply Chain & ABOM
            </h1>
            <p className="mt-2 text-gray-600 dark:text-gray-400">
              Monitor MCP server dependencies, attestation health, and your Agent Bill of Materials.
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
            {activeTab === "analytics" ? (
              <button
                onClick={handleExport}
                className="inline-flex items-center px-4 py-2 border border-transparent rounded-md shadow-sm text-sm font-medium text-white bg-blue-600 hover:bg-blue-700"
              >
                <Download className="h-4 w-4 mr-2" />
                Export Report
              </button>
            ) : (
              <div className="relative">
                <button
                  onClick={() => {
                    const dropdown = document.getElementById("abom-export-dropdown");
                    if (dropdown) dropdown.classList.toggle("hidden");
                  }}
                  className="inline-flex items-center px-4 py-2 border border-transparent rounded-md shadow-sm text-sm font-medium text-white bg-blue-600 hover:bg-blue-700"
                >
                  <Download className="h-4 w-4 mr-2" />
                  Export ABOM
                </button>
                <div
                  id="abom-export-dropdown"
                  className="hidden absolute right-0 mt-2 w-48 bg-white dark:bg-gray-800 rounded-md shadow-lg border border-gray-200 dark:border-gray-700 z-10"
                >
                  <button
                    onClick={() => {
                      handleExportABOM("json");
                      document.getElementById("abom-export-dropdown")?.classList.add("hidden");
                    }}
                    className="flex items-center w-full px-4 py-2 text-sm text-gray-700 dark:text-gray-200 hover:bg-gray-100 dark:hover:bg-gray-700"
                  >
                    <FileJson className="h-4 w-4 mr-2 text-blue-500" />
                    Export as JSON
                  </button>
                  <button
                    onClick={() => {
                      handleExportABOM("yaml");
                      document.getElementById("abom-export-dropdown")?.classList.add("hidden");
                    }}
                    className="flex items-center w-full px-4 py-2 text-sm text-gray-700 dark:text-gray-200 hover:bg-gray-100 dark:hover:bg-gray-700"
                  >
                    <FileCode className="h-4 w-4 mr-2 text-green-500" />
                    Export as YAML
                  </button>
                  <button
                    onClick={() => {
                      handleExportABOM("pdf");
                      document.getElementById("abom-export-dropdown")?.classList.add("hidden");
                    }}
                    className="flex items-center w-full px-4 py-2 text-sm text-gray-700 dark:text-gray-200 hover:bg-gray-100 dark:hover:bg-gray-700"
                  >
                    <Printer className="h-4 w-4 mr-2 text-red-500" />
                    Print / PDF
                  </button>
                </div>
              </div>
            )}
          </div>
        </div>
      </div>

      {/* Tabs */}
      <div className="mb-6">
        <div className="border-b border-gray-200 dark:border-gray-700">
          <nav className="-mb-px flex space-x-8">
            <button
              onClick={() => setActiveTab("analytics")}
              className={`py-4 px-1 border-b-2 font-medium text-sm flex items-center gap-2 ${
                activeTab === "analytics"
                  ? "border-blue-500 text-blue-600 dark:text-blue-400"
                  : "border-transparent text-gray-500 hover:text-gray-700 hover:border-gray-300 dark:text-gray-400 dark:hover:text-gray-300"
              }`}
            >
              <BarChart3 className="h-4 w-4" />
              Analytics
              <span className="ml-1 px-2 py-0.5 text-xs font-medium bg-gray-100 dark:bg-gray-700 text-gray-600 dark:text-gray-300 rounded-full">
                {stats?.totalMCPServers || 0}
              </span>
            </button>
            <button
              onClick={() => setActiveTab("abom")}
              className={`py-4 px-1 border-b-2 font-medium text-sm flex items-center gap-2 ${
                activeTab === "abom"
                  ? "border-blue-500 text-blue-600 dark:text-blue-400"
                  : "border-transparent text-gray-500 hover:text-gray-700 hover:border-gray-300 dark:text-gray-400 dark:hover:text-gray-300"
              }`}
            >
              <Package className="h-4 w-4" />
              ABOM
              <span className="ml-1 px-2 py-0.5 text-xs font-medium bg-blue-100 dark:bg-blue-900/30 text-blue-600 dark:text-blue-300 rounded-full">
                Bill of Materials
              </span>
            </button>
          </nav>
        </div>
      </div>

      {/* Analytics Tab Content */}
      {activeTab === "analytics" && (
        <>
          {/* Stats Grid */}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4 mb-8">
        <StatCard
          icon={Server}
          label="Total MCP Servers"
          value={stats?.totalMCPServers || 0}
          iconBg="bg-purple-100 dark:bg-purple-900/30"
          iconColor="text-purple-600 dark:text-purple-400"
          valueColor="text-purple-600 dark:text-purple-400"
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
          iconBg="bg-green-100 dark:bg-green-900/30"
          iconColor="text-green-600 dark:text-green-400"
          valueColor="text-green-600 dark:text-green-400"
        />
        <StatCard
          icon={Link2}
          label="Active Connections"
          value={stats?.activeConnections || 0}
          suffix={`/ ${stats?.totalConnections || 0}`}
          iconBg="bg-cyan-100 dark:bg-cyan-900/30"
          iconColor="text-cyan-600 dark:text-cyan-400"
          valueColor="text-cyan-600 dark:text-cyan-400"
        />
        <StatCard
          icon={AlertCircle}
          label="Unmapped MCPs"
          value={unmappedMcpCount}
          changeType={
            unmappedMcpCount > 0 ? "negative" : "positive"
          }
          iconBg="bg-orange-100 dark:bg-orange-900/30"
          iconColor="text-orange-600 dark:text-orange-400"
          valueColor="text-orange-600 dark:text-orange-400"
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
          iconBg="bg-yellow-100 dark:bg-yellow-900/30"
          iconColor="text-yellow-600 dark:text-yellow-400"
          valueColor="text-yellow-600 dark:text-yellow-400"
        />
        <StatCard
          icon={Activity}
          label="Attestations (24h)"
          value={stats?.attestationsLast24h || 0}
          iconBg="bg-blue-100 dark:bg-blue-900/30"
          iconColor="text-blue-600 dark:text-blue-400"
          valueColor="text-blue-600 dark:text-blue-400"
        />
        <StatCard
          icon={AlertTriangle}
          label="Capability Drift Alerts"
          value={stats?.capabilityDriftAlerts || 0}
          changeType={
            (stats?.capabilityDriftAlerts || 0) > 0 ? "negative" : "positive"
          }
          iconBg="bg-red-100 dark:bg-red-900/30"
          iconColor="text-red-600 dark:text-red-400"
          valueColor="text-red-600 dark:text-red-400"
        />
        <StatCard
          icon={Box}
          label="Unique Capabilities"
          value={mcpServers.reduce(
            (sum, s) => sum + (s.toolCount || 0),
            0
          )}
          iconBg="bg-indigo-100 dark:bg-indigo-900/30"
          iconColor="text-indigo-600 dark:text-indigo-400"
          valueColor="text-indigo-600 dark:text-indigo-400"
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
        </>
      )}

      {/* ABOM Tab Content */}
      {activeTab === "abom" && (
        <div className="space-y-6">
          {/* ABOM Header Banner - Muted style matching Supply Chain */}
          <div className="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-6">
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-4">
                <Package className="h-8 w-8 text-gray-400" />
                <div>
                  <h2 className="text-xl font-semibold text-gray-900 dark:text-white">Agent Bill of Materials</h2>
                  <p className="text-sm text-gray-500 dark:text-gray-400 mt-1">
                    Complete inventory of all agents, MCP servers, tools, and data access patterns observed by AIM
                  </p>
                </div>
              </div>
              <div className="text-right">
                <p className="text-xs text-gray-500 dark:text-gray-400">Generated</p>
                <p className="text-sm font-medium text-gray-900 dark:text-white">{new Date(abomData.generatedAt).toLocaleString()}</p>
                <p className="text-xs text-gray-500 dark:text-gray-400 mt-1">Version {abomData.version}</p>
              </div>
            </div>
          </div>

          {/* ABOM Summary Stats */}
          <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-6 gap-4">
            <div className="bg-white dark:bg-gray-800 p-4 rounded-lg border border-gray-200 dark:border-gray-700">
              <div className="flex items-center gap-3">
                <div className="p-2 bg-blue-100 dark:bg-blue-900/30 rounded-lg">
                  <Bot className="h-5 w-5 text-blue-600 dark:text-blue-400" />
                </div>
                <div>
                  <p className="text-2xl font-bold text-blue-600 dark:text-blue-400">{abomData.summary.totalAgents}</p>
                  <p className="text-xs text-gray-500 dark:text-gray-400">Agents</p>
                </div>
              </div>
            </div>
            <div className="bg-white dark:bg-gray-800 p-4 rounded-lg border border-gray-200 dark:border-gray-700">
              <div className="flex items-center gap-3">
                <div className="p-2 bg-purple-100 dark:bg-purple-900/30 rounded-lg">
                  <Server className="h-5 w-5 text-purple-600 dark:text-purple-400" />
                </div>
                <div>
                  <p className="text-2xl font-bold text-purple-600 dark:text-purple-400">{abomData.summary.totalMcpServers}</p>
                  <p className="text-xs text-gray-500 dark:text-gray-400">MCP Servers</p>
                </div>
              </div>
            </div>
            <div className="bg-white dark:bg-gray-800 p-4 rounded-lg border border-gray-200 dark:border-gray-700">
              <div className="flex items-center gap-3">
                <div className="p-2 bg-green-100 dark:bg-green-900/30 rounded-lg">
                  <Wrench className="h-5 w-5 text-green-600 dark:text-green-400" />
                </div>
                <div>
                  <p className="text-2xl font-bold text-green-600 dark:text-green-400">{abomData.summary.totalTools}</p>
                  <p className="text-xs text-gray-500 dark:text-gray-400">Tools</p>
                </div>
              </div>
            </div>
            <div className="bg-white dark:bg-gray-800 p-4 rounded-lg border border-gray-200 dark:border-gray-700">
              <div className="flex items-center gap-3">
                <div className="p-2 bg-cyan-100 dark:bg-cyan-900/30 rounded-lg">
                  <Link2 className="h-5 w-5 text-cyan-600 dark:text-cyan-400" />
                </div>
                <div>
                  <p className="text-2xl font-bold text-cyan-600 dark:text-cyan-400">{abomData.summary.totalConnections}</p>
                  <p className="text-xs text-gray-500 dark:text-gray-400">Connections</p>
                </div>
              </div>
            </div>
            <div className="bg-white dark:bg-gray-800 p-4 rounded-lg border border-gray-200 dark:border-gray-700">
              <div className="flex items-center gap-3">
                <div className="p-2 bg-orange-100 dark:bg-orange-900/30 rounded-lg">
                  <Cpu className="h-5 w-5 text-orange-600 dark:text-orange-400" />
                </div>
                <div>
                  <p className="text-2xl font-bold text-orange-600 dark:text-orange-400">{abomData.summary.totalCapabilities}</p>
                  <p className="text-xs text-gray-500 dark:text-gray-400">Capabilities</p>
                </div>
              </div>
            </div>
            <div className="bg-white dark:bg-gray-800 p-4 rounded-lg border border-gray-200 dark:border-gray-700">
              <div className="flex items-center gap-3">
                <div className="p-2 bg-yellow-100 dark:bg-yellow-900/30 rounded-lg">
                  <Database className="h-5 w-5 text-yellow-600 dark:text-yellow-400" />
                </div>
                <div>
                  <p className="text-2xl font-bold text-yellow-600 dark:text-yellow-400">{abomData.summary.dataCategories.length}</p>
                  <p className="text-xs text-gray-500 dark:text-gray-400">Data Categories</p>
                </div>
              </div>
            </div>
          </div>

          {/* Agents Inventory */}
          <div className="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 overflow-hidden">
            <div className="px-6 py-4 border-b border-gray-200 dark:border-gray-700">
              <h3 className="text-lg font-semibold text-gray-900 dark:text-white flex items-center gap-2">
                <Bot className="h-5 w-5 text-gray-500" />
                Agents Inventory
                <span className="ml-2 px-2 py-0.5 text-xs font-medium bg-gray-100 dark:bg-gray-700 text-gray-600 dark:text-gray-300 rounded-full">
                  {agents.length}
                </span>
              </h3>
              <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">
                All registered AI agents with their capabilities and data access permissions
              </p>
            </div>
            <div className="overflow-x-auto">
              <table className="min-w-full divide-y divide-gray-200 dark:divide-gray-700">
                <thead className="bg-gray-50 dark:bg-gray-900">
                  <tr>
                    <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">Agent</th>
                    <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">Status</th>
                    <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">Trust Score</th>
                    <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">Capabilities</th>
                    <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">Data Access</th>
                    <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">Registered By</th>
                    <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">Created</th>
                  </tr>
                </thead>
                <tbody className="bg-white dark:bg-gray-800 divide-y divide-gray-200 dark:divide-gray-700">
                  {agents.length === 0 ? (
                    <tr>
                      <td colSpan={7} className="px-6 py-12 text-center text-gray-500 dark:text-gray-400">
                        <Bot className="h-12 w-12 mx-auto mb-4 opacity-50" />
                        <p className="text-lg font-medium">No agents registered</p>
                        <p className="text-sm mt-1">Agents will appear here when registered with AIM</p>
                      </td>
                    </tr>
                  ) : (
                    paginatedAbomAgents.map((agent) => (
                      <tr
                        key={agent.id}
                        className="hover:bg-gray-50 dark:hover:bg-gray-700/50 cursor-pointer"
                        onClick={() => setSelectedAgent(agent)}
                      >
                        <td className="px-6 py-4 whitespace-nowrap">
                          <div className="flex items-center">
                            <Bot className="h-5 w-5 text-gray-400 mr-3" />
                            <div>
                              <div className="text-sm font-medium text-gray-900 dark:text-white">{agent.name}</div>
                              <div className="text-xs text-gray-500 dark:text-gray-400">{agent.id.slice(0, 8)}...</div>
                            </div>
                          </div>
                        </td>
                        <td className="px-6 py-4 whitespace-nowrap">
                          <StatusBadge status={agent.status} />
                        </td>
                        <td className="px-6 py-4 whitespace-nowrap">
                          <ConfidenceScoreBadge score={agent.trustScore * 100} />
                        </td>
                        <td className="px-6 py-4">
                          <div className="flex flex-wrap gap-1">
                            {(agent.capabilities || []).slice(0, 3).map((cap, idx) => (
                              <span key={idx} className="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300">
                                {cap}
                              </span>
                            ))}
                            {(agent.capabilities || []).length > 3 && (
                              <span className="text-xs text-gray-500">+{agent.capabilities.length - 3} more</span>
                            )}
                            {(!agent.capabilities || agent.capabilities.length === 0) && (
                              <span className="text-xs text-gray-400">None declared</span>
                            )}
                          </div>
                        </td>
                        <td className="px-6 py-4">
                          <div className="flex flex-wrap gap-1">
                            {(agent.dataAccess || []).slice(0, 2).map((data, idx) => (
                              <span key={idx} className="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300">
                                {data}
                              </span>
                            ))}
                            {(agent.dataAccess || []).length > 2 && (
                              <span className="text-xs text-gray-500">+{agent.dataAccess!.length - 2} more</span>
                            )}
                            {(!agent.dataAccess || agent.dataAccess.length === 0) && (
                              <span className="text-xs text-gray-400">None</span>
                            )}
                          </div>
                        </td>
                        <td className="px-6 py-4 whitespace-nowrap">
                          <div className="flex items-center gap-2">
                            <div className={`w-6 h-6 rounded-full flex items-center justify-center ${
                              agent.createdBySdkTokenId
                                ? "bg-purple-100 dark:bg-purple-900/30"
                                : agent.createdByApiKeyId
                                  ? "bg-amber-100 dark:bg-amber-900/30"
                                  : "bg-blue-100 dark:bg-blue-900/30"
                            }`}>
                              {agent.createdBySdkTokenId ? (
                                <Code className="h-3.5 w-3.5 text-purple-600 dark:text-purple-400" />
                              ) : agent.createdByApiKeyId ? (
                                <Key className="h-3.5 w-3.5 text-amber-600 dark:text-amber-400" />
                              ) : (
                                <User className="h-3.5 w-3.5 text-blue-600 dark:text-blue-400" />
                              )}
                            </div>
                            <div>
                              <div className="text-sm text-gray-900 dark:text-white">
                                {agent.createdByName || agent.createdByEmail || (agent.createdBySdkTokenId ? "SDK" : agent.createdByApiKeyId ? "API Key" : "Unknown")}
                              </div>
                              {agent.createdByEmail && agent.createdByName && (
                                <div className="text-xs text-gray-500 dark:text-gray-400">{agent.createdByEmail}</div>
                              )}
                              {!agent.createdByName && !agent.createdByEmail && agent.createdBySdkTokenId && (
                                <div className="text-xs text-gray-500 dark:text-gray-400">via SDK Token</div>
                              )}
                            </div>
                          </div>
                        </td>
                        <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500 dark:text-gray-400">
                          {formatDateTime(agent.createdAt)}
                        </td>
                      </tr>
                    ))
                  )}
                </tbody>
              </table>
            </div>
            {/* Agents Pagination */}
            {agents.length > abomAgentPageSize && (
              <div className="px-6 py-4 border-t border-gray-200 dark:border-gray-700 flex items-center justify-between">
                <div className="text-sm text-gray-500 dark:text-gray-400">
                  Showing {((abomAgentPage - 1) * abomAgentPageSize) + 1} to {Math.min(abomAgentPage * abomAgentPageSize, agents.length)} of {agents.length} agents
                </div>
                <div className="flex items-center gap-2">
                  <button
                    onClick={() => setAbomAgentPage(p => Math.max(1, p - 1))}
                    disabled={abomAgentPage === 1}
                    className="px-3 py-1.5 text-sm font-medium text-gray-700 dark:text-gray-300 bg-white dark:bg-gray-800 border border-gray-300 dark:border-gray-600 rounded-md hover:bg-gray-50 dark:hover:bg-gray-700 disabled:opacity-50 disabled:cursor-not-allowed"
                  >
                    Previous
                  </button>
                  <span className="text-sm text-gray-600 dark:text-gray-400">
                    Page {abomAgentPage} of {totalAbomAgentPages}
                  </span>
                  <button
                    onClick={() => setAbomAgentPage(p => Math.min(totalAbomAgentPages, p + 1))}
                    disabled={abomAgentPage === totalAbomAgentPages}
                    className="px-3 py-1.5 text-sm font-medium text-gray-700 dark:text-gray-300 bg-white dark:bg-gray-800 border border-gray-300 dark:border-gray-600 rounded-md hover:bg-gray-50 dark:hover:bg-gray-700 disabled:opacity-50 disabled:cursor-not-allowed"
                  >
                    Next
                  </button>
                </div>
              </div>
            )}
          </div>

          {/* MCP Tools & Servers */}
          <div className="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 overflow-hidden">
            <div className="px-6 py-4 border-b border-gray-200 dark:border-gray-700">
              <h3 className="text-lg font-semibold text-gray-900 dark:text-white flex items-center gap-2">
                <Server className="h-5 w-5 text-gray-500" />
                MCP Servers & Tools
                <span className="ml-2 px-2 py-0.5 text-xs font-medium bg-gray-100 dark:bg-gray-700 text-gray-600 dark:text-gray-300 rounded-full">
                  {mcpServers.length} servers
                </span>
                <span className="px-2 py-0.5 text-xs font-medium bg-gray-100 dark:bg-gray-700 text-gray-600 dark:text-gray-300 rounded-full">
                  {abomData.summary.totalTools} tools
                </span>
              </h3>
              <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">
                All MCP servers discovered through agent attestations with their exposed tools
              </p>
            </div>
            <div className="overflow-x-auto">
              <table className="min-w-full divide-y divide-gray-200 dark:divide-gray-700">
                <thead className="bg-gray-50 dark:bg-gray-900">
                  <tr>
                    <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">Server</th>
                    <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">URL</th>
                    <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">Status</th>
                    <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">Tools</th>
                    <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">Attestations</th>
                    <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">Confidence</th>
                  </tr>
                </thead>
                <tbody className="bg-white dark:bg-gray-800 divide-y divide-gray-200 dark:divide-gray-700">
                  {mcpServers.length === 0 ? (
                    <tr>
                      <td colSpan={6} className="px-6 py-12 text-center text-gray-500 dark:text-gray-400">
                        <Server className="h-12 w-12 mx-auto mb-4 opacity-50" />
                        <p className="text-lg font-medium">No MCP servers discovered</p>
                        <p className="text-sm mt-1">MCP servers will appear here when agents attest their usage</p>
                      </td>
                    </tr>
                  ) : (
                    paginatedAbomMcpServers.map((server) => (
                      <tr key={server.id} className="hover:bg-gray-50 dark:hover:bg-gray-700/50">
                        <td className="px-6 py-4 whitespace-nowrap">
                          <div className="flex items-center">
                            <Server className="h-5 w-5 text-gray-400 mr-3" />
                            <div className="text-sm font-medium text-gray-900 dark:text-white">{server.name}</div>
                          </div>
                        </td>
                        <td className="px-6 py-4">
                          <div className="text-sm text-gray-500 dark:text-gray-400 truncate max-w-xs">{server.url}</div>
                        </td>
                        <td className="px-6 py-4 whitespace-nowrap">
                          <StatusBadge status={server.status} />
                        </td>
                        <td className="px-6 py-4 whitespace-nowrap">
                          <div className="flex items-center">
                            <Wrench className="h-4 w-4 text-gray-400 mr-1" />
                            <span className="text-sm text-gray-900 dark:text-white">{server.toolCount || 0}</span>
                          </div>
                        </td>
                        <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-900 dark:text-white">
                          {server.attestationCount || 0}
                        </td>
                        <td className="px-6 py-4 whitespace-nowrap">
                          {server.confidenceScore ? (
                            <ConfidenceScoreBadge score={server.confidenceScore} />
                          ) : (
                            <span className="text-sm text-gray-400">N/A</span>
                          )}
                        </td>
                      </tr>
                    ))
                  )}
                </tbody>
              </table>
            </div>
            {/* MCP Servers Pagination */}
            {mcpServers.length > abomMcpPageSize && (
              <div className="px-6 py-4 border-t border-gray-200 dark:border-gray-700 flex items-center justify-between">
                <div className="text-sm text-gray-500 dark:text-gray-400">
                  Showing {((abomMcpPage - 1) * abomMcpPageSize) + 1} to {Math.min(abomMcpPage * abomMcpPageSize, mcpServers.length)} of {mcpServers.length} servers
                </div>
                <div className="flex items-center gap-2">
                  <button
                    onClick={() => setAbomMcpPage(p => Math.max(1, p - 1))}
                    disabled={abomMcpPage === 1}
                    className="px-3 py-1.5 text-sm font-medium text-gray-700 dark:text-gray-300 bg-white dark:bg-gray-800 border border-gray-300 dark:border-gray-600 rounded-md hover:bg-gray-50 dark:hover:bg-gray-700 disabled:opacity-50 disabled:cursor-not-allowed"
                  >
                    Previous
                  </button>
                  <span className="text-sm text-gray-600 dark:text-gray-400">
                    Page {abomMcpPage} of {totalAbomMcpPages}
                  </span>
                  <button
                    onClick={() => setAbomMcpPage(p => Math.min(totalAbomMcpPages, p + 1))}
                    disabled={abomMcpPage === totalAbomMcpPages}
                    className="px-3 py-1.5 text-sm font-medium text-gray-700 dark:text-gray-300 bg-white dark:bg-gray-800 border border-gray-300 dark:border-gray-600 rounded-md hover:bg-gray-50 dark:hover:bg-gray-700 disabled:opacity-50 disabled:cursor-not-allowed"
                  >
                    Next
                  </button>
                </div>
              </div>
            )}
          </div>

          {/* Agent-MCP Connections Map */}
          <div className="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 overflow-hidden">
            <div className="px-6 py-4 border-b border-gray-200 dark:border-gray-700">
              <h3 className="text-lg font-semibold text-gray-900 dark:text-white flex items-center gap-2">
                <Link2 className="h-5 w-5 text-gray-500" />
                Agent-MCP Connection Map
                <span className="ml-2 px-2 py-0.5 text-xs font-medium bg-gray-100 dark:bg-gray-700 text-gray-600 dark:text-gray-300 rounded-full">
                  {connections.length} connections
                </span>
              </h3>
              <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">
                Observed connections between agents and MCP servers from attestation data
              </p>
            </div>
            {connections.length === 0 ? (
              <div className="px-6 py-12 text-center text-gray-500 dark:text-gray-400">
                <Link2 className="h-12 w-12 mx-auto mb-4 opacity-50" />
                <p className="text-lg font-medium">No connections observed</p>
                <p className="text-sm mt-1">Connections will appear here when agents attest MCP server usage</p>
              </div>
            ) : (
              <div className="p-6 grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
                {connections.slice(0, 12).map((conn) => (
                  <div
                    key={conn.id}
                    className="flex items-center gap-3 p-4 bg-gray-50 dark:bg-gray-900/50 rounded-lg border border-gray-200 dark:border-gray-700"
                  >
                    <div className="flex-1">
                      <div className="flex items-center gap-2">
                        <Bot className="h-4 w-4 text-gray-500" />
                        <span className="text-sm font-medium text-gray-900 dark:text-white truncate">
                          {conn.agentName}
                        </span>
                      </div>
                    </div>
                    <ArrowRight className="h-4 w-4 text-gray-400 flex-shrink-0" />
                    <div className="flex-1">
                      <div className="flex items-center gap-2">
                        <Server className="h-4 w-4 text-gray-500" />
                        <span className="text-sm font-medium text-gray-900 dark:text-white truncate">
                          {conn.mcpServerName}
                        </span>
                      </div>
                    </div>
                    <div className="flex-shrink-0">
                      {conn.isActive ? (
                        <CheckCircle2 className="h-4 w-4 text-green-500" />
                      ) : (
                        <XCircle className="h-4 w-4 text-gray-400" />
                      )}
                    </div>
                  </div>
                ))}
                {connections.length > 12 && (
                  <div className="col-span-full text-center text-sm text-gray-500 dark:text-gray-400 py-2">
                    + {connections.length - 12} more connections
                  </div>
                )}
              </div>
            )}
          </div>

          {/* Data Categories */}
          {abomData.summary.dataCategories.length > 0 && (
            <div className="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-6">
              <h3 className="text-lg font-semibold text-gray-900 dark:text-white flex items-center gap-2 mb-4">
                <Database className="h-5 w-5 text-gray-500" />
                Data Access Categories
                <span className="ml-2 px-2 py-0.5 text-xs font-medium bg-gray-100 dark:bg-gray-700 text-gray-600 dark:text-gray-300 rounded-full">
                  {abomData.summary.dataCategories.length} categories
                </span>
              </h3>
              <p className="text-sm text-gray-500 dark:text-gray-400 mb-4">
                Types of sensitive data that agents in your organization have declared access to
              </p>
              <div className="flex flex-wrap gap-2">
                {abomData.summary.dataCategories.map((cat, idx) => (
                  <span
                    key={idx}
                    className="inline-flex items-center px-3 py-1.5 rounded-lg text-sm font-medium bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300"
                  >
                    <Database className="h-4 w-4 mr-2" />
                    {cat}
                  </span>
                ))}
              </div>
            </div>
          )}

          {/* ABOM Notice */}
          <div className="bg-gray-50 dark:bg-gray-900/50 rounded-lg p-4 border border-gray-200 dark:border-gray-700">
            <div className="flex items-start gap-3">
              <div className="p-2 bg-gray-100 dark:bg-gray-700 rounded-lg">
                <Lock className="h-5 w-5 text-gray-600 dark:text-gray-400" />
              </div>
              <div>
                <h4 className="text-sm font-semibold text-gray-900 dark:text-gray-100">
                  This ABOM is Read-Only
                </h4>
                <p className="text-sm text-gray-600 dark:text-gray-300 mt-1">
                  The Agent Bill of Materials is automatically generated from AIM observations and attestations.
                  It cannot be manually modified and represents the actual state of your agent ecosystem.
                  Use the Export button above to download this ABOM for compliance, security audits, or sharing with stakeholders.
                </p>
              </div>
            </div>
          </div>

          {/* Agent Detail Modal */}
          {selectedAgent && (
            <div className="fixed inset-0 bg-black/50 dark:bg-black/70 flex items-center justify-center z-50 p-4">
              <div className="bg-white dark:bg-gray-800 rounded-lg shadow-xl max-w-2xl w-full max-h-[90vh] overflow-hidden">
                {/* Modal Header */}
                <div className="px-6 py-4 border-b border-gray-200 dark:border-gray-700 flex items-center justify-between">
                  <div className="flex items-center gap-3">
                    <div className="p-2 bg-gray-100 dark:bg-gray-700 rounded-lg">
                      <Bot className="h-6 w-6 text-gray-600 dark:text-gray-400" />
                    </div>
                    <div>
                      <h3 className="text-lg font-semibold text-gray-900 dark:text-white">{selectedAgent.name}</h3>
                      <p className="text-sm text-gray-500 dark:text-gray-400">Agent ABOM Details</p>
                    </div>
                  </div>
                  <button
                    onClick={() => setSelectedAgent(null)}
                    className="p-2 hover:bg-gray-100 dark:hover:bg-gray-700 rounded-lg transition-colors"
                  >
                    <X className="h-5 w-5 text-gray-500" />
                  </button>
                </div>

                {/* Modal Body */}
                <div className="px-6 py-4 overflow-y-auto max-h-[calc(90vh-120px)]">
                  <div className="space-y-6">
                    {/* Agent Info */}
                    <div className="grid grid-cols-2 gap-4">
                      <div>
                        <p className="text-xs font-medium text-gray-500 dark:text-gray-400 uppercase">Agent ID</p>
                        <p className="text-sm font-mono text-gray-900 dark:text-white mt-1">{selectedAgent.id}</p>
                      </div>
                      <div>
                        <p className="text-xs font-medium text-gray-500 dark:text-gray-400 uppercase">Status</p>
                        <div className="mt-1">
                          <StatusBadge status={selectedAgent.status} />
                        </div>
                      </div>
                      <div>
                        <p className="text-xs font-medium text-gray-500 dark:text-gray-400 uppercase">Trust Score</p>
                        <div className="mt-1">
                          <ConfidenceScoreBadge score={selectedAgent.trustScore * 100} />
                        </div>
                      </div>
                      <div>
                        <p className="text-xs font-medium text-gray-500 dark:text-gray-400 uppercase">Created</p>
                        <p className="text-sm text-gray-900 dark:text-white mt-1">{formatDateTime(selectedAgent.createdAt)}</p>
                      </div>
                      <div className="col-span-2">
                        <p className="text-xs font-medium text-gray-500 dark:text-gray-400 uppercase">Registered By</p>
                        <div className="mt-1 flex items-center gap-2">
                          <div className={`w-6 h-6 rounded-full flex items-center justify-center ${
                            selectedAgent.createdBySdkTokenId
                              ? "bg-purple-100 dark:bg-purple-900/30"
                              : selectedAgent.createdByApiKeyId
                                ? "bg-amber-100 dark:bg-amber-900/30"
                                : "bg-blue-100 dark:bg-blue-900/30"
                          }`}>
                            {selectedAgent.createdBySdkTokenId ? (
                              <Code className="h-3.5 w-3.5 text-purple-600 dark:text-purple-400" />
                            ) : selectedAgent.createdByApiKeyId ? (
                              <Key className="h-3.5 w-3.5 text-amber-600 dark:text-amber-400" />
                            ) : (
                              <User className="h-3.5 w-3.5 text-blue-600 dark:text-blue-400" />
                            )}
                          </div>
                          <div>
                            <p className="text-sm text-gray-900 dark:text-white">
                              {selectedAgent.createdByName || selectedAgent.createdByEmail || (selectedAgent.createdBySdkTokenId ? "SDK" : selectedAgent.createdByApiKeyId ? "API Key" : "Unknown")}
                            </p>
                            {selectedAgent.createdByEmail && selectedAgent.createdByName && (
                              <p className="text-xs text-gray-500 dark:text-gray-400">{selectedAgent.createdByEmail}</p>
                            )}
                            {!selectedAgent.createdByName && !selectedAgent.createdByEmail && selectedAgent.createdBySdkTokenId && (
                              <p className="text-xs text-gray-500 dark:text-gray-400">via SDK Token</p>
                            )}
                          </div>
                        </div>
                      </div>
                    </div>

                    {/* Capabilities */}
                    <div>
                      <p className="text-xs font-medium text-gray-500 dark:text-gray-400 uppercase mb-2">
                        Declared Capabilities ({(selectedAgent.capabilities || []).length})
                      </p>
                      {(selectedAgent.capabilities || []).length === 0 ? (
                        <p className="text-sm text-gray-400">No capabilities declared</p>
                      ) : (
                        <div className="flex flex-wrap gap-2">
                          {(selectedAgent.capabilities || []).map((cap, idx) => (
                            <span
                              key={idx}
                              className="inline-flex items-center px-3 py-1 rounded-lg text-sm font-medium bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300"
                            >
                              {cap}
                            </span>
                          ))}
                        </div>
                      )}
                    </div>

                    {/* Data Access */}
                    <div>
                      <p className="text-xs font-medium text-gray-500 dark:text-gray-400 uppercase mb-2">
                        Data Access Permissions ({(selectedAgent.dataAccess || []).length})
                      </p>
                      {(selectedAgent.dataAccess || []).length === 0 ? (
                        <p className="text-sm text-gray-400">No data access permissions declared</p>
                      ) : (
                        <div className="flex flex-wrap gap-2">
                          {(selectedAgent.dataAccess || []).map((data, idx) => (
                            <span
                              key={idx}
                              className="inline-flex items-center px-3 py-1 rounded-lg text-sm font-medium bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300"
                            >
                              <Database className="h-4 w-4 mr-1.5" />
                              {data}
                            </span>
                          ))}
                        </div>
                      )}
                    </div>

                    {/* Connected MCP Servers */}
                    <div>
                      <p className="text-xs font-medium text-gray-500 dark:text-gray-400 uppercase mb-2">
                        Connected MCP Servers
                      </p>
                      {connections.filter(c => c.agentId === selectedAgent.id).length === 0 ? (
                        <p className="text-sm text-gray-400">No MCP server connections observed</p>
                      ) : (
                        <div className="space-y-2">
                          {connections
                            .filter(c => c.agentId === selectedAgent.id)
                            .map((conn) => (
                              <div
                                key={conn.id}
                                className="flex items-center justify-between p-3 bg-gray-50 dark:bg-gray-900/50 rounded-lg border border-gray-200 dark:border-gray-700"
                              >
                                <div className="flex items-center gap-2">
                                  <Server className="h-4 w-4 text-gray-500" />
                                  <span className="text-sm font-medium text-gray-900 dark:text-white">
                                    {conn.mcpServerName}
                                  </span>
                                </div>
                                {conn.isActive ? (
                                  <span className="inline-flex items-center text-xs text-green-600 dark:text-green-400">
                                    <CheckCircle2 className="h-3 w-3 mr-1" />
                                    Active
                                  </span>
                                ) : (
                                  <span className="inline-flex items-center text-xs text-gray-500">
                                    <XCircle className="h-3 w-3 mr-1" />
                                    Inactive
                                  </span>
                                )}
                              </div>
                            ))}
                        </div>
                      )}
                    </div>
                  </div>
                </div>
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
      <Suspense fallback={
        <div className="min-h-screen flex items-center justify-center">
          <div className="text-center">
            <Loader2 className="h-8 w-8 animate-spin text-blue-500 mx-auto mb-4" />
            <p className="text-gray-500 dark:text-gray-400">Loading...</p>
          </div>
        </div>
      }>
        <SupplyChainPage />
      </Suspense>
    </AuthGuard>
  );
}
