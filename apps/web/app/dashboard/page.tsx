"use client";

import { Suspense, useCallback, useEffect, useMemo, useState } from "react";
import { useSearchParams } from "next/navigation";
import Link from "next/link";
import { formatDistanceToNowStrict } from "date-fns";
import { ArrowRight, Bell, Server, ShieldCheck, ShieldPlus } from "lucide-react";
import { api, type Agent } from "@/lib/api";
import { getDashboardPermissions, type UserRole } from "@/lib/permissions";
import { effectiveEdgeRoles } from "@/lib/route-permissions";
import { getErrorMessage } from "@/lib/error-messages";
import { usePersona } from "@/lib/persona";
import { AuthGuard } from "@/components/auth-guard";
import { ActivityTimeline } from "@/components/analytics/activity-timeline";
import { Skeleton } from "@/components/ui/skeleton";
import { SecurityLens } from "@/components/overview/security-lens";
import { ExecutiveLens } from "@/components/overview/executive-lens";
import type { LensData } from "@/components/overview/types";
import { cn } from "@/lib/utils";

/* ----------------------------------------------------------------------------
 * Data
 * ------------------------------------------------------------------------- */

interface DashboardStats {
  totalAgents: number;
  verifiedAgents: number;
  pendingAgents: number;
  verificationRate: number;
  avgTrustScore: number;
  totalMcpServers: number;
  activeMcpServers: number;
  totalUsers: number;
  activeUsers: number;
  activeAlerts: number;
  criticalAlerts: number;
  securityIncidents: number;
  organizationId: string;
}

interface VerificationStatistics {
  totalVerifications: number;
  successCount: number;
  failedCount: number;
  pendingCount: number;
  uniqueAgentsVerified: number;
}

interface VerificationActivityMonth {
  month: string;
  verified: number;
  pending: number;
  monthYear: string;
}

interface HomeData {
  stats: DashboardStats | null;
  verification: VerificationStatistics | null;
  activity: VerificationActivityMonth[];
  agents: Agent[];
  user: { name: string; role: UserRole } | null;
}

// The canonical quickstart. Every surface that teaches the install path must match this.
// Python only: the npm SDK does not export secure() and ships no aim-sdk binary (measured
// 2026-08-24, @opena2a/aim-sdk 1.3.0), so a TypeScript command block would not run.
export const QUICKSTART_LANGS = ["python"] as const;
type QuickstartLang = (typeof QUICKSTART_LANGS)[number];

/**
 * `aim-sdk login` defaults to the hosted service (sdk/python/aim_sdk/cli.py DEFAULT_AIM_URL);
 * a self-hosted dashboard passes its own origin, which proxies /api to the backend.
 */
export function quickstartLines(lang: QuickstartLang, origin: string): readonly string[] {
  const login = origin ? `aim-sdk login --url ${origin}` : "aim-sdk login";
  switch (lang) {
    case "python":
    default:
      return ["pip install aim-sdk", login, 'python -c "from aim_sdk import secure; secure(\\"my-first-agent\\")"'];
  }
}

/** The page origin, known only in the browser (empty during prerender). */
function useOrigin() {
  const [origin, setOrigin] = useState("");
  useEffect(() => {
    setOrigin(window.location.origin);
  }, []);
  return origin;
}

/** The role claim of the session token, for when /users/me is unavailable; pending reads as viewer. */
function roleFromToken(): UserRole {
  try {
    const token = api.getToken();
    if (!token) return "viewer";
    const segment = token.split(".")[1].replace(/-/g, "+").replace(/_/g, "/");
    const role = JSON.parse(atob(segment))?.role;
    return role === "admin" || role === "manager" || role === "member" ? role : "viewer";
  } catch {
    return "viewer";
  }
}

function greeting(name?: string) {
  const h = new Date().getHours();
  const part = h < 12 ? "Good morning" : h < 18 ? "Good afternoon" : "Good evening";
  return name ? `${part}, ${name}` : part;
}

function firstName(user: { name?: string; email?: string } | null | undefined) {
  const n = user?.name?.trim();
  if (n) return n.split(/\s+/)[0];
  return user?.email?.split("@")[0] ?? "";
}

function relative(iso?: string) {
  if (!iso) return "";
  try {
    return formatDistanceToNowStrict(new Date(iso), { addSuffix: true });
  } catch {
    return "";
  }
}

