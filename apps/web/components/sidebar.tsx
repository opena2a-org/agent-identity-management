"use client";

import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import {
  Home,
  Shield,
  AlertTriangle,
  CheckCircle,
  Server,
  Key,
  Users,
  Bell,
  LogOut,
  ChevronLeft,
  Menu,
  X,
  Activity,
  Download,
  Lock,
  ShieldCheck,
  CheckSquare,
  ClipboardCheck,
  Tag,
  BarChart3,
  Code,
  Loader2,
} from "lucide-react";
import { useState, useEffect } from "react";
import { api } from "@/lib/api";
import {
  filterNavigationByRole,
  type UserRole,
  type NavSection,
} from "@/lib/permissions";
import { eventEmitter, Events } from "@/lib/events";

// ✅ Navigation with role-based access control
// Organized by natural user workflow: Core → Development → Monitoring → Configuration → Administration
const navigationBase: NavSection[] = [
  {
    title: "Core",
    items: [
      // Everyone starts here
      {
        name: "Dashboard",
        href: "/dashboard",
        icon: Home,
        roles: ["admin", "manager", "member", "viewer"],
      },
      {
        name: "Agents",
        href: "/dashboard/agents",
        icon: Shield,
        roles: ["admin", "manager", "member", "viewer"],
      },
      {
        name: "MCP Servers",
        href: "/dashboard/mcp",
        icon: Server,
        roles: ["admin", "manager", "member"],
      },
    ],
  },
  {
    title: "Development",
    items: [
      // Developer resources and tools
      {
        name: "Developers",
        href: "/dashboard/developers",
        icon: Code,
        roles: ["admin", "manager", "member", "viewer"],
      },
      {
        name: "API Keys",
        href: "/dashboard/api-keys",
        icon: Key,
        roles: ["admin", "manager", "member"],
      },
      {
        name: "Download SDK",
        href: "/dashboard/sdk",
        icon: Download,
        roles: ["admin", "manager", "member"],
      },
      {
        name: "SDK Tokens",
        href: "/dashboard/sdk-tokens",
        icon: Lock,
        roles: ["admin", "manager", "member"],
      },
    ],
  },
  {
    title: "Monitoring",
    items: [
      // Analytics and monitoring for managers
      {
        name: "Agent Verifications",
        href: "/dashboard/monitoring",
        icon: Activity,
        roles: ["admin", "manager"],
      },
      {
        name: "Usage Statistics",
        href: "/dashboard/analytics/usage",
        icon: BarChart3,
        roles: ["admin", "manager"],
      },
      {
        name: "Security",
        href: "/dashboard/security",
        icon: AlertTriangle,
        roles: ["admin", "manager"],
      },
    ],
  },
  {
    title: "Configuration",
    items: [
      // System configuration
      {
        name: "Tags",
        href: "/dashboard/tags",
        icon: Tag,
        roles: ["admin", "manager", "member"],
      },
      // Webhooks hidden per user requirements
    ],
  },
  {
    title: "Administration",
    items: [
      // Admin-only access to user management and audit logs
      {
        name: "Users",
        href: "/dashboard/admin/users",
        icon: Users,
        roles: ["admin"],
      },
      {
        name: "Alerts",
        href: "/dashboard/admin/alerts",
        icon: Bell,
        roles: ["admin", "manager"], // Managers can view alerts
      },
      {
        name: "Verification Approvals",
        href: "/dashboard/admin/verifications",
        icon: CheckCircle,
        roles: ["admin"], // Admin-only manual approvals
      },
      {
        name: "Capability Requests",
        href: "/dashboard/admin/capability-requests",
        icon: CheckSquare,
        roles: ["admin"], // Admin-only capability approval
      },
      {
        name: "Security Policies",
        href: "/dashboard/admin/security-policies",
        icon: ShieldCheck,
        roles: ["admin"], // Admin-only policy management
      },
      {
        name: "Compliance",
        href: "/dashboard/admin/compliance",
        icon: ClipboardCheck,
        roles: ["admin"], // Admin-only compliance monitoring
      },
    ],
  },
];

