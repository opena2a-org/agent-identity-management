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
      className={cn(
        "fixed inset-0 z-[9999] flex items-center justify-center bg-white/40 dark:bg-gray-900/40 backdrop-blur-sm",
        className
      )}
    >
      <div className="flex flex-col items-center gap-3 text-center px-6 py-4 bg-white/80 dark:bg-gray-900/80 rounded-lg shadow-lg">
        <span className="relative flex h-12 w-12 items-center justify-center">
          <span className="absolute inset-0 rounded-full border-2 border-transparent">
            <span className="absolute inset-0 rounded-full border-[3px] border-blue-100 animate-pulse delay-150" />
            <span className="absolute inset-2 rounded-full border-[3px] border-blue-300 animate-pulse delay-300" />
            <span className="absolute inset-4 rounded-full border-[3px] border-blue-500 animate-pulse delay-500" />
          </span>
          <span className="absolute inset-0 rounded-full border-[3px] border-t-blue-500 border-r-transparent border-b-transparent animate-spin" />
        </span>
        {label && (
          <p className="text-sm font-medium text-gray-800 dark:text-gray-100">
            {label}
          </p>
        )}
      </div>
    </div>
  );
}