const AVATAR_TINTS = [
  "from-[#e0f2fe] to-[#bae6fd] text-[#0369a1] dark:from-[#0c4a6e] dark:to-[#075985] dark:text-[#7dd3fc]",
  "from-[#ede9fe] to-[#ddd6fe] text-[#6d28d9] dark:from-[#4c1d95] dark:to-[#5b21b6] dark:text-[#c4b5fd]",
  "from-[#dcfce7] to-[#bbf7d0] text-[#15803d] dark:from-[#14532d] dark:to-[#166534] dark:text-[#86efac]",
  "from-[#ffe4e6] to-[#fecdd3] text-[#be123c] dark:from-[#881337] dark:to-[#9f1239] dark:text-[#fda4af]",
];

/* ----------------------------------------------------------------------------
 * Pieces
 * ------------------------------------------------------------------------- */

function KpiTile({
  label,
  value,
  delta,
  tone = "default",
  href,
}: {
  label: string;
  value: string;
  delta?: string;
  tone?: "default" | "accent" | "alert";
  href?: string;
}) {
  const body = (
    <>
      <p className={cn("text-xs font-semibold", tone === "alert" ? "text-danger-text" : "text-ink-secondary")}>{label}</p>
      <p className={cn("text-kpi mt-2", tone === "accent" && "text-brand-text", tone === "alert" && "text-danger-text")}>{value}</p>
      {delta ? <p className="mt-1 text-xs font-semibold text-ink-secondary">{delta}</p> : null}
    </>
  );
  const cls = cn(tone === "alert" ? "glass-alert" : "glass", "block p-5 transition-transform", href && "hover:-translate-y-0.5");
  return href ? (
    <Link href={href} className={cls}>
      {body}
    </Link>
  ) : (
    <div className={cls}>{body}</div>
  );
}

function CodeBlock({ lines, className }: { lines: readonly string[]; className?: string }) {
  return (
    <pre className={cn("code-block", className)} aria-label="Commands">
      {lines.map((l, i) => (
        <span key={i} className="block">
          <span className="select-none text-ink-inverse-secondary">$ </span>
          {l}
        </span>
      ))}
    </pre>
  );
}

function OriginQuickstart({ className }: { className?: string }) {
  const origin = useOrigin();
  return <CodeBlock lines={quickstartLines("python", origin)} className={className} />;
}

function Quickstart({ compact = false }: { compact?: boolean }) {
  const [lang, setLang] = useState<QuickstartLang>("python");
  const origin = useOrigin();
  return (
    <div className={cn("glass-contrast flex min-w-0 flex-col gap-3 overflow-hidden p-5", compact && "p-4")}>
      <div className="flex items-center justify-between gap-3">
        <h3 className="text-[13.5px] font-bold">Quickstart</h3>
        <div className="flex gap-1.5" role="tablist" aria-label="Language">
          {QUICKSTART_LANGS.map((k) => (
            <button
              key={k}
              type="button"
              role="tab"
              aria-selected={lang === k}
              onClick={() => setLang(k)}
              className={cn(
                "rounded-pill px-3 py-1 text-2xs font-bold transition-colors",
                lang === k ? "bg-brand text-white shadow-glow" : "bg-white/10 text-ink-inverse-secondary hover:text-ink-inverse"
              )}
            >
              {k === "python" ? "Python" : "TypeScript"}
            </button>
          ))}
        </div>
      </div>
      <CodeBlock lines={quickstartLines(lang, origin)} />
      <p className="text-xs leading-relaxed text-ink-inverse-secondary">
        The SDK creates an Ed25519 keypair on your machine, registers the agent under your account and stores the credentials in{" "}
        <span className="font-mono text-ink-inverse">~/.aim/</span>. The private key never leaves your machine.
      </p>
      <Link href="/dashboard/developers" className="inline-flex items-center gap-1.5 self-start rounded-pill bg-white/10 px-4 py-2 text-xs font-bold text-ink-inverse hover:bg-white/15">
        Open the developer guide <ArrowRight className="h-3.5 w-3.5" aria-hidden="true" />
      </Link>
    </div>
  );
}

