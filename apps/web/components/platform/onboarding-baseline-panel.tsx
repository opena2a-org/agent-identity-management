"use client";

import { useCallback, useEffect, useState } from "react";
import { api, type ApiRequestError, type OnboardingBaseline, type OnboardingEventName, type TimeToFirstAgentStats } from "@/lib/api";
import { Skeleton } from "@/components/ui/skeleton";
import { cn } from "@/lib/utils";

/** Seconds as the largest one or two units that fit: "45s", "5m 12s", "3h 20m", "2d 4h". */
export function formatDuration(seconds: number | null): string {
  if (seconds === null || !Number.isFinite(seconds) || seconds < 0) return "–";
  const s = Math.round(seconds);
  if (s < 60) return `${s}s`;
  const units: [number, string][] = [
    [86400, "d"],
    [3600, "h"],
    [60, "m"],
    [1, "s"],
  ];
  const i = units.findIndex(([size]) => s >= size);
  const [big, bigLabel] = units[i];
  const [small, smallLabel] = units[i + 1];
  const whole = Math.floor(s / big);
  const rest = Math.floor((s % big) / small);
  return rest > 0 ? `${whole}${bigLabel} ${rest}${smallLabel}` : `${whole}${bigLabel}`;
}

const EVENT_LABELS: Record<OnboardingEventName, string> = {
  onboarding_viewed: "Saw the first-run screen",
  tab_selected: "Switched SDK tab",
  token_minted: "Created a bootstrap token",
  token_exchanged: "Registered an agent with the token",
  first_agent_registered: "Registered a first agent",
  onboarding_completed: "Completed onboarding",
  onboarding_skipped: "Skipped onboarding",
};

type Range = "recent" | "allTime";

function StatTile({ label, value, detail }: { label: string; value: string; detail?: string }) {
  return (
    <div className="glass p-5">
      <p className="text-xs font-semibold text-ink-secondary">{label}</p>
      <p className="text-kpi mt-2">{value}</p>
      {detail ? <p className="mt-1 text-xs font-semibold text-ink-secondary">{detail}</p> : null}
    </div>
  );
}

function Distribution({ stats }: { stats: TimeToFirstAgentStats }) {
  const max = Math.max(1, ...stats.buckets.map((b) => b.count));
  return (
    <ul className="flex flex-col gap-2" aria-label="Organizations by time to first agent">
      {stats.buckets.map((b) => (
        <li key={b.label} className="grid grid-cols-[7.5rem_1fr_2.5rem] items-center gap-3 text-xs sm:grid-cols-[9rem_1fr_3rem]">
          <span className="font-semibold text-ink-secondary">{b.label}</span>
          <span className="h-3 rounded-r bg-glass-inset-gray" aria-hidden="true">
            <span
              className="block h-3 rounded-r bg-brand"
              style={{ width: b.count > 0 ? `max(${(b.count / max) * 100}%, 4px)` : 0 }}
              title={`${b.count} ${b.count === 1 ? "organization" : "organizations"}, ${b.label}`}
            />
          </span>
          <span className="text-right font-bold tabular-nums text-ink">{b.count.toLocaleString()}</span>
        </li>
      ))}
    </ul>
  );
}

/**
 * Time from an organization's creation to its first agent, over every organization and
 * over organizations created in the recent window, plus the onboarding events of that
 * window. Platform admins only; the backend enforces it, this panel says so on a 403.
 */
