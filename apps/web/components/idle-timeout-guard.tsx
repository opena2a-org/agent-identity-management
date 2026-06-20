"use client";

import { useIdleTimeout } from "@/hooks/use-idle-timeout";

/**
 * Mounts the idle/absolute session timeout and renders a "Stay signed in?"
 * warning modal during the final 2 minutes before auto-logout.
 * Render once inside the authenticated dashboard layout.
 */
export function IdleTimeoutGuard() {
  const { showWarning, secondsLeft, stayActive } = useIdleTimeout();

  if (!showWarning) return null;

  return (
    <div
      className="fixed inset-0 z-[100] flex items-center justify-center bg-black/50 p-4"
      role="alertdialog"
      aria-modal="true"
      aria-labelledby="idle-timeout-title"
      aria-describedby="idle-timeout-desc"
    >
      <div className="w-full max-w-sm rounded-lg bg-white dark:bg-gray-800 shadow-xl border border-gray-200 dark:border-gray-700 p-6">
        <h2
          id="idle-timeout-title"
          className="text-lg font-semibold text-gray-900 dark:text-white"
        >
          Still there?
        </h2>
        <p
          id="idle-timeout-desc"
          className="mt-2 text-sm text-gray-600 dark:text-gray-400"
        >
          You&apos;ll be signed out in{" "}
          <span className="font-semibold text-gray-900 dark:text-white tabular-nums">
            {secondsLeft}s
          </span>{" "}
          due to inactivity, to keep your account secure.
        </p>
        <div className="mt-5 flex justify-end">
          <button
            type="button"
            onClick={stayActive}
            className="inline-flex items-center px-4 py-2 rounded-lg bg-teal-600 hover:bg-teal-700 text-white text-sm font-medium transition-colors focus:outline-none focus:ring-2 focus:ring-teal-500 focus:ring-offset-2 dark:focus:ring-offset-gray-800"
            autoFocus
          >
            Stay signed in
          </button>
        </div>
      </div>
    </div>
  );
}
