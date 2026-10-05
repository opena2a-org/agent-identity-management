import { describe, it, expect, vi, afterEach, beforeEach } from "vitest";
import { render, screen, cleanup, within } from "@testing-library/react";
import type { ReactNode } from "react";
import CredentialsPage from "./page";

// The Credentials page explains both kinds side by side, then lists each kind in its own
// section through the same API calls the two pages it replaces made.

vi.mock("next/navigation", () => ({
  useRouter: () => ({ replace: vi.fn(), push: vi.fn(), prefetch: vi.fn() }),
  usePathname: () => "/dashboard/credentials",
  useSearchParams: () => new URLSearchParams(),
}));
vi.mock("@/components/auth-guard", () => ({ AuthGuard: ({ children }: { children: ReactNode }) => <>{children}</> }));

const api = vi.hoisted(() => ({
  getToken: vi.fn(),
  listAPIKeys: vi.fn(),
  listAgents: vi.fn(),
  listSDKTokens: vi.fn(),
}));
vi.mock("@/lib/api", async (importOriginal) => ({ ...(await importOriginal<object>()), api }));

beforeEach(() => {
  api.getToken.mockReturnValue(null);
  api.listAPIKeys.mockResolvedValue({ apiKeys: [] });
  api.listAgents.mockResolvedValue({ agents: [] });
  api.listSDKTokens.mockResolvedValue({ tokens: [] });
});

afterEach(() => {
  cleanup();
  vi.clearAllMocks();
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
});
