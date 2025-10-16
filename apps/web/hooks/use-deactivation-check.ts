"use client";

import { useEffect, useRef } from "react";
import { useRouter, usePathname } from "next/navigation";
import { api } from "@/lib/api";
import { toast } from "sonner";

/**
 * Hook to check if the current user is deactivated
 * If deactivated, logs them out and redirects to login with a toast message
 */
export function useDeactivationCheck() {
  const router = useRouter();
  const pathname = usePathname();
  const hasChecked = useRef(false);
  useEffect(() => {
    if (hasChecked.current) return;

    const publicRoutes = [
      "/auth/login",
      "/auth/register",
      "/auth/callback",
      "/auth/registration-pending",
    ];
    if (publicRoutes.some((route) => pathname?.startsWith(route))) {
      return;
    }

    const checkUserStatus = async () => {
      try {
        const user = await api.getCurrentUser();

        if (user.status === "deactivated") {
          hasChecked.current = true;

          toast.error("Account Blocked", {
            description:
              "Your account has been deactivated. Please contact your administrator for assistance.",
            duration: 6000,
          });

          api.clearToken();

          setTimeout(() => {
            router.push("/auth/login");
          }, 500);
        }
      } catch (error) {
        console.error("User status check failed:", error);
      }
    };

    checkUserStatus();
  }, [router, pathname]);
}
