"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { activeHubTabHref, hubTabsForPath, visibleHubTabs } from "@/lib/hub-tabs";
import type { UserRole } from "@/lib/permissions";
import { cn } from "@/lib/utils";

/**
 * The tab strip a hub page shows for its sub-destinations (lib/hub-tabs.ts).
 * Rendered once from the dashboard layout, from the shared HUB_TABS data that
 * the edge-parity test walks — pages do not restate their tabs.
 * Styling mirrors components/ui/tabs.tsx (the Glasshouse segmented control),
 * as links rather than in-page tab state.
 */
export function HubTabs({ role }: { role?: UserRole }) {
  const pathname = usePathname();
  if (!pathname) return null;
  const tabs = hubTabsForPath(pathname);
  if (!tabs) return null;
  const visible = visibleHubTabs(tabs, role);
  // A strip with one destination is noise, not navigation.
  if (visible.length < 2) return null;
  const active = activeHubTabHref(visible, pathname);

  return (
    <nav
      aria-label="Section"
      className="scrollbar-none -mt-1 inline-flex max-w-full items-center gap-0.5 self-start overflow-x-auto rounded-pill bg-[var(--segment-track)] p-[3px] text-ink-secondary"
    >
      {visible.map((tab) => {
        const isActive = tab.href === active;
        return (
          <Link
            key={tab.href}
            href={tab.href}
            aria-current={isActive ? "page" : undefined}
            className={cn(
              "inline-flex min-h-9 items-center justify-center whitespace-nowrap rounded-pill px-3.5 py-1.5 text-[13px] font-semibold transition-colors hover:text-ink focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2",
              isActive && "bg-[var(--segment-active)] font-bold text-ink shadow-[var(--segment-active-shadow)]"
            )}
          >
            {tab.name}
          </Link>
        );
      })}
    </nav>
  );
}
