import { describe, it, expect, vi, afterEach, beforeEach } from "vitest";
import { render, screen, cleanup, waitFor } from "@testing-library/react";
import DashboardLayout from "./layout";
import { api } from "@/lib/api";

// The dashboard shell decides who sees a page from the session store only,
// the same store the API client sends. A cookie the browser happens to hold
// is not a session: it cannot open a page, and it cannot widen a role.

const router = vi.hoisted(() => ({ replace: vi.fn(), push: vi.fn(), prefetch: vi.fn() }));
vi.mock("next/navigation", () => ({
  useRouter: () => router,
  usePathname: () => "/dashboard/admin/users",
  useSearchParams: () => new URLSearchParams(),
}));
vi.mock("@/components/sidebar", () => ({ Sidebar: () => null }));
vi.mock("@/components/dashboard-header", () => ({ DashboardHeader: () => null }));
vi.mock("@/components/hub-tabs", () => ({ HubTabs: () => null }));
vi.mock("@/components/mobile-tab-bar", () => ({ MobileTabBar: () => null }));
vi.mock("@/components/idle-timeout-guard", () => ({ IdleTimeoutGuard: () => null }));
vi.mock("@/hooks/use-deactivation-check", () => ({ useDeactivationCheck: () => {} }));

const token = (payload: Record<string, unknown>) =>
  `header.${Buffer.from(JSON.stringify(payload)).toString("base64url")}.signature`;
const adminX = token({ user_id: "user-x", organization_id: "org-1", role: "admin", exp: 4102444800 });
const viewerY = token({ user_id: "user-y", organization_id: "org-1", role: "viewer", exp: 4102444800 });
const adminZ = token({ user_id: "user-z", organization_id: "org-1", role: "admin", exp: 4102444800 });

beforeEach(() => {
  api.clearToken();
  localStorage.clear();
  document.cookie = "access_token=; max-age=0; path=/";
  vi.stubGlobal(
    "fetch",
    vi.fn(async () =>
      new Response(JSON.stringify({ id: "user", role: "admin", email: "seat@example.test" }), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      })
    )
  );
});

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
  router.replace.mockReset();
  router.push.mockReset();
});

const child = () => screen.queryByTestId("page-body");

describe("the dashboard shell decides from the session store only", () => {
  it("an empty store is signed out, whatever cookie the browser holds", async () => {
    document.cookie = `access_token=${adminX}; path=/`;
    render(
      <DashboardLayout>
        <div data-testid="page-body">users</div>
      </DashboardLayout>
    );
    await waitFor(() => expect(router.replace).toHaveBeenCalledWith("/auth/login?returnUrl=%2Fdashboard%2Fadmin%2Fusers"));
    expect(child()).toBeNull();
  });

  it("a viewer in the store is kept out of an admin route even with an admin cookie", async () => {
    document.cookie = `access_token=${adminX}; path=/`;
    api.setToken(viewerY, "viewer.refresh.value", "new-session");
    render(
      <DashboardLayout>
        <div data-testid="page-body">users</div>
      </DashboardLayout>
    );
    await waitFor(() => expect(router.replace).toHaveBeenCalledWith("/dashboard/forbidden"));
    expect(child()).toBeNull();
  });

  it("an admin in the store reaches the admin route", async () => {
    api.setToken(adminZ, "admin.refresh.value", "new-session");
    render(
      <DashboardLayout>
        <div data-testid="page-body">users</div>
      </DashboardLayout>
    );
    await waitFor(() => expect(child()).not.toBeNull());
    expect(router.replace).not.toHaveBeenCalled();
  });
});
