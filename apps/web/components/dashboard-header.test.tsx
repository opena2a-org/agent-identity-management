import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import type { ReactNode } from "react";
import { DashboardHeader } from "./dashboard-header";

const router = vi.hoisted(() => ({ replace: vi.fn(), push: vi.fn(), prefetch: vi.fn() }));
vi.mock("next/navigation", () => ({ useRouter: () => router }));
vi.mock("next/link", () => ({
  default: ({ href, children, ...rest }: { href: string; children: ReactNode }) => (
    <a href={href} {...rest}>
      {children}
    </a>
  ),
}));
vi.mock("@/components/sidebar", () => ({ AimLogo: () => null }));
// The profile as GET /api/v1/auth/me answers it, which names no sign-in provider.
vi.mock("@/lib/api", () => ({
  api: {
    getCurrentUser: vi.fn(async () => ({ email: "member@example.test", name: "Member", role: "member" })),
    getAlerts: vi.fn(async () => ({ criticalCount: 0, highCount: 0, unacknowledgedCount: 0 })),
    getToken: () => null,
    clearToken: vi.fn(),
    logout: vi.fn(async () => {}),
  },
}));

afterEach(() => {
  cleanup();
  vi.clearAllMocks();
});

describe("the header account menu", () => {
  it("offers every signed-in user the deletion of their account, on its own page", async () => {
    render(<DashboardHeader />);
    await screen.findByText("Member");
    fireEvent.click(screen.getByRole("button", { expanded: false }));

    const item = screen.getByRole("menuitem", { name: "Delete account" });
    fireEvent.click(item);

    expect(router.push).toHaveBeenCalledWith("/auth/delete-account");
    expect(screen.queryByRole("menu")).toBeNull();
  });
});
