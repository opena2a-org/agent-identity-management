import {
  AlertTriangle,
  Bell,
  BookOpen,
  CheckCircle,
  CheckSquare,
  ClipboardCheck,
  Code,
  GitBranch,
  Home,
  Key,
  Link2,
  Lock,
  Search,
  Server,
  Shield,
  ShieldCheck,
  Tag,
  Users,
} from "lucide-react";
import type { NavSection } from "@/lib/permissions";
import type { Persona } from "@/lib/persona";

export const SECTION_MAIN = "";
export const SECTION_BUILD = "Build";
export const SECTION_MONITORING = "Monitoring";
export const SECTION_ACCOUNT = "Account";

/**
 * Navigation with role-based access control. `roles` is the authorization
 * filter (see filterNavigationByRole); the persona lens only reorders sections.
 */
export const navigationBase: NavSection[] = [
  {
    title: SECTION_MAIN,
    items: [
      { name: "Overview", href: "/dashboard", icon: Home, roles: ["admin", "manager", "member", "viewer"] },
      { name: "Agents", href: "/dashboard/agents", icon: Shield, roles: ["admin", "manager", "member", "viewer"] },
      { name: "MCP servers", href: "/dashboard/mcp", icon: Server, roles: ["admin", "manager", "member"] },
      { name: "MCP discovery", href: "/dashboard/mcp/discovery", icon: Search, roles: ["admin", "manager"] },
      { name: "Supply chain", href: "/dashboard/mcp/supply-chain", icon: GitBranch, roles: ["admin", "manager"] },
      { name: "A2A protocol", href: "/dashboard/a2a", icon: Link2, roles: ["admin", "manager", "member"] },
    ],
  },
  {
    title: SECTION_BUILD,
    items: [
      { name: "API keys", href: "/dashboard/api-keys", icon: Key, roles: ["admin", "manager", "member"] },
      { name: "SDK tokens", href: "/dashboard/sdk-tokens", icon: Lock, roles: ["admin", "manager", "member"] },
      { name: "SDK & docs", href: "/dashboard/sdk", icon: Code, roles: ["admin", "manager", "member"] },
      { name: "Developer guide", href: "/dashboard/developers", icon: BookOpen, roles: ["admin", "manager", "member", "viewer"] },
    ],
  },
  {
    title: SECTION_MONITORING,
    items: [
      { name: "Security", href: "/dashboard/security", icon: AlertTriangle, roles: ["admin", "manager"] },
      { name: "Alerts", href: "/dashboard/admin/alerts", icon: Bell, roles: ["admin", "manager"] },
    ],
  },
  {
    // Administering your own organization (every signup administers its own account).
    title: SECTION_ACCOUNT,
    items: [
      { name: "JIT requests", href: "/dashboard/admin/jit-requests", icon: CheckCircle, roles: ["admin"] },
      { name: "Capability requests", href: "/dashboard/admin/capability-requests", icon: CheckSquare, roles: ["admin"] },
      { name: "Security policies", href: "/dashboard/admin/security-policies", icon: ShieldCheck, roles: ["admin"] },
      { name: "Compliance", href: "/dashboard/admin/compliance", icon: ClipboardCheck, roles: ["admin"] },
      { name: "Users", href: "/dashboard/admin/users", icon: Users, roles: ["admin"] },
      { name: "Tags", href: "/dashboard/tags", icon: Tag, roles: ["admin", "manager", "member"] },
    ],
  },
];

const SECTION_ORDER: Record<Persona, string[]> = {
  developer: [SECTION_MAIN, SECTION_BUILD, SECTION_MONITORING, SECTION_ACCOUNT],
  security: [SECTION_MONITORING, SECTION_MAIN, SECTION_ACCOUNT, SECTION_BUILD],
  executive: [SECTION_MAIN, SECTION_ACCOUNT, SECTION_MONITORING, SECTION_BUILD],
};

/**
 * Reorders already-authorized sections for a lens. Pure and lossless: it never
 * adds, removes or renames an item, so authorization stays exactly what the
 * role filter produced (proved by lib/persona.test.ts).
 */
export function orderNavigationForPersona(sections: NavSection[], persona: Persona): NavSection[] {
  const order = SECTION_ORDER[persona] ?? SECTION_ORDER.developer;
  const rank = (title?: string) => {
    const i = order.indexOf(title ?? SECTION_MAIN);
    return i === -1 ? order.length : i;
  };
  return [...sections].sort((a, b) => rank(a.title) - rank(b.title));
}

/** Where the "Overview" tab and the logo lead for a lens. Always inside /dashboard. */
export function overviewHrefForPersona(persona: Persona): string {
  return persona === "security" ? "/dashboard/security" : "/dashboard";
}
