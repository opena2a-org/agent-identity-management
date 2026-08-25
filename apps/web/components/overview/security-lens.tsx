"use client";

import Link from "next/link";
import { AlertTriangle, ArrowRight } from "lucide-react";
import { CardTitle, EmptyNote, KpiTile, StatusChip, relativeTime, shortTime, trustDisplay } from "@/components/overview/shared";
import type { LensData, StatsData, VerificationStatistics } from "@/components/overview/types";
import { cn } from "@/lib/utils";

/**
 * Security practitioner lens of the overview: what needs attention first, the verification
 * stream, policy coverage. Same objects as the developer lens, reordered and emphasized.
 * Every value comes from /security/metrics, /security/violations, /verification-events/*.
 */
export function SecurityLens({ stats, verification, lens }: { stats: StatsData; verification: VerificationStatistics | null; lens: LensData }) {
  const m = lens.security;
  const violations = lens.violations?.violations ?? [];
  const openViolations = lens.violations?.total ?? violations.length;
  const events = lens.events ?? [];
  const approved = verification && verification.totalVerifications > 0 ? Math.round((verification.successCount / verification.totalVerifications) * 1000) / 10 : null;
  const hero = violations[0] ?? null;
  const heroAgent = hero ? stats.agentsById?.[hero.agentId] : undefined;

  return (
    <div className="flex flex-col gap-4">
      <div className="grid grid-cols-2 gap-3 lg:grid-cols-4 lg:gap-4">
        <KpiTile label="Open violations" value={openViolations.toLocaleString()} delta={m ? `${m.highSeverityCount.toLocaleString()} high severity` : undefined} tone={openViolations > 0 ? "alert" : "default"} href="/dashboard/security/violations" />
        <KpiTile label="Verifications today" value={verification ? verification.totalVerifications.toLocaleString() : "–"} delta={approved !== null ? `${approved}% approved` : verification ? "none yet" : "unavailable"} />
        <KpiTile label="Blocked today" value={m ? m.actionsBlockedToday.toLocaleString() : "–"} delta={m ? `${m.blockedThreats.toLocaleString()} of ${m.totalThreats.toLocaleString()} threats blocked` : "unavailable"} />
        <KpiTile label="Avg trust" value={trustDisplay(m?.averageTrustScore ?? stats.avgTrustScore) ?? "–"} delta={`across ${stats.totalAgents.toLocaleString()} ${stats.totalAgents === 1 ? "agent" : "agents"}`} tone="accent" />
      </div>

      <div className="grid gap-4 lg:grid-cols-[1.7fr_1fr] [&>*]:min-w-0">
        <div className="flex min-w-0 flex-col gap-4">
          {hero ? (
            <div className="glass-alert flex flex-col gap-4 p-5 sm:p-6">
              <p className="inline-flex items-center gap-2 text-2xs font-bold uppercase tracking-[0.06em] text-danger-text">
                <AlertTriangle className="h-4 w-4" aria-hidden="true" /> Needs attention
              </p>
              <div className="flex flex-wrap items-center gap-4">
                <span className="inline-flex h-[52px] w-[52px] items-center justify-center rounded-inset bg-danger-fill text-lg font-bold text-danger-text">
                  {(hero.agentName || "?").slice(0, 1).toLowerCase()}
                </span>
                <div className="min-w-0 flex-1">
                  <p className="truncate text-lg font-bold tracking-[-0.02em] text-ink">{hero.agentName || hero.agentId}</p>
                  <p className="truncate text-xs font-semibold text-danger-text">
                    capability violation: {hero.attemptedCapability}
                    {hero.registeredCapabilities?.length ? ` (registered: ${hero.registeredCapabilities.join(", ")})` : ""}
                  </p>
                </div>
                {heroAgent && typeof heroAgent.trustScore === "number" && (
                  <div className="text-right">
                    <p className="text-[16px] font-bold tracking-[-0.02em] text-danger-text">{trustDisplay(heroAgent.trustScore)}</p>
                    <p className="text-overline">Trust</p>
                  </div>
                )}
              </div>
              <div className="grid grid-cols-3 gap-2.5">
                {[
                  ["Severity", hero.severity],
                  ["Blocked", relativeTime(hero.createdAt)],
                  ["Status", hero.isBlocked ? "Contained" : "Not blocked"],
                ].map(([k, v]) => (
                  <div key={k} className="rounded-inset-sm border border-glass-inset-border bg-glass-inset-gray px-3 py-2.5">
                    <p className="text-overline">{k}</p>
                    <p className={cn("mt-0.5 text-[13px] font-bold", k === "Severity" && "capitalize", v === "Contained" ? "text-success-text" : "text-ink")}>{v}</p>
                  </div>
                ))}
              </div>
              <div className="flex flex-wrap items-center justify-between gap-3">
                <p className="text-xs font-medium text-ink-secondary">
                  {hero.isBlocked ? "The action was denied before it ran." : "The action was not blocked; review the agent's policy."}
                  {typeof hero.trustScoreImpact === "number" && hero.trustScoreImpact !== 0 ? ` Trust impact ${hero.trustScoreImpact > 0 ? "+" : ""}${hero.trustScoreImpact.toFixed(2)}.` : ""}
                </p>
                <Link href="/dashboard/security/violations" className="inline-flex h-9 items-center rounded-pill bg-brand px-5 text-[13px] font-bold text-white shadow-accent hover:bg-brand-hover">
                  Review
                </Link>
              </div>
            </div>
          ) : (
            <div className="glass p-5 sm:p-6">
              <CardTitle sub="No open violations. Denied actions appear here the moment policy blocks one.">Nothing needs attention</CardTitle>
            </div>
          )}

          <div className="glass p-5">
            <CardTitle sub="Coverage of the controls that decide what an agent may do.">Policies and attestations</CardTitle>
            {m ? (
              <div className="mt-3 grid grid-cols-1 gap-3 sm:grid-cols-3">
                {[
                  { label: "Trusted agents", value: `${m.agentsTrusted.toLocaleString()} of ${m.agentsMonitored.toLocaleString()}`, unit: "monitored", ok: m.agentsMonitored === 0 || m.agentsTrusted === m.agentsMonitored },
                  { label: "MCP attestations", value: `${m.mcpServersVerified.toLocaleString()} of ${m.mcpServersTotal.toLocaleString()}`, unit: "verified", ok: m.mcpServersTotal === 0 || m.mcpServersVerified === m.mcpServersTotal },
                  { label: "Open incidents", value: m.openIncidents.toLocaleString(), unit: m.openIncidents === 1 ? "incident" : "incidents", ok: m.openIncidents === 0 },
                ].map((t) => (
                  <div key={t.label} className="rounded-inset border border-glass-inset-border bg-glass-inset-gray p-3.5">
                    <p className="text-xs font-semibold text-ink-secondary">{t.label}</p>
                    <p className={cn("mt-1 text-[20px] font-bold tracking-[-0.02em]", t.ok ? "text-success-text" : "text-danger-text")}>
                      {t.value} <span className="text-2xs font-semibold text-ink-tertiary">{t.unit}</span>
                    </p>
                  </div>
                ))}
              </div>
            ) : (
              <EmptyNote>Security metrics are unavailable for this role.</EmptyNote>
            )}
            {m && m.riskByCategory?.length > 0 && (
              <div className="mt-4">
                <p className="text-overline">Blocked by capability</p>
                <ul className="mt-2 divide-y divide-divider">
                  {m.riskByCategory.slice(0, 5).map((r) => (
                    <li key={r.category} className="flex items-center justify-between py-2 text-xs">
                      <span className="font-mono font-semibold text-ink">{r.category}</span>
                      <span className="flex items-center gap-2">
                        <span className="text-ink-secondary">{r.blocked.toLocaleString()} blocked</span>
                        <StatusChip kind={r.riskLevel === "critical" || r.riskLevel === "high" ? "deny" : r.riskLevel === "medium" ? "pending" : "neutral"}>{r.riskLevel}</StatusChip>
                      </span>
                    </li>
                  ))}
                </ul>
              </div>
            )}
          </div>
        </div>

        <div className="glass flex min-w-0 flex-col p-5">
          <CardTitle
            action={
              <span className="inline-flex items-center gap-1.5 text-2xs font-semibold text-ink-secondary">
                <span className="h-1.5 w-1.5 rounded-full bg-success shadow-[0_0_6px_rgba(52,199,89,0.7)]" aria-hidden="true" />
                last 60 minutes
              </span>
            }
          >
            Verification stream
          </CardTitle>
          {events.length === 0 ? (
            <EmptyNote>No verifications in the last hour. Each request an agent makes is checked against policy before it runs and lands here.</EmptyNote>
          ) : (
            <ul className="mt-1 divide-y divide-divider">
              {events.slice(0, 8).map((e) => {
                const deny = e.status === "failed" || e.status === "denied" || e.status === "timeout";
                return (
                  <li key={e.id} className={cn("flex items-center gap-3 py-2.5", deny && "-mx-2 rounded-avatar bg-danger-fill px-2")}>
                    <span className={cn("w-12 text-2xs font-semibold", deny ? "text-danger-text" : "text-ink-tertiary")}>{shortTime(e.startedAt || e.createdAt)}</span>
                    <span className="min-w-0 flex-1">
                      <span className="block truncate text-[13px] font-bold text-ink">{e.agentName || e.agentId}</span>
                      {deny && <span className="block truncate text-2xs font-semibold text-danger-text">{e.verificationType} failed</span>}
                    </span>
                    <StatusChip kind={deny ? "deny" : e.status === "pending" ? "pending" : "pass"}>{deny ? "Deny" : e.status === "pending" ? "Pending" : "Pass"}</StatusChip>
                  </li>
                );
              })}
            </ul>
          )}
          <p className="mt-auto pt-4 text-2xs text-ink-tertiary">Every request is checked against policy before it runs.</p>
          <Link href="/dashboard/security" className="mt-2 inline-flex items-center gap-1 text-xs font-semibold text-brand-text hover:underline">
            Open Security <ArrowRight className="h-3.5 w-3.5" aria-hidden="true" />
          </Link>
        </div>
      </div>
    </div>
  );
}
