"use client";

import { useEffect, useRef } from "react";
import { useRouter } from "next/navigation";
import { api } from "@/lib/api";
import { toast } from "sonner";

/**
 * Hook to check if the current user is deactivated
 * If deactivated, logs them out and redirects to login with a toast message.
 * This hook runs only inside the dashboard shell, so every page it sees is a gated
 * page; there is no public-route skip list here.
 */
export function useDeactivationCheck() {
  const router = useRouter();
  const hasChecked = useRef(false);
  useEffect(() => {
    if (hasChecked.current) return;

    const checkUserStatus = async () => {
      try {
        // Check if user is logged in before making API call
        const token = api.getToken();
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
        const token = api.getToken();
        if (token) {
          console.error("User status check failed:", error);
        }
      }
    };

    checkUserStatus();
  }, [router]);
}
