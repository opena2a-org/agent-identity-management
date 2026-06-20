"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { usePathname, useRouter } from "next/navigation";
import { api } from "@/lib/api";
import { toast } from "sonner";

// Session-timeout policy (CSR/CPO, 2026-06-19):
//   - 30 min of inactivity -> auto-logout, with a 2-min warning beforehand.
//   - 8 h absolute cap regardless of activity.
// These are CLIENT-side convenience boundaries; the real security boundaries
// are the server access-token exp and the refresh-token expiry. Values are
// overridable via env for testing.
const num = (v: string | undefined, fallback: number) => {
  const n = v ? Number(v) : NaN;
  return Number.isFinite(n) && n > 0 ? n : fallback;
};

const IDLE_LIMIT_MS = num(process.env.NEXT_PUBLIC_IDLE_TIMEOUT_MS, 30 * 60 * 1000);
const WARN_BEFORE_MS = num(process.env.NEXT_PUBLIC_IDLE_WARN_MS, 2 * 60 * 1000);
const ABSOLUTE_CAP_MS = num(process.env.NEXT_PUBLIC_SESSION_CAP_MS, 8 * 60 * 60 * 1000);
const TICK_MS = 5 * 1000;
const ACTIVITY_THROTTLE_MS = 5 * 1000;

const TOKEN_KEY = "auth_token";
const LAST_ACTIVITY_KEY = "last_activity";
const SESSION_START_KEY = "session_start";

const PUBLIC_ROUTE_PREFIXES = [
  "/auth/login",
  "/auth/register",
  "/auth/callback",
  "/auth/registration-pending",
  "/auth/error",
];

type IdleState = { showWarning: boolean; secondsLeft: number };

function now() {
  return Date.now();
}

function readLS(key: string): string | null {
  try {
    return localStorage.getItem(key);
  } catch {
    return null;
  }
}

/**
 * Drives idle/absolute session timeout on the dashboard shell.
 * Returns warning state so a UI component can show a "Stay signed in?" prompt.
 */
export function useIdleTimeout(): IdleState & { stayActive: () => void } {
  const router = useRouter();
  const pathname = usePathname();
  const [state, setState] = useState<IdleState>({ showWarning: false, secondsLeft: 0 });
  const lastWriteRef = useRef(0);
  const warningActiveRef = useRef(false);
  const loggingOutRef = useRef(false);

  const isPublicRoute = PUBLIC_ROUTE_PREFIXES.some((p) => pathname?.startsWith(p));

  const clearWarning = useCallback(() => {
    if (warningActiveRef.current) {
      warningActiveRef.current = false;
      setState({ showWarning: false, secondsLeft: 0 });
    }
  }, []);

  const recordActivity = useCallback(() => {
    const t = now();
    // Real activity must dismiss an active warning AND reset the idle clock
    // immediately — bypass the write throttle in that case so the user is not
    // logged out while interacting.
    if (warningActiveRef.current) {
      clearWarning();
      lastWriteRef.current = 0;
    }
    if (t - lastWriteRef.current < ACTIVITY_THROTTLE_MS) return;
    lastWriteRef.current = t;
    try {
      localStorage.setItem(LAST_ACTIVITY_KEY, String(t));
    } catch {
      /* ignore storage failures */
    }
  }, [clearWarning]);

  const stayActive = useCallback(() => {
    lastWriteRef.current = 0; // force the write through the throttle
    clearWarning();
    recordActivity();
  }, [clearWarning, recordActivity]);

  useEffect(() => {
    if (isPublicRoute) return;
    // Only run for an authenticated session.
    if (!readLS(TOKEN_KEY)) return;

    // Seed activity + absolute-session start if absent. (api.setToken also
    // resets these on login so a stale window can't carry across sessions.)
    try {
      if (!readLS(LAST_ACTIVITY_KEY)) localStorage.setItem(LAST_ACTIVITY_KEY, String(now()));
      if (!readLS(SESSION_START_KEY)) localStorage.setItem(SESSION_START_KEY, String(now()));
    } catch {
      /* ignore */
    }

    const redirectToLogin = (reason?: "idle" | "cap") => {
      if (loggingOutRef.current) return;
      loggingOutRef.current = true;
      if (reason) {
        toast.info(
          reason === "idle"
            ? "Signed out after 30 minutes of inactivity."
            : "Your 8-hour session ended. Please sign in again.",
          { duration: 6000 }
        );
      }
      router.replace(reason ? "/auth/login?reason=timeout" : "/auth/login");
    };

    const doLogout = async (reason: "idle" | "cap") => {
      if (loggingOutRef.current) return;
      try {
        await api.logout();
      } catch {
        api.clearToken();
      }
      redirectToLogin(reason);
    };

    const activityEvents = ["pointerdown", "keydown", "scroll", "touchstart"];
    activityEvents.forEach((e) =>
      window.addEventListener(e, recordActivity, { passive: true })
    );

    // Cross-tab coordination via storage events.
    const onStorage = (e: StorageEvent) => {
      // Signed out in another tab -> follow suit (the other tab cleared keys).
      if (e.key === TOKEN_KEY && e.newValue === null) {
        redirectToLogin();
        return;
      }
      // Another tab recorded activity -> clear our warning.
      if (e.key === LAST_ACTIVITY_KEY) clearWarning();
    };
    window.addEventListener("storage", onStorage);

    const interval = setInterval(() => {
      // If the token vanished (logged out in this or another tab), bail out.
      if (!readLS(TOKEN_KEY)) {
        redirectToLogin();
        return;
      }
      const t = now();
      const last = Number(readLS(LAST_ACTIVITY_KEY)) || t;
      const start = Number(readLS(SESSION_START_KEY)) || t;
      const idle = t - last;
      const sessionAge = t - start;

      if (sessionAge >= ABSOLUTE_CAP_MS) {
        void doLogout("cap");
        return;
      }
      if (idle >= IDLE_LIMIT_MS) {
        void doLogout("idle");
        return;
      }
      if (idle >= IDLE_LIMIT_MS - WARN_BEFORE_MS) {
        warningActiveRef.current = true;
        const secondsLeft = Math.max(0, Math.ceil((IDLE_LIMIT_MS - idle) / 1000));
        setState({ showWarning: true, secondsLeft });
      } else {
        clearWarning();
      }
    }, TICK_MS);

    return () => {
      clearInterval(interval);
      activityEvents.forEach((e) => window.removeEventListener(e, recordActivity));
      window.removeEventListener("storage", onStorage);
    };
  }, [isPublicRoute, recordActivity, clearWarning, router]);

  return { ...state, stayActive };
}
