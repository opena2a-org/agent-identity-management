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
        // Check if user is logged in before making API call
        const token = localStorage.getItem('auth_token');
        if (!token) {
          return; // No token, user is not logged in, skip check
        }

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
        // Only log errors if we actually have a token (user should be logged in)
        const token = localStorage.getItem('auth_token');
        if (token) {
          console.error("User status check failed:", error);
        }
      }
    };

    checkUserStatus();
  }, [router, pathname]);
}
