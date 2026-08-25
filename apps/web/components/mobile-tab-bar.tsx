"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { Bell, Home, Menu, Shield, ShieldPlus } from "lucide-react";
import type { UserRole } from "@/lib/permissions";
import { cn } from "@/lib/utils";

interface MobileTabBarProps {
  role?: UserRole;
  onOpenMenu: () => void;
}

/**
 * Bottom tab bar for small screens (the sidebar is hidden below lg). The center
 * "Secure" action starts agent registration. Alerts points at the alerts page
 * for roles that can read it and at the overview otherwise: the tab bar never
 * grants a route the role cannot open.
 */
export function MobileTabBar({ role, onOpenMenu }: MobileTabBarProps) {
  const pathname = usePathname();
  const canSeeAlerts = role === "admin" || role === "manager";
  const alertsHref = canSeeAlerts ? "/dashboard/admin/alerts" : "/dashboard";

  const tabs = [
    { name: "Overview", href: "/dashboard", icon: Home, exact: true },
    { name: "Agents", href: "/dashboard/agents", icon: Shield, exact: false },
  ];
  const isActive = (href: string, exact: boolean) =>
    exact ? pathname === href : pathname === href || pathname.startsWith(href + "/");

  const item = "flex min-w-[44px] flex-col items-center gap-1 py-1 text-[10px] font-semibold";

  return (
    <nav
      aria-label="Primary"
      className="fixed inset-x-0 bottom-0 z-30 flex items-center justify-around border-t border-glass-chrome-border bg-glass-chrome px-2 pb-[max(env(safe-area-inset-bottom),22px)] pt-2.5 backdrop-blur-chrome lg:hidden"
      style={{ boxShadow: "0 -8px 30px rgba(15, 23, 42, 0.06)" }}
    >
      {tabs.map(({ name, href, icon: Icon, exact }) => {
        const active = isActive(href, exact);
        return (
          <Link key={href} href={href} aria-current={active ? "page" : undefined} className={cn(item, active ? "font-bold text-brand-text" : "text-ink-tertiary")}>
            <Icon className="h-[22px] w-[22px]" strokeWidth={2} aria-hidden="true" />
            {name}
          </Link>
        );
      })}

      <Link href="/dashboard/agents?register=1" className={cn(item, "p-0 text-ink-tertiary")}>
        <span className="-mt-3.5 inline-flex h-[46px] w-[46px] items-center justify-center rounded-full bg-brand text-white shadow-[0_8px_20px_rgba(0,113,227,0.35)]">
          <ShieldPlus className="h-[22px] w-[22px]" strokeWidth={2.2} aria-hidden="true" />
        </span>
        Secure
      </Link>

      <Link href={alertsHref} className={cn(item, isActive("/dashboard/admin/alerts", false) ? "font-bold text-brand-text" : "text-ink-tertiary")}>
        <Bell className="h-[22px] w-[22px]" strokeWidth={2} aria-hidden="true" />
        Alerts
      </Link>

      <button type="button" onClick={onOpenMenu} className={cn(item, "text-ink-tertiary")} aria-label="Open menu">
        <Menu className="h-[22px] w-[22px]" strokeWidth={2} aria-hidden="true" />
        Menu
      </button>
    </nav>
  );
}
