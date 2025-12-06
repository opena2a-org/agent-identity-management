"use client";

import { useState, useEffect, useCallback } from "react";
import { useRouter } from "next/navigation";
import Link from "next/link";
import { ChevronDown, LogOut, Lock, Loader2, Bell } from "lucide-react";
import { api } from "@/lib/api";
import { type UserRole } from "@/lib/permissions";
import { eventEmitter } from "@/lib/event-emitter";

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
            const payload = JSON.parse(atob(token.split(".")[1]));

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

  const getRoleBadge = (role?: UserRole) => {
    if (!role) return null;

    const roleColors = {
      admin:
        "bg-purple-100 dark:bg-purple-900/30 text-purple-700 dark:text-purple-300",
      manager:
        "bg-blue-100 dark:bg-blue-900/30 text-blue-700 dark:text-blue-300",
      member:
        "bg-green-100 dark:bg-green-900/30 text-green-700 dark:text-green-300",
      viewer: "bg-gray-100 dark:bg-gray-800 text-gray-700 dark:text-gray-300",
    };

    const roleLabels = {
      admin: "System Administrator",
      manager: "Manager",
      member: "Member",
      viewer: "Viewer",
    };

    return (
      <span
        className={`inline-flex items-center px-2 py-0.5 rounded text-xs font-medium ${roleColors[role]}`}
      >
        {roleLabels[role]}
      </span>
    );
  };

  return (
    <header className="sticky top-0 z-30 bg-white dark:bg-gray-900 border-b border-gray-200 dark:border-gray-800 shadow-sm">
      <div className="flex items-center justify-end h-16 px-4 sm:px-6 lg:px-8">
        {/* Security Alert Bell */}
        {priorityAlertCount > 0 && (
          <Link
            href="/dashboard/security"
            className="relative p-2 mr-3 rounded-lg hover:bg-gray-100 dark:hover:bg-gray-800 transition-colors"
            title={`${priorityAlertCount} high-priority alert${priorityAlertCount > 1 ? 's' : ''}`}
          >
            <Bell className="h-5 w-5 text-red-500 dark:text-red-400" />
            <span className="absolute -top-1 -right-1 flex items-center justify-center min-w-[18px] h-[18px] px-1 text-xs font-bold text-white bg-red-500 rounded-full">
              {priorityAlertCount > 99 ? '99+' : priorityAlertCount}
            </span>
          </Link>
        )}

        {/* User Profile Dropdown */}
        <div className="relative">
          <button
            onClick={() => setIsDropdownOpen(!isDropdownOpen)}
            className="flex items-center gap-3 px-3 py-2 rounded-lg hover:bg-gray-100 dark:hover:bg-gray-800 transition-colors"
          >
            {/* Avatar */}
            <div className="w-9 h-9 bg-gradient-to-br from-purple-500 to-pink-500 rounded-full flex items-center justify-center text-white font-semibold">
              {user?.email?.[0]?.toUpperCase() || "U"}
            </div>

            {/* User Info */}
            <div className="flex flex-col items-start min-w-0">
              <div className="flex items-center gap-2">
                {getRoleBadge(user?.role)}
              </div>
              <p className="text-sm text-gray-600 dark:text-gray-400 truncate max-w-[200px]">
                {user?.email || "Loading..."}
              </p>
            </div>

            {/* Dropdown Icon */}
            <ChevronDown
              className={`h-4 w-4 text-gray-500 transition-transform ${isDropdownOpen ? "rotate-180" : ""}`}
            />
          </button>

          {/* Dropdown Menu */}
          {isDropdownOpen && (
            <>
              {/* Backdrop */}
              <div
                className="fixed inset-0 z-40"
                onClick={() => setIsDropdownOpen(false)}
              />

              {/* Menu */}
              <div className="absolute right-0 mt-2 w-64 bg-white dark:bg-gray-800 rounded-lg shadow-lg border border-gray-200 dark:border-gray-700 z-50">
                {/* User Info Header */}
                <div className="px-4 py-3 border-b border-gray-200 dark:border-gray-700">
                  <p className="text-sm font-medium text-gray-900 dark:text-white">
                    {user?.displayName || "User Account"}
                  </p>
                  <p className="text-xs text-gray-500 dark:text-gray-400 truncate">
                    {user?.email || "Loading..."}
                  </p>
                </div>

                {/* Menu Items */}
                <div className="py-2">
                  {user?.provider === "local" && (
                    <button
                      onClick={() => {
                        setIsDropdownOpen(false);
                        router.push("/auth/change-password");
                      }}
                      className="w-full flex items-center gap-3 px-4 py-2 text-sm text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 transition-colors"
                    >
                      <Lock className="h-4 w-4" />
                      <span>Change Password</span>
                    </button>
                  )}

                  <button
                    onClick={handleLogout}
                    disabled={isLoggingOut}
                    className="w-full flex items-center gap-3 px-4 py-2 text-sm text-red-600 dark:text-red-400 hover:bg-red-50 dark:hover:bg-red-900/20 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
                  >
                    {isLoggingOut ? (
                      <Loader2 className="h-4 w-4 animate-spin" />
                    ) : (
                      <LogOut className="h-4 w-4" />
                    )}
                    <span>{isLoggingOut ? "Logging out..." : "Logout"}</span>
                  </button>
                </div>
              </div>
            </>
          )}
        </div>
      </div>
    </header>
  );
}
