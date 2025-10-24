"use client";

import { Sidebar } from "@/components/sidebar";
import { DashboardHeader } from "@/components/dashboard-header";
import { useDeactivationCheck } from "@/hooks/use-deactivation-check";

export default function DashboardLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  // Check if user is deactivated and logout if necessary
  useDeactivationCheck();

  return (
    <div className="flex min-h-screen bg-gray-50 dark:bg-gray-950">
      <Sidebar />

      {/* Main Content Area with Header */}
      <div className="flex-1 flex flex-col overflow-hidden">
        <DashboardHeader />

        {/* Page Content */}
        <main className="flex-1 overflow-auto">
          <div className="w-full px-4 sm:px-6 lg:px-8 py-8">{children}</div>
        </main>
      </div>
    </div>
  );
}
