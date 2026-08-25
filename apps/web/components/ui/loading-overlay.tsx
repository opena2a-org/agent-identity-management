"use client";

import { cn } from "@/lib/utils";

interface LoadingOverlayProps {
  show: boolean;
  label?: string;
  className?: string;
}

export function LoadingOverlay({
  show,
  label = "Processing...",
  className,
}: LoadingOverlayProps) {
  if (!show) return null;

  return (
    <div
      role="status"
      aria-live="polite"
      className={cn(
        "absolute inset-0 z-50 flex items-center justify-center bg-glass-chrome backdrop-blur-sm",
        className
      )}
    >
      <div className="glass flex flex-col items-center gap-3 px-6 py-4 text-center">
        <span className="relative flex h-10 w-10 items-center justify-center">
          <span className="absolute inset-0 rounded-full border-[3px] border-track" />
          <span className="absolute inset-0 animate-spin rounded-full border-[3px] border-transparent border-t-brand" />
        </span>
        {label && (
          <p className="text-sm font-semibold text-ink">{label}</p>
        )}
      </div>
    </div>
  );
}
