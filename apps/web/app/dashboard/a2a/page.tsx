"use client";

import { useState, useEffect, useMemo } from "react";
import { useRouter } from "next/navigation";
import {
  Link2,
  CheckCircle2,
  XCircle,
  Clock,
  Shield,
  RefreshCw,
  Trash2,
  Loader2,
  AlertCircle,
  Globe,
  Search,
  Filter,
  Users,
  Activity,
  BadgeCheck,
  ExternalLink,
  FileCheck2,
  Zap,
  ArrowRight,
  TrendingUp,
  BarChart3,
  Network,
  BookOpen,
  ShieldCheck,
  Lock,
  Unlock,
  Play,
  Pause,
  CheckCircle,
  XOctagon,
  AlertTriangle,
  Bot,
  ArrowLeftRight,
} from "lucide-react";
import {
  api,
  A2AAgentCard,
  A2ATrustScore,
  A2AConsent,
  A2ATask,
  A2ASkill,
  A2APeerTrust,
} from "@/lib/api";
import { useDebounce } from "@/hooks/use-debounce";
import { formatDateTime, formatRelativeTime } from "@/lib/date-utils";
import { getErrorMessage } from "@/lib/error-messages";
import { AuthGuard } from "@/components/auth-guard";
import { ConfirmDialog } from "@/components/modals/confirm-dialog";
import {
  BarChart,
  Bar,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
  PieChart,
  Pie,
  Cell,
  Legend,
} from "recharts";

// ============================================
// TYPES
// ============================================

type TabType = "overview" | "cards" | "consent" | "tasks" | "trust" | "skills";

interface TrustScoreEntry {
  agentId: string;
  agentName?: string;
  a2aTrustScore: number;
  totalTasksAsClient: number;
  totalTasksAsRemote: number;
  tasksCompleted: number;
  tasksFailed: number;
  uniquePeersCount: number;
  computedAt?: string;
}

interface TabConfig {
  id: TabType;
  label: string;
  icon: React.ElementType;
  description: string;
}

const TABS: TabConfig[] = [
  { id: "overview", label: "Overview", icon: BarChart3, description: "A2A protocol dashboard and analytics" },
  { id: "cards", label: "Agent Cards", icon: Link2, description: "Registered A2A agent cards" },
  { id: "consent", label: "Consent", icon: ShieldCheck, description: "GDPR/PSD2 consent management" },
  { id: "tasks", label: "Tasks", icon: Activity, description: "A2A task history and audit trail" },
  { id: "trust", label: "Trust", icon: Network, description: "Peer trust relationships" },
  { id: "skills", label: "Skills", icon: Zap, description: "Discover agent capabilities" },
];

// ============================================
// UTILITY COMPONENTS
// ============================================