export function OnboardingBaselinePanel() {
  const [data, setData] = useState<OnboardingBaseline | null>(null);
  const [error, setError] = useState<{ forbidden: boolean; message: string } | null>(null);
  const [loading, setLoading] = useState(true);
  const [range, setRange] = useState<Range>("recent");

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      setData(await api.getOnboardingMetrics());
    } catch (e) {
      const err = e as ApiRequestError;
      setError({ forbidden: err.status === 403, message: err.message || "The onboarding metrics could not load." });
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    load();
  }, [load]);

  if (loading) {
    return (
      <section className="flex flex-col gap-4" aria-busy="true" aria-label="Loading onboarding metrics">
        <Skeleton className="h-6 w-56" />
        <div className="grid grid-cols-2 gap-3 lg:grid-cols-4">
          {[0, 1, 2, 3].map((i) => (
            <Skeleton key={i} className="h-28" />
          ))}
        </div>
      </section>
    );
  }

  if (error || !data) {
    return (
      <section className="glass p-6" aria-labelledby="onboarding-baseline-title">
        <h2 id="onboarding-baseline-title" className="text-headline">
          Time to first agent
        </h2>
        <p className="mt-2 text-sm text-ink-secondary">
          {error?.forbidden
            ? "These metrics cover every organization, so they are limited to platform admins: an organization admin whose email is listed in AIM_PLATFORM_ADMINS."
            : (error?.message ?? "No data was returned.")}
        </p>
        {!error?.forbidden && (
          <button
            type="button"
            onClick={load}
            className="mt-4 inline-flex h-10 items-center rounded-pill bg-brand px-5 text-sm font-bold text-white shadow-glow hover:bg-brand-hover"
          >
            Try again
          </button>
        )}
      </section>
    );
  }

  const stats = range === "recent" ? data.recent : data.allTime;
  const scope = range === "recent" ? `organizations created in the last ${data.windowDays} days` : "every organization";

  return (
    <section className="flex flex-col gap-4" aria-labelledby="onboarding-baseline-title">
      <div className="flex flex-wrap items-end justify-between gap-3 px-1">
        <div>
          <p className="text-overline">Onboarding</p>
          <h2 id="onboarding-baseline-title" className="text-headline mt-1">
            Time to first agent
          </h2>
          <p className="mt-1 max-w-2xl text-xs leading-relaxed text-ink-secondary">
            From an organization&apos;s creation to its earliest agent, over {scope}. Computed from the organization and agent creation
            times, so organizations from before onboarding events were recorded are included.
          </p>
        </div>
        <div className="flex gap-1.5" role="group" aria-label="Organizations measured">
          {(
            [
              ["recent", `Last ${data.windowDays} days`],
              ["allTime", "All time"],
            ] as [Range, string][]
          ).map(([key, label]) => (
            <button
              key={key}
              type="button"
              aria-pressed={range === key}
              onClick={() => setRange(key)}
              className={cn(
                "rounded-pill px-3 py-1 text-2xs font-bold transition-colors",
                range === key ? "bg-brand text-white shadow-glow" : "bg-glass-inset-gray text-ink-secondary hover:text-ink"
              )}
            >
              {label}
            </button>
          ))}
        </div>
      </div>

      <div className="grid grid-cols-2 gap-3 lg:grid-cols-4 lg:gap-4">
        <StatTile label="Median" value={formatDuration(stats.medianSeconds)} detail={stats.fastestSeconds !== null ? `fastest ${formatDuration(stats.fastestSeconds)}` : "no first agent yet"} />
        <StatTile label="75th percentile" value={formatDuration(stats.p75Seconds)} />
        <StatTile label="90th percentile" value={formatDuration(stats.p90Seconds)} />
        <StatTile
          label="Organizations with an agent"
          value={`${stats.organizationsWithAgent.toLocaleString()} of ${stats.organizations.toLocaleString()}`}
          detail={stats.organizations > 0 ? `${Math.round(stats.conversionRate * 1000) / 10}%` : "no organizations"}
        />
      </div>

      <div className="grid gap-4 lg:grid-cols-[1.4fr_1fr] [&>*]:min-w-0">
        <div className="glass p-5">
          <h3 className="text-[15px] font-bold tracking-[-0.02em] text-ink">Distribution</h3>
          <p className="mb-4 mt-1 text-xs text-ink-secondary">Organizations by how long their first agent took.</p>
          <Distribution stats={stats} />
          {stats.excludedNegative > 0 && (
            <p className="mt-3 text-xs text-ink-secondary">
              {stats.excludedNegative} {stats.excludedNegative === 1 ? "organization has" : "organizations have"} an agent older than the
              organization (seeded or migrated data) and {stats.excludedNegative === 1 ? "is" : "are"} left out of the timings.
            </p>
          )}
        </div>

        <div className="glass p-5">
          <h3 className="text-[15px] font-bold tracking-[-0.02em] text-ink">Onboarding events</h3>
          <p className="mb-3 mt-1 text-xs text-ink-secondary">Last {data.windowDays} days, every organization.</p>
          <table className="w-full text-xs">
            <thead>
              <tr className="text-left text-ink-secondary">
                <th scope="col" className="pb-2 font-semibold">
                  Step
                </th>
                <th scope="col" className="pb-2 pl-3 text-right font-semibold">
                  Organizations
                </th>
                <th scope="col" className="pb-2 pl-3 text-right font-semibold">
                  Events
                </th>
              </tr>
            </thead>
            <tbody>
              {data.events.map((e) => (
                <tr key={e.event} className="border-t border-stroke">
                  <th scope="row" className="py-2 text-left font-semibold text-ink">
                    {EVENT_LABELS[e.event] ?? e.event}
                  </th>
                  <td className="py-2 pl-3 text-right tabular-nums text-ink">{e.organizations.toLocaleString()}</td>
                  <td className="py-2 pl-3 text-right tabular-nums text-ink-secondary">{e.total.toLocaleString()}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>
    </section>
  );
}