function VerificationActivityCard({ activity }: { activity: VerificationActivityMonth[] }) {
  const max = Math.max(1, ...activity.map((m) => m.verified + m.pending));
  return (
    <div className="glass flex flex-col p-5">
      <div className="flex items-center justify-between">
        <h3 className="text-[13.5px] font-bold text-ink">Verification activity</h3>
        {activity.length > 0 && (
          <span className="text-2xs text-ink-tertiary">last {activity.length} months</span>
        )}
      </div>
      {activity.length === 0 ? (
        <p className="mt-3 text-xs text-ink-secondary">No verifications yet. They show up here as your agents check in.</p>
      ) : (
        <>
          <div className="mt-3 flex h-16 items-end gap-1.5" role="img" aria-label={`Verified per month: ${activity.map((m) => `${m.month} ${m.verified}`).join(", ")}`}>
            {activity.map((m, i) => {
              const total = m.verified + m.pending;
              const h = Math.max(4, Math.round((total / max) * 100));
              const last = i === activity.length - 1;
              return (
                <div key={m.monthYear} className="flex flex-1 flex-col justify-end" style={{ height: "100%" }} title={`${m.month}: ${m.verified} verified, ${m.pending} pending`}>
                  <div className="w-full rounded-[4px] bg-brand" style={{ height: `${h}%`, opacity: last ? 1 : 0.2 + 0.5 * (total / max) }} />
                </div>
              );
            })}
          </div>
          <div className="mt-2 flex justify-between text-2xs text-ink-tertiary">
            <span>{activity[0]?.month}</span>
            <span>{activity[activity.length - 1]?.month}</span>
          </div>
        </>
      )}
    </div>
  );
}

function AgentsCard({ agents, total }: { agents: Agent[]; total: number }) {
  return (
    <div className="glass flex flex-col p-5">
      <div className="flex items-center justify-between">
        <h3 className="text-[15px] font-bold tracking-[-0.02em] text-ink">Agents</h3>
        <Link href="/dashboard/agents" className="text-xs font-semibold text-brand-text hover:underline">
          See all {total > agents.length ? `(${total})` : ""}
        </Link>
      </div>
      <ul className="mt-2 divide-y divide-divider">
        {agents.map((a, i) => {
          const score = typeof a.trustScore === "number" ? a.trustScore : null;
          const pct = score === null ? 0 : Math.round((score <= 1 ? score : score / 100) * 100);
          const attention = a.status === "suspended" || a.status === "revoked";
          return (
            <li key={a.id}>
              <Link href={`/dashboard/agents/${a.id}`} className="flex items-center gap-3.5 py-3 hover:opacity-90">
                <span className={cn("inline-flex h-[34px] w-[34px] flex-shrink-0 items-center justify-center rounded-avatar bg-gradient-to-br text-[13px] font-bold", AVATAR_TINTS[i % AVATAR_TINTS.length])}>
                  {(a.displayName || a.name || "?").slice(0, 1).toLowerCase()}
                </span>
                <span className="min-w-0 flex-1">
                  <span className="block truncate text-[13.5px] font-bold text-ink">{a.displayName || a.name}</span>
                  <span className={cn("block truncate text-xs", attention ? "font-semibold text-danger-text" : "text-ink-tertiary")}>
                    {a.status}
                    {a.updatedAt ? ` · updated ${relative(a.updatedAt)}` : ""}
                  </span>
                </span>
                {score !== null && (
                  <>
                    <span className={cn("text-[13px] font-bold", attention ? "text-danger-text" : "text-brand-text")}>{(pct / 100).toFixed(2)}</span>
                    <span className="hidden h-[5px] w-[90px] overflow-hidden rounded-[3px] bg-track sm:block" aria-hidden="true">
                      <span className={cn("block h-full rounded-[3px]", attention ? "bg-danger" : "bg-bar")} style={{ width: `${pct}%` }} />
                    </span>
                  </>
                )}
              </Link>
            </li>
          );
        })}
      </ul>
    </div>
  );
}

function FirstAgentCard() {
  return (
    <div className="glass flex flex-col gap-4 p-6">
      <div className="flex items-start gap-3">
        <span className="inline-flex h-10 w-10 flex-shrink-0 items-center justify-center rounded-full bg-brand-soft text-brand-text">
          <ShieldPlus className="h-5 w-5" aria-hidden="true" />
        </span>
        <div>
          <h3 className="text-[15px] font-bold tracking-[-0.02em] text-ink">Secure your first agent</h3>
          <p className="mt-1 text-xs leading-relaxed text-ink-secondary">
            Three commands give an agent a verifiable identity. Refresh this page once it has checked in and it appears here with its trust score.
          </p>
        </div>
      </div>
      <OriginQuickstart className="!bg-glass-inset-gray !text-ink" />
      <div className="flex flex-wrap items-center gap-2">
        <Link href="/dashboard/agents?register=1" className="inline-flex h-9 items-center gap-2 rounded-pill bg-brand px-4 text-xs font-bold text-white shadow-glow hover:bg-brand-hover">
          Secure it in the browser instead
        </Link>
        <Link href="/dashboard/developers" className="inline-flex h-9 items-center rounded-pill border border-stroke bg-glass px-4 text-xs font-bold text-ink">
          Developer guide
        </Link>
      </div>
    </div>
  );
}

