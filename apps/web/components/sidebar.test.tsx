import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";
import type { ReactNode } from "react";
import { activeNavigationGrounds, drawnTextColour, shortfalls } from "@/tests/text-colour";
import { Sidebar } from "./sidebar";

const router = vi.hoisted(() => ({ replace: vi.fn(), push: vi.fn(), prefetch: vi.fn() }));
vi.mock("next/navigation", () => ({
  useRouter: () => router,
  usePathname: () => "/dashboard",
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

afterEach(cleanup);

describe("the shared navigation", () => {
  // The active link and its icon read 4.38:1 in the dark drawer in the brand ink.
  it("draws the active link and its icon at 4.5:1 or more on every surface it sits on, in the sidebar and the drawer", async () => {
    render(<Sidebar mobileOpen />);
    const active = await screen.findAllByRole("link", { current: "page", hidden: true });
    expect(active.map((link) => link.textContent)).toEqual(["Overview", "Overview"]);
    for (const link of active) {
      const colour = drawnTextColour(link);
      expect(shortfalls(colour, activeNavigationGrounds)).toEqual([]);
      const icon = link.querySelector("svg");
      expect(icon, "the active link's icon").not.toBeNull();
      expect(drawnTextColour(icon!), "the icon is drawn in the link's colour").toBe(colour);
    }
  });

  it("keeps the closed phone drawer inert and out of the accessibility tree", async () => {
    const { container } = render(<Sidebar />);
    await screen.findAllByRole("link", { current: "page", hidden: true });
    expect(screen.queryByRole("dialog")).toBeNull();
    const drawer = container.querySelector('aside[role="dialog"]');
    expect(drawer?.hasAttribute("inert")).toBe(true);
    expect(drawer?.getAttribute("aria-hidden")).toBe("true");
  });
});
