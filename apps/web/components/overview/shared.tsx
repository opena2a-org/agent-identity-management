"use client";

import Link from "next/link";
import { formatDistanceToNowStrict } from "date-fns";
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

export function trustDisplay(score: number | null | undefined) {
  if (typeof score !== "number" || Number.isNaN(score)) return null;
  return (score <= 1 ? score : score / 100).toFixed(2);
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
      <p className={cn("font-semibold", size === "sm" ? "text-2xs" : "text-xs", tone === "alert" ? "text-danger-text" : "text-ink-secondary")}>{label}</p>
      <p
        className={cn(
          "font-bold tracking-[-0.03em] leading-none",
          size === "sm" ? "mt-1.5 text-[28px]" : "mt-2 text-[32px]",
          tone === "accent" && "text-brand-text",
          tone === "alert" && "text-danger-text",
          tone === "success" && "text-success-text",
          tone === "default" && "text-ink"
        )}
      >
        {value}
      </p>
      {delta ? <p className={cn("mt-1 font-semibold text-ink-secondary", size === "sm" ? "text-2xs" : "text-xs")}>{delta}</p> : null}
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
        "inline-flex items-center gap-1.5 rounded-pill border px-2.5 py-0.5 text-2xs font-bold",
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
        <h3 className="text-[14px] font-bold tracking-[-0.02em] text-ink">{children}</h3>
        {sub ? <p className="mt-0.5 text-xs text-ink-tertiary">{sub}</p> : null}
      </div>
      {action}
    </div>
  );
}

export function EmptyNote({ children }: { children: React.ReactNode }) {
  return <p className="mt-3 text-xs leading-relaxed text-ink-secondary">{children}</p>;
}

/**
 * Recovery framing for the posture score. HELD by CCO (2026-08-24): a bare arithmetic delta
 * names no action and leaks the hidden grade boundaries, so nothing renders it today. It
 * returns once /security/metrics exposes the score components so the copy can name the lever
 * ("+9 available by attesting 2 MCP servers"). Do not re-wire it to the next multiple of ten.
 */
export function postureDelta(_score: number) {
  return "";
}
