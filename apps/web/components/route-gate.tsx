"use client";

import { useEffect, useState, type ReactNode } from "react";
import { usePathname, useRouter } from "next/navigation";
import { api } from "@/lib/api";
import { decodeJwtPayload } from "@/lib/jwt-payload";
import { ROUTE_PERMISSIONS } from "@/lib/route-permissions";
import type { UserRole } from "@/lib/permissions";

/**
 * The dashboard's route gate. It decides from the same session store the API
 * client sends (api.getToken(), the localStorage pair) and nothing else: a
 * cookie the browser holds cannot open a page or widen a role. The decision
 * mirrors the former edge middleware exactly: no or undecodable token goes to
 * sign-in with the return path; "pending" counts as "viewer"; on every
 * ROUTE_PERMISSIONS prefix that matches, a role outside the allowed set goes
 * to /dashboard/forbidden; otherwise the page renders. Nothing here verifies
 * the token; the API does, on every request.
 */
export type GateVerdict = { kind: "pass"; to: null } | { kind: "login" | "forbidden"; to: string };

export function gateVerdict(pathname: string, token: string | null | undefined): GateVerdict {
  const login = { kind: "login" as const, to: `/auth/login?returnUrl=${encodeURIComponent(pathname)}` };
  if (!token) return login;
  const payload = decodeJwtPayload(token);
  if (!payload) return login;
  const role = payload.role;
  const normalized = (role === "pending" ? "viewer" : role) as UserRole | undefined;
  for (const [route, allowedRoles] of Object.entries(ROUTE_PERMISSIONS)) {
    if (pathname.startsWith(route) && (!normalized || !allowedRoles.includes(normalized))) {
      return { kind: "forbidden", to: "/dashboard/forbidden" };
    }
  }
  return { kind: "pass", to: null };
}

function accountOf(token: string | null): string | null {
  const payload = decodeJwtPayload(token);
  return payload ? `${payload.user_id ?? ""}|${payload.organization_id ?? ""}` : null;
}

export function RouteGate({ children }: { children: ReactNode }) {
  const router = useRouter();
  const pathname = usePathname();
  const [decided, setDecided] = useState(false);

  useEffect(() => {
    const verdict = gateVerdict(pathname, api.getToken());
    if (verdict.kind === "pass") {
      setDecided(true);
      return;
    }
    setDecided(false);
    router.replace(verdict.to);
  }, [pathname, router]);

  // A tab is bound to the account it rendered: when another tab signs in as a
  // different account, this tab reloads as that account instead of mixing the
  // two. A removed token is handled by the idle-timeout listener.
  useEffect(() => {
    const bound = accountOf(api.getToken());
    const onStorage = (event: StorageEvent) => {
      if (event.key !== "auth_token" || event.newValue === null) return;
      if (accountOf(event.newValue) !== bound) window.location.reload();
    };
    window.addEventListener("storage", onStorage);
    return () => window.removeEventListener("storage", onStorage);
  }, []);

  if (!decided) {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <p className="text-sm text-ink-secondary">Verifying authentication...</p>
      </div>
    );
  }
  return <>{children}</>;
}
