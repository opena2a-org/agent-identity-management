"use client";

import { useEffect, useState } from "react";
import { useRouter, usePathname } from "next/navigation";
import { api } from "@/lib/api";
import { getDashboardPermissions, UserRole } from "@/lib/permissions";
import { toast } from "sonner";

interface RouteConfig {
  path: string;
  requiredPermission: keyof ReturnType<typeof getDashboardPermissions>;
}

const PROTECTED_ROUTES: RouteConfig[] = [
  { path: "/dashboard/admin", requiredPermission: "canViewAdmin" },
  { path: "/dashboard/admin/users", requiredPermission: "canViewAdmin" },
  { path: "/dashboard/admin/alerts", requiredPermission: "canViewAlerts" },
  { path: "/dashboard/admin/audit", requiredPermission: "canViewAudit" },
  { path: "/dashboard/admin/security-policies", requiredPermission: "canViewAdmin" },
  { path: "/dashboard/admin/capability-requests", requiredPermission: "canViewCapabilityRequests" },
  { path: "/dashboard/security", requiredPermission: "canViewSecurity" },
  { path: "/dashboard/monitoring", requiredPermission: "canViewMonitoring" },
  { path: "/dashboard/analytics", requiredPermission: "canViewAnalytics" },
];

export function useRouteGuard() {
  const router = useRouter();
  const pathname = usePathname();
  const [isChecking, setIsChecking] = useState(true);
  const [hasAccess, setHasAccess] = useState(false);

  useEffect(() => {
    const checkAccess = async () => {
      try {
        setIsChecking(true);

        // Get current user
        const user = await api.getCurrentUser();
        const userRole = (user.role === "pending" ? "viewer" : user.role) as UserRole;

        // Get permissions for this role
        const permissions = getDashboardPermissions(userRole);

        // Check if current route requires special permission
        const route = PROTECTED_ROUTES.find((r) => pathname?.startsWith(r.path));

        if (route) {
          const hasPermission = permissions[route.requiredPermission];

          if (!hasPermission) {
            // User doesn't have permission
            toast.error("Access Denied", {
              description: "You don't have permission to access this page.",
              duration: 4000,
            });

            // Redirect to dashboard
            router.replace("/dashboard");
            setHasAccess(false);
            return;
          }
        }

        // User has access
        setHasAccess(true);
      } catch (error) {
        console.error("Route guard check failed:", error);
        // On error, redirect to login
        router.replace("/auth/login");
        setHasAccess(false);
      } finally {
        setIsChecking(false);
      }
    };

    checkAccess();
  }, [pathname, router]);

  return { isChecking, hasAccess };
}

