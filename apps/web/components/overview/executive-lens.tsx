"use client";

import Link from "next/link";
import { ArrowRight } from "lucide-react";
import type { Agent } from "@/lib/api";
import { DonutChart, chartSeries, type DonutSlice } from "@/components/charts";
import { CardTitle, EmptyNote, KpiTile, VerificationsPerMonthChart, relativeTime, trustDisplay } from "@/components/overview/shared";
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
  const buckets: DonutSlice[] = [
    { name: "High (0.80+)", color: chartSeries.brand, value: 0 },
    { name: "Medium (0.50–0.79)", color: chartSeries.sky, value: 0 },
    { name: "Low (below 0.50)", color: chartSeries.amber, value: 0 },
    { name: "Not scored", color: chartSeries.muted, value: 0 },
  ];
  for (const a of agents) {
    const s = typeof a.trustScore === "number" ? (a.trustScore <= 1 ? a.trustScore : a.trustScore / 100) : null;
    if (s === null) buckets[3].value++;
    else if (s >= 0.8) buckets[0].value++;
    else if (s >= 0.5) buckets[1].value++;
    else buckets[2].value++;
  }
  const total = agents.length;
  const avgTrust = trustDisplay(stats.avgTrustScore);

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
            <VerificationsPerMonthChart activity={activity} legend className="mt-4 h-[234px]" />
          )}
        </div>

        <div className="glass p-5">
          <CardTitle sub="By trust score, from the agents list.">Trust distribution</CardTitle>
          {total === 0 ? (
            <EmptyNote>No agents yet.</EmptyNote>
          ) : (
            <div className="mt-3 flex items-center gap-5">
              <DonutChart
                title="Agents by trust score"
                data={buckets}
                columns={["Trust score", "Agents"]}
                center={avgTrust !== null ? { value: avgTrust, caption: "Avg trust" } : undefined}
                tooltip={false}
                innerRadius={38}
                outerRadius={52}
                paddingAngle={2}
                className="h-[162px] w-[118px] flex-shrink-0"
              />
              <ul className="flex flex-1 flex-col gap-2">
                {buckets.map((b) => (
                  <li key={b.name} className="flex items-center gap-2 text-xs text-ink-body">
                    <span className="h-[9px] w-[9px] rounded-[2px]" style={{ background: b.color }} aria-hidden="true" />
                    <span className="flex-1">{b.name}</span>
                    <span className="font-bold text-ink">{b.value}</span>
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
                <span className="text-3xl font-bold tracking-[-0.03em] text-ink">{m.securityScore}</span>
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
            <span className="text-3xl font-bold tracking-[-0.03em] text-ink">{stats.totalMcpServers.toLocaleString()}</span>
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
              <p className="mt-2 text-xl font-bold capitalize tracking-[-0.02em] text-ink">{c.complianceLevel.replace(/[_-]/g, " ")}</p>
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
              <Link href="/dashboard/compliance" className="mt-2 inline-flex min-h-6 items-center gap-1 text-xs font-semibold text-brand-text hover:underline">
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
              <p className="mt-2 text-base font-bold tracking-[-0.02em] text-ink">{recommended.text}</p>
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
                    <span className={cn("block text-xs text-ink-tertiary")}>{relativeTime(b.createdAt)}{b.details ? ` · ${b.details}` : ""}</span>
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
