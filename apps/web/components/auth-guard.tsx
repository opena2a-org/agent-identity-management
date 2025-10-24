"use client";

import { useEffect, useState } from "react";
import { useRouter, usePathname } from "next/navigation";
import { Loader2 } from "lucide-react";

interface AuthGuardProps {
  children: React.ReactNode;
  requireAuth?: boolean;
}

/**
 * AuthGuard Component
 * 
 * Protects routes that require authentication. If the user is not authenticated,
 * redirects them to the login page with a return URL.
 * 
 * Usage:
 * ```tsx
 * <AuthGuard>
 *   <DashboardContent />
 * </AuthGuard>
 * ```
 */
export function AuthGuard({ children, requireAuth = true }: AuthGuardProps) {
  const router = useRouter();
  const pathname = usePathname();
  const [isAuthenticated, setIsAuthenticated] = useState<boolean | null>(null);

  useEffect(() => {
    if (!requireAuth) {
      setIsAuthenticated(true);
      return;
    }

    // Check for authentication token
    const token = localStorage.getItem("auth_token");

    if (!token) {
      // No token found, redirect to login with return URL
      const returnUrl = encodeURIComponent(pathname);
      router.replace(`/auth/login?returnUrl=${returnUrl}`);
      setIsAuthenticated(false);
    } else {
      // Token exists, user is authenticated
      setIsAuthenticated(true);
    }
  }, [requireAuth, pathname, router]);

  // Show loading state while checking authentication
  if (isAuthenticated === null) {
    return (
      <div className="flex items-center justify-center min-h-screen">
        <div className="flex flex-col items-center gap-4">
          <Loader2 className="h-8 w-8 animate-spin text-blue-600" />
          <p className="text-sm text-gray-500">Verifying authentication...</p>
        </div>
      </div>
    );
  }

  // If not authenticated, show nothing (user is being redirected)
  if (!isAuthenticated) {
    return null;
  }

  // User is authenticated, render children
  return <>{children}</>;
}
