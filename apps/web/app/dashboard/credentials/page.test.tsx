import { describe, it, expect, vi, afterEach, beforeEach } from "vitest";
import { render, screen, cleanup, within, waitFor } from "@testing-library/react";
import CredentialsPage from "./page";

// The Credentials page explains both kinds side by side, then lists each kind in its own
// section through the same API calls the two pages it replaces made. It renders behind the
// real AuthGuard, so a signed-out visitor never reaches those calls.

const nav = vi.hoisted(() => ({
  router: { replace: vi.fn(), push: vi.fn(), prefetch: vi.fn() },
  search: new URLSearchParams(),
}));
vi.mock("next/navigation", () => ({
  useRouter: () => nav.router,
  usePathname: () => "/dashboard/credentials",
  useSearchParams: () => nav.search,
}));

const api = vi.hoisted(() => ({
  getToken: vi.fn(),
  listAPIKeys: vi.fn(),
  listAgents: vi.fn(),
  listSDKTokens: vi.fn(),
}));
vi.mock("@/lib/api", async (importOriginal) => ({ ...(await importOriginal<object>()), api }));

// A JWT-shaped session token; only its payload is read.
const SESSION = `e30.${btoa(JSON.stringify({ role: "admin" }))}.sig`;
const CREATED = "2026-01-01T00:00:00Z";
const EXPIRES = "2099-01-01T00:00:00Z";

function apiKey(id: string, name: string) {
  return { id, agentId: "agent-1", name, prefix: `aim_${id}`, isActive: true, createdAt: CREATED, expiresAt: EXPIRES };
}

function sdkToken(id: string, deviceName: string) {
  return { id, tokenId: `jti-${id}`, deviceName, usageCount: 0, createdAt: CREATED, expiresAt: EXPIRES };
}

const scrollIntoView = vi.fn();

beforeEach(() => {
  nav.search = new URLSearchParams();
  api.getToken.mockReturnValue(SESSION);
  api.listAPIKeys.mockResolvedValue({ apiKeys: [] });
  api.listAgents.mockResolvedValue({ agents: [] });
  api.listSDKTokens.mockResolvedValue({ tokens: [] });
  Element.prototype.scrollIntoView = scrollIntoView;
});

afterEach(() => {
  cleanup();
  vi.clearAllMocks();
  window.history.replaceState(null, "", "/");
});

describe("Credentials page", () => {
  it("explains both kinds side by side, each linking to its section", () => {
    render(<CredentialsPage />);
    expect(screen.getByRole("heading", { level: 1, name: "Credentials" })).toBeTruthy();
    expect(screen.getByRole("link", { name: "API keys" }).getAttribute("href")).toBe("#api-keys");
    expect(screen.getByRole("link", { name: "SDK tokens" }).getAttribute("href")).toBe("#sdk-tokens");
    expect(screen.getByText(/An API key belongs to one agent/)).toBeTruthy();
    expect(screen.getByText(/An SDK token is issued with each SDK download/)).toBeTruthy();
  });

  it("lists each kind in its own section, through the existing API calls", async () => {
    render(<CredentialsPage />);
    const keys = await screen.findByRole("region", { name: "API keys" });
    const tokens = await screen.findByRole("region", { name: "SDK tokens" });
    expect(keys.id).toBe("api-keys");
    expect(tokens.id).toBe("sdk-tokens");
    expect(await within(keys).findByText("No API keys found")).toBeTruthy();
    expect(await within(tokens).findByText("No SDK tokens found")).toBeTruthy();
    expect(api.listAPIKeys).toHaveBeenCalledTimes(1);
    expect(api.listAgents).toHaveBeenCalledTimes(1);
    expect(api.listSDKTokens).toHaveBeenCalledWith(true);
  });

  it("sends a signed-out visitor to sign-in without rendering the page or calling the API", async () => {
    api.getToken.mockReturnValue(null);
    render(<CredentialsPage />);
    await waitFor(() =>
      expect(nav.router.replace).toHaveBeenCalledWith("/auth/login?returnUrl=%2Fdashboard%2Fcredentials"),
    );
    expect(screen.queryByRole("heading", { name: "Credentials" })).toBeNull();
    expect(api.listAPIKeys).not.toHaveBeenCalled();
    expect(api.listAgents).not.toHaveBeenCalled();
    expect(api.listSDKTokens).not.toHaveBeenCalled();
  });

  it("scrolls to the section a link names once both sections have loaded", async () => {
    api.listAPIKeys.mockResolvedValue({ apiKeys: [apiKey("key-1", "Key one"), apiKey("key-2", "Key two")] });
    const listed: boolean[] = [];
    scrollIntoView.mockImplementation(() => {
      listed.push(screen.queryByText("Key two") !== null && screen.queryByText("No SDK tokens found") !== null);
    });
    window.history.replaceState(null, "", "/dashboard/credentials#sdk-tokens");

    render(<CredentialsPage />);

    await waitFor(() => expect(scrollIntoView).toHaveBeenCalledTimes(1));
    expect(scrollIntoView.mock.contexts[0]).toBe(document.getElementById("sdk-tokens"));
    expect(listed).toEqual([true]);
  });

  it("leaves the scroll position alone when the address names no section", async () => {
    window.history.replaceState(null, "", "/dashboard/credentials#elsewhere");
    render(<CredentialsPage />);
    expect(await screen.findByText("No SDK tokens found")).toBeTruthy();
    expect(await screen.findByText("No API keys found")).toBeTruthy();
    expect(scrollIntoView).not.toHaveBeenCalled();
  });

  it("marks the API key the highlight parameter names", async () => {
    nav.search = new URLSearchParams("highlight=key-2");
    api.listAPIKeys.mockResolvedValue({ apiKeys: [apiKey("key-1", "Key one"), apiKey("key-2", "Key two")] });
    render(<CredentialsPage />);
    const named = (await screen.findByText("Key two")).closest("tr");
    const other = screen.getByText("Key one").closest("tr");
    expect(named?.getAttribute("aria-current")).toBe("true");
    expect(other?.hasAttribute("aria-current")).toBe(false);
  });

  it("marks the SDK token the highlight parameter names", async () => {
    nav.search = new URLSearchParams("highlight=tok-2");
    api.listSDKTokens.mockResolvedValue({ tokens: [sdkToken("tok-1", "Laptop"), sdkToken("tok-2", "Build server")] });
    render(<CredentialsPage />);
    const tokens = await screen.findByRole("region", { name: "SDK tokens" });
    const marked = await waitFor(() => {
      const found = tokens.querySelectorAll('[aria-current="true"]');
      expect(found).toHaveLength(1);
      return found[0];
    });
    expect(within(marked as HTMLElement).getByText("Build server")).toBeTruthy();
    expect(within(marked as HTMLElement).queryByText("Laptop")).toBeNull();
  });
});
