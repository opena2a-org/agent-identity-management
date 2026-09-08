/**
 * Role requirements for dashboard routes, enforced at the edge (middleware.ts in the OSS
 * build, proxy.ts on the hosted product). Kept in one module so the navigation tests can
 * assert that nothing the navigation renders is blocked by the edge.
 *
 * Matching semantics (mirrors the edge loop exactly): every entry whose path is a prefix of
 * the requested pathname applies, and access requires the role to be in EVERY matching
 * entry — an intersection, because the loop denies on any failing match and does not break.
 * Consequence worth knowing: a specific entry cannot WIDEN what a broader prefix allows
 * (e.g. '/dashboard/admin/alerts' listing managers is still narrowed to admin by
 * '/dashboard/admin').
 */
import type { UserRole } from "@/lib/permissions";

export const ROUTE_PERMISSIONS: Record<string, UserRole[]> = {
  // Top-level /admin (registration review lives here in the OSS tree): admins only.
  // Measured 2026-08-24: without this entry the proxy passed any authenticated role.
  "/admin": ["admin"],
  "/dashboard/admin": ["admin"],
  "/dashboard/admin/users": ["admin"],
  "/dashboard/admin/alerts": ["admin", "manager"],
  "/dashboard/admin/audit": ["admin", "manager"],
  "/dashboard/admin/security-policies": ["admin"],
  "/dashboard/admin/capability-requests": ["admin"],
  "/dashboard/security": ["admin", "manager", "member"],
  // Ships in the same commit as the route's first nav exposure (Developers →
  // Webhooks tab). Role set traces to the backend gate: MemberMiddleware's
  // admin/manager/member allow-list on the /webhooks group.
  "/dashboard/webhooks": ["admin", "manager", "member"],
};

export const ALL_ROLES: UserRole[] = ["admin", "manager", "member", "viewer"];

/** Roles the edge allows for a pathname: the intersection of all matching prefix entries. */
export function effectiveEdgeRoles(pathname: string): UserRole[] {
  return ALL_ROLES.filter((role) =>
    Object.entries(ROUTE_PERMISSIONS).every(
      ([prefix, roles]) => !pathname.startsWith(prefix) || roles.includes(role)
    )
  );
}
