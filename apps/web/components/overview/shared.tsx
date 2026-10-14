"use client";

import Link from "next/link";
import { formatDistanceToNowStrict } from "date-fns";
import { TrendChart, chartSeries } from "@/components/charts";
import type { VerificationActivityMonth } from "@/components/overview/types";
import { cn } from "@/lib/utils";

/** Presentation helpers shared by the overview lenses. Every number rendered through these comes from an endpoint. */

export function relativeTime(iso?: string | null) {
  if (!iso) return "";
  try {
    return formatDistanceToNowStrict(new Date(iso), { addSuffix: true });
  } catch {
    return "";
  }
}

export function shortTime(iso?: string | null) {
  if (!iso) return "";
  const ms = Date.now() - new Date(iso).getTime();
  if (Number.isNaN(ms)) return "";
  if (ms < 60_000) return "now";
  const m = Math.floor(ms / 60_000);
  if (m < 60) return `${m}m ago`;
  const h = Math.floor(m / 60);
  if (h < 24) return `${h}h ago`;
  return `${Math.floor(h / 24)}d ago`;
}

/** Trust scores display as whole numbers on the 0-100 scale, never decimals. */
export function trustDisplay(score: number | null | undefined) {
  if (typeof score !== "number" || Number.isNaN(score)) return null;
  return String(Math.round(score <= 1 ? score * 100 : score));
}

export function KpiTile({
  label,
  value,
  delta,
  tone = "default",
  href,
  size = "md",
}: {
  label: string;
  value: string;
  delta?: string;
  tone?: "default" | "accent" | "alert" | "success";
  href?: string;
  size?: "md" | "sm";
}) {
  const body = (
    <>
      <p className={cn("text-xs font-semibold", tone === "alert" ? "text-danger-text" : "text-ink-secondary")}>{label}</p>
      <p
        className={cn(
          "font-bold tracking-[-0.03em] leading-none",
          size === "sm" ? "mt-1.5 text-2xl" : "mt-2 text-3xl",
          tone === "accent" && "text-brand-text",
          tone === "alert" && "text-danger-text",
          tone === "success" && "text-success-text",
          tone === "default" && "text-ink"
        )}
      >
        {value}
      </p>
      {delta ? <p className="mt-1 text-xs font-semibold text-ink-secondary">{delta}</p> : null}
    </>
  );
  const cls = cn(tone === "alert" ? "glass-alert" : "glass", "block transition-transform", size === "sm" ? "rounded-card-sm p-4" : "p-5", href && "hover:-translate-y-0.5");
  return href ? (
    <Link href={href} className={cls}>
      {body}
    </Link>
  ) : (
    <div className={cls}>{body}</div>
  );
}

export function StatusChip({ kind, children }: { kind: "pass" | "deny" | "pending" | "neutral"; children: React.ReactNode }) {
  return (
    <span
      className={cn(
        "inline-flex items-center gap-1.5 rounded-pill border px-2.5 py-0.5 text-xs font-bold",
        kind === "pass" && "border-success-border bg-success-fill text-success-text",
        kind === "deny" && "border-danger-border bg-danger-fill text-danger-text",
        kind === "pending" && "border-warning-border bg-warning-fill text-warning-text",
        kind === "neutral" && "border-glass-inset-border bg-glass-inset-gray text-ink-body"
      )}
    >
      {children}
    </span>
  );
}

export function CardTitle({ children, sub, action }: { children: React.ReactNode; sub?: string; action?: React.ReactNode }) {
  return (
    <div className="flex items-start justify-between gap-3">
      <div>
        <h3 className="text-sm font-bold tracking-[-0.02em] text-ink">{children}</h3>
        {sub ? <p className="mt-0.5 text-xs text-ink-tertiary">{sub}</p> : null}
      </div>
      {action}
    </div>
  );
}

/**
 * Verified and pending verifications per month from /analytics/verification-activity, one stacked bar
 * per month. The title names the window the endpoint returned, not the one requested.
 */
export function VerificationsPerMonthChart({
  activity,
  axes = true,
  legend = false,
  className,
}: {
  activity: VerificationActivityMonth[];
  axes?: boolean;
  legend?: boolean;
  className?: string;
}) {
  return (
    <TrendChart
      title={`Verifications per month, last ${activity.length} ${activity.length === 1 ? "month" : "months"}`}
      data={activity.map(({ month, verified, pending }) => ({ month, verified, pending }))}
      xKey="month"
      series={[
        { key: "verified", name: "Verified", color: chartSeries.brand, kind: "bar", stack: "verifications" },
        { key: "pending", name: "Pending", color: chartSeries.muted, kind: "bar", stack: "verifications" },
      ]}
      axes={axes}
      legend={legend}
      className={className}
    />
  );
}

export function EmptyNote({ children }: { children: React.ReactNode }) {
  return <p className="mt-3 text-xs leading-relaxed text-ink-secondary">{children}</p>;
}

/**
 * Recovery framing for the posture score. Withheld for now: a bare arithmetic delta
 * names no action and leaks the hidden grade boundaries, so nothing renders it today. It
 * returns once /security/metrics exposes the score components so the copy can name the lever
 * ("+9 available by attesting 2 MCP servers"). Do not re-wire it to the next multiple of ten.
 */
export function postureDelta(_score: number) {
  return "";
}