export function Sidebar() {
  const pathname = usePathname();
  const router = useRouter();
  const [collapsed, setCollapsed] = useState(false);
  const [mobileOpen, setMobileOpen] = useState(false);
  const [isLoading, setIsLoading] = useState(true); // ✅ Add loading state
  const [isLoggingOut, setIsLoggingOut] = useState(false);
  const [user, setUser] = useState<{
    email: string;
    display_name?: string;
    role?: UserRole;
    provider?: string;
  } | null>(null);
  const [alertCount, setAlertCount] = useState<number>(0);
  const [capabilityRequestCount, setCapabilityRequestCount] =
    useState<number>(0);
  const [verificationCount, setVerificationCount] = useState<number>(0);
  const [navigation, setNavigation] = useState<NavSection[]>([]);

  useEffect(() => {
    // Fetch current user
    const fetchUser = async () => {
      try {
        setIsLoading(true); // ✅ Start loading
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
        // Silently handle errors - don't throw to UI
        console.log("API call failed, using token fallback");

        // Fallback: decode user info from JWT token
        const token = api.getToken();
        if (token) {
          try {
            const payload = JSON.parse(atob(token.split(".")[1]));

            // Check if token is expired
            const now = Math.floor(Date.now() / 1000);
            if (payload?.exp && payload.exp < now) {
              // Token expired - clear and redirect
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
          // No token at all - redirect to login
          setTimeout(() => router.push("/auth/login"), 0);
        }
      } finally {
        setIsLoading(false); // ✅ Stop loading
      }
    };
    fetchUser();
  }, [router]);

  // ✅ Filter navigation based on user role using permissions system
  useEffect(() => {
    if (!user?.role) return;

    const filteredNav = filterNavigationByRole(navigationBase, user.role);
    setNavigation(filteredNav);
  }, [user?.role]);

  useEffect(() => {
    // Fetch alert count, capability request count, and verification approvals
    const fetchCounts = async () => {
      try {
        // Fetch alert count (for admin and manager)
        if (user?.role && user.role !== "viewer") {
          const alertCountData = await api.getUnacknowledgedAlertCount();
          setAlertCount(alertCountData);
        }

        // Fetch capability request count (admin only)
        if (user?.role === "admin") {
          const capabilityCountData =
            await api.getPendingCapabilityRequestsCount();
          setCapabilityRequestCount(capabilityCountData);

          const pendingVerificationCount =
            await api.getPendingVerificationCount();
          setVerificationCount(pendingVerificationCount);
        }

        // Update navigation with badges
        setNavigation((prev) =>
          prev.map((section) => ({
            ...section,
            items: section.items.map((item) => {
              // Update Alerts badge
              if (item.name === "Alerts" && alertCount > 0) {
                return { ...item, badge: alertCount };
              }
              // Update Capability Requests badge
              if (
                item.name === "Capability Requests" &&
                capabilityRequestCount > 0
              ) {
                return { ...item, badge: capabilityRequestCount };
              }
              // Update Verification Approvals badge
              if (
                item.name === "Verification Approvals" &&
                verificationCount > 0
              ) {
                return { ...item, badge: verificationCount };
              }
              // Remove badges when count is 0
              if (
                (item.name === "Alerts" && alertCount === 0) ||
                (item.name === "Capability Requests" &&
                  capabilityRequestCount === 0) ||
                (item.name === "Verification Approvals" &&
                  verificationCount === 0)
              ) {
                const { badge, ...itemWithoutBadge } = item;
                return itemWithoutBadge;
              }
              return item;
            }),
          }))
        );
      } catch (error) {
        console.log("Failed to fetch counts:", error);
      }
    };

    const fetchVerificationCount = async () => {
      if (user?.role !== "admin") return;
      try {
        const count = await api.getPendingVerificationCount();
        setVerificationCount(count);
      } catch (error) {
        console.log("Failed to fetch verification count:", error);
      }
    };

    // Only fetch if user has permission
    if (user?.role && user.role !== "viewer") {
      fetchCounts();
      // Refresh counts every 30 seconds
      const interval = setInterval(fetchCounts, 30000);
      let verificationInterval: NodeJS.Timeout | undefined;
      if (user?.role === "admin") {
        fetchVerificationCount();
        // Refresh verification count every 1 minute (60 seconds)
        verificationInterval = setInterval(fetchVerificationCount, 60000);
      }

      // Listen for real-time events
      const unsubscribeAlertAck = eventEmitter.on(
        Events.ALERT_ACKNOWLEDGED,
        fetchCounts
      );
      const unsubscribeAlertResolved = eventEmitter.on(
        Events.ALERT_RESOLVED,
        fetchCounts
      );
      const unsubscribeCapabilityApproved = eventEmitter.on(
        Events.CAPABILITY_REQUEST_APPROVED,
        fetchCounts
      );
      const unsubscribeCapabilityRejected = eventEmitter.on(
        Events.CAPABILITY_REQUEST_REJECTED,
        fetchCounts
      );
      const unsubscribeVerificationApproved = eventEmitter.on(
        Events.VERIFICATION_APPROVED,
        fetchVerificationCount
      );
      const unsubscribeVerificationDenied = eventEmitter.on(
        Events.VERIFICATION_DENIED,
        fetchVerificationCount
      );

      return () => {
        clearInterval(interval);
        if (verificationInterval) {
          clearInterval(verificationInterval);
        }
        unsubscribeAlertAck();
        unsubscribeAlertResolved();
        unsubscribeCapabilityApproved();
        unsubscribeCapabilityRejected();
        unsubscribeVerificationApproved();
        unsubscribeVerificationDenied();
      };
    }
  }, [user?.role, alertCount, capabilityRequestCount, verificationCount]);

  const handleLogout = async () => {
    setIsLoggingOut(true);
    try {
      await api.logout();
      router.push("/auth/login");
    } catch (error) {
      console.error("Logout failed:", error);
      // Force logout even if API call fails
      api.clearToken();
      router.push("/auth/login");
    } finally {
      // Keep loading state until redirect completes
      // setIsLoggingOut(false); - Don't set to false, let redirect happen
    }
  };

  const isActive = (href: string) => {
    if (!pathname) return false;
    if (href === "/dashboard") {
      return pathname === "/dashboard";
    }
    // Exact match OR starts with href followed by '/' (to avoid partial matches like /dashboard/sdk matching /dashboard/sdk-tokens)
    return pathname === href || pathname.startsWith(href + "/");
  };

  // ✅ Sidebar Loading Skeleton
  const SidebarSkeleton = () => (
    <>
      {/* Logo Skeleton */}
      <div className="px-4 py-4 border-b border-gray-200 dark:border-gray-700">
        <div className="flex items-center gap-3">
          <div className="w-8 h-8 bg-gray-200 dark:bg-gray-700 rounded-lg animate-pulse" />
          {!collapsed && (
            <div className="flex flex-col gap-1 flex-1">
              <div className="h-5 w-16 bg-gray-200 dark:bg-gray-700 rounded animate-pulse" />
              <div className="h-3 w-24 bg-gray-200 dark:bg-gray-700 rounded animate-pulse" />
            </div>
          )}
        </div>
      </div>

      {/* Navigation Skeleton */}
      <nav className="flex-1 px-3 py-4 space-y-6 overflow-y-auto">
        {/* Main Section */}
        <div className="space-y-1">
          {[...Array(6)].map((_, idx) => (
            <div
              key={idx}
              className={`flex items-center gap-3 px-3 py-2 rounded-lg ${collapsed ? "justify-center" : ""}`}
            >
              <div className="w-5 h-5 bg-gray-200 dark:bg-gray-700 rounded animate-pulse" />
              {!collapsed && (
                <div className="h-4 flex-1 bg-gray-200 dark:bg-gray-700 rounded animate-pulse" />
              )}
            </div>
          ))}
        </div>

        {/* Administration Section */}
        {!collapsed && (
          <div className="space-y-1">
            <div className="h-3 w-24 mx-3 bg-gray-200 dark:bg-gray-700 rounded animate-pulse mb-2" />
            {[...Array(4)].map((_, idx) => (
              <div
                key={idx}
                className="flex items-center gap-3 px-3 py-2 rounded-lg"
              >
                <div className="w-5 h-5 bg-gray-200 dark:bg-gray-700 rounded animate-pulse" />
                <div className="h-4 flex-1 bg-gray-200 dark:bg-gray-700 rounded animate-pulse" />
              </div>
            ))}
          </div>
        )}
      </nav>
    </>
  );

  const SidebarContent = () => (
    <>
      {isLoading ? (
        <SidebarSkeleton />
      ) : (
        <>
          {/* Logo */}
          <div className="relative flex items-center justify-between px-4 py-4 border-b border-gray-200 dark:border-gray-700">
            <Link href="/dashboard" className="flex items-center gap-3">
              <div className="w-8 h-8 bg-gradient-to-br from-blue-500 to-blue-600 rounded-lg flex items-center justify-center">
                <Shield className="h-5 w-5 text-white" />
              </div>
              {!collapsed && (
                <div className="flex flex-col">
                  <span className="text-lg font-bold text-gray-900 dark:text-white">
                    AIM
                  </span>
                  <span className="text-xs text-gray-500 dark:text-gray-400">
                    Agent Identity Management
                  </span>
                </div>
              )}
            </Link>
            {collapsed && (
              <button
                onClick={() => setCollapsed(false)}
                className="absolute top-1/2 -translate-y-1/2 right-0 z-10 translate-x-1/2 rounded-full border border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-900 shadow p-1 text-gray-500 hover:text-gray-700 dark:text-gray-300 dark:hover:text-gray-100"
                aria-label="Expand sidebar"
              >
                <ChevronLeft className="h-4 w-4 rotate-180" />
              </button>
            )}
            {!collapsed && (
              <button
                onClick={() => setCollapsed(true)}
                className="lg:flex hidden p-1 rounded-full border border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-900 shadow text-gray-500 hover:text-gray-700 dark:text-gray-300 dark:hover:text-gray-100"
              >
                <ChevronLeft className="h-5 w-5" />
              </button>
            )}
          </div>

          {/* Navigation */}
          <nav className="flex-1 px-3 py-4 space-y-6 overflow-y-auto">
            {navigation.map((section, idx) => (
              <div key={idx} className="space-y-1">
                {section.title && !collapsed && (
                  <h3 className="px-3 text-xs font-semibold text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                    {section.title}
                  </h3>
                )}
                <div className="space-y-1">
                  {section.items.map((item) => {
                    const active = isActive(item.href);
                    return (
                      <Link
                        key={item.name}
                        href={item.href}
                        onClick={() => setMobileOpen(false)}
                        className={`
                          flex items-center gap-3 px-3 py-2 rounded-lg transition-all
                          ${active
                            ? "bg-blue-50 dark:bg-blue-900/20 text-blue-600 dark:text-blue-400"
                            : "text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-800"
                          }
                          ${collapsed ? "justify-center" : ""}
                        `}
                        title={collapsed ? item.name : undefined}
                      >
                        <item.icon
                          className={`h-5 w-5 flex-shrink-0 ${active ? "text-blue-600 dark:text-blue-400" : ""}`}
                        />
                        {!collapsed && (
                          <>
                            <span className="flex-1 font-medium">
                              {item.name}
                            </span>
                            {item.badge && (
                              <span className="inline-flex items-center justify-center px-2 py-0.5 text-xs font-bold text-white bg-red-500 rounded-full">
                                {item.badge}
                              </span>
                            )}
                          </>
                        )}
                      </Link>
                    );
                  })}
                </div>
              </div>
            ))}
          </nav>
        </>
      )}
    </>
  );

  return (
    <>
      {/* Mobile Menu Button */}
      <button
        onClick={() => setMobileOpen(!mobileOpen)}
        className="lg:hidden fixed top-4 right-4 z-50 p-2 bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg shadow-lg"
      >
        {mobileOpen ? <X className="h-6 w-6" /> : <Menu className="h-6 w-6" />}
      </button>

      {/* Mobile Overlay */}
      {mobileOpen && (
        <div
          className="lg:hidden fixed inset-0 bg-black/50 z-40"
          onClick={() => setMobileOpen(false)}
        />
      )}

      {/* Desktop Sidebar */}
      <aside
        className={`
          hidden lg:flex flex-col bg-white dark:bg-gray-900 border-r border-gray-200 dark:border-gray-700
          transition-all duration-300 ease-in-out sticky top-0 h-screen
          ${collapsed ? "w-20" : "w-64"}
        `}
      >
        <SidebarContent />
      </aside>

      {/* Mobile Sidebar */}
      <aside
        className={`
          lg:hidden fixed top-0 left-0 bottom-0 z-40 w-64 bg-white dark:bg-gray-900 border-r border-gray-200 dark:border-gray-700
          transform transition-transform duration-300 ease-in-out
          ${mobileOpen ? "translate-x-0" : "-translate-x-full"}
        `}
      >
        <SidebarContent />
      </aside>
    </>
  );
}