function StatCard({ stat }: { stat: { name: string; value: string | number; icon: React.ElementType; change?: string; changeType?: "positive" | "negative" } }) {
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
                <div className={`ml-2 flex items-baseline text-sm font-semibold ${
                  stat.changeType === "positive" ? "text-green-600" : "text-red-600"
                }`}>
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

function StatusBadge({ status, size = "sm" }: { status: string; size?: "sm" | "md" }) {
  const getStyles = () => {
    switch (status.toLowerCase()) {
      case "verified":
      case "granted":
      case "completed":
        return "bg-green-100 dark:bg-green-900/30 text-green-800 dark:text-green-300";
      case "pending":
      case "submitted":
      case "working":
        return "bg-yellow-100 dark:bg-yellow-900/30 text-yellow-800 dark:text-yellow-300";
      case "revoked":
      case "denied":
      case "failed":
      case "cancelled":
        return "bg-red-100 dark:bg-red-900/30 text-red-800 dark:text-red-300";
      case "input_needed":
        return "bg-blue-100 dark:bg-blue-900/30 text-blue-800 dark:text-blue-300";
      default:
        return "bg-gray-100 dark:bg-gray-700 text-gray-800 dark:text-gray-300";
    }
  };

  const getIcon = () => {
    switch (status.toLowerCase()) {
      case "verified":
      case "granted":
      case "completed":
        return CheckCircle2;
      case "pending":
      case "submitted":
        return Clock;
      case "working":
        return Play;
      case "revoked":
      case "denied":
      case "failed":
        return XCircle;
      case "cancelled":
        return XOctagon;
      case "input_needed":
        return AlertTriangle;
      default:
        return Clock;
    }
  };

  const Icon = getIcon();
  const sizeClass = size === "md" ? "px-3 py-1 text-sm" : "px-2.5 py-0.5 text-xs";

  return (
    <span className={`inline-flex items-center gap-1 ${sizeClass} rounded-full font-medium capitalize ${getStyles()}`}>
      <Icon className={size === "md" ? "h-4 w-4" : "h-3 w-3"} />
      {status.replace("_", " ")}
    </span>
  );
}

function TrustScoreBadge({ score, size = "sm" }: { score: number; size?: "sm" | "md" }) {
  const getColor = () => {
    if (score >= 0.8) return "bg-green-100 dark:bg-green-900/30 text-green-800 dark:text-green-300";
    if (score >= 0.5) return "bg-yellow-100 dark:bg-yellow-900/30 text-yellow-800 dark:text-yellow-300";
    return "bg-red-100 dark:bg-red-900/30 text-red-800 dark:text-red-300";
  };

  const sizeClass = size === "md" ? "px-3 py-1 text-sm" : "px-2.5 py-0.5 text-xs";

  return (
    <span className={`inline-flex items-center ${sizeClass} rounded-full font-medium ${getColor()}`}>
      {(score * 100).toFixed(0)}%
    </span>
  );
}

function EmptyState({ icon: Icon, title, description, action }: {
  icon: React.ElementType;
  title: string;
  description: string;
  action?: { label: string; onClick: () => void };
}) {
  return (
    <div className="text-center py-12 space-y-4">
      <Icon className="mx-auto h-12 w-12 text-gray-400" />
      <div>
        <h3 className="text-sm font-medium text-gray-900 dark:text-gray-100">{title}</h3>
        <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">{description}</p>
      </div>
      {action && (
        <button
          onClick={action.onClick}
          className="px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 transition-colors text-sm"
        >
          {action.label}
        </button>
      )}
    </div>
  );
}

function LoadingState() {
  return (
    <div className="flex items-center justify-center py-12">
      <Loader2 className="h-8 w-8 animate-spin text-blue-600" />
    </div>
  );
}

// ============================================
// TAB COMPONENTS
// ============================================

function OverviewTab({ agentCards, tasks, trustScores, loading }: {
  agentCards: A2AAgentCard[];
  tasks: A2ATask[];
  trustScores: Map<string, number>;
  loading: boolean;
}) {
  const stats = useMemo(() => ({
    totalCards: agentCards.length,
    verifiedCards: agentCards.filter(c => c.verified).length,
    totalSkills: agentCards.reduce((acc, c) => acc + (c.skills?.length || 0), 0),
    withAttestation: agentCards.filter(c => c.aimAttestation).length,
  }), [agentCards]);

  const statCards = [
    { name: "Agent Cards", value: stats.totalCards, icon: Link2 },
    { name: "Verified", value: stats.verifiedCards, icon: BadgeCheck },
    { name: "AIM Attested", value: stats.withAttestation, icon: Shield },
    { name: "Total Skills", value: stats.totalSkills, icon: Zap },
  ];

  const taskStateData = useMemo(() => {
    const counts: Record<string, number> = {};
    tasks.forEach(t => { counts[t.state] = (counts[t.state] || 0) + 1; });
    const colorMap: Record<string, string> = {
      COMPLETED: "#22c55e", WORKING: "#eab308", FAILED: "#ef4444",
      SUBMITTED: "#3b82f6", CANCELLED: "#6b7280", INPUT_NEEDED: "#8b5cf6",
    };
    return Object.entries(counts).map(([name, value]) => ({
      name, value, fill: colorMap[name] || "#94a3b8",
    }));
  }, [tasks]);

  const trustDistData = useMemo(() => {
    const buckets = { "90-100%": 0, "70-89%": 0, "50-69%": 0, "<50%": 0 };
    trustScores.forEach((score) => {
      const pct = score * 100;
      if (pct >= 90) buckets["90-100%"]++;
      else if (pct >= 70) buckets["70-89%"]++;
      else if (pct >= 50) buckets["50-69%"]++;
      else buckets["<50%"]++;
    });
    return Object.entries(buckets).map(([range, count]) => ({ range, count }));
  }, [trustScores]);

  if (loading) return <LoadingState />;

  return (
    <div className="space-y-6">
      {/* Stats Grid */}
      <div className="grid grid-cols-1 gap-5 sm:grid-cols-2 lg:grid-cols-4">
        {statCards.map((stat) => (
          <StatCard key={stat.name} stat={stat} />
        ))}
      </div>

      {/* Charts Row */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        {/* Task State Distribution */}
        <div className="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-6">
          <h3 className="text-lg font-medium text-gray-900 dark:text-gray-100 mb-4">
            Task State Distribution
          </h3>
          <div className="h-64">
            <ResponsiveContainer width="100%" height="100%">
              <PieChart>
                <Pie
                  data={taskStateData}
                  cx="50%"
                  cy="50%"
                  innerRadius={60}
                  outerRadius={80}
                  paddingAngle={5}
                  dataKey="value"
                  label={({ name, percent }: { name: string; percent: number }) => `${name} ${(percent * 100).toFixed(0)}%`}
                >
                  {taskStateData.map((entry, index) => (
                    <Cell key={`cell-${index}`} fill={entry.fill} />
                  ))}
                </Pie>
                <Tooltip />
              </PieChart>
            </ResponsiveContainer>
          </div>
        </div>

        {/* Trust Score Distribution */}
        <div className="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-6">
          <h3 className="text-lg font-medium text-gray-900 dark:text-gray-100 mb-4">
            Trust Score Distribution
          </h3>
          <div className="h-64">
            <ResponsiveContainer width="100%" height="100%">
              <BarChart data={trustDistData}>
                <CartesianGrid strokeDasharray="3 3" className="stroke-gray-200 dark:stroke-gray-700" />
                <XAxis dataKey="range" className="text-xs" />
                <YAxis className="text-xs" />
                <Tooltip />
                <Bar dataKey="count" fill="#3b82f6" radius={[4, 4, 0, 0]} />
              </BarChart>
            </ResponsiveContainer>
          </div>
        </div>
      </div>

      {/* A2A Protocol Info */}
      <div className="bg-gradient-to-r from-blue-50 to-indigo-50 dark:from-blue-900/20 dark:to-indigo-900/20 border border-blue-200 dark:border-blue-800 rounded-lg p-6">
        <div className="flex items-start gap-4">
          <div className="flex-shrink-0 bg-blue-100 dark:bg-blue-900/50 p-3 rounded-lg">
            <Network className="h-6 w-6 text-blue-600 dark:text-blue-400" />
          </div>
          <div className="flex-1">
            <h3 className="text-lg font-semibold text-blue-900 dark:text-blue-100">
              A2A Protocol with AIM Security
            </h3>
            <p className="mt-2 text-sm text-blue-800 dark:text-blue-200">
              The Agent-to-Agent (A2A) protocol enables secure communication between AI agents.
              AIM enhances A2A with cryptographic attestation, trust scoring, and GDPR/PSD2 compliant
              consent management for enterprise-grade agent interoperability.
            </p>
            <div className="mt-4 flex flex-wrap gap-3">
              <span className="inline-flex items-center gap-1 px-3 py-1 rounded-full text-xs font-medium bg-blue-100 dark:bg-blue-900/50 text-blue-800 dark:text-blue-200">
                <Shield className="h-3 w-3" /> Ed25519 Signatures
              </span>
              <span className="inline-flex items-center gap-1 px-3 py-1 rounded-full text-xs font-medium bg-green-100 dark:bg-green-900/50 text-green-800 dark:text-green-200">
                <ShieldCheck className="h-3 w-3" /> GDPR Compliant
              </span>
              <span className="inline-flex items-center gap-1 px-3 py-1 rounded-full text-xs font-medium bg-purple-100 dark:bg-purple-900/50 text-purple-800 dark:text-purple-200">
                <TrendingUp className="h-3 w-3" /> Trust Scoring
              </span>
              <span className="inline-flex items-center gap-1 px-3 py-1 rounded-full text-xs font-medium bg-orange-100 dark:bg-orange-900/50 text-orange-800 dark:text-orange-200">
                <Activity className="h-3 w-3" /> Task Audit Trail
              </span>
            </div>
          </div>
          <a
            href="https://google.github.io/A2A/"
            target="_blank"
            rel="noopener noreferrer"
            className="flex items-center gap-2 px-4 py-2 text-blue-600 dark:text-blue-400 border border-blue-300 dark:border-blue-700 rounded-lg hover:bg-blue-100 dark:hover:bg-blue-900/30 transition-colors text-sm"
          >
            <ExternalLink className="h-4 w-4" />
            A2A Spec
          </a>
        </div>
      </div>
    </div>
  );
}

function AgentCardsTab({ agentCards, loading, onRefresh, onDelete }: {
  agentCards: A2AAgentCard[];
  loading: boolean;
  onRefresh: (card: A2AAgentCard) => void;
  onDelete: (card: A2AAgentCard) => void;
}) {
  const [searchTerm, setSearchTerm] = useState("");
  const [statusFilter, setStatusFilter] = useState("all");
  const debouncedSearch = useDebounce(searchTerm, 300);

  const filteredCards = useMemo(() => {
    return agentCards.filter(card => {
      const matchesSearch = !debouncedSearch ||
        card.name?.toLowerCase().includes(debouncedSearch.toLowerCase()) ||
        card.url?.toLowerCase().includes(debouncedSearch.toLowerCase());
      const matchesStatus = statusFilter === "all" ||
        (statusFilter === "verified" && card.verified) ||
        (statusFilter === "pending" && !card.verified);
      return matchesSearch && matchesStatus;
    });
  }, [agentCards, debouncedSearch, statusFilter]);

  if (loading) return <LoadingState />;

  return (
    <div className="space-y-4">
      {/* Filters */}
      <div className="flex flex-col sm:flex-row gap-4">
        <div className="flex-1 relative">
          <Search className="absolute left-3 top-1/2 transform -translate-y-1/2 h-4 w-4 text-gray-400" />
          <input
            type="text"
            placeholder="Search by name or URL..."
            value={searchTerm}
            onChange={(e) => setSearchTerm(e.target.value)}
            className="w-full pl-10 pr-4 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-900 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-transparent"
          />
        </div>
        <select
          value={statusFilter}
          onChange={(e) => setStatusFilter(e.target.value)}
          className="px-4 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-900 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500"
        >
          <option value="all">All Status</option>
          <option value="verified">Verified</option>
          <option value="pending">Pending</option>
        </select>
      </div>

      {/* Cards Table */}
      <div className="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 overflow-hidden">
        <div className="overflow-x-auto">
          <table className="min-w-full divide-y divide-gray-200 dark:divide-gray-700">
            <thead className="bg-gray-50 dark:bg-gray-800">
              <tr>
                <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase">Agent</th>
                <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase">URL</th>
                <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase">Status</th>
                <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase">Skills</th>
                <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase">Attestation</th>
                <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-200 dark:divide-gray-700">
              {filteredCards.map((card) => (
                <tr key={card.id} className="hover:bg-gray-50 dark:hover:bg-gray-800/50">
                  <td className="px-4 py-3">
                    <div className="flex items-center gap-3">
                      <div className="h-8 w-8 bg-blue-100 dark:bg-blue-900/30 rounded-lg flex items-center justify-center">
                        <Bot className="h-4 w-4 text-blue-600 dark:text-blue-400" />
                      </div>
                      <div>
                        <div className="text-sm font-medium text-gray-900 dark:text-gray-100">{card.name}</div>
                        <div className="text-xs text-gray-500 dark:text-gray-400">v{card.version || "1.0"}</div>
                      </div>
                    </div>
                  </td>
                  <td className="px-4 py-3">
                    <a href={card.url} target="_blank" rel="noopener noreferrer" className="text-sm text-blue-600 dark:text-blue-400 hover:underline flex items-center gap-1">
                      <Globe className="h-3 w-3" />
                      <span className="truncate max-w-[200px]">{card.url}</span>
                    </a>
                  </td>
                  <td className="px-4 py-3">
                    <StatusBadge status={card.verified ? "verified" : "pending"} />
                  </td>
                  <td className="px-4 py-3">
                    <div className="flex items-center gap-1">
                      <Zap className="h-3 w-3 text-gray-400" />
                      <span className="text-sm text-gray-900 dark:text-gray-100">{card.skills?.length || 0}</span>
                    </div>
                  </td>
                  <td className="px-4 py-3">
                    {card.aimAttestation ? (
                      <span className="inline-flex items-center gap-1 text-xs text-green-600 dark:text-green-400">
                        <Shield className="h-3 w-3" />
                        AIM Attested
                      </span>
                    ) : (
                      <span className="text-xs text-gray-400">—</span>
                    )}
                  </td>
                  <td className="px-4 py-3">
                    <div className="flex items-center gap-2">
                      <button onClick={() => onRefresh(card)} className="p-1 text-gray-400 hover:text-blue-600 transition-colors" title="Refresh">
                        <RefreshCw className="h-4 w-4" />
                      </button>
                      <button onClick={() => onDelete(card)} className="p-1 text-gray-400 hover:text-red-600 transition-colors" title="Delete">
                        <Trash2 className="h-4 w-4" />
                      </button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
        {filteredCards.length === 0 && (
          <EmptyState
            icon={Link2}
            title="No agent cards found"
            description={searchTerm ? "Try adjusting your search criteria" : "Register an agent card to enable A2A protocol communication"}
          />
        )}
      </div>
    </div>
  );
}

function ConsentTab({ consents: initialConsents, loading }: { consents: A2AConsent[]; loading: boolean }) {
  const [consents, setConsents] = useState<A2AConsent[]>(initialConsents);
  const [userIdFilter, setUserIdFilter] = useState("");

  useEffect(() => {
    setConsents(initialConsents);
  }, [initialConsents]);

  useEffect(() => {
    if (userIdFilter) {
      api.listA2AConsents(userIdFilter).then(data => {
        const raw = data.consents || data as any;
        setConsents(Array.isArray(raw) ? raw : []);
      }).catch(() => setConsents([]));
    } else {
      setConsents(initialConsents);
    }
  }, [userIdFilter, initialConsents]);

  const displayConsents = consents;

  if (loading) return <LoadingState />;

  return (
    <div className="space-y-6">
      {/* Info Banner */}
      <div className="bg-green-50 dark:bg-green-900/20 border border-green-200 dark:border-green-800 rounded-lg p-4">
        <div className="flex items-start gap-3">
          <ShieldCheck className="h-5 w-5 text-green-600 dark:text-green-400 flex-shrink-0 mt-0.5" />
          <div>
            <h3 className="text-sm font-medium text-green-800 dark:text-green-200">GDPR/PSD2 Compliant Consent Management</h3>
            <p className="text-sm text-green-700 dark:text-green-300 mt-1">
              All cross-agent data sharing requires explicit user consent. Consents are tracked with full audit trail,
              expiration management, and revocation support.
            </p>
          </div>
        </div>
      </div>

      {/* Search */}
      <div className="flex gap-4">
        <div className="flex-1 relative">
          <Search className="absolute left-3 top-1/2 transform -translate-y-1/2 h-4 w-4 text-gray-400" />
          <input
            type="text"
            placeholder="Search by user ID..."
            value={userIdFilter}
            onChange={(e) => setUserIdFilter(e.target.value)}
            className="w-full pl-10 pr-4 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-900 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500"
          />
        </div>
      </div>

      {/* Consents Table */}
      <div className="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 overflow-hidden">
        <div className="overflow-x-auto">
          <table className="min-w-full divide-y divide-gray-200 dark:divide-gray-700">
            <thead className="bg-gray-50 dark:bg-gray-800">
              <tr>
                <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase">User</th>
                <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase">Agents</th>
                <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase">Purpose</th>
                <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase">Data Types</th>
                <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase">Status</th>
                <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase">Expires</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-200 dark:divide-gray-700">
              {displayConsents.map((consent) => (
                <tr key={consent.id} className="hover:bg-gray-50 dark:hover:bg-gray-800/50">
                  <td className="px-4 py-3">
                    <div className="flex items-center gap-2">
                      <Users className="h-4 w-4 text-gray-400" />
                      <span className="text-sm text-gray-900 dark:text-gray-100">{consent.userId}</span>
                    </div>
                  </td>
                  <td className="px-4 py-3">
                    <div className="flex items-center gap-1 text-sm">
                      <span className="text-gray-900 dark:text-gray-100">{consent.sourceAgentId.slice(0, 8)}...</span>
                      <ArrowRight className="h-3 w-3 text-gray-400" />
                      <span className="text-gray-900 dark:text-gray-100">{consent.targetAgentId.slice(0, 8)}...</span>
                    </div>
                  </td>
                  <td className="px-4 py-3">
                    <span className="text-sm text-gray-900 dark:text-gray-100">{consent.purpose}</span>
                  </td>
                  <td className="px-4 py-3">
                    <div className="flex flex-wrap gap-1">
                      {consent.dataTypes.slice(0, 2).map((type) => (
                        <span key={type} className="px-2 py-0.5 bg-gray-100 dark:bg-gray-700 rounded text-xs text-gray-700 dark:text-gray-300">
                          {type}
                        </span>
                      ))}
                      {consent.dataTypes.length > 2 && (
                        <span className="px-2 py-0.5 bg-gray-100 dark:bg-gray-700 rounded text-xs text-gray-500">
                          +{consent.dataTypes.length - 2}
                        </span>
                      )}
                    </div>
                  </td>
                  <td className="px-4 py-3">
                    <StatusBadge status={consent.status} />
                  </td>
                  <td className="px-4 py-3">
                    <span className="text-sm text-gray-500 dark:text-gray-400">
                      {consent.expiresAt ? formatRelativeTime(consent.expiresAt) : "Never"}
                    </span>
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

function TasksTab({ tasks, loading }: { tasks: A2ATask[]; loading: boolean }) {
  const [stateFilter, setStateFilter] = useState("all");

  const filteredTasks = stateFilter === "all" ? tasks : tasks.filter(t => t.state === stateFilter);

  const stateCounts = useMemo(() => {
    const counts: Record<string, number> = {};
    tasks.forEach(t => { counts[t.state] = (counts[t.state] || 0) + 1; });
    return counts;
  }, [tasks]);

  if (loading) return <LoadingState />;

  return (
    <div className="space-y-4">
      {/* Stats */}
      <div className="grid grid-cols-2 sm:grid-cols-4 lg:grid-cols-6 gap-3">
        {[
          { label: "Total", value: tasks.length, color: "text-gray-900 dark:text-gray-100" },
          { label: "Completed", value: stateCounts["completed"] || 0, color: "text-green-600" },
          { label: "Working", value: stateCounts["working"] || 0, color: "text-yellow-600" },
          { label: "Submitted", value: stateCounts["submitted"] || 0, color: "text-blue-600" },
          { label: "Failed", value: stateCounts["failed"] || 0, color: "text-red-600" },
          { label: "Cancelled", value: stateCounts["cancelled"] || 0, color: "text-gray-500" },
        ].map(s => (
          <div key={s.label} className="bg-white dark:bg-gray-800 p-3 rounded-lg border border-gray-200 dark:border-gray-700 text-center">
            <div className={`text-xl font-semibold ${s.color}`}>{s.value}</div>
            <div className="text-xs text-gray-500 dark:text-gray-400">{s.label}</div>
          </div>
        ))}
      </div>

      {/* Filters */}
      <div className="flex gap-4">
        <select
          value={stateFilter}
          onChange={(e) => setStateFilter(e.target.value)}
          className="px-4 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-900 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500"
        >
          <option value="all">All States ({tasks.length})</option>
          <option value="submitted">Submitted ({stateCounts["submitted"] || 0})</option>
          <option value="working">Working ({stateCounts["working"] || 0})</option>
          <option value="input_needed">Input Needed ({stateCounts["input_needed"] || 0})</option>
          <option value="completed">Completed ({stateCounts["completed"] || 0})</option>
          <option value="failed">Failed ({stateCounts["failed"] || 0})</option>
          <option value="cancelled">Cancelled ({stateCounts["cancelled"] || 0})</option>
        </select>
      </div>

      {/* Tasks Table */}
      <div className="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 overflow-hidden">
        <div className="overflow-x-auto">
          <table className="min-w-full divide-y divide-gray-200 dark:divide-gray-700">
            <thead className="bg-gray-50 dark:bg-gray-800">
              <tr>
                <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase">Task ID</th>
                <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase">Client → Remote</th>
                <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase">Skill</th>
                <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase">State</th>
                <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase">Messages</th>
                <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase">Duration</th>
                <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase">Created</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-200 dark:divide-gray-700">
              {filteredTasks.map((task) => (
                <tr key={task.id} className="hover:bg-gray-50 dark:hover:bg-gray-800/50">
                  <td className="px-4 py-3">
                    <span className="text-sm font-mono text-gray-900 dark:text-gray-100">{task.externalTaskId || task.id.slice(0, 8)}</span>
                  </td>
                  <td className="px-4 py-3">
                    <div className="flex items-center gap-1 text-sm">
                      <span className="text-gray-900 dark:text-gray-100">{task.clientAgentName || task.clientAgentId?.slice(0, 8) || "—"}</span>
                      <ArrowLeftRight className="h-3 w-3 text-gray-400" />
                      <span className="text-gray-900 dark:text-gray-100">{task.remoteAgentName || task.remoteAgentId?.slice(0, 8) || "—"}</span>
                    </div>
                  </td>
                  <td className="px-4 py-3">
                    <span className="text-sm text-gray-600 dark:text-gray-400">{task.skillId || "—"}</span>
                  </td>
                  <td className="px-4 py-3">
                    <StatusBadge status={task.state} />
                  </td>
                  <td className="px-4 py-3">
                    <span className="text-sm text-gray-900 dark:text-gray-100">{task.messageCount || 0}</span>
                  </td>
                  <td className="px-4 py-3">
                    <span className="text-sm text-gray-500 dark:text-gray-400">
                      {task.durationMs ? `${task.durationMs}ms` : "—"}
                    </span>
                  </td>
                  <td className="px-4 py-3">
                    <span className="text-sm text-gray-500 dark:text-gray-400">
                      {task.createdAt ? formatRelativeTime(task.createdAt) : "—"}
                    </span>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
        {filteredTasks.length === 0 && (
          <EmptyState
            icon={Activity}
            title="No tasks found"
            description={stateFilter !== "all" ? "Try changing the state filter" : "Tasks will appear here when agents communicate via A2A protocol"}
          />
        )}
      </div>
    </div>
  );
}

function TrustTab({ trustScores, agentCards, loading }: {
  trustScores: TrustScoreEntry[];
  agentCards: A2AAgentCard[];
  loading: boolean;
}) {
  // Build agent name lookup from cards
  const agentNameMap = useMemo(() => {
    const map = new Map<string, string>();
    agentCards.forEach(c => {
      if (c.agentId && c.name) map.set(c.agentId, c.name);
    });
    return map;
  }, [agentCards]);

  const enrichedScores = useMemo(() =>
    trustScores.map(s => ({
      ...s,
      agentName: s.agentName || agentNameMap.get(s.agentId) || `Agent ${s.agentId.slice(0, 8)}`,
    })),
    [trustScores, agentNameMap]
  );

  if (loading) return <LoadingState />;

  return (
    <div className="space-y-6">
      {/* Summary stats */}
      <div className="grid grid-cols-2 sm:grid-cols-4 gap-4">
        {[
          { label: "Agents Scored", value: trustScores.length, icon: Bot },
          { label: "Avg Trust Score", value: trustScores.length > 0
            ? `${(trustScores.reduce((a, s) => a + (s.a2aTrustScore || 0), 0) / trustScores.length * 100).toFixed(0)}%`
            : "N/A", icon: Shield },
          { label: "Total Tasks", value: trustScores.reduce((a, s) => a + s.totalTasksAsClient + s.totalTasksAsRemote, 0), icon: Activity },
          { label: "Total Peers", value: trustScores.reduce((a, s) => a + s.uniquePeersCount, 0), icon: Users },
        ].map(s => (
          <StatCard key={s.label} stat={{ name: s.label, value: s.value, icon: s.icon }} />
        ))}
      </div>

      {/* Agent Trust Scores Table */}
      <div className="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 overflow-hidden">
        <div className="px-6 py-4 border-b border-gray-200 dark:border-gray-700">
          <h3 className="text-lg font-medium text-gray-900 dark:text-gray-100">Agent A2A Trust Scores</h3>
        </div>
        <div className="overflow-x-auto">
          <table className="min-w-full divide-y divide-gray-200 dark:divide-gray-700">
            <thead className="bg-gray-50 dark:bg-gray-800">
              <tr>
                <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase">Agent</th>
                <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase">Trust Score</th>
                <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase">Tasks (Client)</th>
                <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase">Tasks (Remote)</th>
                <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase">Completed</th>
                <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase">Failed</th>
                <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase">Unique Peers</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-200 dark:divide-gray-700">
              {enrichedScores.map((score) => (
                <tr key={score.agentId} className="hover:bg-gray-50 dark:hover:bg-gray-800/50">
                  <td className="px-4 py-3">
                    <div className="flex items-center gap-2">
                      <Bot className="h-4 w-4 text-blue-500" />
                      <span className="text-sm font-medium text-gray-900 dark:text-gray-100">{score.agentName}</span>
                    </div>
                  </td>
                  <td className="px-4 py-3">
                    <TrustScoreBadge score={score.a2aTrustScore || 0} />
                  </td>
                  <td className="px-4 py-3 text-sm text-gray-900 dark:text-gray-100">{score.totalTasksAsClient}</td>
                  <td className="px-4 py-3 text-sm text-gray-900 dark:text-gray-100">{score.totalTasksAsRemote}</td>
                  <td className="px-4 py-3 text-sm text-green-600">{score.tasksCompleted}</td>
                  <td className="px-4 py-3 text-sm text-red-600">{score.tasksFailed}</td>
                  <td className="px-4 py-3 text-sm text-gray-900 dark:text-gray-100">{score.uniquePeersCount}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
        {enrichedScores.length === 0 && (
          <EmptyState
            icon={Network}
            title="No trust scores yet"
            description="Trust scores are computed when agents interact via A2A protocol"
          />
        )}
      </div>
    </div>
  );
}

function SkillsTab({ agentCards, loading }: { agentCards: A2AAgentCard[]; loading: boolean }) {
  const [searchQuery, setSearchQuery] = useState("");
  const [skills, setSkills] = useState<Array<A2ASkill & { agentId: string; agentName: string }>>([]);
  const debouncedQuery = useDebounce(searchQuery, 300);

  useEffect(() => {
    if (debouncedQuery.length >= 2) {
      api.searchA2ASkills(debouncedQuery).then(data => {
        setSkills(data.skills || []);
      }).catch(() => setSkills([]));
    } else {
      // Show all skills from agent cards
      const allSkills: Array<A2ASkill & { agentId: string; agentName: string }> = [];
      agentCards.forEach(card => {
        (card.skills || []).forEach(skill => {
          allSkills.push({ ...skill, agentId: card.agentId, agentName: card.name });
        });
      });
      setSkills(allSkills);
    }
  }, [debouncedQuery, agentCards]);

  if (loading) return <LoadingState />;

  return (
    <div className="space-y-4">
      {/* Search */}
      <div className="relative">
        <Search className="absolute left-3 top-1/2 transform -translate-y-1/2 h-4 w-4 text-gray-400" />
        <input
          type="text"
          placeholder="Search skills by name, description, or tags..."
          value={searchQuery}
          onChange={(e) => setSearchQuery(e.target.value)}
          className="w-full pl-10 pr-4 py-3 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-900 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500"
        />
      </div>

      {/* Skills Grid */}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
        {skills.map((skill, idx) => (
          <div key={`${skill.id}-${idx}`} className="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-4 hover:shadow-md transition-shadow">
            <div className="flex items-start justify-between">
              <div className="flex items-center gap-2">
                <div className="h-8 w-8 bg-purple-100 dark:bg-purple-900/30 rounded-lg flex items-center justify-center">
                  <Zap className="h-4 w-4 text-purple-600 dark:text-purple-400" />
                </div>
                <div>
                  <h4 className="text-sm font-medium text-gray-900 dark:text-gray-100">{skill.name}</h4>
                  <p className="text-xs text-gray-500 dark:text-gray-400">by {skill.agentName}</p>
                </div>
              </div>
            </div>
            <p className="mt-3 text-sm text-gray-600 dark:text-gray-400 line-clamp-2">
              {skill.description || "No description available"}
            </p>
            {skill.tags && skill.tags.length > 0 && (
              <div className="mt-3 flex flex-wrap gap-1">
                {skill.tags.slice(0, 3).map((tag) => (
                  <span key={tag} className="px-2 py-0.5 bg-gray-100 dark:bg-gray-700 rounded text-xs text-gray-600 dark:text-gray-400">
                    {tag}
                  </span>
                ))}
              </div>
            )}
            {skill.inputModes && (
              <div className="mt-3 flex items-center gap-2 text-xs text-gray-500 dark:text-gray-400">
                <span>Input: {skill.inputModes.join(", ")}</span>
              </div>
            )}
          </div>
        ))}
      </div>

      {skills.length === 0 && (
        <EmptyState
          icon={Zap}
          title="No skills found"
          description={searchQuery ? "Try adjusting your search query" : "Skills will appear here when agents register their capabilities"}
        />
      )}
    </div>
  );
}

// ============================================
// MAIN COMPONENT
// ============================================

export default function A2AProtocolPage() {
  const [activeTab, setActiveTab] = useState<TabType>("overview");
  const [loading, setLoading] = useState(true);
  const [agentCards, setAgentCards] = useState<A2AAgentCard[]>([]);
  const [tasks, setTasks] = useState<A2ATask[]>([]);
  const [consents, setConsents] = useState<A2AConsent[]>([]);
  const [trustScores, setTrustScores] = useState<TrustScoreEntry[]>([]);
  const [showDeleteConfirm, setShowDeleteConfirm] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<A2AAgentCard | null>(null);
  const [deleteLoading, setDeleteLoading] = useState(false);

  // Derived trust score map for OverviewTab
  const trustScoreMap = useMemo(() => {
    const map = new Map<string, number>();
    trustScores.forEach(s => {
      if (s.a2aTrustScore != null) map.set(s.agentId, s.a2aTrustScore);
    });
    return map;
  }, [trustScores]);

  const fetchData = async () => {
    setLoading(true);
    try {
      const [cardsData, tasksData, consentsData, trustData] = await Promise.allSettled([
        api.listA2AAgentCards(),
        api.listA2ATasks({ limit: 500 }),
        api.listA2AConsents(),
        api.listA2ATrustScores(),
      ]);
      if (cardsData.status === "fulfilled") setAgentCards(cardsData.value.cards || []);
      if (tasksData.status === "fulfilled") setTasks(tasksData.value.tasks || []);
      if (consentsData.status === "fulfilled") setConsents(consentsData.value.consents || []);
      if (trustData.status === "fulfilled") setTrustScores(trustData.value.scores || []);
    } catch (err) {
      console.error("Failed to fetch A2A data:", err);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchData();
  }, []);

  const handleRefresh = async (card: A2AAgentCard) => {
    try {
      await api.refreshA2AAttestation(card.agentId);
      fetchData();
    } catch (err) {
      console.error("Failed to refresh attestation:", err);
    }
  };

  const handleDelete = async () => {
    if (!deleteTarget) return;
    setDeleteLoading(true);
    try {
      await api.deleteA2AAgentCard(deleteTarget.agentId);
      setAgentCards(prev => prev.filter(c => c.id !== deleteTarget.id));
      setShowDeleteConfirm(false);
      setDeleteTarget(null);
    } catch (err) {
      console.error("Failed to delete card:", err);
    } finally {
      setDeleteLoading(false);
    }
  };

  const requestDelete = (card: A2AAgentCard) => {
    setDeleteTarget(card);
    setShowDeleteConfirm(true);
  };

  const renderTab = () => {
    switch (activeTab) {
      case "overview":
        return <OverviewTab agentCards={agentCards} tasks={tasks} trustScores={trustScoreMap} loading={loading} />;
      case "cards":
        return <AgentCardsTab agentCards={agentCards} loading={loading} onRefresh={handleRefresh} onDelete={requestDelete} />;
      case "consent":
        return <ConsentTab consents={consents} loading={loading} />;
      case "tasks":
        return <TasksTab tasks={tasks} loading={loading} />;
      case "trust":
        return <TrustTab trustScores={trustScores} agentCards={agentCards} loading={loading} />;
      case "skills":
        return <SkillsTab agentCards={agentCards} loading={loading} />;
      default:
        return null;
    }
  };

  return (
    <AuthGuard>
      <div className="space-y-6">
        {/* Header */}
        <div className="flex items-center justify-between">
          <div>
            <h1 className="text-2xl font-bold text-gray-900 dark:text-white flex items-center gap-3">
              <Network className="h-7 w-7 text-blue-600" />
              A2A Protocol
            </h1>
            <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">
              Secure agent-to-agent communication with AIM attestation and trust scoring
            </p>
          </div>
          <div className="flex items-center gap-3">
            <button
              onClick={fetchData}
              className="flex items-center gap-2 px-4 py-2 text-gray-600 dark:text-gray-300 border border-gray-300 dark:border-gray-600 rounded-lg hover:bg-gray-50 dark:hover:bg-gray-800 transition-colors"
            >
              <RefreshCw className="h-4 w-4" />
              Refresh
            </button>
            <a
              href="https://google.github.io/A2A/"
              target="_blank"
              rel="noopener noreferrer"
              className="flex items-center gap-2 px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 transition-colors"
            >
              <ExternalLink className="h-4 w-4" />
              A2A Spec
            </a>
          </div>
        </div>

        {/* Tabs */}
        <div className="border-b border-gray-200 dark:border-gray-700">
          <nav className="flex space-x-8 overflow-x-auto" aria-label="Tabs">
            {TABS.map((tab) => (
              <button
                key={tab.id}
                onClick={() => setActiveTab(tab.id)}
                className={`flex items-center gap-2 py-4 px-1 border-b-2 font-medium text-sm whitespace-nowrap transition-colors ${
                  activeTab === tab.id
                    ? "border-blue-500 text-blue-600 dark:text-blue-400"
                    : "border-transparent text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-300 hover:border-gray-300"
                }`}
              >
                <tab.icon className="h-4 w-4" />
                {tab.label}
              </button>
            ))}
          </nav>
        </div>

        {/* Tab Content */}
        <div className="min-h-[400px]">
          {renderTab()}
        </div>

        {/* Delete Confirmation */}
        <ConfirmDialog
          isOpen={showDeleteConfirm}
          title="Delete Agent Card"
          message={`Are you sure you want to delete the agent card for "${deleteTarget?.name}"? This action cannot be undone.`}
          confirmText="Delete"
          cancelText="Cancel"
          variant="danger"
          loading={deleteLoading}
          onConfirm={handleDelete}
          onCancel={() => {
            if (!deleteLoading) {
              setShowDeleteConfirm(false);
              setDeleteTarget(null);
            }
          }}
        />
      </div>
    </AuthGuard>
  );
}
