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
import { buttonVariants } from "@/components/ui/button";
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

// Glass tooltip chrome for recharts. Colors come from tokens, never hex literals.
const CHART_TOOLTIP_STYLE = {
  backgroundColor: "var(--glass-fill)",
  border: "1px solid var(--glass-border)",
  borderRadius: "12px",
  boxShadow: "var(--shadow-card)",
  color: "var(--text-primary)",
} as const;

const TABS: TabConfig[] = [
  { id: "overview", label: "Overview", icon: BarChart3, description: "A2A protocol dashboard and analytics" },
  { id: "cards", label: "Agent cards", icon: Link2, description: "Registered A2A agent cards" },
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
                <div className={`ml-2 flex items-baseline text-sm font-semibold ${
                  stat.changeType === "positive" ? "text-success-text" : "text-danger-text"
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
        return "border border-success-border bg-success-fill text-success-text";
      case "pending":
      case "submitted":
      case "working":
        return "border border-warning-border bg-warning-fill text-warning-text";
      case "revoked":
      case "denied":
      case "failed":
      case "cancelled":
        return "border border-danger-border bg-danger-fill text-danger-text";
      case "input_needed":
        return "border border-stroke bg-brand-soft text-brand-text";
      default:
        return "border border-stroke bg-glass-inset-gray text-ink-body";
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
    <span className={`inline-flex items-center gap-1 ${sizeClass} rounded-pill font-medium capitalize ${getStyles()}`}>
      <Icon className={size === "md" ? "h-4 w-4" : "h-3 w-3"} />
      {status.replace("_", " ")}
    </span>
  );
}

function TrustScoreBadge({ score, size = "sm" }: { score: number; size?: "sm" | "md" }) {
  const getColor = () => {
    if (score >= 0.8) return "border border-success-border bg-success-fill text-success-text";
    if (score >= 0.5) return "border border-warning-border bg-warning-fill text-warning-text";
    return "border border-danger-border bg-danger-fill text-danger-text";
  };

  const sizeClass = size === "md" ? "px-3 py-1 text-sm" : "px-2.5 py-0.5 text-xs";

  return (
    <span className={`inline-flex items-center ${sizeClass} rounded-pill font-medium ${getColor()}`}>
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
      <Icon className="mx-auto h-12 w-12 text-ink-tertiary" />
      <div>
        <h3 className="text-sm font-medium text-ink">{title}</h3>
        <p className="mt-1 text-sm text-ink-secondary">{description}</p>
      </div>
      {action && (
        <button
          onClick={action.onClick}
          className={buttonVariants({ size: "sm" })}
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
      <Loader2 className="h-8 w-8 animate-spin text-brand-text" />
    </div>
  );
}

// Inline spinner for a single section/chart so the rest of the page can
// paint while that section's data is still loading.
function ChartLoading({ className = "h-full" }: { className?: string }) {
  return (
    <div className={`flex items-center justify-center ${className}`}>
      <Loader2 className="h-6 w-6 animate-spin text-brand-text" />
    </div>
  );
}

// ============================================
// TAB COMPONENTS
// ============================================

function OverviewTab({ agentCards, tasks, trustScores, cardsLoading, tasksLoading, trustLoading }: {
  agentCards: A2AAgentCard[];
  tasks: A2ATask[];
  trustScores: Map<string, number>;
  cardsLoading: boolean;
  tasksLoading: boolean;
  trustLoading: boolean;
}) {
  const stats = useMemo(() => ({
    totalCards: agentCards.length,
    verifiedCards: agentCards.filter(c => c.verified).length,
    totalSkills: agentCards.reduce((acc, c) => acc + (c.skills?.length || 0), 0),
    withAttestation: agentCards.filter(c => c.aimAttestation).length,
  }), [agentCards]);

  const statCards = [
    { name: "Agent cards", value: stats.totalCards, icon: Link2 },
    { name: "Verified", value: stats.verifiedCards, icon: BadgeCheck },
    { name: "AIM attested", value: stats.withAttestation, icon: Shield },
    { name: "Total skills", value: stats.totalSkills, icon: Zap },
  ];

  const taskStateData = useMemo(() => {
    const counts: Record<string, number> = {};
    tasks.forEach(t => { counts[t.state] = (counts[t.state] || 0) + 1; });
    const colorMap: Record<string, string> = {
      COMPLETED: "var(--green)", WORKING: "var(--amber)", FAILED: "var(--red)",
      SUBMITTED: "var(--brand)", CANCELLED: "var(--text-secondary)", INPUT_NEEDED: "var(--brand-indigo)",
    };
    return Object.entries(counts).map(([name, value]) => ({
      name, value, fill: colorMap[name] || "var(--text-tertiary)",
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

  return (
    <div className="space-y-6">
      {/* Stats Grid */}
      {cardsLoading ? (
        <ChartLoading className="py-8" />
      ) : (
        <div className="grid grid-cols-1 gap-5 sm:grid-cols-2 lg:grid-cols-4">
          {statCards.map((stat) => (
            <StatCard key={stat.name} stat={stat} />
          ))}
        </div>
      )}

      {/* Charts Row */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        {/* Task State Distribution */}
        <div className="glass p-6">
          <h3 className="text-lg font-medium text-ink mb-4">
            Task state distribution
          </h3>
          <div className="h-64">
            {tasksLoading ? <ChartLoading /> : (
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
                <Tooltip contentStyle={CHART_TOOLTIP_STYLE} />
              </PieChart>
            </ResponsiveContainer>
            )}
          </div>
        </div>

        {/* Trust Score Distribution */}
        <div className="glass p-6">
          <h3 className="text-lg font-medium text-ink mb-4">
            Trust score distribution
          </h3>
          <div className="h-64">
            {trustLoading ? <ChartLoading /> : (
            <ResponsiveContainer width="100%" height="100%">
              <BarChart data={trustDistData}>
                <CartesianGrid strokeDasharray="3 3" className="stroke-divider" />
                <XAxis dataKey="range" className="text-xs" stroke="var(--text-tertiary)" />
                <YAxis className="text-xs" stroke="var(--text-tertiary)" />
                <Tooltip contentStyle={CHART_TOOLTIP_STYLE} cursor={{ fill: "var(--surface-inset-gray)" }} />
                <Bar dataKey="count" fill="var(--brand)" radius={[4, 4, 0, 0]} />
              </BarChart>
            </ResponsiveContainer>
            )}
          </div>
        </div>
      </div>

      {/* A2A Protocol Info */}
      <div className="glass p-6">
        <div className="flex items-start gap-4">
          <div className="flex-shrink-0 bg-brand-soft p-3 rounded-inset">
            <Network className="h-6 w-6 text-brand-text" />
          </div>
          <div className="flex-1">
            <h3 className="text-lg font-semibold text-ink">
              A2A protocol with AIM security
            </h3>
            <p className="mt-2 text-sm text-ink-body">
              The Agent-to-Agent (A2A) protocol enables secure communication between AI agents.
              AIM enhances A2A with cryptographic attestation, trust scoring, and recorded, revocable
              consent for agent-to-agent data sharing.
            </p>
            <div className="mt-4 flex flex-wrap gap-3">
              <span className="inline-flex items-center gap-1 px-3 py-1 rounded-pill text-xs font-medium bg-brand-soft text-brand-text">
                <Shield className="h-3 w-3" /> Ed25519 signatures
              </span>
              <span className="inline-flex items-center gap-1 px-3 py-1 rounded-pill text-xs font-medium bg-success-fill text-success-text">
                <ShieldCheck className="h-3 w-3" /> Consent records
              </span>
              <span className="inline-flex items-center gap-1 px-3 py-1 rounded-pill text-xs font-medium bg-brand-soft text-brand-text">
                <TrendingUp className="h-3 w-3" /> Trust scoring
              </span>
              <span className="inline-flex items-center gap-1 px-3 py-1 rounded-pill text-xs font-medium bg-glass-inset-gray text-ink-body">
                <Activity className="h-3 w-3" /> Task audit trail
              </span>
            </div>
          </div>
          <a
            href="https://google.github.io/A2A/"
            target="_blank"
            rel="noopener noreferrer"
            className="flex items-center gap-2 px-4 py-2 rounded-pill border border-stroke text-sm font-semibold text-brand-text transition-colors hover:bg-brand-soft"
          >
            <ExternalLink className="h-4 w-4" />
            A2A spec
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
          <Search className="absolute left-3 top-1/2 transform -translate-y-1/2 h-4 w-4 text-ink-tertiary" />
          <input
            type="text"
            placeholder="Search by name or URL..."
            value={searchTerm}
            onChange={(e) => setSearchTerm(e.target.value)}
            className="w-full pl-10 pr-4 py-2 border border-stroke rounded-inset bg-glass-inset text-ink placeholder:text-ink-tertiary focus:outline-none focus:ring-2 focus:ring-ring focus:border-transparent"
          />
        </div>
        <select
          value={statusFilter}
          onChange={(e) => setStatusFilter(e.target.value)}
          className="px-4 py-2 border border-stroke rounded-inset bg-glass-inset text-ink focus:outline-none focus:ring-2 focus:ring-ring"
        >
          <option value="all">All statuses</option>
          <option value="verified">Verified</option>
          <option value="pending">Pending</option>
        </select>
      </div>

      {/* Cards Table */}
      <div className="glass overflow-hidden">
        <div className="overflow-x-auto">
          <table className="min-w-full divide-y divide-divider">
            <thead className="bg-glass-inset-gray">
              <tr>
                <th className="px-4 py-3 text-left text-xs font-medium text-ink-tertiary uppercase">Agent</th>
                <th className="px-4 py-3 text-left text-xs font-medium text-ink-tertiary uppercase">URL</th>
                <th className="px-4 py-3 text-left text-xs font-medium text-ink-tertiary uppercase">Status</th>
                <th className="px-4 py-3 text-left text-xs font-medium text-ink-tertiary uppercase">Skills</th>
                <th className="px-4 py-3 text-left text-xs font-medium text-ink-tertiary uppercase">Attestation</th>
                <th className="px-4 py-3 text-left text-xs font-medium text-ink-tertiary uppercase">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-divider">
              {filteredCards.map((card) => (
                <tr key={card.id} className="hover:bg-glass-inset-gray">
                  <td className="px-4 py-3">
                    <div className="flex items-center gap-3">
                      <div className="h-8 w-8 bg-brand-soft rounded-inset-sm flex items-center justify-center">
                        <Bot className="h-4 w-4 text-brand-text" />
                      </div>
                      <div>
                        <div className="text-sm font-medium text-ink">{card.name}</div>
                        <div className="text-xs text-ink-tertiary">v{card.version || "1.0"}</div>
                      </div>
                    </div>
                  </td>
                  <td className="px-4 py-3">
                    <a href={card.url} target="_blank" rel="noopener noreferrer" className="text-sm text-brand-text hover:underline flex items-center gap-1">
                      <Globe className="h-3 w-3" />
                      <span className="truncate max-w-[200px]">{card.url}</span>
                    </a>
                  </td>
                  <td className="px-4 py-3">
                    <StatusBadge status={card.verified ? "verified" : "pending"} />
                  </td>
                  <td className="px-4 py-3">
                    <div className="flex items-center gap-1">
                      <Zap className="h-3 w-3 text-ink-tertiary" />
                      <span className="text-sm text-ink">{card.skills?.length || 0}</span>
                    </div>
                  </td>
                  <td className="px-4 py-3">
                    {card.aimAttestation ? (
                      <span className="inline-flex items-center gap-1 text-xs text-success-text">
                        <Shield className="h-3 w-3" />
                        AIM attested
                      </span>
                    ) : (
                      <span className="text-xs text-ink-tertiary">—</span>
                    )}
                  </td>
                  <td className="px-4 py-3">
                    <div className="flex items-center gap-2">
                      <button onClick={() => onRefresh(card)} className="p-1 text-ink-tertiary hover:text-brand-text transition-colors" title="Refresh">
                        <RefreshCw className="h-4 w-4" />
                      </button>
                      <button onClick={() => onDelete(card)} className="p-1 text-ink-tertiary hover:text-danger-text transition-colors" title="Delete">
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
    if (!userIdFilter) {
      setConsents(initialConsents);
      return;
    }
    // Debounce the server-side filter so typing does not fire one request
    // per keystroke.
    const handle = setTimeout(() => {
      api.listA2AConsents(userIdFilter).then(data => {
        const raw = data.consents || data as any;
        setConsents(Array.isArray(raw) ? raw : []);
      }).catch(() => setConsents([]));
    }, 350);
    return () => clearTimeout(handle);
  }, [userIdFilter, initialConsents]);

  const displayConsents = consents;

  if (loading) return <LoadingState />;

  return (
    <div className="space-y-6">
      {/* Info Banner */}
      <div className="bg-success-fill border border-success-border rounded-card p-4">
        <div className="flex items-start gap-3">
          <ShieldCheck className="h-5 w-5 text-success-text flex-shrink-0 mt-0.5" />
          <div>
            <h3 className="text-sm font-medium text-success-text">GDPR/PSD2 compliant consent management</h3>
            <p className="text-sm text-ink-body mt-1">
              All cross-agent data sharing requires explicit user consent. Consents are tracked with full audit trail,
              expiration management, and revocation support.
            </p>
          </div>
        </div>
      </div>

      {/* Search */}
      <div className="flex gap-4">
        <div className="flex-1 relative">
          <Search className="absolute left-3 top-1/2 transform -translate-y-1/2 h-4 w-4 text-ink-tertiary" />
          <input
            type="text"
            placeholder="Search by user ID..."
            value={userIdFilter}
            onChange={(e) => setUserIdFilter(e.target.value)}
            className="w-full pl-10 pr-4 py-2 border border-stroke rounded-inset bg-glass-inset text-ink placeholder:text-ink-tertiary focus:outline-none focus:ring-2 focus:ring-ring"
          />
        </div>
      </div>

      {/* Consents Table */}
      <div className="glass overflow-hidden">
        <div className="overflow-x-auto">
          <table className="min-w-full divide-y divide-divider">
            <thead className="bg-glass-inset-gray">
              <tr>
                <th className="px-4 py-3 text-left text-xs font-medium text-ink-tertiary uppercase">User</th>
                <th className="px-4 py-3 text-left text-xs font-medium text-ink-tertiary uppercase">Agents</th>
                <th className="px-4 py-3 text-left text-xs font-medium text-ink-tertiary uppercase">Purpose</th>
                <th className="px-4 py-3 text-left text-xs font-medium text-ink-tertiary uppercase">Data types</th>
                <th className="px-4 py-3 text-left text-xs font-medium text-ink-tertiary uppercase">Status</th>
                <th className="px-4 py-3 text-left text-xs font-medium text-ink-tertiary uppercase">Expires</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-divider">
              {displayConsents.map((consent) => (
                <tr key={consent.id} className="hover:bg-glass-inset-gray">
                  <td className="px-4 py-3">
                    <div className="flex items-center gap-2">
                      <Users className="h-4 w-4 text-ink-tertiary" />
                      <span className="text-sm text-ink">{consent.userId}</span>
                    </div>
                  </td>
                  <td className="px-4 py-3">
                    <div className="flex items-center gap-1 text-sm">
                      <span className="text-ink">{consent.sourceAgentId.slice(0, 8)}...</span>
                      <ArrowRight className="h-3 w-3 text-ink-tertiary" />
                      <span className="text-ink">{consent.targetAgentId.slice(0, 8)}...</span>
                    </div>
                  </td>
                  <td className="px-4 py-3">
                    <span className="text-sm text-ink">{consent.purpose}</span>
                  </td>
                  <td className="px-4 py-3">
                    <div className="flex flex-wrap gap-1">
                      {consent.dataTypes.slice(0, 2).map((type) => (
                        <span key={type} className="px-2 py-0.5 bg-glass-inset-gray rounded text-xs text-ink-body">
                          {type}
                        </span>
                      ))}
                      {consent.dataTypes.length > 2 && (
                        <span className="px-2 py-0.5 bg-glass-inset-gray rounded text-xs text-ink-tertiary">
                          +{consent.dataTypes.length - 2}
                        </span>
                      )}
                    </div>
                  </td>
                  <td className="px-4 py-3">
                    <StatusBadge status={consent.status} />
                  </td>
                  <td className="px-4 py-3">
                    <span className="text-sm text-ink-secondary">
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

  // Task states arrive upper-case from the backend; the counts and the filter compare lower-case.
  const filteredTasks = stateFilter === "all" ? tasks : tasks.filter(t => (t.state || "").toLowerCase() === stateFilter);

  const stateCounts = useMemo(() => {
    const counts: Record<string, number> = {};
    tasks.forEach(t => { const key = (t.state || "").toLowerCase(); counts[key] = (counts[key] || 0) + 1; });
    return counts;
  }, [tasks]);

  if (loading) return <LoadingState />;

  return (
    <div className="space-y-4">
      {/* Stats */}
      <div className="grid grid-cols-2 sm:grid-cols-4 lg:grid-cols-6 gap-3">
        {[
          { label: "Total", value: tasks.length, color: "text-ink" },
          { label: "Completed", value: stateCounts["completed"] || 0, color: "text-success-text" },
          { label: "Working", value: stateCounts["working"] || 0, color: "text-warning-text" },
          { label: "Submitted", value: stateCounts["submitted"] || 0, color: "text-brand-text" },
          { label: "Failed", value: stateCounts["failed"] || 0, color: "text-danger-text" },
          { label: "Cancelled", value: stateCounts["cancelled"] || 0, color: "text-ink-secondary" },
        ].map(s => (
          <div key={s.label} className="glass p-3 text-center">
            <div className={`text-xl font-semibold ${s.color}`}>{s.value}</div>
            <div className="text-xs text-ink-secondary">{s.label}</div>
          </div>
        ))}
      </div>

      {/* Filters */}
      <div className="flex gap-4">
        <select
          value={stateFilter}
          onChange={(e) => setStateFilter(e.target.value)}
          className="px-4 py-2 border border-stroke rounded-inset bg-glass-inset text-ink focus:outline-none focus:ring-2 focus:ring-ring"
        >
          <option value="all">All states ({tasks.length})</option>
          <option value="submitted">Submitted ({stateCounts["submitted"] || 0})</option>
          <option value="working">Working ({stateCounts["working"] || 0})</option>
          <option value="input_needed">Input needed ({stateCounts["input_needed"] || 0})</option>
          <option value="completed">Completed ({stateCounts["completed"] || 0})</option>
          <option value="failed">Failed ({stateCounts["failed"] || 0})</option>
          <option value="cancelled">Cancelled ({stateCounts["cancelled"] || 0})</option>
        </select>
      </div>

      {/* Tasks Table */}
      <div className="glass overflow-hidden">
        <div className="overflow-x-auto">
          <table className="min-w-full divide-y divide-divider">
            <thead className="bg-glass-inset-gray">
              <tr>
                <th className="px-4 py-3 text-left text-xs font-medium text-ink-tertiary uppercase">Task ID</th>
                <th className="px-4 py-3 text-left text-xs font-medium text-ink-tertiary uppercase">Client → Remote</th>
                <th className="px-4 py-3 text-left text-xs font-medium text-ink-tertiary uppercase">Skill</th>
                <th className="px-4 py-3 text-left text-xs font-medium text-ink-tertiary uppercase">State</th>
                <th className="px-4 py-3 text-left text-xs font-medium text-ink-tertiary uppercase">Messages</th>
                <th className="px-4 py-3 text-left text-xs font-medium text-ink-tertiary uppercase">Duration</th>
                <th className="px-4 py-3 text-left text-xs font-medium text-ink-tertiary uppercase">Created</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-divider">
              {filteredTasks.map((task) => (
                <tr key={task.id} className="hover:bg-glass-inset-gray">
                  <td className="px-4 py-3">
                    <span className="text-sm font-mono text-ink">{task.externalTaskId || task.id.slice(0, 8)}</span>
                  </td>
                  <td className="px-4 py-3">
                    <div className="flex items-center gap-1 text-sm">
                      <span className="text-ink">{task.clientAgentName || task.clientAgentId?.slice(0, 8) || "—"}</span>
                      <ArrowLeftRight className="h-3 w-3 text-ink-tertiary" />
                      <span className="text-ink">{task.remoteAgentName || task.remoteAgentId?.slice(0, 8) || "—"}</span>
                    </div>
                  </td>
                  <td className="px-4 py-3">
                    <span className="text-sm text-ink-body">{task.skillId || "—"}</span>
                  </td>
                  <td className="px-4 py-3">
                    <StatusBadge status={task.state} />
                  </td>
                  <td className="px-4 py-3">
                    <span className="text-sm text-ink">{task.messageCount || 0}</span>
                  </td>
                  <td className="px-4 py-3">
                    <span className="text-sm text-ink-secondary">
                      {task.durationMs ? `${task.durationMs}ms` : "—"}
                    </span>
                  </td>
                  <td className="px-4 py-3">
                    <span className="text-sm text-ink-secondary">
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
          { label: "Agents scored", value: trustScores.length, icon: Bot },
          { label: "Avg trust score", value: trustScores.length > 0
            ? `${(trustScores.reduce((a, s) => a + (s.a2aTrustScore || 0), 0) / trustScores.length * 100).toFixed(0)}%`
            : "N/A", icon: Shield },
          { label: "Total tasks", value: trustScores.reduce((a, s) => a + s.totalTasksAsClient + s.totalTasksAsRemote, 0), icon: Activity },
          { label: "Total peers", value: trustScores.reduce((a, s) => a + s.uniquePeersCount, 0), icon: Users },
        ].map(s => (
          <StatCard key={s.label} stat={{ name: s.label, value: s.value, icon: s.icon }} />
        ))}
      </div>

      {/* Agent Trust Scores Table */}
      <div className="glass overflow-hidden">
        <div className="px-6 py-4 border-b border-divider">
          <h3 className="text-lg font-medium text-ink">Agent A2A trust scores</h3>
        </div>
        <div className="overflow-x-auto">
          <table className="min-w-full divide-y divide-divider">
            <thead className="bg-glass-inset-gray">
              <tr>
                <th className="px-4 py-3 text-left text-xs font-medium text-ink-tertiary uppercase">Agent</th>
                <th className="px-4 py-3 text-left text-xs font-medium text-ink-tertiary uppercase">Trust score</th>
                <th className="px-4 py-3 text-left text-xs font-medium text-ink-tertiary uppercase">Tasks (client)</th>
                <th className="px-4 py-3 text-left text-xs font-medium text-ink-tertiary uppercase">Tasks (remote)</th>
                <th className="px-4 py-3 text-left text-xs font-medium text-ink-tertiary uppercase">Completed</th>
                <th className="px-4 py-3 text-left text-xs font-medium text-ink-tertiary uppercase">Failed</th>
                <th className="px-4 py-3 text-left text-xs font-medium text-ink-tertiary uppercase">Unique peers</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-divider">
              {enrichedScores.map((score) => (
                <tr key={score.agentId} className="hover:bg-glass-inset-gray">
                  <td className="px-4 py-3">
                    <div className="flex items-center gap-2">
                      <Bot className="h-4 w-4 text-brand-text" />
                      <span className="text-sm font-medium text-ink">{score.agentName}</span>
                    </div>
                  </td>
                  <td className="px-4 py-3">
                    <TrustScoreBadge score={score.a2aTrustScore || 0} />
                  </td>
                  <td className="px-4 py-3 text-sm text-ink">{score.totalTasksAsClient}</td>
                  <td className="px-4 py-3 text-sm text-ink">{score.totalTasksAsRemote}</td>
                  <td className="px-4 py-3 text-sm text-success-text">{score.tasksCompleted}</td>
                  <td className="px-4 py-3 text-sm text-danger-text">{score.tasksFailed}</td>
                  <td className="px-4 py-3 text-sm text-ink">{score.uniquePeersCount}</td>
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
        <Search className="absolute left-3 top-1/2 transform -translate-y-1/2 h-4 w-4 text-ink-tertiary" />
        <input
          type="text"
          placeholder="Search skills by name, description, or tags..."
          value={searchQuery}
          onChange={(e) => setSearchQuery(e.target.value)}
          className="w-full pl-10 pr-4 py-3 border border-stroke rounded-inset bg-glass-inset text-ink placeholder:text-ink-tertiary focus:outline-none focus:ring-2 focus:ring-ring"
        />
      </div>

      {/* Skills Grid */}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
        {skills.map((skill, idx) => (
          <div key={`${skill.id}-${idx}`} className="glass p-4 transition-shadow hover:shadow-chrome">
            <div className="flex items-start justify-between">
              <div className="flex items-center gap-2">
                <div className="h-8 w-8 bg-brand-soft rounded-inset-sm flex items-center justify-center">
                  <Zap className="h-4 w-4 text-brand-text" />
                </div>
                <div>
                  <h4 className="text-sm font-medium text-ink">{skill.name}</h4>
                  <p className="text-xs text-ink-tertiary">by {skill.agentName}</p>
                </div>
              </div>
            </div>
            <p className="mt-3 text-sm text-ink-body line-clamp-2">
              {skill.description || "No description available"}
            </p>
            {skill.tags && skill.tags.length > 0 && (
              <div className="mt-3 flex flex-wrap gap-1">
                {skill.tags.slice(0, 3).map((tag) => (
                  <span key={tag} className="px-2 py-0.5 bg-glass-inset-gray rounded text-xs text-ink-body">
                    {tag}
                  </span>
                ))}
              </div>
            )}
            {skill.inputModes && (
              <div className="mt-3 flex items-center gap-2 text-xs text-ink-secondary">
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
  // Per-dataset loading flags so each section paints as its call resolves,
  // instead of the whole page blocking on the slowest of the four (the
  // 500-task fetch).
  const [cardsLoading, setCardsLoading] = useState(true);
  const [tasksLoading, setTasksLoading] = useState(true);
  const [consentsLoading, setConsentsLoading] = useState(true);
  const [trustLoading, setTrustLoading] = useState(true);
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

  const fetchData = () => {
    // Fire all four in parallel (as before) but resolve each independently
    // so a section renders the moment its own data arrives.
    setCardsLoading(true);
    setTasksLoading(true);
    setConsentsLoading(true);
    setTrustLoading(true);

    api.listA2AAgentCards()
      .then(data => setAgentCards(data.cards || []))
      .catch(err => console.error("Failed to fetch A2A agent cards:", err))
      .finally(() => setCardsLoading(false));

    api.listA2ATasks({ limit: 500 })
      .then(data => setTasks(data.tasks || []))
      .catch(err => console.error("Failed to fetch A2A tasks:", err))
      .finally(() => setTasksLoading(false));

    api.listA2AConsents()
      .then(data => setConsents(data.consents || []))
      .catch(err => console.error("Failed to fetch A2A consents:", err))
      .finally(() => setConsentsLoading(false));

    api.listA2ATrustScores()
      .then(data => setTrustScores(data.scores || []))
      .catch(err => console.error("Failed to fetch A2A trust scores:", err))
      .finally(() => setTrustLoading(false));
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
        return <OverviewTab agentCards={agentCards} tasks={tasks} trustScores={trustScoreMap} cardsLoading={cardsLoading} tasksLoading={tasksLoading} trustLoading={trustLoading} />;
      case "cards":
        return <AgentCardsTab agentCards={agentCards} loading={cardsLoading} onRefresh={handleRefresh} onDelete={requestDelete} />;
      case "consent":
        return <ConsentTab consents={consents} loading={consentsLoading} />;
      case "tasks":
        return <TasksTab tasks={tasks} loading={tasksLoading} />;
      case "trust":
        return <TrustTab trustScores={trustScores} agentCards={agentCards} loading={trustLoading || cardsLoading} />;
      case "skills":
        return <SkillsTab agentCards={agentCards} loading={cardsLoading} />;
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
            <h1 className="text-2xl font-bold text-ink flex items-center gap-3">
              <Network className="h-7 w-7 text-brand-text" />
              A2A protocol
            </h1>
            <p className="mt-1 text-sm text-ink-secondary">
              Secure agent-to-agent communication with AIM attestation and trust scoring
            </p>
          </div>
          <div className="flex items-center gap-3">
            <button
              onClick={fetchData}
              className="flex items-center gap-2 px-4 py-2 text-sm font-semibold text-ink-body border border-stroke rounded-pill transition-colors hover:bg-glass-inset-gray"
            >
              <RefreshCw className="h-4 w-4" />
              Refresh
            </button>
            <a
              href="https://google.github.io/A2A/"
              target="_blank"
              rel="noopener noreferrer"
              className={buttonVariants()}
            >
              <ExternalLink className="h-4 w-4" />
              A2A spec
            </a>
          </div>
        </div>

        {/* Tabs */}
        <div className="border-b border-divider">
          <nav className="flex space-x-8 overflow-x-auto" aria-label="Tabs">
            {TABS.map((tab) => (
              <button
                key={tab.id}
                onClick={() => setActiveTab(tab.id)}
                className={`flex items-center gap-2 py-4 px-1 border-b-2 font-medium text-sm whitespace-nowrap transition-colors ${
                  activeTab === tab.id
                    ? "border-brand text-brand-text"
                    : "border-transparent text-ink-secondary hover:text-ink hover:border-stroke"
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
          title="Delete agent card"
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
