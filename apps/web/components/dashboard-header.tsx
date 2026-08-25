"use client";

import { useState, useEffect, useCallback } from "react";
import { decodeJwtPayload } from "@/lib/jwt-payload";
import { useRouter } from "next/navigation";
import Link from "next/link";
import { Bell, ChevronDown, Loader2, Lock, LogOut } from "lucide-react";
import { api } from "@/lib/api";
import { getRoleInfo, type UserRole } from "@/lib/permissions";
import { eventEmitter } from "@/lib/event-emitter";
import { AimLogo } from "@/components/sidebar";
import { cn } from "@/lib/utils";

export function DashboardHeader() {
  const router = useRouter();
  const [isDropdownOpen, setIsDropdownOpen] = useState(false);
  const [isLoggingOut, setIsLoggingOut] = useState(false);
  const [user, setUser] = useState<{
    email: string;
    displayName?: string;
    role?: UserRole;
    provider?: string;
  } | null>(null);
  const [alertCounts, setAlertCounts] = useState({
    critical: 0,
    high: 0,
    unacknowledged: 0,
  });

  // Fetch high-priority alert counts for notification badge
  const fetchAlertCounts = useCallback(async () => {
    try {
      const data = await api.getAlerts(1, 0); // Just need the counts
      setAlertCounts({
        critical: data.criticalCount || 0,
        high: data.highCount || 0,
        unacknowledged: data.unacknowledgedCount || 0,
      });
    } catch (error) {
      // Silently fail - user may not have permission to view alerts
    }
  }, []);

  // Calculate the priority alert count (critical + high unacknowledged)
  const priorityAlertCount = alertCounts.critical + alertCounts.high;

  useEffect(() => {
    const fetchUser = async () => {
      try {
        const userData = await api.getCurrentUser();
        const normalizedRole: UserRole | undefined =
          userData?.role === "pending"
            ? "viewer"
            : (userData?.role as UserRole);
        setUser({
          email: userData?.email || "",
          displayName:
            (userData as any)?.name || userData?.email?.split("@")[0] || "User",
          role: normalizedRole,
          provider: (userData as any)?.provider || undefined,
        });
      } catch (error) {
        console.log("API call failed, using token fallback");

        const token = api.getToken();
        if (token) {
          try {
            const payload = decodeJwtPayload(token);
            if (!payload) throw new Error("token payload is not decodable");

            const now = Math.floor(Date.now() / 1000);
            if (payload?.exp && payload.exp < now) {
              api.clearToken();
              setTimeout(() => router.push("/auth/login"), 0);
              return;
            }

            setUser({
              email: payload?.email || "",
              displayName: payload?.email?.split("@")[0] || "User",
              role: (payload?.role as UserRole) || "viewer",
            });
          } catch (e) {
            console.log("Token invalid, redirecting to login");
            api.clearToken();
            setTimeout(() => router.push("/auth/login"), 0);
          }
        } else {
          setTimeout(() => router.push("/auth/login"), 0);
        }
      }
    };

    fetchUser();
  }, [router]);

  // Fetch alert counts on mount and listen for updates
  useEffect(() => {
    fetchAlertCounts();

    // Refresh alert counts every 30 seconds
    const interval = setInterval(fetchAlertCounts, 30000);

    // Listen for alert-related events to refresh counts
    const handleAlertEvent = () => {
      fetchAlertCounts();
    };

    eventEmitter.on("ALERT_ACKNOWLEDGED", handleAlertEvent);
    eventEmitter.on("ALERT_RESOLVED", handleAlertEvent);
    eventEmitter.on("ALERT_CREATED", handleAlertEvent);

    return () => {
      clearInterval(interval);
      eventEmitter.off("ALERT_ACKNOWLEDGED", handleAlertEvent);
      eventEmitter.off("ALERT_RESOLVED", handleAlertEvent);
      eventEmitter.off("ALERT_CREATED", handleAlertEvent);
    };
  }, [fetchAlertCounts]);

  const handleLogout = async () => {
    setIsLoggingOut(true);
    try {
      await api.logout();
      router.push("/auth/login");
    } catch (error) {
      console.error("Logout failed:", error);
      api.clearToken();
      router.push("/auth/login");
    } finally {
      // Keep loading state until redirect completes
      // setIsLoggingOut(false); - Don't set to false, let redirect happen
    }
  };

  const roleLabel = user?.role ? getRoleInfo(user.role).label : null;
  // Interim admin-only: the edge gate blocks managers from /dashboard/admin/alerts (prefix
  // intersection); restore manager access when alerts move out of the admin prefix.
  const canSeeAlerts = user?.role === "admin";
  const initial = (user?.displayName || user?.email || "?").slice(0, 1).toUpperCase();

  return (
    <header className="flex items-center justify-between gap-3 px-1 py-1">
      {/* The sidebar carries the logo on large screens; show it here on small ones. */}
      <Link href="/dashboard" className="flex items-center gap-2.5 lg:invisible" aria-label="Overview">
        <AimLogo />
        <span className="text-base font-bold tracking-[-0.02em] text-ink">AIM</span>
      </Link>

      <div className="flex items-center gap-2">
        {canSeeAlerts && (
          <Link
            href="/dashboard/admin/alerts"
            className="relative inline-flex h-9 w-9 items-center justify-center rounded-pill border border-glass-border bg-glass text-ink-secondary backdrop-blur-card hover:text-ink"
            aria-label={priorityAlertCount > 0 ? `${priorityAlertCount} priority alerts` : "Alerts"}
          >
            <Bell className="h-4 w-4" aria-hidden="true" />
            {priorityAlertCount > 0 && (
              <span className="absolute -right-1 -top-1 inline-flex min-w-[18px] items-center justify-center rounded-pill bg-danger px-1 text-[10px] font-bold leading-[18px] text-white">
                {priorityAlertCount > 99 ? "99+" : priorityAlertCount}
              </span>
            )}
          </Link>
        )}

        <div className="relative">
          <button
            type="button"
            onClick={() => setIsDropdownOpen(!isDropdownOpen)}
            aria-haspopup="menu"
            aria-expanded={isDropdownOpen}
            className="flex items-center gap-2 rounded-pill border border-glass-border bg-glass py-1 pl-1 pr-2.5 backdrop-blur-card hover:bg-glass-inset"
          >
            <span className="inline-flex h-7 w-7 items-center justify-center rounded-full bg-gradient-to-br from-[#a78bfa] to-[#6366f1] text-xs font-bold text-white">
              {initial}
            </span>
            <span className="hidden max-w-[160px] truncate text-xs font-bold text-ink sm:block">
              {user?.displayName || "Account"}
            </span>
            <ChevronDown
              className={cn("h-3.5 w-3.5 text-ink-tertiary transition-transform", isDropdownOpen && "rotate-180")}
              aria-hidden="true"
            />
          </button>

          {isDropdownOpen && (
            <>
              <div className="fixed inset-0 z-40" onClick={() => setIsDropdownOpen(false)} aria-hidden="true" />
              <div role="menu" className="glass absolute right-0 z-50 mt-2 w-64 p-1.5">
                <div className="px-3 py-2.5">
                  <p className="truncate text-sm font-bold text-ink">{user?.displayName || "Account"}</p>
                  <p className="truncate text-2xs text-ink-tertiary">{user?.email || "Loading..."}</p>
                  {roleLabel && (
                    <span className="mt-1.5 inline-flex rounded-pill bg-brand-soft px-2 py-0.5 text-2xs font-bold text-brand-text">
                      {roleLabel}
                    </span>
                  )}
                </div>
                <div className="my-1 border-t border-divider" />
                {user?.provider === "local" && (
                  <button
                    type="button"
                    role="menuitem"
                    onClick={() => {
                      setIsDropdownOpen(false);
                      router.push("/auth/change-password");
                    }}
                    className="flex w-full items-center gap-2.5 rounded-nav px-3 py-2 text-sm font-medium text-ink-body hover:bg-glass-inset-gray hover:text-ink"
                  >
                    <Lock className="h-4 w-4 text-ink-tertiary" aria-hidden="true" />
                    <span>Change password</span>
                  </button>
                )}
                <button
                  type="button"
                  role="menuitem"
                  onClick={handleLogout}
                  disabled={isLoggingOut}
                  className="flex w-full items-center gap-2.5 rounded-nav px-3 py-2 text-sm font-medium text-danger-text hover:bg-danger-fill disabled:opacity-60"
                >
                  {isLoggingOut ? <Loader2 className="h-4 w-4 animate-spin" aria-hidden="true" /> : <LogOut className="h-4 w-4" aria-hidden="true" />}
                  <span>{isLoggingOut ? "Signing out..." : "Sign out"}</span>
                </button>
              </div>
            </>
          )}
        </div>
      </div>
    </header>
  );
}
