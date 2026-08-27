import type { UserRole } from "@/lib/permissions";
import { HOSTED_DEPLOYMENT } from "@/lib/deployment";

/**
 * The sub-destinations of each top-level navigation entry, rendered as a tab
 * strip on the hub's pages (components/hub-tabs.tsx). Defined once, keyed by
 * the NavEntry key, and walked by lib/navigation-edge.test.ts exactly like the
 * sidebar entries and the mobile tabs — a tab the edge gate blocks for a role
 * cannot render for that role. Every href is an existing route: Stage 1 of the
 * IA consolidation moves no pages.
 */
export interface HubTab {
  name: string;
  href: string;
  roles: UserRole[];
  /**
   * Rendered only on the self-hosted build: hosted signups auto-approve, so
   * its registration queue can never have contents.
   */
  ossOnly?: boolean;
}

export const HUB_TABS: Record<string, HubTab[]> = {
  agents: [
    { name: "Agents", href: "/dashboard/agents", roles: ["admin", "manager", "member", "viewer"] },
    { name: "Connections", href: "/dashboard/a2a", roles: ["admin", "manager", "member"] },
  ],
  mcp: [
    { name: "Servers", href: "/dashboard/mcp", roles: ["admin", "manager", "member"] },
    { name: "Discovery", href: "/dashboard/mcp/discovery", roles: ["admin", "manager"] },
    { name: "Supply chain", href: "/dashboard/mcp/supply-chain", roles: ["admin", "manager"] },
  ],
  security: [
    { name: "Overview", href: "/dashboard/security", roles: ["admin", "manager"] },
    { name: "Violations", href: "/dashboard/security/violations", roles: ["admin", "manager"] },
    // Interim: the /dashboard/admin prefix narrows these to admin at the edge
    // (see lib/route-permissions.ts); Stage 2 relocates them out of that prefix.
    { name: "Alerts", href: "/dashboard/admin/alerts", roles: ["admin"] },
    { name: "Policies", href: "/dashboard/admin/security-policies", roles: ["admin"] },
    { name: "JIT requests", href: "/dashboard/admin/jit-requests", roles: ["admin"] },
    { name: "Capability requests", href: "/dashboard/admin/capability-requests", roles: ["admin"] },
  ],
  developers: [
    { name: "Guide", href: "/dashboard/developers", roles: ["admin", "manager", "member", "viewer"] },
    { name: "SDK & docs", href: "/dashboard/sdk", roles: ["admin", "manager", "member"] },
    { name: "API keys", href: "/dashboard/api-keys", roles: ["admin", "manager", "member"] },
    { name: "SDK tokens", href: "/dashboard/sdk-tokens", roles: ["admin", "manager", "member"] },
    // Role set matches the backend gate (MemberMiddleware allow-list) and the
    // matching ROUTE_PERMISSIONS entry that ships in the same commit.
    { name: "Webhooks", href: "/dashboard/webhooks", roles: ["admin", "manager", "member"] },
  ],
  organization: [
    { name: "Users", href: "/dashboard/admin/users", roles: ["admin"] },
    { name: "Tags", href: "/dashboard/tags", roles: ["admin", "manager", "member"] },
    { name: "Registrations", href: "/admin/registrations", roles: ["admin"], ossOnly: true },
  ],
};

/** Segment-safe prefix match: /dashboard/sdk never claims /dashboard/sdk-tokens. */
export function pathWithinTab(pathname: string, href: string): boolean {
  return pathname === href || pathname.startsWith(href + "/");
}

/** The tabs a role may see, on this deployment flavor. */
export function visibleHubTabs(tabs: HubTab[], role: UserRole | undefined): HubTab[] {
  if (!role) return [];
  return tabs.filter((t) => t.roles.includes(role) && (!t.ossOnly || !HOSTED_DEPLOYMENT));
}

/** The hub a pathname belongs to, or null (a page that is nobody's tab shows no strip). */
export function hubTabsForPath(pathname: string): HubTab[] | null {
  for (const tabs of Object.values(HUB_TABS)) {
    if (tabs.some((t) => pathWithinTab(pathname, t.href))) return tabs;
  }
  return null;
}

/** The active tab for a pathname: the most specific matching href wins. */
export function activeHubTabHref(tabs: HubTab[], pathname: string): string | null {
  let active: string | null = null;
  for (const t of tabs) {
    if (pathWithinTab(pathname, t.href) && (active === null || t.href.length > active.length)) {
      active = t.href;
    }
  }
  return active;
}

/** Whether a top-level entry is the one the pathname sits under (sidebar active state). */
export function navEntryIsActive(key: string, entryHref: string, pathname: string): boolean {
  if (key === "overview") return pathname === "/dashboard";
  if (pathWithinTab(pathname, entryHref)) return true;
  return (HUB_TABS[key] ?? []).some((t) => pathWithinTab(pathname, t.href));
}
