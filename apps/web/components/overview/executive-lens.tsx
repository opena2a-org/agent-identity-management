"use client";

import Link from "next/link";
import { ArrowRight } from "lucide-react";
import type { Agent } from "@/lib/api";
import { CardTitle, EmptyNote, KpiTile, relativeTime, trustDisplay } from "@/components/overview/shared";
import type { LensData, StatsData, VerificationActivityMonth, VerificationStatistics } from "@/components/overview/types";
import type { UserRole } from "@/lib/permissions";
import { effectiveEdgeRoles } from "@/lib/route-permissions";
import { cn } from "@/lib/utils";

/**
 * Executive lens of the overview: posture, coverage, incidents and one recommended action,
 * recovery-framed (no letter grades). Tiles without a backend source are not rendered, and
 * links are rendered only for roles the edge gate lets through.
 */
export function ExecutiveLens({
  stats,
  verification,
  activity,
  agents,
  lens,
  role,
}: {
  stats: StatsData;
  verification: VerificationStatistics | null;
  activity: VerificationActivityMonth[];
  agents: Agent[];
  lens: LensData;
  role: UserRole;
}) {
  const m = lens.security;
  const c = lens.compliance;
  const openViolations = lens.violations?.total ?? null;
  const canOpenSecurity = effectiveEdgeRoles("/dashboard/security").includes(role);
  const alertsHref = effectiveEdgeRoles("/dashboard/admin/alerts").includes(role) ? "/dashboard/admin/alerts" : canOpenSecurity ? "/dashboard/security" : undefined;
  const approved = verification && verification.totalVerifications > 0 ? Math.round((verification.successCount / verification.totalVerifications) * 1000) / 10 : null;
  const weekAgo = Date.now() - 7 * 86400000;
  const newThisWeek = agents.filter((a) => a.createdAt && new Date(a.createdAt).getTime() >= weekAgo).length;

  // Trust distribution from the agents list (0-1 scale), bucketed by score.
  const buckets = [
    { label: "High (0.80+)", color: "var(--brand)", n: 0 },
    { label: "Medium (0.50–0.79)", color: "var(--brand-sky)", n: 0 },
    { label: "Low (below 0.50)", color: "var(--amber)", n: 0 },
    { label: "Not scored", color: "var(--track)", n: 0 },
  ];
  for (const a of agents) {
    const s = typeof a.trustScore === "number" ? (a.trustScore <= 1 ? a.trustScore : a.trustScore / 100) : null;
    if (s === null) buckets[3].n++;
    else if (s >= 0.8) buckets[0].n++;
    else if (s >= 0.5) buckets[1].n++;
    else buckets[2].n++;
  }
  const total = agents.length;
  const r = 52;
  const circ = 2 * Math.PI * r;
  let offset = 0;
  const arcs = buckets.map((b) => {
    const frac = total ? b.n / total : 0;
    const arc = { ...b, dash: `${frac * circ} ${circ}`, offset: -offset * circ };
    offset += frac;
    return arc;
  });

  const recommended =
    stats.criticalAlerts > 0
      ? { text: `Review ${stats.criticalAlerts.toLocaleString()} critical ${stats.criticalAlerts === 1 ? "alert" : "alerts"}.`, href: alertsHref }
      : m && m.openIncidents > 0
      ? { text: `Review ${m.openIncidents.toLocaleString()} open ${m.openIncidents === 1 ? "incident" : "incidents"}.`, href: canOpenSecurity ? "/dashboard/security" : undefined }
      : stats.activeAlerts > 0
        ? { text: `Review ${stats.activeAlerts.toLocaleString()} open ${stats.activeAlerts === 1 ? "alert" : "alerts"}.`, href: alertsHref }
      : stats.pendingAgents > 0
        ? { text: `Verify ${stats.pendingAgents.toLocaleString()} pending ${stats.pendingAgents === 1 ? "agent" : "agents"}.`, href: "/dashboard/agents?filter=pending" }
        : m && m.mcpServersTotal > m.mcpServersVerified
          ? { text: `Attest ${(m.mcpServersTotal - m.mcpServersVerified).toLocaleString()} MCP ${m.mcpServersTotal - m.mcpServersVerified === 1 ? "server" : "servers"}.`, href: "/dashboard/mcp" }
          : null;

  return (
    <div className="flex flex-col gap-4">
      <div className="grid grid-cols-2 gap-3 md:grid-cols-3 xl:grid-cols-5 xl:gap-3.5">
        <KpiTile size="sm" label="Agents registered" value={stats.totalAgents.toLocaleString()} delta={newThisWeek > 0 ? `+${newThisWeek} in the last 7 days` : "none new in the last 7 days"} href="/dashboard/agents" />
        <KpiTile size="sm" label="MCP servers" value={stats.totalMcpServers.toLocaleString()} delta={m ? `${m.mcpServersVerified.toLocaleString()} of ${m.mcpServersTotal.toLocaleString()} attested` : `${stats.activeMcpServers.toLocaleString()} active`} href="/dashboard/mcp" />
        <KpiTile size="sm" label="Verifications, last 24h" value={verification ? verification.totalVerifications.toLocaleString() : "–"} delta={approved !== null ? `${approved}% approved` : verification ? "none yet" : "unavailable"} />
        {openViolations !== null && (
          <KpiTile size="sm" label="Violations, all time" value={openViolations.toLocaleString()} delta={m ? `${m.highSeverityCount.toLocaleString()} high severity` : undefined} tone={openViolations > 0 ? "alert" : "default"} href={canOpenSecurity ? "/dashboard/security/violations" : undefined} />
        )}
        {m && (
          <KpiTile size="sm" label="Security posture" value={m.securityScore.toLocaleString()} delta="of 100" tone="accent" href={canOpenSecurity ? "/dashboard/security" : undefined} />
        )}
      </div>

      <div className="grid gap-4 lg:grid-cols-[1.7fr_1fr] [&>*]:min-w-0">
        <div className="glass p-5">
          <CardTitle sub={verification ? `${verification.totalVerifications.toLocaleString()} today${approved !== null ? ` · ${approved}% approved` : ""}` : undefined}>Verification activity</CardTitle>
          {activity.length === 0 ? (
            <EmptyNote>No verifications recorded yet.</EmptyNote>
          ) : (
            <>
              <div className="mt-4 flex h-[150px] items-end gap-2" role="img" aria-label={`Verified per month: ${activity.map((a) => `${a.month} ${a.verified}`).join(", ")}`}>
                {activity.map((a, i) => {
                  const max = Math.max(1, ...activity.map((x) => x.verified + x.pending));
                  const h = Math.max(3, Math.round(((a.verified + a.pending) / max) * 100));
                  return (
                    <div key={a.monthYear} className="flex h-full flex-1 flex-col justify-end" title={`${a.month}: ${a.verified} verified, ${a.pending} pending`}>
                      <div className="w-full rounded-[4px] bg-brand" style={{ height: `${h}%`, opacity: i === activity.length - 1 ? 1 : 0.25 + 0.5 * ((a.verified + a.pending) / max) }} />
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

        <div className="glass p-5">
          <CardTitle sub="By trust score, from the agents list.">Trust distribution</CardTitle>
          {total === 0 ? (
            <EmptyNote>No agents yet.</EmptyNote>
          ) : (
            <div className="mt-3 flex items-center gap-5">
              <svg viewBox="0 0 140 140" width="118" height="118" role="img" aria-label={arcs.map((a) => `${a.label}: ${a.n}`).join(", ")}>
                <circle cx="70" cy="70" r={r} fill="none" stroke="var(--track)" strokeWidth="16" />
                {arcs.map((a) => (
                  <circle key={a.label} cx="70" cy="70" r={r} fill="none" stroke={a.color} strokeWidth="16" strokeDasharray={a.dash} strokeDashoffset={a.offset} transform="rotate(-90 70 70)" />
                ))}
                <text x="70" y="66" textAnchor="middle" fontSize="22" fontWeight="700" fill="var(--text-primary)">{trustDisplay(stats.avgTrustScore) ?? "–"}</text>
                <text x="70" y="82" textAnchor="middle" fontSize="9" letterSpacing="0.5" fill="var(--text-tertiary)">Avg trust</text>
              </svg>
              <ul className="flex flex-1 flex-col gap-2">
                {arcs.map((a) => (
                  <li key={a.label} className="flex items-center gap-2 text-xs text-ink-body">
                    <span className="h-[9px] w-[9px] rounded-[2px]" style={{ background: a.color }} aria-hidden="true" />
                    <span className="flex-1">{a.label}</span>
                    <span className="font-bold text-ink">{a.n}</span>
                  </li>
                ))}
              </ul>
            </div>
          )}
        </div>
      </div>

      <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-4 [&>*]:min-w-0">
        <div className="glass flex flex-col p-4">
          <CardTitle>Security</CardTitle>
          {m ? (
            <>
              <p className="mt-2 flex items-baseline gap-1.5">
                <span className="text-[30px] font-bold tracking-[-0.03em] text-ink">{m.securityScore}</span>
                <span className="text-xs font-semibold text-ink-tertiary">/ 100 posture</span>
              </p>
              <dl className="mt-auto pt-3 text-xs">
                {[
                  ["Open violations", openViolations !== null ? openViolations.toLocaleString() : "–"],
                  ["Open incidents", m.openIncidents.toLocaleString()],
                  ["Trusted agents", `${m.agentsTrusted.toLocaleString()} / ${m.agentsMonitored.toLocaleString()}`],
                ].map(([k, v]) => (
                  <div key={k} className="flex justify-between py-1">
                    <dt className="text-ink-secondary">{k}</dt>
                    <dd className="font-bold text-ink">{v}</dd>
                  </div>
                ))}
              </dl>
            </>
          ) : (
            <EmptyNote>Security metrics need the manager role. Ask an organization admin to change your role.</EmptyNote>
          )}
        </div>

        <div className="glass flex flex-col p-4">
          <CardTitle>MCP servers</CardTitle>
          <p className="mt-2 flex items-baseline gap-1.5">
            <span className="text-[30px] font-bold tracking-[-0.03em] text-ink">{stats.totalMcpServers.toLocaleString()}</span>
            <span className="text-xs font-semibold text-ink-tertiary">registered</span>
          </p>
          {m && m.mcpServersTotal > m.mcpServersVerified ? (
            <p className="text-xs font-semibold text-warning-text">{(m.mcpServersTotal - m.mcpServersVerified).toLocaleString()} awaiting attestation</p>
          ) : (
            <p className="text-xs font-semibold text-success-text">{stats.activeMcpServers.toLocaleString()} active</p>
          )}
          <dl className="mt-auto pt-3 text-xs">
            {[
              ["Attested", m ? `${m.mcpServersVerified.toLocaleString()} / ${m.mcpServersTotal.toLocaleString()}` : "–"],
              ["Active", stats.activeMcpServers.toLocaleString()],
            ].map(([k, v]) => (
              <div key={k} className="flex justify-between py-1">
                <dt className="text-ink-secondary">{k}</dt>
                <dd className="font-bold text-ink">{v}</dd>
              </div>
            ))}
          </dl>
        </div>

        <div className="glass flex flex-col p-4">
          <CardTitle>Compliance</CardTitle>
          {c ? (
            <>
              <p className="mt-2 text-[22px] font-bold capitalize tracking-[-0.02em] text-ink">{c.complianceLevel.replace(/[_-]/g, " ")}</p>
              <p className="text-xs font-semibold text-ink-secondary">{Math.round(c.verificationRate)}% of agents verified</p>
              <dl className="mt-auto pt-3 text-xs">
                {[
                  ["Verified agents", `${c.verifiedAgents.toLocaleString()} / ${c.totalAgents.toLocaleString()}`],
                  ["Recent audit events", c.recentAuditCount.toLocaleString()],
                ].map(([k, v]) => (
                  <div key={k} className="flex justify-between py-1">
                    <dt className="text-ink-secondary">{k}</dt>
                    <dd className="font-bold text-ink">{v}</dd>
                  </div>
                ))}
              </dl>
              <Link href="/dashboard/admin/compliance" className="mt-2 inline-flex items-center gap-1 text-xs font-semibold text-brand-text hover:underline">
                Open compliance <ArrowRight className="h-3.5 w-3.5" aria-hidden="true" />
              </Link>
            </>
          ) : (
            <EmptyNote>Compliance status needs the admin role. Ask an organization admin to change your role.</EmptyNote>
          )}
        </div>

        <div className="glass flex flex-col p-4">
          <CardTitle>Recommended action</CardTitle>
          {recommended ? (
            <>
              <p className="mt-2 text-[15px] font-bold tracking-[-0.02em] text-ink">{recommended.text}</p>
              {recommended.href && (
                <Link href={recommended.href} className="mt-auto inline-flex h-9 items-center justify-center gap-1.5 self-start rounded-pill bg-brand px-4 text-xs font-bold text-white shadow-glow hover:bg-brand-hover">
                  Open <ArrowRight className="h-3.5 w-3.5" aria-hidden="true" />
                </Link>
              )}
            </>
          ) : m && openViolations !== null ? (
            <EmptyNote>Nothing is waiting on you: no open alerts, incidents or pending agents, and every MCP server is attested.</EmptyNote>
          ) : (
            <EmptyNote>Recommended actions need the security metrics, which could not be loaded. Refresh to retry.</EmptyNote>
          )}
          {m?.recentBlockedActions?.length ? (
            <ul className="mt-4 divide-y divide-divider border-t border-divider pt-1">
              {m.recentBlockedActions.slice(0, 3).map((b) => (
                <li key={b.id} className="flex gap-2.5 py-2 text-xs">
                  <span className="mt-1.5 h-[7px] w-[7px] flex-shrink-0 rounded-full bg-danger" aria-hidden="true" />
                  <span className="min-w-0">
                    <span className="block truncate text-ink"><b>{b.agentName || b.agentId}</b> blocked from {b.attemptedCapability}</span>
                    <span className={cn("block text-2xs text-ink-tertiary")}>{relativeTime(b.createdAt)}{b.details ? ` · ${b.details}` : ""}</span>
                  </span>
                </li>
              ))}
            </ul>
          ) : null}
        </div>
      </div>
    </div>
  );
}
