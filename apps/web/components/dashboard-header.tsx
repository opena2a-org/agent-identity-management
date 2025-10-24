"use client";

import { useState, useEffect } from "react";
import { useRouter } from "next/navigation";
import { ChevronDown, LogOut, Lock, User, Loader2, Building2 } from "lucide-react";
import { api, type Organization } from "@/lib/api";
import { type UserRole } from "@/lib/permissions";
import { Badge } from "@/components/ui/badge";

export function DashboardHeader() {
  const router = useRouter();
  const [isDropdownOpen, setIsDropdownOpen] = useState(false);
  const [isLoggingOut, setIsLoggingOut] = useState(false);
  const [organization, setOrganization] = useState<Organization | null>(null);
  const [user, setUser] = useState<{
    email: string;
    display_name?: string;
    role?: UserRole;
    provider?: string;
  } | null>(null);

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
          display_name:
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
              display_name: payload?.email?.split("@")[0] || "User",
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

    const fetchOrganization = async () => {
      try {
        const orgData = await api.getCurrentOrganization();
        setOrganization(orgData);
      } catch (error) {
        console.log("Failed to fetch organization:", error);
      }
    };

    fetchUser();
    fetchOrganization();
  }, [router]);

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
      {/* Organization Context Bar */}
      {organization && (
        <div className="flex items-center gap-2 px-4 sm:px-6 lg:px-8 py-2 bg-blue-50 dark:bg-blue-900/20 border-b border-blue-100 dark:border-blue-800/30">
          <Building2 className="h-4 w-4 text-blue-600 dark:text-blue-400" />
          <span className="text-sm font-medium text-blue-900 dark:text-blue-100">
            {organization.name}
          </span>
          <Badge variant="outline" className="text-xs capitalize">
            {organization.plan}
          </Badge>
          {organization.max_agents && (
            <span className="text-xs text-blue-600 dark:text-blue-400 ml-auto">
              Max Agents: {organization.max_agents}
            </span>
          )}
        </div>
      )}
      
      <div className="flex items-center justify-end h-16 px-4 sm:px-6 lg:px-8">
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
                    {user?.display_name || "User Account"}
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
