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

// A trust_score_low policy triggers below rules.trust_threshold on the [0,1] scale of a
// trust score, and below 0.3 when the rules carry no numeric trust_threshold. The card
// showed "Rules: 3 configured" for a policy evaluated at 0.5 and for one evaluated at the
// default, so an admin could not read from it which score either applies.

function agentPolicy(name: string, policyType: string, rules: Record<string, unknown>) {
  return {
    id: `policy-${name}`,
    name,
    description: "",
    policyType,
    enforcementAction: "block_and_alert",
    severityThreshold: "critical",
    rules,
    appliesTo: "all",
    isEnabled: true,
    priority: 300,
    createdAt: "2026-01-01T00:00:00Z",
    updatedAt: "2026-01-01T00:00:00Z",
  };
}

// The text of each "Trust score below:" line, in the order the cards are listed.
async function trustThresholdLines(firstPolicyName: string): Promise<(string | null | undefined)[]> {
  render(<SecurityPoliciesPage />);
  await screen.findByRole("heading", { name: firstPolicyName });
  return screen.queryAllByText("Trust score below:").map((label) => label.parentElement?.textContent);
}

describe("Security policies page, trust score policy card", () => {
  it("shows the trust score each trust_score_low policy triggers below", async () => {
    api.getSecurityPolicies.mockResolvedValue([
      agentPolicy("Critical Trust Score Block", "trust_score_low", {
        trust_threshold: 0.5,
        auto_disable: true,
        notify: ["admin", "security"],
      }),
      agentPolicy("Low Trust Score Alert", "trust_score_low", {
        trust_threshold: 0.7,
        duration: "1h",
        notify: ["admin"],
      }),
    ]);
    expect(await trustThresholdLines("Critical Trust Score Block")).toEqual([
      "Trust score below: 50%",
      "Trust score below: 70%",
    ]);
  });

  it("shows the default when the rules carry the threshold under a key the evaluator does not read", async () => {
    api.getSecurityPolicies.mockResolvedValue([
      agentPolicy("Unread key", "trust_score_low", { threshold: 50, auto_disable: true, notify: ["admin"] }),
    ]);
    expect(await trustThresholdLines("Unread key")).toEqual([
      "Trust score below: 30% (default: these rules set no numeric trust_threshold)",
    ]);
  });

  it("says when a stored percent puts every score below the threshold", async () => {
    api.getSecurityPolicies.mockResolvedValue([
      agentPolicy("Percent", "trust_score_low", { trust_threshold: 70 }),
    ]);
    expect(await trustThresholdLines("Percent")).toEqual([
      "Trust score below: 7000% (trust_threshold 70 is above the 0-1 scale: every score is below it)",
    ]);
  });

  it("shows no trust score line on a policy of another type", async () => {
    api.getSecurityPolicies.mockResolvedValue([
      agentPolicy("Block Capability Violations", "capability_violation", { trust_threshold: 0.5 }),
      allowlistPolicy,
    ]);
    expect(await trustThresholdLines("Block Capability Violations")).toEqual([]);
  });
});

// On a 375px screen the page ran 83px past the screen's edge: each policy card kept its
// 180px Enforcement Mode column beside the text, 24px off it, and the global enforcement
// row kept its Monitoring switch beside the description. Below the sm breakpoint a card
// stacks the column under its text, and the page's rows wrap.
describe("Security policies page on a narrow screen", () => {
  it("stacks each policy card's enforcement controls under its text below sm", async () => {
    api.getSecurityPolicies.mockResolvedValue([
      agentPolicy("Block Capability Violations", "capability_violation", {}),
      allowlistPolicy,
    ]);
    render(<SecurityPoliciesPage />);
    await screen.findByText("Block Capability Violations");
    const columns = screen.getAllByText("Enforcement Mode").map((label) => label.parentElement?.parentElement);
    expect(columns).toHaveLength(2);
    for (const column of columns) {
      const card = column?.parentElement;
      expect(card?.classList.contains("flex-col")).toBe(true);
      expect(card?.classList.contains("sm:flex-row")).toBe(true);
      expect(column?.classList.contains("ml-6")).toBe(false);
      expect(column?.classList.contains("sm:ml-6")).toBe(true);
    }
  });

  it("wraps the header and the global enforcement row", async () => {
    render(<SecurityPoliciesPage />);
    const title = await screen.findByRole("heading", { level: 1, name: /Security Policies/ });
    expect(title.parentElement?.parentElement?.classList.contains("flex-wrap")).toBe(true);
    const enforcement = screen.getByRole("heading", { name: "Global Enforcement Mode" });
    expect(enforcement.closest(".justify-between")?.classList.contains("flex-wrap")).toBe(true);
  });
});
