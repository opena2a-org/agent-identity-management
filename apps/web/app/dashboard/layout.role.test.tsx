import { describe, it, expect, vi, afterEach, beforeEach } from "vitest";
import { render, screen, cleanup, within, waitFor } from "@testing-library/react";
import DashboardLayout from "./layout";
import { api } from "@/lib/api";

// The bottom tab bar gets its role from the dashboard shell. /auth/me sits under the
// strictly rate-limited /auth prefix and every dashboard load calls it from several
// places, so a 429 there is ordinary. The shell then keeps the role from the session
// store (the one the route gate and the sidebar drawer already read) instead of
// dropping to the viewer tab set while the drawer shows the admin entries.

vi.mock("next/navigation", () => ({
  useRouter: () => ({ replace: vi.fn(), push: vi.fn(), prefetch: vi.fn() }),
  usePathname: () => "/dashboard/developers",
  useSearchParams: () => new URLSearchParams(),
}));
vi.mock("@/components/sidebar", () => ({ Sidebar: () => null }));
vi.mock("@/components/dashboard-header", () => ({ DashboardHeader: () => null }));
vi.mock("@/components/hub-tabs", () => ({ HubTabs: () => null }));
vi.mock("@/components/idle-timeout-guard", () => ({ IdleTimeoutGuard: () => null }));
vi.mock("@/hooks/use-deactivation-check", () => ({ useDeactivationCheck: () => {} }));

const token = (payload: Record<string, unknown>) =>
  `header.${Buffer.from(JSON.stringify(payload)).toString("base64url")}.signature`;
const admin = token({ user_id: "user-a", organization_id: "org-1", role: "admin", exp: 4102444800 });

const me = (status: number, body: Record<string, unknown>) =>
  vi.stubGlobal(
    "fetch",
    vi.fn(async () => new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } }))
  );

const tabNames = () =>
  within(screen.getByRole("navigation", { name: "Primary" }))
    .getAllByRole("link")
    .map((link) => link.textContent);

beforeEach(() => {
  api.clearToken();
  localStorage.clear();
  api.setToken(admin, "admin.refresh.value", "new-session");
});

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

const renderShell = () =>
  render(
    <DashboardLayout>
      <div data-testid="page-body">developers</div>
    </DashboardLayout>
  );

describe("the bottom tab bar shows the signed-in role's tabs", () => {
  it("keeps the admin tabs when /auth/me is rate limited", async () => {
    me(429, { error: "Rate limit exceeded. Please try again later." });
    renderShell();
    await waitFor(() => expect(screen.queryByTestId("page-body")).not.toBeNull());
    await waitFor(() => expect(fetch).toHaveBeenCalled());
    expect(tabNames()).toEqual(["Overview", "Agents", "Secure", "Security"]);
  });

  it("shows the admin tabs when /auth/me answers admin", async () => {
    me(200, { id: "user-a", role: "admin", email: "admin@example.test" });
    renderShell();
    await waitFor(() => expect(tabNames()).toEqual(["Overview", "Agents", "Secure", "Security"]));
  });

  it("follows /auth/me when the role changed after sign-in", async () => {
    me(200, { id: "user-a", role: "viewer", email: "admin@example.test" });
    renderShell();
    await waitFor(() => expect(tabNames()).toEqual(["Overview", "Agents", "Guide"]));
  });
});
