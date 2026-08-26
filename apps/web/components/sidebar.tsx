"use client";

import Link from "next/link";
import { decodeJwtPayload } from "@/lib/jwt-payload";
import { usePathname, useRouter } from "next/navigation";
import { useEffect, useState } from "react";
import { LogOut, Loader2, X } from "lucide-react";
import { api } from "@/lib/api";
import {
  filterNavigationByRole,
  type NavSection,
  type UserRole,
} from "@/lib/permissions";
import { navigationBase, orderNavigationForPersona } from "@/lib/navigation";
import { usePersona } from "@/lib/persona";
import { eventEmitter, Events } from "@/lib/events";
import { PersonaSwitch } from "@/components/persona-switch";
import { cn } from "@/lib/utils";

export interface SidebarProps {
  /** Controlled mobile drawer state (the bottom tab bar's Menu opens it). */
  mobileOpen?: boolean;
  onMobileOpenChange?: (open: boolean) => void;
}

export function AimLogo({ size = 30, className }: { size?: number; className?: string }) {
  return (
    <span
      aria-hidden="true"
      className={cn("inline-flex items-center justify-center", className)}
      style={{ width: size, height: size }}
    >
      {/* The brand mark with a real alpha channel (public/aim-mark.png): the earlier
          asset's baked white shield face read as a plate in the header. */}
      {/* eslint-disable-next-line @next/next/no-img-element -- static brand asset, no optimization needed */}
      <img src="/aim-mark.png" alt="" width={size} height={size} style={{ width: "100%", height: "100%", objectFit: "contain" }} />
    </span>
  );
}

