"use client";

import { useState, useEffect, useMemo, Suspense } from "react";
import { useDebounce } from "@/hooks/use-debounce";
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
import { escapeHtml } from "@/lib/html-escape";
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
  primary: "var(--brand)",
  success: "var(--green)",
  warning: "var(--amber)",
  danger: "var(--red)",
  info: "var(--brand-sky)",
  purple: "var(--brand-indigo)",
};

const CONFIDENCE_COLORS = [
  { range: "90-100%", color: "var(--green)", fill: "var(--green-fill)" },
  { range: "70-89%", color: "var(--brand)", fill: "var(--brand-soft)" },
  { range: "50-69%", color: "var(--amber)", fill: "var(--amber-fill)" },
  { range: "0-49%", color: "var(--red)", fill: "var(--red-fill)" },
];

function StatCard({
  icon: Icon,
  label,
  value,
  change,
  changeType,
  suffix,
  iconBg = "bg-glass-inset-gray",
  iconColor = "text-ink-secondary",
  valueColor = "text-ink",
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
    <div className="glass p-6">
      <div className="flex items-center">
        <div className={`flex-shrink-0 p-2 rounded-lg ${iconBg}`}>
          <Icon className={`h-5 w-5 ${iconColor}`} />
        </div>
        <div className="ml-4 w-0 flex-1">
          <dl>
            <dt className="text-sm font-medium text-ink-secondary truncate">
              {label}
            </dt>
            <dd className="flex items-baseline">
              <div className={`text-2xl font-semibold ${valueColor}`}>
                {value}
                {suffix && (
                  <span className="text-lg font-normal text-ink-secondary ml-1">
                    {suffix}
                  </span>
                )}
              </div>
              {change && (
                <div
                  className={`ml-2 flex items-baseline text-sm font-semibold ${
                    changeType === "positive"
                      ? "text-success-text"
                      : changeType === "negative"
                      ? "text-danger-text"
                      : "text-ink-secondary"
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
        return "bg-success-fill text-success-text";
      case "pending":
        return "bg-warning-fill text-warning-text";
      case "inactive":
      case "revoked":
        return "bg-danger-fill text-danger-text";
      default:
        return "bg-glass-inset-gray text-ink-body";
    }
  };

  return (
    <span
      className={`inline-flex items-center px-2.5 py-0.5 rounded-pill text-xs font-medium ${getStatusStyles(
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
      return "bg-success-fill text-success-text";
    if (score >= 70)
      return "bg-brand-soft text-brand-text";
    if (score >= 50)
      return "bg-warning-fill text-warning-text";
    return "bg-danger-fill text-danger-text";
  };

  return (
    <span
      className={`inline-flex items-center px-2.5 py-0.5 rounded-pill text-xs font-medium ${getScoreStyles(
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

  // Debounce search inputs for better performance
  const debouncedMcpSearch = useDebounce(mcpSearchFilter, 300);
  const debouncedConnSearch = useDebounce(connSearchFilter, 300);

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

  // Filtered and paginated MCP servers (memoized for performance)
  const filteredMcpServers = useMemo(() => {
    return mcpServers.filter((server) => {
      const matchesSearch = debouncedMcpSearch === "" ||
        server.name.toLowerCase().includes(debouncedMcpSearch.toLowerCase()) ||
        server.url.toLowerCase().includes(debouncedMcpSearch.toLowerCase());
      const matchesStatus = mcpStatusFilter === "all" || server.status === mcpStatusFilter;
      return matchesSearch && matchesStatus;
    });
  }, [mcpServers, debouncedMcpSearch, mcpStatusFilter]);
  const totalMcpPages = Math.ceil(filteredMcpServers.length / mcpPageSize);
  const paginatedMcpServers = filteredMcpServers.slice(
    (mcpPage - 1) * mcpPageSize,
    mcpPage * mcpPageSize
  );

  // Filtered and paginated connections (memoized for performance)
  const filteredConnections = useMemo(() => {
    return connections.filter((conn) => {
      const matchesSearch = debouncedConnSearch === "" ||
        conn.agentName.toLowerCase().includes(debouncedConnSearch.toLowerCase()) ||
        conn.mcpServerName.toLowerCase().includes(debouncedConnSearch.toLowerCase());
      const matchesStatus = connStatusFilter === "all" ||
        (connStatusFilter === "active" && conn.isActive) ||
        (connStatusFilter === "inactive" && !conn.isActive);
      return matchesSearch && matchesStatus;
    });
  }, [connections, debouncedConnSearch, connStatusFilter]);
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
    // Every data value is escaped: the report is written into a same-origin window.
    const esc = escapeHtml;
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
          <p>Version: ${esc(data.version)}</p>
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
          <tr><th>Name</th><th>Status</th><th>Trust score</th><th>Capabilities</th><th>Data access</th></tr>
          ${data.agents.map((a) => `
            <tr>
              <td>${esc(a.name)}</td>
              <td><span class="badge badge-${a.status === "active" ? "green" : "yellow"}">${esc(a.status)}</span></td>
              <td>${Math.round((a.trustScore || 0) * 100)}%</td>
              <td>${(a.capabilities || []).map((c) => `<span class="badge badge-blue">${esc(c)}</span>`).join(" ")}</td>
              <td>${(a.dataAccess || []).map((d) => `<span class="badge badge-yellow">${esc(d)}</span>`).join(" ")}</td>
            </tr>
          `).join("")}
        </table>

        <h2>MCP Servers</h2>
        <table>
          <tr><th>Name</th><th>URL</th><th>Status</th><th>Tools</th><th>Attestations</th></tr>
          ${data.mcpServers.map((s) => `
            <tr>
              <td>${esc(s.name)}</td>
              <td style="font-size:12px;color:#6b7280;">${esc(s.url)}</td>
              <td><span class="badge badge-${s.status === "verified" ? "green" : "yellow"}">${esc(s.status)}</span></td>
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
              <td>${esc(c.agentName)}</td>
              <td>${esc(c.mcpServerName)}</td>
              <td>${esc(c.connectionType)}</td>
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
          <Loader2 className="h-8 w-8 animate-spin text-brand-text mx-auto mb-4" />
          <p className="text-ink-secondary">
            Loading supply chain data...
          </p>
        </div>
      </div>
    );
  }

  return (
    <div className="min-h-screen p-6">
      {/* Header */}
      <div className="mb-6">
        <div className="flex items-center justify-between">
          <div>
            <h1 className="text-3xl font-bold text-ink flex items-center gap-3">
              <GitBranch className="h-8 w-8 text-brand-text" />
              Supply chain & ABOM
            </h1>
            <p className="mt-2 text-ink-secondary">
              Monitor MCP server dependencies, attestation health, and your Agent Bill of Materials.
            </p>
          </div>
          <div className="flex gap-3">
            <button
              onClick={() => fetchData(true)}
              disabled={refreshing}
              className="inline-flex items-center px-4 py-2 border border-stroke rounded-pill text-sm font-medium text-ink-body hover:bg-glass-inset-gray disabled:opacity-50"
            >
              <RefreshCw
                className={`h-4 w-4 mr-2 ${refreshing ? "animate-spin" : ""}`}
              />
              Refresh
            </button>
            {activeTab === "analytics" ? (
              <button
                onClick={handleExport}
                className="inline-flex items-center px-4 py-2 rounded-pill bg-brand text-sm font-medium text-white shadow-glow hover:bg-brand-hover"
              >
                <Download className="h-4 w-4 mr-2" />
                Export report
              </button>
            ) : (
              <div className="relative">
                <button
                  onClick={() => {
                    const dropdown = document.getElementById("abom-export-dropdown");
                    if (dropdown) dropdown.classList.toggle("hidden");
                  }}
                  className="inline-flex items-center px-4 py-2 rounded-pill bg-brand text-sm font-medium text-white shadow-glow hover:bg-brand-hover"
                >
                  <Download className="h-4 w-4 mr-2" />
                  Export ABOM
                </button>
                <div
                  id="abom-export-dropdown"
                  className="hidden absolute right-0 mt-2 w-48 glass-chrome overflow-hidden z-10"
                >
                  <button
                    onClick={() => {
                      handleExportABOM("json");
                      document.getElementById("abom-export-dropdown")?.classList.add("hidden");
                    }}
                    className="flex items-center w-full px-4 py-2 text-sm text-ink-body hover:bg-glass-inset-gray"
                  >
                    <FileJson className="h-4 w-4 mr-2 text-brand-text" />
                    Export as JSON
                  </button>
                  <button
                    onClick={() => {
                      handleExportABOM("yaml");
                      document.getElementById("abom-export-dropdown")?.classList.add("hidden");
                    }}
                    className="flex items-center w-full px-4 py-2 text-sm text-ink-body hover:bg-glass-inset-gray"
                  >
                    <FileCode className="h-4 w-4 mr-2 text-success-text" />
                    Export as YAML
                  </button>
                  <button
                    onClick={() => {
                      handleExportABOM("pdf");
                      document.getElementById("abom-export-dropdown")?.classList.add("hidden");
                    }}
                    className="flex items-center w-full px-4 py-2 text-sm text-ink-body hover:bg-glass-inset-gray"
                  >
                    <Printer className="h-4 w-4 mr-2 text-danger-text" />
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
        <div className="border-b border-divider">
          <nav className="-mb-px flex space-x-8">
            <button
              onClick={() => setActiveTab("analytics")}
              className={`py-4 px-1 border-b-2 font-medium text-sm flex items-center gap-2 ${
                activeTab === "analytics"
                  ? "border-brand text-brand-text"
                  : "border-transparent text-ink-secondary hover:text-ink hover:border-stroke"
              }`}
            >
              <BarChart3 className="h-4 w-4" />
              Analytics
              <span className="ml-1 px-2 py-0.5 text-xs font-medium bg-glass-inset-gray text-ink-secondary rounded-pill">
                {stats?.totalMCPServers || 0}
              </span>
            </button>
            <button
              onClick={() => setActiveTab("abom")}
              className={`py-4 px-1 border-b-2 font-medium text-sm flex items-center gap-2 ${
                activeTab === "abom"
                  ? "border-brand text-brand-text"
                  : "border-transparent text-ink-secondary hover:text-ink hover:border-stroke"
              }`}
            >
              <Package className="h-4 w-4" />
              ABOM
              <span className="ml-1 px-2 py-0.5 text-xs font-medium bg-brand-soft text-brand-text rounded-pill">
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
          label="Total MCP servers"
          value={stats?.totalMCPServers || 0}
          iconBg="bg-brand-soft"
          iconColor="text-brand-indigo"
          valueColor="text-brand-indigo"
        />
        <StatCard
          icon={Shield}
          label="Verified servers"
          value={stats?.verifiedServers || 0}
          change={
            stats && stats.totalMCPServers > 0
              ? `${((stats.verifiedServers / stats.totalMCPServers) * 100).toFixed(0)}%`
              : undefined
          }
          changeType="positive"
          iconBg="bg-success-fill"
          iconColor="text-success-text"
          valueColor="text-success-text"
        />
        <StatCard
          icon={Link2}
          label="Active connections"
          value={stats?.activeConnections || 0}
          suffix={`/ ${stats?.totalConnections || 0}`}
          iconBg="bg-brand-soft"
          iconColor="text-brand-sky"
          valueColor="text-brand-sky"
        />
        <StatCard
          icon={AlertCircle}
          label="Unmapped MCPs"
          value={unmappedMcpCount}
          changeType={
            unmappedMcpCount > 0 ? "negative" : "positive"
          }
          iconBg="bg-warning-fill"
          iconColor="text-warning-text"
          valueColor="text-warning-text"
        />
      </div>

      {/* Secondary Stats */}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4 mb-8">
        <StatCard
          icon={Clock}
          label="Pending verification"
          value={stats?.pendingServers || 0}
          changeType={
            (stats?.pendingServers || 0) > 0 ? "negative" : "positive"
          }
          iconBg="bg-warning-fill"
          iconColor="text-warning-text"
          valueColor="text-warning-text"
        />
        <StatCard
          icon={Activity}
          label="Attestations (24h)"
          value={stats?.attestationsLast24h || 0}
          iconBg="bg-brand-soft"
          iconColor="text-brand-text"
          valueColor="text-brand-text"
        />
        <StatCard
          icon={AlertTriangle}
          label="Capability drift alerts"
          value={stats?.capabilityDriftAlerts || 0}
          changeType={
            (stats?.capabilityDriftAlerts || 0) > 0 ? "negative" : "positive"
          }
          iconBg="bg-danger-fill"
          iconColor="text-danger-text"
          valueColor="text-danger-text"
        />
        <StatCard
          icon={Box}
          label="Total tools"
          value={mcpServers.reduce(
            (sum, s) => sum + (s.toolCount || 0),
            0
          )}
          iconBg="bg-brand-soft"
          iconColor="text-brand-indigo"
          valueColor="text-brand-indigo"
        />
      </div>

      {/* Charts Row */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6 mb-8">
        {/* Attestation Trend Chart */}
        <div className="glass p-6">
          <h3 className="text-lg font-semibold text-ink mb-4 flex items-center gap-2">
            <BarChart3 className="h-5 w-5 text-brand-text" />
            Attestation activity (7 days)
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
                  className="stroke-divider"
                />
                <XAxis
                  dataKey="date"
                  tick={{ fill: "var(--text-tertiary)", fontSize: 12 }}
                />
                <YAxis tick={{ fill: "var(--text-tertiary)", fontSize: 12 }} />
                <Tooltip
                  contentStyle={{
                    backgroundColor: "var(--glass-fill)",
                    border: "1px solid var(--glass-border)",
                    borderRadius: "12px",
                    boxShadow: "var(--shadow-card)",
                    color: "var(--text-primary)",
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
        <div className="glass p-6">
          <h3 className="text-lg font-semibold text-ink mb-4 flex items-center gap-2">
            <Shield className="h-5 w-5 text-success-text" />
            Confidence score distribution
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
                  label={({ name, value }: { name: string; value: number }) => (value > 0 ? `${value}` : "")}
                >
                  {confidenceDistribution.map((entry, index) => (
                    <Cell key={`cell-${index}`} fill={entry.color} />
                  ))}
                </Pie>
                <Tooltip
                  contentStyle={{
                    backgroundColor: "var(--glass-fill)",
                    border: "1px solid var(--glass-border)",
                    borderRadius: "12px",
                    boxShadow: "var(--shadow-card)",
                    color: "var(--text-primary)",
                  }}
                />
                <Legend
                  wrapperStyle={{ paddingTop: "20px" }}
                  formatter={(value: string) => (
                    <span className="text-ink-secondary text-sm">
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
      <div className="glass overflow-hidden mb-8">
        <div className="px-6 py-4 border-b border-divider">
          <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4">
            <div>
              <h3 className="text-lg font-semibold text-ink flex items-center gap-2">
                <Server className="h-5 w-5 text-brand-indigo" />
                MCP server dependencies
                <span className="ml-2 px-2 py-0.5 text-xs font-medium bg-glass-inset-gray text-ink-secondary rounded-pill">
                  {filteredMcpServers.length}
                </span>
              </h3>
              <p className="mt-1 text-sm text-ink-secondary">
                All MCP servers in your supply chain with their attestation status
              </p>
            </div>
            <div className="flex flex-col sm:flex-row gap-2">
              <div className="relative">
                <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-ink-tertiary" />
                <input
                  type="text"
                  placeholder="Search servers..."
                  value={mcpSearchFilter}
                  onChange={(e) => {
                    setMcpSearchFilter(e.target.value);
                    setMcpPage(1);
                  }}
                  className="pl-9 pr-3 py-2 text-sm border border-stroke rounded-inset-sm bg-glass-inset text-ink placeholder:text-ink-tertiary focus:outline-none focus:ring-2 focus:ring-brand focus:border-transparent w-full sm:w-48"
                />
              </div>
              <select
                value={mcpStatusFilter}
                onChange={(e) => {
                  setMcpStatusFilter(e.target.value);
                  setMcpPage(1);
                }}
                className="px-3 py-2 text-sm border border-stroke rounded-inset-sm bg-glass-inset text-ink focus:outline-none focus:ring-2 focus:ring-brand focus:border-transparent"
              >
                <option value="all">All statuses</option>
                <option value="verified">Verified</option>
                <option value="pending">Pending</option>
                <option value="active">Active</option>
                <option value="inactive">Inactive</option>
              </select>
            </div>
          </div>
        </div>
        <div className="overflow-x-auto">
          <table className="min-w-full divide-y divide-divider">
            <thead className="bg-glass-inset-gray">
              <tr>
                <th className="px-6 py-3 text-left text-xs font-medium text-ink-secondary uppercase tracking-wider">
                  Server
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium text-ink-secondary uppercase tracking-wider">
                  Status
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium text-ink-secondary uppercase tracking-wider">
                  Attestations
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium text-ink-secondary uppercase tracking-wider">
                  Capabilities
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium text-ink-secondary uppercase tracking-wider">
                  Last attested
                </th>
                <th className="px-6 py-3 text-right text-xs font-medium text-ink-secondary uppercase tracking-wider">
                  Actions
                </th>
              </tr>
            </thead>
            <tbody className="divide-y divide-divider">
              {paginatedMcpServers.length === 0 ? (
                <tr>
                  <td
                    colSpan={6}
                    className="px-6 py-12 text-center text-ink-secondary"
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
                    className="hover:bg-glass-inset-gray"
                  >
                    <td className="px-6 py-4 whitespace-nowrap">
                      <div className="flex items-center">
                        <Server className="h-5 w-5 text-ink-tertiary mr-3" />
                        <div>
                          <div className="text-sm font-medium text-ink">
                            {server.name}
                          </div>
                          <div className="text-sm text-ink-secondary truncate max-w-xs">
                            {server.url}
                          </div>
                        </div>
                      </div>
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap">
                      <StatusBadge status={server.status} />
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap text-sm text-ink">
                      {server.attestationCount || 0}
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap text-sm text-ink-secondary">
                      {server.toolCount || 0} tools
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap text-sm text-ink-secondary">
                      {server.lastAttestedAt || server.lastVerifiedAt
                        ? formatDateTime(server.lastAttestedAt || server.lastVerifiedAt || "")
                        : "Never"}
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap text-right text-sm font-medium">
                      <button
                        onClick={() =>
                          router.push(`/dashboard/mcp/${server.id}`)
                        }
                        className="text-brand-text hover:text-brand"
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
          <div className="px-6 py-4 border-t border-divider flex flex-col sm:flex-row items-center justify-between gap-4">
            <div className="flex items-center gap-2 text-sm text-ink-secondary">
              <span>Show</span>
              <select
                value={mcpPageSize}
                onChange={(e) => {
                  setMcpPageSize(Number(e.target.value));
                  setMcpPage(1);
                }}
                className="px-2 py-1 border border-stroke rounded-inset-sm bg-glass-inset text-ink text-sm"
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
                className="p-2 border border-stroke rounded-inset-sm bg-glass-inset text-ink-body hover:bg-glass-inset-gray disabled:opacity-50 disabled:cursor-not-allowed"
              >
                <ChevronLeft className="h-4 w-4" />
              </button>
              <span className="text-sm text-ink-body">
                Page {mcpPage} of {totalMcpPages || 1}
              </span>
              <button
                onClick={() => setMcpPage((p) => Math.min(totalMcpPages, p + 1))}
                disabled={mcpPage >= totalMcpPages}
                className="p-2 border border-stroke rounded-inset-sm bg-glass-inset text-ink-body hover:bg-glass-inset-gray disabled:opacity-50 disabled:cursor-not-allowed"
              >
                <ChevronRight className="h-4 w-4" />
              </button>
            </div>
          </div>
        )}
      </div>

      {/* Agent Connections Table */}
      <div className="glass overflow-hidden">
        <div className="px-6 py-4 border-b border-divider">
          <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4">
            <div>
              <h3 className="text-lg font-semibold text-ink flex items-center gap-2">
                <Link2 className="h-5 w-5 text-brand-sky" />
                Agent-MCP connections
                <span className="ml-2 px-2 py-0.5 text-xs font-medium bg-glass-inset-gray text-ink-secondary rounded-pill">
                  {filteredConnections.length}
                </span>
              </h3>
              <p className="mt-1 text-sm text-ink-secondary">
                Active connections between agents and MCP servers
              </p>
            </div>
            <div className="flex flex-col sm:flex-row gap-2">
              <div className="relative">
                <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-ink-tertiary" />
                <input
                  type="text"
                  placeholder="Search agents or servers..."
                  value={connSearchFilter}
                  onChange={(e) => {
                    setConnSearchFilter(e.target.value);
                    setConnPage(1);
                  }}
                  className="pl-9 pr-3 py-2 text-sm border border-stroke rounded-inset-sm bg-glass-inset text-ink placeholder:text-ink-tertiary focus:outline-none focus:ring-2 focus:ring-brand focus:border-transparent w-full sm:w-56"
                />
              </div>
              <select
                value={connStatusFilter}
                onChange={(e) => {
                  setConnStatusFilter(e.target.value);
                  setConnPage(1);
                }}
                className="px-3 py-2 text-sm border border-stroke rounded-inset-sm bg-glass-inset text-ink focus:outline-none focus:ring-2 focus:ring-brand focus:border-transparent"
              >
                <option value="all">All statuses</option>
                <option value="active">Active</option>
                <option value="inactive">Inactive</option>
              </select>
            </div>
          </div>
        </div>
        <div className="overflow-x-auto">
          <table className="min-w-full divide-y divide-divider">
            <thead className="bg-glass-inset-gray">
              <tr>
                <th className="px-6 py-3 text-left text-xs font-medium text-ink-secondary uppercase tracking-wider">
                  Agent
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium text-ink-secondary uppercase tracking-wider">
                  MCP server
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium text-ink-secondary uppercase tracking-wider">
                  Connection type
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium text-ink-secondary uppercase tracking-wider">
                  Attestations
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium text-ink-secondary uppercase tracking-wider">
                  First connected
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium text-ink-secondary uppercase tracking-wider">
                  Last attested
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium text-ink-secondary uppercase tracking-wider">
                  Status
                </th>
              </tr>
            </thead>
            <tbody className="divide-y divide-divider">
              {paginatedConnections.length === 0 ? (
                <tr>
                  <td
                    colSpan={7}
                    className="px-6 py-12 text-center text-ink-secondary"
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
                    className="hover:bg-glass-inset-gray"
                  >
                    <td className="px-6 py-4 whitespace-nowrap">
                      <div className="text-sm font-medium text-ink">
                        {conn.agentName || conn.agentId.slice(0, 8)}
                      </div>
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap">
                      <div className="text-sm text-ink">
                        {conn.mcpServerName || conn.mcpServerId.slice(0, 8)}
                      </div>
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap">
                      <span
                        className={`inline-flex items-center px-2.5 py-0.5 rounded-pill text-xs font-medium ${
                          conn.connectionType === "attested"
                            ? "bg-success-fill text-success-text"
                            : "bg-glass-inset-gray text-ink-body"
                        }`}
                      >
                        {conn.connectionType}
                      </span>
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap text-sm text-ink">
                      {conn.attestationCount}
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap text-sm text-ink-secondary">
                      {formatDateTime(conn.firstConnectedAt)}
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap text-sm text-ink-secondary">
                      {conn.lastAttestedAt
                        ? formatDateTime(conn.lastAttestedAt)
                        : "Never"}
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap">
                      {conn.isActive ? (
                        <span className="inline-flex items-center text-success-text">
                          <CheckCircle2 className="h-4 w-4 mr-1" />
                          Active
                        </span>
                      ) : (
                        <span className="inline-flex items-center text-ink-secondary">
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
          <div className="px-6 py-4 border-t border-divider flex flex-col sm:flex-row items-center justify-between gap-4">
            <div className="flex items-center gap-2 text-sm text-ink-secondary">
              <span>Show</span>
              <select
                value={connPageSize}
                onChange={(e) => {
                  setConnPageSize(Number(e.target.value));
                  setConnPage(1);
                }}
                className="px-2 py-1 border border-stroke rounded-inset-sm bg-glass-inset text-ink text-sm"
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
                className="p-2 border border-stroke rounded-inset-sm bg-glass-inset text-ink-body hover:bg-glass-inset-gray disabled:opacity-50 disabled:cursor-not-allowed"
              >
                <ChevronLeft className="h-4 w-4" />
              </button>
              <span className="text-sm text-ink-body">
                Page {connPage} of {totalConnPages || 1}
              </span>
              <button
                onClick={() => setConnPage((p) => Math.min(totalConnPages, p + 1))}
                disabled={connPage >= totalConnPages}
                className="p-2 border border-stroke rounded-inset-sm bg-glass-inset text-ink-body hover:bg-glass-inset-gray disabled:opacity-50 disabled:cursor-not-allowed"
              >
                <ChevronRight className="h-4 w-4" />
              </button>
            </div>
          </div>
        )}
      </div>

      {/* Capability Drift Alerts */}
      {driftAlerts.length > 0 && (
        <div className="glass overflow-hidden mt-8">
          <div className="px-6 py-4 border-b border-divider">
            <h3 className="text-lg font-semibold text-ink flex items-center gap-2">
              <AlertTriangle className="h-5 w-5 text-warning-text" />
              Capability drift alerts
              <span className="ml-2 px-2 py-0.5 text-xs font-medium bg-warning-fill text-warning-text rounded-pill">
                {driftAlertCount}
              </span>
            </h3>
            <p className="mt-1 text-sm text-ink-secondary">
              Detected changes in MCP server capabilities that may require attention
            </p>
          </div>
          <div className="overflow-x-auto">
            <table className="min-w-full divide-y divide-divider">
              <thead className="bg-glass-inset-gray">
                <tr>
                  <th className="px-6 py-3 text-left text-xs font-medium text-ink-secondary uppercase tracking-wider">
                    Severity
                  </th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-ink-secondary uppercase tracking-wider">
                    MCP server
                  </th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-ink-secondary uppercase tracking-wider">
                    Capability
                  </th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-ink-secondary uppercase tracking-wider">
                    Change type
                  </th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-ink-secondary uppercase tracking-wider">
                    Description
                  </th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-ink-secondary uppercase tracking-wider">
                    Detected
                  </th>
                </tr>
              </thead>
              <tbody className="divide-y divide-divider">
                {paginatedDriftAlerts.map((alert) => (
                  <tr
                    key={alert.id}
                    className="hover:bg-glass-inset-gray"
                  >
                    <td className="px-6 py-4 whitespace-nowrap">
                      <span
                        className={`inline-flex items-center px-2.5 py-0.5 rounded-pill text-xs font-medium ${
                          alert.severity === "high"
                            ? "bg-danger-fill text-danger-text"
                            : alert.severity === "medium"
                            ? "bg-warning-fill text-warning-text"
                            : "bg-brand-soft text-brand-text"
                        }`}
                      >
                        {alert.severity === "high" && (
                          <AlertCircle className="h-3 w-3 mr-1" />
                        )}
                        {alert.severity}
                      </span>
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap">
                      <div className="text-sm font-medium text-ink">
                        {alert.mcpServerName}
                      </div>
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap">
                      <div className="text-sm text-ink">
                        {alert.capabilityName}
                      </div>
                      <div className="text-xs text-ink-secondary">
                        {alert.capabilityType}
                      </div>
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap">
                      <span
                        className={`inline-flex items-center px-2.5 py-0.5 rounded-pill text-xs font-medium ${
                          alert.driftType === "added"
                            ? "bg-success-fill text-success-text"
                            : alert.driftType === "removed"
                            ? "bg-danger-fill text-danger-text"
                            : "bg-warning-fill text-warning-text"
                        }`}
                      >
                        {alert.driftType}
                      </span>
                    </td>
                    <td className="px-6 py-4">
                      <div
                        className="text-sm text-ink-secondary max-w-md truncate"
                        title={alert.description}
                      >
                        {alert.description}
                      </div>
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap text-sm text-ink-secondary">
                      {formatDateTime(alert.detectedAt)}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          {/* Drift Alerts Pagination */}
          {driftAlerts.length > 0 && (
            <div className="px-6 py-4 border-t border-divider flex flex-col sm:flex-row items-center justify-between gap-4">
              <div className="flex items-center gap-2 text-sm text-ink-secondary">
                <span>Show</span>
                <select
                  value={driftPageSize}
                  onChange={(e) => {
                    setDriftPageSize(Number(e.target.value));
                    setDriftPage(1);
                  }}
                  className="px-2 py-1 border border-stroke rounded-inset-sm bg-glass-inset text-ink text-sm"
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
                  className="p-2 border border-stroke rounded-inset-sm bg-glass-inset text-ink-body hover:bg-glass-inset-gray disabled:opacity-50 disabled:cursor-not-allowed"
                >
                  <ChevronLeft className="h-4 w-4" />
                </button>
                <span className="text-sm text-ink-body">
                  Page {driftPage} of {totalDriftPages || 1}
                </span>
                <button
                  onClick={() => setDriftPage((p) => Math.min(totalDriftPages, p + 1))}
                  disabled={driftPage >= totalDriftPages}
                  className="p-2 border border-stroke rounded-inset-sm bg-glass-inset text-ink-body hover:bg-glass-inset-gray disabled:opacity-50 disabled:cursor-not-allowed"
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
          <div className="glass p-6">
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-4">
                <Package className="h-8 w-8 text-ink-tertiary" />
                <div>
                  <h2 className="text-xl font-semibold text-ink">Agent Bill of Materials</h2>
                  <p className="text-sm text-ink-secondary mt-1">
                    Complete inventory of all agents, MCP servers, tools, and data access patterns observed by AIM
                  </p>
                </div>
              </div>
              <div className="text-right">
                <p className="text-xs text-ink-secondary">Generated</p>
                <p className="text-sm font-medium text-ink">{new Date(abomData.generatedAt).toLocaleString()}</p>
                <p className="text-xs text-ink-secondary mt-1">Version {abomData.version}</p>
              </div>
            </div>
          </div>

          {/* ABOM Summary Stats */}
          <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-6 gap-4">
            <div className="glass p-4">
              <div className="flex items-center gap-3">
                <div className="p-2 bg-brand-soft rounded-lg">
                  <Bot className="h-5 w-5 text-brand-text" />
                </div>
                <div>
                  <p className="text-2xl font-bold text-brand-text">{abomData.summary.totalAgents}</p>
                  <p className="text-xs text-ink-secondary">Agents</p>
                </div>
              </div>
            </div>
            <div className="glass p-4">
              <div className="flex items-center gap-3">
                <div className="p-2 bg-brand-soft rounded-lg">
                  <Server className="h-5 w-5 text-brand-indigo" />
                </div>
                <div>
                  <p className="text-2xl font-bold text-brand-indigo">{abomData.summary.totalMcpServers}</p>
                  <p className="text-xs text-ink-secondary">MCP servers</p>
                </div>
              </div>
            </div>
            <div className="glass p-4">
              <div className="flex items-center gap-3">
                <div className="p-2 bg-success-fill rounded-lg">
                  <Wrench className="h-5 w-5 text-success-text" />
                </div>
                <div>
                  <p className="text-2xl font-bold text-success-text">{abomData.summary.totalTools}</p>
                  <p className="text-xs text-ink-secondary">Tools</p>
                </div>
              </div>
            </div>
            <div className="glass p-4">
              <div className="flex items-center gap-3">
                <div className="p-2 bg-brand-soft rounded-lg">
                  <Link2 className="h-5 w-5 text-brand-sky" />
                </div>
                <div>
                  <p className="text-2xl font-bold text-brand-sky">{abomData.summary.totalConnections}</p>
                  <p className="text-xs text-ink-secondary">Connections</p>
                </div>
              </div>
            </div>
            <div className="glass p-4">
              <div className="flex items-center gap-3">
                <div className="p-2 bg-warning-fill rounded-lg">
                  <Cpu className="h-5 w-5 text-warning-text" />
                </div>
                <div>
                  <p className="text-2xl font-bold text-warning-text">{abomData.summary.totalCapabilities}</p>
                  <p className="text-xs text-ink-secondary">Capabilities</p>
                </div>
              </div>
            </div>
            <div className="glass p-4">
              <div className="flex items-center gap-3">
                <div className="p-2 bg-warning-fill rounded-lg">
                  <Database className="h-5 w-5 text-warning-text" />
                </div>
                <div>
                  <p className="text-2xl font-bold text-warning-text">{abomData.summary.dataCategories.length}</p>
                  <p className="text-xs text-ink-secondary">Data categories</p>
                </div>
              </div>
            </div>
          </div>

          {/* Agents Inventory */}
          <div className="glass overflow-hidden">
            <div className="px-6 py-4 border-b border-divider">
              <h3 className="text-lg font-semibold text-ink flex items-center gap-2">
                <Bot className="h-5 w-5 text-ink-secondary" />
                Agents inventory
                <span className="ml-2 px-2 py-0.5 text-xs font-medium bg-glass-inset-gray text-ink-secondary rounded-pill">
                  {agents.length}
                </span>
              </h3>
              <p className="mt-1 text-sm text-ink-secondary">
                All registered AI agents with their capabilities and data access permissions
              </p>
            </div>
            <div className="overflow-x-auto">
              <table className="min-w-full divide-y divide-divider">
                <thead className="bg-glass-inset-gray">
                  <tr>
                    <th className="px-6 py-3 text-left text-xs font-medium text-ink-secondary uppercase tracking-wider">Agent</th>
                    <th className="px-6 py-3 text-left text-xs font-medium text-ink-secondary uppercase tracking-wider">Status</th>
                    <th className="px-6 py-3 text-left text-xs font-medium text-ink-secondary uppercase tracking-wider">Trust score</th>
                    <th className="px-6 py-3 text-left text-xs font-medium text-ink-secondary uppercase tracking-wider">Capabilities</th>
                    <th className="px-6 py-3 text-left text-xs font-medium text-ink-secondary uppercase tracking-wider">Data access</th>
                    <th className="px-6 py-3 text-left text-xs font-medium text-ink-secondary uppercase tracking-wider">Registered by</th>
                    <th className="px-6 py-3 text-left text-xs font-medium text-ink-secondary uppercase tracking-wider">Created</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-divider">
                  {agents.length === 0 ? (
                    <tr>
                      <td colSpan={7} className="px-6 py-12 text-center text-ink-secondary">
                        <Bot className="h-12 w-12 mx-auto mb-4 opacity-50" />
                        <p className="text-lg font-medium">No agents registered</p>
                        <p className="text-sm mt-1">Agents will appear here when registered with AIM</p>
                      </td>
                    </tr>
                  ) : (
                    paginatedAbomAgents.map((agent) => (
                      <tr
                        key={agent.id}
                        className="hover:bg-glass-inset-gray cursor-pointer"
                        onClick={() => setSelectedAgent(agent)}
                      >
                        <td className="px-6 py-4 whitespace-nowrap">
                          <div className="flex items-center">
                            <Bot className="h-5 w-5 text-ink-tertiary mr-3" />
                            <div>
                              <div className="text-sm font-medium text-ink">{agent.name}</div>
                              <div className="text-xs text-ink-secondary">{agent.id.slice(0, 8)}...</div>
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
                              <span key={idx} className="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-glass-inset-gray text-ink-body">
                                {cap}
                              </span>
                            ))}
                            {(agent.capabilities || []).length > 3 && (
                              <span className="text-xs text-ink-secondary">+{agent.capabilities.length - 3} more</span>
                            )}
                            {(!agent.capabilities || agent.capabilities.length === 0) && (
                              <span className="text-xs text-ink-tertiary">None declared</span>
                            )}
                          </div>
                        </td>
                        <td className="px-6 py-4">
                          <div className="flex flex-wrap gap-1">
                            {(agent.dataAccess || []).slice(0, 2).map((data, idx) => (
                              <span key={idx} className="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-glass-inset-gray text-ink-body">
                                {data}
                              </span>
                            ))}
                            {(agent.dataAccess || []).length > 2 && (
                              <span className="text-xs text-ink-secondary">+{agent.dataAccess!.length - 2} more</span>
                            )}
                            {(!agent.dataAccess || agent.dataAccess.length === 0) && (
                              <span className="text-xs text-ink-tertiary">None</span>
                            )}
                          </div>
                        </td>
                        <td className="px-6 py-4 whitespace-nowrap">
                          <div className="flex items-center gap-2">
                            <div className={`w-6 h-6 rounded-full flex items-center justify-center ${
                              agent.createdBySdkTokenId
                                ? "bg-brand-soft"
                                : agent.createdByApiKeyId
                                  ? "bg-warning-fill"
                                  : "bg-brand-soft"
                            }`}>
                              {agent.createdBySdkTokenId ? (
                                <Code className="h-3.5 w-3.5 text-brand-indigo" />
                              ) : agent.createdByApiKeyId ? (
                                <Key className="h-3.5 w-3.5 text-warning-text" />
                              ) : (
                                <User className="h-3.5 w-3.5 text-brand-text" />
                              )}
                            </div>
                            <div>
                              <div className="text-sm text-ink">
                                {agent.createdByName || agent.createdByEmail || (agent.createdBySdkTokenId ? "SDK" : agent.createdByApiKeyId ? "API key" : "Unknown")}
                              </div>
                              {agent.createdByEmail && agent.createdByName && (
                                <div className="text-xs text-ink-secondary">{agent.createdByEmail}</div>
                              )}
                              {!agent.createdByName && !agent.createdByEmail && agent.createdBySdkTokenId && (
                                <div className="text-xs text-ink-secondary">via SDK token</div>
                              )}
                            </div>
                          </div>
                        </td>
                        <td className="px-6 py-4 whitespace-nowrap text-sm text-ink-secondary">
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
              <div className="px-6 py-4 border-t border-divider flex items-center justify-between">
                <div className="text-sm text-ink-secondary">
                  Showing {((abomAgentPage - 1) * abomAgentPageSize) + 1} to {Math.min(abomAgentPage * abomAgentPageSize, agents.length)} of {agents.length} agents
                </div>
                <div className="flex items-center gap-2">
                  <button
                    onClick={() => setAbomAgentPage(p => Math.max(1, p - 1))}
                    disabled={abomAgentPage === 1}
                    className="px-3 py-1.5 text-sm font-medium text-ink-body border border-stroke rounded-pill hover:bg-glass-inset-gray disabled:opacity-50 disabled:cursor-not-allowed"
                  >
                    Previous
                  </button>
                  <span className="text-sm text-ink-secondary">
                    Page {abomAgentPage} of {totalAbomAgentPages}
                  </span>
                  <button
                    onClick={() => setAbomAgentPage(p => Math.min(totalAbomAgentPages, p + 1))}
                    disabled={abomAgentPage === totalAbomAgentPages}
                    className="px-3 py-1.5 text-sm font-medium text-ink-body border border-stroke rounded-pill hover:bg-glass-inset-gray disabled:opacity-50 disabled:cursor-not-allowed"
                  >
                    Next
                  </button>
                </div>
              </div>
            )}
          </div>

          {/* MCP Tools & Servers */}
          <div className="glass overflow-hidden">
            <div className="px-6 py-4 border-b border-divider">
              <h3 className="text-lg font-semibold text-ink flex items-center gap-2">
                <Server className="h-5 w-5 text-ink-secondary" />
                MCP servers & tools
                <span className="ml-2 px-2 py-0.5 text-xs font-medium bg-glass-inset-gray text-ink-secondary rounded-pill">
                  {mcpServers.length} servers
                </span>
                <span className="px-2 py-0.5 text-xs font-medium bg-glass-inset-gray text-ink-secondary rounded-pill">
                  {abomData.summary.totalTools} tools
                </span>
              </h3>
              <p className="mt-1 text-sm text-ink-secondary">
                All MCP servers discovered through agent attestations with their exposed tools
              </p>
            </div>
            <div className="overflow-x-auto">
              <table className="min-w-full divide-y divide-divider">
                <thead className="bg-glass-inset-gray">
                  <tr>
                    <th className="px-6 py-3 text-left text-xs font-medium text-ink-secondary uppercase tracking-wider">Server</th>
                    <th className="px-6 py-3 text-left text-xs font-medium text-ink-secondary uppercase tracking-wider">URL</th>
                    <th className="px-6 py-3 text-left text-xs font-medium text-ink-secondary uppercase tracking-wider">Status</th>
                    <th className="px-6 py-3 text-left text-xs font-medium text-ink-secondary uppercase tracking-wider">Tools</th>
                    <th className="px-6 py-3 text-left text-xs font-medium text-ink-secondary uppercase tracking-wider">Attestations</th>
                    <th className="px-6 py-3 text-left text-xs font-medium text-ink-secondary uppercase tracking-wider">Confidence</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-divider">
                  {mcpServers.length === 0 ? (
                    <tr>
                      <td colSpan={6} className="px-6 py-12 text-center text-ink-secondary">
                        <Server className="h-12 w-12 mx-auto mb-4 opacity-50" />
                        <p className="text-lg font-medium">No MCP servers discovered</p>
                        <p className="text-sm mt-1">MCP servers will appear here when agents attest their usage</p>
                      </td>
                    </tr>
                  ) : (
                    paginatedAbomMcpServers.map((server) => (
                      <tr key={server.id} className="hover:bg-glass-inset-gray">
                        <td className="px-6 py-4 whitespace-nowrap">
                          <div className="flex items-center">
                            <Server className="h-5 w-5 text-ink-tertiary mr-3" />
                            <div className="text-sm font-medium text-ink">{server.name}</div>
                          </div>
                        </td>
                        <td className="px-6 py-4">
                          <div className="text-sm text-ink-secondary truncate max-w-xs">{server.url}</div>
                        </td>
                        <td className="px-6 py-4 whitespace-nowrap">
                          <StatusBadge status={server.status} />
                        </td>
                        <td className="px-6 py-4 whitespace-nowrap">
                          <div className="flex items-center">
                            <Wrench className="h-4 w-4 text-ink-tertiary mr-1" />
                            <span className="text-sm text-ink">{server.toolCount || 0}</span>
                          </div>
                        </td>
                        <td className="px-6 py-4 whitespace-nowrap text-sm text-ink">
                          {server.attestationCount || 0}
                        </td>
                        <td className="px-6 py-4 whitespace-nowrap">
                          {server.confidenceScore ? (
                            <ConfidenceScoreBadge score={server.confidenceScore} />
                          ) : (
                            <span className="text-sm text-ink-tertiary">N/A</span>
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
              <div className="px-6 py-4 border-t border-divider flex items-center justify-between">
                <div className="text-sm text-ink-secondary">
                  Showing {((abomMcpPage - 1) * abomMcpPageSize) + 1} to {Math.min(abomMcpPage * abomMcpPageSize, mcpServers.length)} of {mcpServers.length} servers
                </div>
                <div className="flex items-center gap-2">
                  <button
                    onClick={() => setAbomMcpPage(p => Math.max(1, p - 1))}
                    disabled={abomMcpPage === 1}
                    className="px-3 py-1.5 text-sm font-medium text-ink-body border border-stroke rounded-pill hover:bg-glass-inset-gray disabled:opacity-50 disabled:cursor-not-allowed"
                  >
                    Previous
                  </button>
                  <span className="text-sm text-ink-secondary">
                    Page {abomMcpPage} of {totalAbomMcpPages}
                  </span>
                  <button
                    onClick={() => setAbomMcpPage(p => Math.min(totalAbomMcpPages, p + 1))}
                    disabled={abomMcpPage === totalAbomMcpPages}
                    className="px-3 py-1.5 text-sm font-medium text-ink-body border border-stroke rounded-pill hover:bg-glass-inset-gray disabled:opacity-50 disabled:cursor-not-allowed"
                  >
                    Next
                  </button>
                </div>
              </div>
            )}
          </div>

          {/* Agent-MCP Connections Map */}
          <div className="glass overflow-hidden">
            <div className="px-6 py-4 border-b border-divider">
              <h3 className="text-lg font-semibold text-ink flex items-center gap-2">
                <Link2 className="h-5 w-5 text-ink-secondary" />
                Agent-MCP connection map
                <span className="ml-2 px-2 py-0.5 text-xs font-medium bg-glass-inset-gray text-ink-secondary rounded-pill">
                  {connections.length} connections
                </span>
              </h3>
              <p className="mt-1 text-sm text-ink-secondary">
                Observed connections between agents and MCP servers from attestation data
              </p>
            </div>
            {connections.length === 0 ? (
              <div className="px-6 py-12 text-center text-ink-secondary">
                <Link2 className="h-12 w-12 mx-auto mb-4 opacity-50" />
                <p className="text-lg font-medium">No connections observed</p>
                <p className="text-sm mt-1">Connections will appear here when agents attest MCP server usage</p>
              </div>
            ) : (
              <div className="p-6 grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
                {connections.slice(0, 12).map((conn) => (
                  <div
                    key={conn.id}
                    className="flex items-center gap-3 p-4 rounded-inset bg-glass-inset-gray"
                  >
                    <div className="flex-1">
                      <div className="flex items-center gap-2">
                        <Bot className="h-4 w-4 text-ink-secondary" />
                        <span className="text-sm font-medium text-ink truncate">
                          {conn.agentName}
                        </span>
                      </div>
                    </div>
                    <ArrowRight className="h-4 w-4 text-ink-tertiary flex-shrink-0" />
                    <div className="flex-1">
                      <div className="flex items-center gap-2">
                        <Server className="h-4 w-4 text-ink-secondary" />
                        <span className="text-sm font-medium text-ink truncate">
                          {conn.mcpServerName}
                        </span>
                      </div>
                    </div>
                    <div className="flex-shrink-0">
                      {conn.isActive ? (
                        <CheckCircle2 className="h-4 w-4 text-success-text" />
                      ) : (
                        <XCircle className="h-4 w-4 text-ink-tertiary" />
                      )}
                    </div>
                  </div>
                ))}
                {connections.length > 12 && (
                  <div className="col-span-full text-center text-sm text-ink-secondary py-2">
                    + {connections.length - 12} more connections
                  </div>
                )}
              </div>
            )}
          </div>

          {/* Data Categories */}
          {abomData.summary.dataCategories.length > 0 && (
            <div className="glass p-6">
              <h3 className="text-lg font-semibold text-ink flex items-center gap-2 mb-4">
                <Database className="h-5 w-5 text-ink-secondary" />
                Data access categories
                <span className="ml-2 px-2 py-0.5 text-xs font-medium bg-glass-inset-gray text-ink-secondary rounded-pill">
                  {abomData.summary.dataCategories.length} categories
                </span>
              </h3>
              <p className="text-sm text-ink-secondary mb-4">
                Types of sensitive data that agents in your organization have declared access to
              </p>
              <div className="flex flex-wrap gap-2">
                {abomData.summary.dataCategories.map((cat, idx) => (
                  <span
                    key={idx}
                    className="inline-flex items-center px-3 py-1.5 rounded-lg text-sm font-medium bg-glass-inset-gray text-ink-body"
                  >
                    <Database className="h-4 w-4 mr-2" />
                    {cat}
                  </span>
                ))}
              </div>
            </div>
          )}

          {/* ABOM Notice */}
          <div className="glass p-4">
            <div className="flex items-start gap-3">
              <div className="p-2 bg-glass-inset-gray rounded-inset-sm">
                <Lock className="h-5 w-5 text-ink-secondary" />
              </div>
              <div>
                <h4 className="text-sm font-semibold text-ink">
                  This ABOM is read-only
                </h4>
                <p className="text-sm text-ink-secondary mt-1">
                  The Agent Bill of Materials is automatically generated from AIM observations and attestations.
                  It cannot be manually modified and represents the actual state of your agent ecosystem.
                  Use the Export button above to download this ABOM for compliance, security audits, or sharing with stakeholders.
                </p>
              </div>
            </div>
          </div>

          {/* Agent Detail Modal */}
          {selectedAgent && (
            <div className="fixed inset-0 bg-black/50 backdrop-blur-sm flex items-center justify-center z-50 p-4">
              <div className="glass-chrome max-w-2xl w-full max-h-[90vh] overflow-hidden">
                {/* Modal Header */}
                <div className="px-6 py-4 border-b border-divider flex items-center justify-between">
                  <div className="flex items-center gap-3">
                    <div className="p-2 bg-glass-inset-gray rounded-inset-sm">
                      <Bot className="h-6 w-6 text-ink-secondary" />
                    </div>
                    <div>
                      <h3 className="text-lg font-semibold text-ink">{selectedAgent.name}</h3>
                      <p className="text-sm text-ink-secondary">Agent ABOM details</p>
                    </div>
                  </div>
                  <button
                    onClick={() => setSelectedAgent(null)}
                    className="p-2 hover:bg-glass-inset-gray rounded-inset-sm transition-colors"
                  >
                    <X className="h-5 w-5 text-ink-secondary" />
                  </button>
                </div>

                {/* Modal Body */}
                <div className="px-6 py-4 overflow-y-auto max-h-[calc(90vh-120px)]">
                  <div className="space-y-6">
                    {/* Agent Info */}
                    <div className="grid grid-cols-2 gap-4">
                      <div>
                        <p className="text-xs font-medium text-ink-secondary uppercase">Agent ID</p>
                        <p className="text-sm font-mono text-ink mt-1">{selectedAgent.id}</p>
                      </div>
                      <div>
                        <p className="text-xs font-medium text-ink-secondary uppercase">Status</p>
                        <div className="mt-1">
                          <StatusBadge status={selectedAgent.status} />
                        </div>
                      </div>
                      <div>
                        <p className="text-xs font-medium text-ink-secondary uppercase">Trust score</p>
                        <div className="mt-1">
                          <ConfidenceScoreBadge score={selectedAgent.trustScore * 100} />
                        </div>
                      </div>
                      <div>
                        <p className="text-xs font-medium text-ink-secondary uppercase">Created</p>
                        <p className="text-sm text-ink mt-1">{formatDateTime(selectedAgent.createdAt)}</p>
                      </div>
                      <div className="col-span-2">
                        <p className="text-xs font-medium text-ink-secondary uppercase">Registered by</p>
                        <div className="mt-1 flex items-center gap-2">
                          <div className={`w-6 h-6 rounded-full flex items-center justify-center ${
                            selectedAgent.createdBySdkTokenId
                              ? "bg-brand-soft"
                              : selectedAgent.createdByApiKeyId
                                ? "bg-warning-fill"
                                : "bg-brand-soft"
                          }`}>
                            {selectedAgent.createdBySdkTokenId ? (
                              <Code className="h-3.5 w-3.5 text-brand-indigo" />
                            ) : selectedAgent.createdByApiKeyId ? (
                              <Key className="h-3.5 w-3.5 text-warning-text" />
                            ) : (
                              <User className="h-3.5 w-3.5 text-brand-text" />
                            )}
                          </div>
                          <div>
                            <p className="text-sm text-ink">
                              {selectedAgent.createdByName || selectedAgent.createdByEmail || (selectedAgent.createdBySdkTokenId ? "SDK" : selectedAgent.createdByApiKeyId ? "API key" : "Unknown")}
                            </p>
                            {selectedAgent.createdByEmail && selectedAgent.createdByName && (
                              <p className="text-xs text-ink-secondary">{selectedAgent.createdByEmail}</p>
                            )}
                            {!selectedAgent.createdByName && !selectedAgent.createdByEmail && selectedAgent.createdBySdkTokenId && (
                              <p className="text-xs text-ink-secondary">via SDK token</p>
                            )}
                          </div>
                        </div>
                      </div>
                    </div>

                    {/* Capabilities */}
                    <div>
                      <p className="text-xs font-medium text-ink-secondary uppercase mb-2">
                        Declared capabilities ({(selectedAgent.capabilities || []).length})
                      </p>
                      {(selectedAgent.capabilities || []).length === 0 ? (
                        <p className="text-sm text-ink-tertiary">No capabilities declared</p>
                      ) : (
                        <div className="flex flex-wrap gap-2">
                          {(selectedAgent.capabilities || []).map((cap, idx) => (
                            <span
                              key={idx}
                              className="inline-flex items-center px-3 py-1 rounded-lg text-sm font-medium bg-glass-inset-gray text-ink-body"
                            >
                              {cap}
                            </span>
                          ))}
                        </div>
                      )}
                    </div>

                    {/* Data Access */}
                    <div>
                      <p className="text-xs font-medium text-ink-secondary uppercase mb-2">
                        Data access permissions ({(selectedAgent.dataAccess || []).length})
                      </p>
                      {(selectedAgent.dataAccess || []).length === 0 ? (
                        <p className="text-sm text-ink-tertiary">No data access permissions declared</p>
                      ) : (
                        <div className="flex flex-wrap gap-2">
                          {(selectedAgent.dataAccess || []).map((data, idx) => (
                            <span
                              key={idx}
                              className="inline-flex items-center px-3 py-1 rounded-lg text-sm font-medium bg-glass-inset-gray text-ink-body"
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
                      <p className="text-xs font-medium text-ink-secondary uppercase mb-2">
                        Connected MCP servers
                      </p>
                      {connections.filter(c => c.agentId === selectedAgent.id).length === 0 ? (
                        <p className="text-sm text-ink-tertiary">No MCP server connections observed</p>
                      ) : (
                        <div className="space-y-2">
                          {connections
                            .filter(c => c.agentId === selectedAgent.id)
                            .map((conn) => (
                              <div
                                key={conn.id}
                                className="flex items-center justify-between p-3 rounded-inset bg-glass-inset-gray"
                              >
                                <div className="flex items-center gap-2">
                                  <Server className="h-4 w-4 text-ink-secondary" />
                                  <span className="text-sm font-medium text-ink">
                                    {conn.mcpServerName}
                                  </span>
                                </div>
                                {conn.isActive ? (
                                  <span className="inline-flex items-center text-xs text-success-text">
                                    <CheckCircle2 className="h-3 w-3 mr-1" />
                                    Active
                                  </span>
                                ) : (
                                  <span className="inline-flex items-center text-xs text-ink-secondary">
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
            <Loader2 className="h-8 w-8 animate-spin text-brand-text mx-auto mb-4" />
            <p className="text-ink-secondary">Loading...</p>
          </div>
        </div>
      }>
        <SupplyChainPage />
      </Suspense>
    </AuthGuard>
  );
}
