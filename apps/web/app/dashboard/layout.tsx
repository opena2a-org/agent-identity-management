"use client";

import { useEffect, useState } from "react";
import { Sidebar } from "@/components/sidebar";
import { DashboardHeader } from "@/components/dashboard-header";
import { HubTabs } from "@/components/hub-tabs";
import { MobileTabBar } from "@/components/mobile-tab-bar";
import { IdleTimeoutGuard } from "@/components/idle-timeout-guard";
import { useDeactivationCheck } from "@/hooks/use-deactivation-check";
import { api } from "@/lib/api";
import type { UserRole } from "@/lib/permissions";

export default function DashboardLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  // Check if user is deactivated and logout if necessary
  useDeactivationCheck();
  const [mobileNavOpen, setMobileNavOpen] = useState(false);
  const [role, setRole] = useState<UserRole | undefined>(undefined);

  useEffect(() => {
    let cancelled = false;
    api
      .getCurrentUser()
      .then((u) => {
        if (cancelled) return;
        setRole(u?.role === "pending" ? "viewer" : (u?.role as UserRole | undefined));
      })
      .catch(() => {
        /* the sidebar and header handle the unauthenticated redirect */
      });
    return () => {
      cancelled = true;
    };
  }, []);

  return (
    <div className="glass-page glass-page--layered relative min-h-screen">
      <div className="glass-page-wash" aria-hidden="true" />
      {/* Idle / absolute session timeout (30m idle, 8h cap) */}
      <IdleTimeoutGuard />
      <div className="flex gap-5 p-4 pb-28 sm:p-6 lg:pb-6">
        <Sidebar mobileOpen={mobileNavOpen} onMobileOpenChange={setMobileNavOpen} />
        <div className="flex min-w-0 flex-1 flex-col gap-4">
          <DashboardHeader />
          <HubTabs role={role} />
          <main className="min-w-0 flex-1">{children}</main>
        </div>
      </div>
      <MobileTabBar role={role} onOpenMenu={() => setMobileNavOpen(true)} />
    </div>
  );
}
