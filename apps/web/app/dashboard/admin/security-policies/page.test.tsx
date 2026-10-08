import { describe, it, expect, vi, afterEach, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor, cleanup, within } from "@testing-library/react";
import type { ReactNode } from "react";
import SecurityPoliciesPage from "./page";

// An admin edits an MCP policy from its card: the Edit control opens the Edit MCP Policy
// dialog with the stored rules loaded, and saving sends the floor the admin typed as a
// percentage on the [0,1] scale the API stores. Before the Edit control existed the dialog
// had no way to open, so neither conversion ever ran.

vi.mock("next/navigation", () => ({
  useRouter: () => ({ replace: vi.fn(), push: vi.fn(), prefetch: vi.fn() }),
  usePathname: () => "/dashboard/admin/security-policies",
  useSearchParams: () => new URLSearchParams(),
}));
vi.mock("@/components/auth-guard", () => ({ AuthGuard: ({ children }: { children: ReactNode }) => <>{children}</> }));

const api = vi.hoisted(() => ({
  getToken: vi.fn(),
  getSecurityPolicies: vi.fn(),
  getEnforcementSettings: vi.fn(),
  createSecurityPolicy: vi.fn(),
  updateSecurityPolicy: vi.fn(),
}));
vi.mock("@/lib/api", async (importOriginal) => ({ ...(await importOriginal<object>()), api }));

const adminToken = `header.${btoa(JSON.stringify({ role: "admin" }))}.signature`;

const allowlistPolicy = {
  id: "policy-allowlist",
  name: "Trusted MCP Server Domains",
  description: "Allow MCP servers only from trusted domains",
  policyType: "mcp_allowlist",
  enforcementAction: "alert_only",
  severityThreshold: "high",
  rules: {
    allowedDomains: ["*.example.org", "localhost"],
    allowedNames: [],
    allowedCapabilities: ["tools"],
    requireVerified: true,
    minTrustScore: 0.5,
    minAttestations: 0,
  },
  appliesTo: "agent_type:mcp",
  isEnabled: false,
  priority: 100,
  createdAt: "2026-01-01T00:00:00Z",
  updatedAt: "2026-01-01T00:00:00Z",
};

beforeEach(() => {
  api.getToken.mockReturnValue(adminToken);
  api.getSecurityPolicies.mockResolvedValue([allowlistPolicy]);
  api.getEnforcementSettings.mockResolvedValue({ enforcementMode: "monitoring" });
  api.createSecurityPolicy.mockResolvedValue({});
  api.updateSecurityPolicy.mockResolvedValue({});
});

afterEach(() => {
  cleanup();
  vi.clearAllMocks();
});

// The rule inputs carry a visible label in the same wrapper but no label association.
function inputUnder(container: HTMLElement, label: string): HTMLInputElement {
  const input = within(container).getByText(label).parentElement?.querySelector("input");
  if (!input) throw new Error(`no input under label "${label}"`);
  return input;
}

async function openEditDialog(): Promise<HTMLElement> {
  render(<SecurityPoliciesPage />);
  fireEvent.click(await screen.findByRole("button", { name: `Edit ${allowlistPolicy.name}` }));
  return screen.findByRole("dialog", { name: "Edit MCP Policy" });
}

describe("Security policies page, MCP policy editing", () => {
  it("opens the Edit MCP Policy dialog from the policy card with the stored rules loaded", async () => {
    const dialog = await openEditDialog();
    expect(inputUnder(dialog, "Policy Name").value).toBe(allowlistPolicy.name);
    expect(inputUnder(dialog, "Allowed Domains (comma-separated)").value).toBe("*.example.org, localhost");
    // Stored as 0.5 on the [0,1] scale; the form edits a percentage.
    expect(inputUnder(dialog, "Min Trust Score (%)").value).toBe("50");
  });

  it("saves the typed percentage on the [0,1] scale and keeps the stored severity, scope and state", async () => {
    const dialog = await openEditDialog();
    fireEvent.change(inputUnder(dialog, "Min Trust Score (%)"), { target: { value: "70" } });
    fireEvent.click(within(dialog).getByRole("button", { name: "Update Policy" }));

    await waitFor(() => expect(api.updateSecurityPolicy).toHaveBeenCalledTimes(1));
    const [id, body] = api.updateSecurityPolicy.mock.calls[0];
    expect(id).toBe(allowlistPolicy.id);
    expect(body).toMatchObject({
      name: allowlistPolicy.name,
      policyType: "mcp_allowlist",
      enforcementAction: "alert_only",
      severityThreshold: "high",
      appliesTo: "agent_type:mcp",
      isEnabled: false,
      priority: 100,
    });
    expect(body.rules).toMatchObject({
      allowedDomains: ["*.example.org", "localhost"],
      allowedCapabilities: ["tools"],
      requireVerified: true,
      minTrustScore: 0.7,
    });
  });

  it("creates an MCP policy with the typed percentage on the [0,1] scale", async () => {
    render(<SecurityPoliciesPage />);
    fireEvent.click(await screen.findByRole("button", { name: "Create MCP Policy" }));
    const dialog = await screen.findByRole("dialog", { name: "Create MCP Policy" });
    fireEvent.change(inputUnder(dialog, "Policy Name"), { target: { value: "Floor at 70" } });
    fireEvent.change(inputUnder(dialog, "Min Trust Score (%)"), { target: { value: "70" } });
    fireEvent.click(within(dialog).getByRole("button", { name: "Create Policy" }));

    await waitFor(() => expect(api.createSecurityPolicy).toHaveBeenCalledTimes(1));
    expect(api.createSecurityPolicy.mock.calls[0][0].rules).toMatchObject({ minTrustScore: 0.7 });
  });
});