export function Sidebar({ mobileOpen: mobileOpenProp, onMobileOpenChange }: SidebarProps = {}) {
  const pathname = usePathname();
  const router = useRouter();
  const [mobileOpenState, setMobileOpenState] = useState(false);
  const mobileOpen = mobileOpenProp ?? mobileOpenState;
  const setMobileOpen = (open: boolean) => {
    setMobileOpenState(open);
    onMobileOpenChange?.(open);
  };
  const persona = usePersona((s) => s.persona);
  const [isLoading, setIsLoading] = useState(true); // Add loading state
  const [isLoggingOut, setIsLoggingOut] = useState(false);
  const [user, setUser] = useState<{
    email: string;
    display_name?: string;
    role?: UserRole;
    provider?: string;
  } | null>(null);
  const [alertCount, setAlertCount] = useState<number>(0);
  const [securityAlertCount, setSecurityAlertCount] = useState<number>(0);
  const [capabilityRequestCount, setCapabilityRequestCount] =
    useState<number>(0);
  const [verificationCount, setVerificationCount] = useState<number>(0);
  const [navigation, setNavigation] = useState<NavSection[]>([]);

  useEffect(() => {
    // Fetch current user
    const fetchUser = async () => {
      try {
        setIsLoading(true); // Start loading
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
            const payload = decodeJwtPayload(token);
            if (!payload) throw new Error("token payload is not decodable");

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
        setIsLoading(false); // Stop loading
      }
    };
    fetchUser();
  }, [router]);

  // The navigation is derived from the role filter, the lens order and the live counts.
  // Badges are matched by href so a label change cannot detach them, and a lens switch
  // rebuilds the list with the counts it already has.
  useEffect(() => {
    if (!user?.role) return;

    const badges: Record<string, number> = {
      "/dashboard/security": securityAlertCount,
      "/dashboard/admin/alerts": alertCount,
      "/dashboard/admin/capability-requests": capabilityRequestCount,
      "/dashboard/admin/jit-requests": verificationCount,
    };
    const filteredNav = filterNavigationByRole(navigationBase, user.role).map((section) => ({
      ...section,
      items: section.items.map((item) => {
        const count = badges[item.href];
        if (count && count > 0) return { ...item, badge: count };
        const { badge: _badge, ...withoutBadge } = item;
        return withoutBadge;
      }),
    }));
    // The lens only reorders what the role filter already allowed (lib/persona.test.ts).
    setNavigation(orderNavigationForPersona(filteredNav, persona));
  }, [user?.role, persona, alertCount, securityAlertCount, capabilityRequestCount, verificationCount]);

  useEffect(() => {
    // Fetch alert count, capability request count, and verification approvals
    const fetchCounts = async () => {
      try {
        // Fetch alert count (for admin and manager)
        if (user?.role && user.role !== "viewer") {
          const alertCountData = await api.getUnacknowledgedAlertCount();
          setAlertCount(alertCountData);

          // Fetch security alert count (critical + high priority)
          try {
            const alertData = await api.getAlerts(1, 0);
            const securityCount = (alertData.criticalCount || 0) + (alertData.highCount || 0);
            setSecurityAlertCount(securityCount);
          } catch {
            // Silently fail if user doesn't have permission
          }
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

        // Badges are applied by the navigation effect from these counts.
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
  }, [user?.role, alertCount, securityAlertCount, capabilityRequestCount, verificationCount]);

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
    // Exact match only for navigation items that have child pages
    // This prevents /dashboard/mcp from being highlighted when on /dashboard/mcp/discovery
    const hasChildPages = ["/dashboard/mcp", "/dashboard/admin"];
    if (hasChildPages.some(parent => href === parent)) {
      return pathname === href;
    }
    // For other items, match exact or children (for dynamic routes like /dashboard/agents/[id])
    return pathname === href || pathname.startsWith(href + "/");
  };

  // Sidebar Loading Skeleton
  const NavList = () => (
    <nav className="flex min-h-0 flex-1 flex-col gap-4 overflow-y-auto pr-0.5" aria-label="Main">
      {navigation.map((section, idx) => (
        <div key={section.title || idx} className="flex flex-col gap-[2px]">
          {section.title && (
            <h3 className="text-overline mb-0.5 pl-3">{section.title}</h3>
          )}
          {section.items.map((item) => {
            const active = isActive(item.href);
            return (
              <Link
                key={item.href}
                href={item.href}
                onClick={() => setMobileOpen(false)}
                aria-current={active ? "page" : undefined}
                className={cn(
                  "flex items-center gap-2.5 rounded-nav px-[11px] py-[7px] text-[13px] transition-colors",
                  active
                    ? "bg-nav-active font-bold text-brand-text"
                    : "font-medium text-ink-body hover:bg-glass-inset-gray hover:text-ink"
                )}
              >
                <item.icon
                  className={cn("h-4 w-4 flex-shrink-0", active ? "text-brand-text" : "text-ink-tertiary")}
                  strokeWidth={2}
                  aria-hidden="true"
                />
                <span className="flex-1 truncate">{item.name}</span>
                {item.badge ? (
                  <span
                    className="inline-flex min-w-[20px] items-center justify-center rounded-pill bg-danger px-1.5 py-0.5 text-2xs font-bold text-white"
                    aria-label={`${item.badge} pending`}
                  >
                    {item.badge}
                  </span>
                ) : null}
              </Link>
            );
          })}
        </div>
      ))}
    </nav>
  );

  const SidebarSkeleton = () => (
    <div className="flex flex-col gap-2 pt-2" aria-busy="true" aria-label="Loading navigation">
      {[...Array(7)].map((_, idx) => (
        <div key={idx} className="flex items-center gap-2.5 px-[11px] py-[9px]">
          <div className="h-4 w-4 animate-pulse rounded bg-track" />
          <div className="h-3.5 flex-1 animate-pulse rounded bg-track" />
        </div>
      ))}
    </div>
  );

  const SidebarContent = ({ inDrawer = false }: { inDrawer?: boolean }) => (
    <div className="flex min-h-0 flex-1 flex-col">
      <div className="flex flex-shrink-0 items-center justify-between px-2.5 pb-4">
        <Link href="/dashboard" className="flex items-center gap-2.5" onClick={() => setMobileOpen(false)}>
          <AimLogo />
          <span className="text-base font-bold tracking-[-0.02em] text-ink">AIM</span>
        </Link>
        {inDrawer && (
          <button
            type="button"
            onClick={() => setMobileOpen(false)}
            className="rounded-pill p-1.5 text-ink-secondary hover:bg-glass-inset-gray"
            aria-label="Close menu"
          >
            <X className="h-5 w-5" />
          </button>
        )}
      </div>

      <PersonaSwitch className="mx-1.5 mb-4 flex-shrink-0" />

      {isLoading ? <SidebarSkeleton /> : <NavList />}

      <div className="mt-4 flex flex-shrink-0 flex-col gap-1 border-t border-divider pt-3">
        {user && (
          <div className="flex items-center gap-2.5 px-2">
            <span className="inline-flex h-8 w-8 flex-shrink-0 items-center justify-center rounded-full bg-gradient-to-br from-[#7c3aed] to-[#4f46e5] text-xs font-bold text-white">
              {(user.display_name || user.email || "?").slice(0, 1).toUpperCase()}
            </span>
            <span className="min-w-0 flex-1">
              <span className="block truncate text-xs font-bold text-ink">{user.display_name}</span>
              <span className="block truncate text-2xs text-ink-tertiary">{user.email}</span>
            </span>
          </div>
        )}
        <button
          type="button"
          onClick={handleLogout}
          disabled={isLoggingOut}
          className="flex items-center gap-2.5 rounded-nav px-[11px] py-[7px] text-[13px] font-medium text-ink-body hover:bg-glass-inset-gray hover:text-ink disabled:opacity-60"
        >
          {isLoggingOut ? <Loader2 className="h-4 w-4 animate-spin" /> : <LogOut className="h-4 w-4 text-ink-tertiary" />}
          <span>{isLoggingOut ? "Signing out..." : "Sign out"}</span>
        </button>
      </div>
    </div>
  );

  return (
    <>
      {/* Desktop: floating glass chrome */}
      <aside className="glass-chrome sticky top-6 hidden h-[calc(100vh-3rem)] w-56 flex-shrink-0 flex-col px-3.5 py-5 lg:flex">
        <SidebarContent />
      </aside>

      {/* Mobile: drawer opened from the bottom tab bar */}
      {mobileOpen && (
        <div
          className="fixed inset-0 z-40 bg-black/40 backdrop-blur-sm lg:hidden"
          onClick={() => setMobileOpen(false)}
          aria-hidden="true"
        />
      )}
      <aside
        className={cn(
          "glass-chrome fixed bottom-3 left-3 top-3 z-50 flex w-[280px] max-w-[85vw] flex-col px-3.5 py-5 transition-transform duration-300 ease-out lg:hidden",
          mobileOpen ? "translate-x-0" : "-translate-x-[120%]"
        )}
        aria-hidden={!mobileOpen}
      >
        <SidebarContent inDrawer />
      </aside>
    </>
  );
}
