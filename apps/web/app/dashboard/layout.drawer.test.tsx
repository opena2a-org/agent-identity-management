import { afterEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import Link from "next/link";
import type { ReactNode } from "react";
import DashboardLayout from "./layout";

// On a phone the navigation drawer opens from the bottom tab bar over a backdrop.
// It is a modal dialog: while it is open, nothing behind the backdrop is reachable
// by the keyboard or exposed to a screen reader, and closing it hands focus back.

const router = vi.hoisted(() => ({ replace: vi.fn(), push: vi.fn(), prefetch: vi.fn() }));
vi.mock("next/navigation", () => ({
  useRouter: () => router,
  usePathname: () => "/dashboard",
  useSearchParams: () => new URLSearchParams(),
}));
vi.mock("next/link", () => ({
  default: ({ href, children, ...rest }: { href: string; children: ReactNode }) => (
    <a href={href} {...rest}>
      {children}
    </a>
  ),
}));
vi.mock("@/lib/api", () => ({
  api: {
    getCurrentUser: vi.fn(async () => ({ email: "admin@example.test", name: "Admin", role: "admin" })),
    getToken: () => null,
    clearToken: vi.fn(),
    logout: vi.fn(async () => {}),
    getUnacknowledgedAlertCount: vi.fn(async () => 0),
    getAlerts: vi.fn(async () => ({ criticalCount: 0, highCount: 0 })),
    getPendingCapabilityRequestsCount: vi.fn(async () => 0),
    getPendingVerificationCount: vi.fn(async () => 0),
  },
}));
vi.mock("@/components/route-gate", () => ({ RouteGate: ({ children }: { children: ReactNode }) => <>{children}</> }));
vi.mock("@/components/dashboard-header", () => ({
  DashboardHeader: () => (
    <header>
      <button type="button">Account</button>
    </header>
  ),
}));
vi.mock("@/components/hub-tabs", () => ({ HubTabs: () => null }));
vi.mock("@/components/idle-timeout-guard", () => ({ IdleTimeoutGuard: () => null }));
vi.mock("@/hooks/use-deactivation-check", () => ({ useDeactivationCheck: () => {} }));

afterEach(cleanup);

// The links and buttons a phone draws outside the drawer. The desktop sidebar
// (`hidden lg:flex`) is not drawn below the desktop breakpoint, and jsdom applies
// no stylesheet, so it is left out by its class.
function controlsBehindTheDrawer(): HTMLElement[] {
  const drawer = document.querySelector('aside[role="dialog"]');
  return [...document.body.querySelectorAll<HTMLElement>("a, button")].filter(
    (el) => !drawer?.contains(el) && !el.closest(".hidden"),
  );
}

async function renderDashboard() {
  render(
    <DashboardLayout>
      <Link href="/dashboard/agents">All agents</Link>
      <button type="button">Refresh</button>
    </DashboardLayout>,
  );
  // The tab bar shows Secure once the role is read; the drawer lists its entries once the user is.
  await screen.findByRole("link", { name: "Secure" });
  await screen.findAllByRole("link", { name: "Organization", hidden: true });
  return screen.getByRole("button", { name: "Open menu" });
}

describe("the phone navigation drawer", () => {
  it("is a modal dialog that leaves every link and button behind its backdrop inert", async () => {
    const openMenu = await renderDashboard();
    const behind = controlsBehindTheDrawer();
    expect(behind.map((el) => el.textContent)).toEqual(
      expect.arrayContaining(["Account", "All agents", "Refresh", "Overview", "Agents", "Secure", "Menu"]),
    );
    expect(behind.filter((el) => el.closest("[inert]"))).toEqual([]);

    openMenu.focus();
    fireEvent.click(openMenu);

    const dialog = screen.getByRole("dialog", { name: "Navigation" });
    expect(dialog.getAttribute("aria-modal")).toBe("true");
    expect(dialog.closest("[inert]")).toBeNull();
    expect(document.activeElement).toBe(dialog);
    const reachable = controlsBehindTheDrawer().filter((el) => !el.closest("[inert]"));
    expect(reachable.map((el) => el.textContent)).toEqual([]);
  });

  it("closes on Escape, returns focus to the menu button and leaves nothing inert", async () => {
    const openMenu = await renderDashboard();
    openMenu.focus();
    fireEvent.click(openMenu);
    expect(screen.getByRole("dialog", { name: "Navigation" })).toBeTruthy();

    act(() => {
      fireEvent.keyDown(document, { key: "Escape" });
    });

    expect(screen.queryByRole("dialog")).toBeNull();
    expect(document.activeElement).toBe(openMenu);
    expect(controlsBehindTheDrawer().filter((el) => el.closest("[inert]"))).toEqual([]);
  });

  it("closes when the window reaches the desktop layout, where the drawer is not drawn", async () => {
    const listeners = new Set<() => void>();
    const desktop = { matches: false };
    vi.stubGlobal("matchMedia", (query: string) => ({
      get matches() {
        return query === "(min-width: 1024px)" && desktop.matches;
      },
      addEventListener: (_: string, fn: () => void) => listeners.add(fn),
      removeEventListener: (_: string, fn: () => void) => listeners.delete(fn),
    }));
    try {
      const openMenu = await renderDashboard();
      fireEvent.click(openMenu);
      expect(screen.getByRole("dialog", { name: "Navigation" })).toBeTruthy();

      act(() => {
        desktop.matches = true;
        listeners.forEach((fn) => fn());
      });

      expect(screen.queryByRole("dialog")).toBeNull();
      expect(document.querySelector("[inert]:not(aside)")).toBeNull();
    } finally {
      vi.unstubAllGlobals();
    }
  });
});
