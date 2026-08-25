"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { BookOpen, Home, Menu, Shield, ShieldAlert, ShieldPlus } from "lucide-react";
import { getAgentPermissions, getDashboardPermissions, type UserRole } from "@/lib/permissions";
import { cn } from "@/lib/utils";

interface MobileTabBarProps {
  role?: UserRole;
  onOpenMenu: () => void;
}

export interface MobileTab {
  name: string;
  href: string;
  icon: typeof Home;
  exact: boolean;
}

/**
 * The tabs a role sees. Property (tested in lib/navigation-edge.test.ts): every rendered tab
 * resolves to a route the role can open — no tab may silently retarget or redirect.
 * The Secure action is shown only to roles that may register agents; the last slot is
 * Security for the roles the sidebar and the backend admit there, the developer guide otherwise.
 */
export function tabsForRole(role?: UserRole): MobileTab[] {
  const tabs: MobileTab[] = [
    { name: "Overview", href: "/dashboard", icon: Home, exact: true },
    { name: "Agents", href: "/dashboard/agents", icon: Shield, exact: false },
  ];
  if (getAgentPermissions(role).canCreateAgent) tabs.push({ name: "Secure", href: "/dashboard/agents?register=1", icon: ShieldPlus, exact: false });
  tabs.push(
    getDashboardPermissions(role).canViewSecurity
      ? { name: "Security", href: "/dashboard/security", icon: ShieldAlert, exact: false }
      : { name: "Guide", href: "/dashboard/developers", icon: BookOpen, exact: false }
  );
  return tabs;
}

/**
 * Bottom tab bar for small screens (the sidebar is hidden below lg). The center "Secure"
 * action starts agent registration; Menu opens the navigation drawer.
 */
export function MobileTabBar({ role, onOpenMenu }: MobileTabBarProps) {
  const pathname = usePathname();
  const tabs = tabsForRole(role);
  const [overview, agents] = tabs;
  const secure = tabs.find((t) => t.name === "Secure");
  const fourth = tabs[tabs.length - 1];

  const isActive = (href: string, exact: boolean) => {
    const path = href.split("?")[0];
    return exact ? pathname === path : pathname === path || pathname.startsWith(path + "/");
  };

  const item = "flex min-w-[44px] flex-col items-center gap-1 py-1 text-[10px] font-semibold";
  const renderTab = ({ name, href, icon: Icon, exact }: MobileTab) => {
    const active = isActive(href, exact);
    return (
      <Link key={name} href={href} aria-current={active ? "page" : undefined} className={cn(item, active ? "font-bold text-brand-text" : "text-ink-tertiary")}>
        <Icon className="h-[22px] w-[22px]" strokeWidth={2} aria-hidden="true" />
        {name}
      </Link>
    );
  };

  return (
    <nav
      aria-label="Primary"
      className="fixed inset-x-0 bottom-0 z-30 flex items-center justify-around border-t border-glass-chrome-border bg-glass-chrome px-2 pb-[max(env(safe-area-inset-bottom),22px)] pt-2.5 backdrop-blur-chrome lg:hidden"
      style={{ boxShadow: "0 -8px 30px rgba(15, 23, 42, 0.06)" }}
    >
      {renderTab(overview)}
      {renderTab(agents)}

      {secure && (
        <Link href={secure.href} className={cn(item, "p-0 text-ink-tertiary")}>
          <span className="-mt-3.5 inline-flex h-[46px] w-[46px] items-center justify-center rounded-full bg-brand text-white shadow-[0_8px_20px_rgba(0,113,227,0.35)]">
            <ShieldPlus className="h-[22px] w-[22px]" strokeWidth={2.2} aria-hidden="true" />
          </span>
          {secure.name}
        </Link>
      )}

      {renderTab(fourth)}

      <button type="button" onClick={onOpenMenu} className={cn(item, "text-ink-tertiary")} aria-label="Open menu">
        <Menu className="h-[22px] w-[22px]" strokeWidth={2} aria-hidden="true" />
        Menu
      </button>
    </nav>
  );
}