function HomeSkeleton() {
  return (
    <div className="flex flex-col gap-4" aria-busy="true" aria-label="Loading overview">
      <div className="px-1">
        <Skeleton className="h-3 w-40" />
        <Skeleton className="mt-2 h-7 w-96 max-w-full" />
      </div>
      <div className="grid grid-cols-2 gap-3 lg:grid-cols-4 lg:gap-4">
        {[0, 1, 2, 3].map((i) => (
          <div key={i} className="glass p-5">
            <Skeleton className="h-3 w-24" />
            <Skeleton className="mt-3 h-8 w-16" />
          </div>
        ))}
      </div>
      <div className="grid gap-4 lg:grid-cols-[1.7fr_1fr]">
        <div className="glass h-64 p-5" />
        <div className="glass h-64 p-5" />
      </div>
    </div>
  );
}

/* ----------------------------------------------------------------------------
 * Page
 * ------------------------------------------------------------------------- */

function DashboardContent() {
  const searchParams = useSearchParams();
  const persona = usePersona((s) => s.persona);
  const [data, setData] = useState<HomeData | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [lensData, setLensData] = useState<LensData | null>(null);
  const [lensLoading, setLensLoading] = useState(false);

  const load = useCallback(async () => {
    // A token handed over on the URL (device flow, OAuth) becomes the session.
    const token = searchParams.get("token");
    if (token) {
      api.setToken(token, undefined, "new-session");
      window.history.replaceState({}, "", "/dashboard");
    }
    setLoading(true);
    setError(null);
    const [stats, verification, activity, agents, user] = await Promise.allSettled([
      api.getDashboardStats(),
      api.getVerificationStatistics("24h"),
      api.getVerificationActivity(6),
      api.listAgents(),
      api.getCurrentUser(),
    ]);
    if (stats.status !== "fulfilled") {
      setError(getErrorMessage(stats.reason, { resource: "dashboard data", action: "load" }));
      setLoading(false);
      return;
    }
    const role: UserRole = user.status === "fulfilled" && user.value?.role && user.value.role !== "pending" ? (user.value.role as UserRole) : roleFromToken();
    setData({
      stats: stats.value as DashboardStats,
      verification: verification.status === "fulfilled" ? (verification.value as VerificationStatistics) : null,
      activity: activity.status === "fulfilled" ? (activity.value?.activity ?? []) : [],
      agents: agents.status === "fulfilled" ? agents.value?.agents ?? [] : [],
      user: { name: user.status === "fulfilled" ? firstName(user.value) : "", role },
    });
    setLoading(false);
  }, [searchParams]);

  useEffect(() => {
    load();
  }, [load]);

  const permissions = useMemo(() => getDashboardPermissions(data?.user?.role), [data?.user?.role]);

  // The Security and Executive lenses read endpoints the developer lens does not need. They are
  // fetched only for roles the permission map already allows; a lens never widens authorization.
  useEffect(() => {
    if (persona === "developer") return;
    if (!data?.user || !permissions.canViewSecurityMetrics) {
      setLensData({ security: null, violations: null, events: null, compliance: null });
      return;
    }
    let cancelled = false;
    setLensLoading(true);
    Promise.allSettled([
      api.getSecurityMetrics(),
      api.getSecurityViolations(20, 0),
      api.getRecentVerificationEvents(60),
      permissions.canViewAdmin ? api.getComplianceStatus() : Promise.reject(new Error("admin only")),
    ]).then(([security, violations, events, compliance]) => {
      if (cancelled) return;
      setLensData({
        security: security.status === "fulfilled" ? (security.value as LensData["security"]) : null,
        violations: violations.status === "fulfilled" ? (violations.value as LensData["violations"]) : null,
        events: events.status === "fulfilled" ? ((events.value as { events: LensData["events"] }).events ?? []) : null,
        compliance: compliance.status === "fulfilled" ? (compliance.value as LensData["compliance"]) : null,
      });
      setLensLoading(false);
    });
    return () => {
      cancelled = true;
    };
  }, [data?.user, persona, permissions.canViewSecurityMetrics, permissions.canViewAdmin]);

  if (loading) return <HomeSkeleton />;
  if (error || !data?.stats) {
    return (
      <div className="glass mx-auto mt-10 max-w-md p-8 text-center">
        <p className="text-overline">Overview</p>
        <h2 className="text-headline mt-2">The overview could not load.</h2>
        <p className="mt-2 text-sm text-ink-secondary">{error ?? "No data was returned."}</p>
        <button type="button" onClick={load} className="mt-5 inline-flex h-10 items-center rounded-pill bg-brand px-5 text-sm font-bold text-white shadow-glow hover:bg-brand-hover">
          Try again
        </button>
      </div>
    );
  }

  const { stats, verification, activity, agents } = data;
  const zero = stats.totalAgents === 0 && agents.length === 0;
  const approvedPct =
    verification && verification.totalVerifications > 0
      ? Math.round((verification.successCount / verification.totalVerifications) * 1000) / 10
      : null;

  // Violations carry no open/resolved state, so the count is all-time. Null means the role may
  // not read them or the request failed; the headline then falls through to the developer one.
  const violationCount = lensData?.violations?.total ?? null;
  const lensReady = persona === "developer" || (lensData !== null && !lensLoading);
  const developerHeadline =
    stats.criticalAlerts > 0
      ? `${stats.criticalAlerts} critical ${stats.criticalAlerts === 1 ? "alert needs" : "alerts need"} attention.`
      : stats.pendingAgents > 0
        ? `${stats.pendingAgents} ${stats.pendingAgents === 1 ? "agent is" : "agents are"} waiting for verification.`
        : "No critical alerts. No agents waiting for verification.";
  const headline = zero
    ? "No agents secured yet."
    : persona === "security" && lensReady && violationCount !== null
      ? violationCount > 0
        ? `${violationCount} capability ${violationCount === 1 ? "violation" : "violations"} recorded.`
        : "No capability violations recorded."
      : persona === "executive" && lensReady
        ? `${stats.verifiedAgents.toLocaleString()} of ${stats.totalAgents.toLocaleString()} agents verified.`
        : developerHeadline;
  // Only asserted when every input it depends on is known and zero.
  const allClear =
    stats.criticalAlerts === 0 && stats.activeAlerts === 0 && violationCount === 0 && verification !== null && verification.failedCount === 0;
  const role = data.user?.role ?? "viewer";
  const canOpenAlerts = permissions.canViewAlerts && effectiveEdgeRoles("/dashboard/admin/alerts").includes(role);

  const sortedAgents = [...agents]
    .sort((a, b) => (a.status === "suspended" || a.status === "revoked" ? -1 : 0) - (b.status === "suspended" || b.status === "revoked" ? -1 : 0) || (b.updatedAt ?? "").localeCompare(a.updatedAt ?? ""))
    .slice(0, 6);

  return (
    <div className="flex flex-col gap-4" data-persona={persona}>
      {/* Header */}
      <div className="flex flex-wrap items-end justify-between gap-3 px-1">
        <div>
          <p className="text-xs font-semibold text-ink-secondary">{persona === "executive" && !zero ? "Overview" : greeting(data.user?.name)}</p>
          <h1 className="text-headline mt-0.5">{headline}</h1>
          {persona === "executive" && !zero && lensReady && (stats.criticalAlerts > 0 || allClear) && (
            <p className="mt-1 text-xs text-ink-secondary">
              {stats.criticalAlerts > 0
                ? `${stats.criticalAlerts} critical ${stats.criticalAlerts === 1 ? "alert needs" : "alerts need"} attention.`
                : "No open alerts, no failed verifications in the last 24 hours, no violations recorded."}
            </p>
          )}
        </div>
        {!zero && (
          <Link href="/dashboard/agents?register=1" className="hidden h-10 items-center gap-2 rounded-pill bg-brand px-5 text-[13.5px] font-bold text-white shadow-glow hover:bg-brand-hover sm:inline-flex">
            <ShieldPlus className="h-4 w-4" aria-hidden="true" />
            Secure an agent
          </Link>
        )}
      </div>

      {!zero && persona !== "developer" && !lensReady && <HomeSkeleton />}
      {!zero && persona === "security" && lensReady && lensData && (
        <SecurityLens stats={{ ...stats, agentsById: Object.fromEntries(agents.map((a) => [a.id, a])) }} verification={verification} lens={lensData} role={role} />
      )}
      {!zero && persona === "executive" && lensReady && lensData && (
        <ExecutiveLens stats={stats} verification={verification} activity={activity} agents={agents} lens={lensData} role={role} />
      )}
      {(zero || persona === "developer") && (
        <>
      {/* KPIs */}
      <div className="grid grid-cols-2 gap-3 lg:grid-cols-4 lg:gap-4">
        <KpiTile label="Agents secured" value={stats.totalAgents.toLocaleString()} delta={`${stats.verifiedAgents.toLocaleString()} verified`} href="/dashboard/agents" />
        <KpiTile
          label="Verifications, last 24h"
          value={verification ? verification.totalVerifications.toLocaleString() : "–"}
          delta={approvedPct !== null ? `${approvedPct}% approved` : verification ? "none yet" : "unavailable"}
        />
        <KpiTile
          label="Avg trust score"
          value={stats.totalAgents > 0 ? (stats.avgTrustScore <= 1 ? stats.avgTrustScore : stats.avgTrustScore / 100).toFixed(2) : "–"}
          delta={stats.totalAgents > 0 ? `across ${stats.totalAgents.toLocaleString()} ${stats.totalAgents === 1 ? "agent" : "agents"}` : "no agents yet"}
          tone="accent"
        />
        <KpiTile
          label="Open alerts"
          value={stats.activeAlerts.toLocaleString()}
          delta={stats.criticalAlerts > 0 ? `${stats.criticalAlerts} critical` : "none critical"}
          tone={stats.criticalAlerts > 0 ? "alert" : "default"}
          href={canOpenAlerts ? "/dashboard/admin/alerts" : undefined}
        />
      </div>

      {/* Main */}
      <div className="grid gap-4 lg:grid-cols-[1.7fr_1fr] [&>*]:min-w-0">
        {zero ? <FirstAgentCard /> : <AgentsCard agents={sortedAgents} total={stats.totalAgents} />}
        <div className="flex min-w-0 flex-col gap-4">
          <Quickstart compact />
          <VerificationActivityCard activity={activity} />
        </div>
      </div>

      {/* Secondary, role-gated */}
      <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3 [&>*]:min-w-0">
        {permissions.canViewMCPStats && (
          <Link href="/dashboard/mcp" className="glass flex items-start gap-3 p-5 hover:-translate-y-0.5 transition-transform">
            <Server className="mt-0.5 h-4 w-4 text-ink-tertiary" aria-hidden="true" />
            <span>
              <span className="block text-[13.5px] font-bold text-ink">MCP servers</span>
              <span className="block text-xs text-ink-secondary">
                {stats.totalMcpServers.toLocaleString()} registered · {stats.activeMcpServers.toLocaleString()} active
              </span>
            </span>
          </Link>
        )}
        {permissions.canViewSecurityMetrics && (
          <Link href="/dashboard/security" className="glass flex items-start gap-3 p-5 hover:-translate-y-0.5 transition-transform">
            <ShieldCheck className="mt-0.5 h-4 w-4 text-ink-tertiary" aria-hidden="true" />
            <span>
              <span className="block text-[13.5px] font-bold text-ink">Security</span>
              <span className="block text-xs text-ink-secondary">
                {stats.securityIncidents.toLocaleString()} {stats.securityIncidents === 1 ? "incident" : "incidents"} · {stats.activeAlerts.toLocaleString()} open {stats.activeAlerts === 1 ? "alert" : "alerts"}
              </span>
            </span>
          </Link>
        )}
        {canOpenAlerts && (
          <Link href="/dashboard/admin/alerts" className="glass flex items-start gap-3 p-5 hover:-translate-y-0.5 transition-transform">
            <Bell className="mt-0.5 h-4 w-4 text-ink-tertiary" aria-hidden="true" />
            <span>
              <span className="block text-[13.5px] font-bold text-ink">Alerts</span>
              <span className="block text-xs text-ink-secondary">{stats.criticalAlerts.toLocaleString()} critical waiting for review</span>
            </span>
          </Link>
        )}
      </div>

        </>
      )}
      {permissions.canViewRecentActivity && !zero && <ActivityTimeline defaultLimit={8} />}
    </div>
  );
}

export default function DashboardPage() {
  return (
    <AuthGuard>
      <Suspense fallback={<HomeSkeleton />}>
        <DashboardContent />
      </Suspense>
    </AuthGuard>
  );
}
