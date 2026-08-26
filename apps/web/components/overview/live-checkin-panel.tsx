"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { ArrowRight, Radar } from "lucide-react";
import { api, type Agent } from "@/lib/api";
import { cn } from "@/lib/utils";

/** Poll cadence, matching the cloud onboarding page's first-check-in listener. */
export const CHECKIN_POLL_MS = 3000;
const LONG_WAIT_MS = 120000;

/**
 * Live wait panel for the empty overview: polls the agents list until the first agent
 * checks in, then shows its identity. Rendered only when the overview loaded with zero
 * agents, so any agent the poll returns is the first arrival. The parent owns the
 * arrival (and passes a stable `onArrived`) so the panel survives the overview refetch
 * that follows it.
 */
export function LiveCheckinPanel({ arrived, onArrived }: { arrived: Agent | null; onArrived: (agent: Agent) => void }) {
  const [pollError, setPollError] = useState(false);
  const [polls, setPolls] = useState(0);

  useEffect(() => {
    if (arrived) return;
    let cancelled = false;
    const tick = async () => {
      try {
        const { agents } = await api.listAgents();
        if (cancelled) return;
        setPollError(false);
        if (agents.length > 0) {
          const newest = [...agents].sort((a, b) => (b.createdAt ?? "").localeCompare(a.createdAt ?? ""))[0];
          onArrived(newest);
        } else {
          setPolls((n) => n + 1);
        }
      } catch {
        if (!cancelled) setPollError(true);
      }
    };
    tick();
    const timer = setInterval(tick, CHECKIN_POLL_MS);
    return () => {
      cancelled = true;
      clearInterval(timer);
    };
  }, [arrived, onArrived]);

  const waitedLong = !arrived && polls * CHECKIN_POLL_MS >= LONG_WAIT_MS;
  const trust =
    arrived && typeof arrived.trustScore === "number" ? Math.round(arrived.trustScore <= 1 ? arrived.trustScore * 100 : arrived.trustScore) : null;

  return (
    <div
      className={cn(
        "flex flex-col gap-3 rounded-[20px] border p-5 shadow-panel backdrop-blur-[20px]",
        arrived ? "border-success-border bg-success-fill" : "border-glass-border bg-glass"
      )}
      role="status"
      aria-live="polite"
    >
      {arrived ? (
        <div className="flex flex-col gap-3 sm:flex-row sm:items-center">
          <span className="inline-flex h-10 w-10 flex-shrink-0 items-center justify-center rounded-avatar bg-gradient-to-br from-[#e0f2fe] to-[#bae6fd] text-sm font-bold text-[#0369a1] dark:from-[#0c4a6e] dark:to-[#075985] dark:text-[#7dd3fc]">
            {(arrived.displayName || arrived.name || "?").slice(0, 1).toLowerCase()}
          </span>
          <div className="min-w-0 flex-1">
            <p className="text-[13.5px] font-bold text-ink">
              {arrived.status === "verified" ? "Your agent checked in and is verified." : "Your agent checked in."}
            </p>
            <p className="truncate text-xs text-ink-secondary">
              <span className="font-bold">{arrived.displayName || arrived.name}</span>
              <span className="capitalize"> · {arrived.status}</span>
              {trust !== null && <> · trust score {trust}</>}
            </p>
          </div>
          <Link
            href={`/dashboard/agents/${arrived.id}`}
            className="inline-flex h-9 flex-shrink-0 items-center gap-1.5 self-start rounded-pill bg-brand px-4 text-xs font-bold text-white shadow-glow hover:bg-brand-hover sm:self-auto"
          >
            Open the agent <ArrowRight className="h-3.5 w-3.5" aria-hidden="true" />
          </Link>
        </div>
      ) : (
        <>
          <div className="flex items-center gap-4">
            {/* The listening radar: rings emanate while the panel waits. */}
            <span className="relative inline-flex h-14 w-14 flex-shrink-0 items-center justify-center" aria-hidden="true">
              <span className="absolute inset-0 rounded-full border-2 border-brand animate-radar-ring motion-reduce:animate-none" />
              <span className="absolute inset-0 rounded-full border-2 border-brand animate-radar-ring motion-reduce:animate-none [animation-delay:0.8s]" />
              <span className="absolute inset-0 rounded-full border-2 border-brand animate-radar-ring motion-reduce:animate-none [animation-delay:1.6s]" />
              <span className="relative inline-flex h-9 w-9 items-center justify-center rounded-full bg-brand-soft text-brand-text">
                <Radar className="h-5 w-5" />
              </span>
            </span>
            <div className="min-w-0 flex-1">
              <p className="text-[13.5px] font-bold tracking-[-0.02em] text-ink">
                {pollError ? "Cannot reach the API right now; retrying." : "Listening for the first check-in"}
              </p>
              <p className="mt-0.5 text-xs leading-relaxed text-ink-secondary">
                Run the install commands below — the moment your agent calls in, it appears here with its identity and trust score, and this page fills in on its own.
              </p>
            </div>
            <p className="hidden flex-shrink-0 text-2xs text-ink-tertiary sm:block">Checks every {CHECKIN_POLL_MS / 1000} seconds.</p>
          </div>
          {waitedLong && (
            <p className="rounded-inset-sm bg-glass-inset-gray p-2.5 text-2xs leading-relaxed text-ink-secondary">
              Nothing has checked in yet. This panel keeps listening; you can also{" "}
              <Link href="/dashboard/agents?register=1" className="font-semibold text-brand-text hover:underline">
                secure it in the browser
              </Link>{" "}
              instead.
            </p>
          )}
        </>
      )}
    </div>
  );
}
