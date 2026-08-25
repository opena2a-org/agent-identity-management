"use client";

import { useEffect } from "react";
import Link from "next/link";

export default function GlobalError({
  error,
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  useEffect(() => {
    // Surface the failure in the console for support; nothing sensitive is logged here.
    console.error("Unhandled error", error.digest ?? error.message);
  }, [error]);

  return (
    <main className="glass-page relative flex min-h-screen items-center justify-center overflow-hidden p-6">
      <div className="glass-chrome w-full max-w-md p-8 text-center">
        <p className="text-overline">Something went wrong</p>
        <h1 className="text-headline mt-2">This page could not load.</h1>
        <p className="mt-2 text-sm text-ink-secondary">
          Nothing was changed. Try again, or go back to the dashboard.
          {error.digest ? (
            <>
              {" "}
              Reference: <span className="font-mono text-xs text-ink">{error.digest}</span>
            </>
          ) : null}
        </p>
        <div className="mt-6 flex flex-wrap items-center justify-center gap-3">
          <button
            type="button"
            onClick={reset}
            className="inline-flex h-10 items-center rounded-pill bg-brand px-5 text-sm font-bold text-white shadow-accent hover:bg-brand-hover"
          >
            Try again
          </button>
          <Link
            href="/dashboard"
            className="inline-flex h-10 items-center rounded-pill border border-stroke bg-glass px-5 text-sm font-bold text-ink"
          >
            Go to the dashboard
          </Link>
        </div>
      </div>
    </main>
  );
}
