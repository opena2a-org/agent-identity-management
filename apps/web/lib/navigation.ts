import {
  AlertTriangle,
  Building2,
  ClipboardCheck,
  Code,
  Home,
  Server,
  Shield,
} from "lucide-react";
import type { UserRole } from "@/lib/permissions";
import type { Persona } from "@/lib/persona";

/**
 * Top-level navigation: seven flat entries. The
 * former sections' sub-destinations are tabs on each hub page, defined once in
 * lib/hub-tabs.ts and walked by the same edge-parity test as this list.
 * `roles` is the authorization filter (see filterNavigationByRole); the
 * persona lens only reorders entries.
 */
export interface NavEntry {
  /** Stable identity for ordering, active-state and hub-tab lookup — labels may be reworded. */
  key: string;
  name: string;
  href: string;
  icon: any;
  roles: UserRole[];
  /**
   * Role-specific target: the entry lands different roles on different
   * existing routes (the mobile tab bar's per-role pattern). The edge-parity
   * test resolves per role, so a target the edge blocks cannot ship.
   */
  hrefByRole?: Partial<Record<UserRole, string>>;
  badge?: number;
}

export const navigationBase: NavEntry[] = [
  { key: "overview", name: "Overview", href: "/dashboard", icon: Home, roles: ["admin", "manager", "member", "viewer"] },
  { key: "agents", name: "Agents", href: "/dashboard/agents", icon: Shield, roles: ["admin", "manager", "member", "viewer"] },
  { key: "mcp", name: "MCP servers", href: "/dashboard/mcp", icon: Server, roles: ["admin", "manager", "member"] },
  { key: "security", name: "Security", href: "/dashboard/security", icon: AlertTriangle, roles: ["admin", "manager"] },
  // Direct route until the Stage 2 move to /dashboard/compliance (with its gate entry).
  { key: "compliance", name: "Compliance", href: "/dashboard/admin/compliance", icon: ClipboardCheck, roles: ["admin"] },
  { key: "developers", name: "Developers", href: "/dashboard/developers", icon: Code, roles: ["admin", "manager", "member", "viewer"] },
  // Ships as "Organization" until an OSS /settings surface exists.
  {
    key: "organization",
    name: "Organization",
    href: "/dashboard/tags",
    icon: Building2,
    roles: ["admin", "manager", "member"],
    hrefByRole: { admin: "/dashboard/admin/users" },
  },
];

/** The route an entry opens for a role. */
export function resolveNavHref(entry: NavEntry, role: UserRole | undefined): string {
  return (role && entry.hrefByRole?.[role]) || entry.href;
}

/**
 * Entry-level persona ranks: Overview is always
 * first and Organization always last; the lens orders what sits between.
 */
const ENTRY_ORDER: Record<Persona, string[]> = {
  developer: ["agents", "developers", "mcp", "security", "compliance"],
  security: ["security", "agents", "mcp", "compliance", "developers"],
  executive: ["compliance", "security", "agents", "mcp", "developers"],
};

/**
 * Reorders already-authorized entries for a lens. Pure and lossless: it never
 * adds, removes or renames an entry, so authorization stays exactly what the
 * role filter produced (proved by lib/persona.test.ts).
 */
export function orderNavigationForPersona<T extends NavEntry>(entries: T[], persona: Persona): T[] {
  const order = ENTRY_ORDER[persona] ?? ENTRY_ORDER.developer;
  const rank = (entry: NavEntry) => {
    if (entry.key === "overview") return -1;
    if (entry.key === "organization") return order.length + 1;
    const i = order.indexOf(entry.key);
    return i === -1 ? order.length : i;
  };
  return [...entries].sort((a, b) => rank(a) - rank(b));
}
