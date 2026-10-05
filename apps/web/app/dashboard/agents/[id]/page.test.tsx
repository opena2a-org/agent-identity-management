import { describe, it, expect, vi, afterEach, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor, cleanup, within } from "@testing-library/react";
import type { ReactNode } from "react";
import AgentDetailsPage from "./page";
import type { Agent } from "@/lib/api";

// The agent header keeps a state and an action apart: a verified agent shows a status
// badge, an agent that can still be verified shows a button. A disabled button is only
// ever an action that is unavailable for a moment, never the way a state is displayed.

vi.mock("next/navigation", () => ({
  useRouter: () => ({ replace: vi.fn(), push: vi.fn(), prefetch: vi.fn() }),
  usePathname: () => "/dashboard/agents/7b1d2c7e-58a4-4f0e-9a51-0c2f6d1e8a11",
  useSearchParams: () => new URLSearchParams(),
}));
vi.mock("@/components/auth-guard", () => ({ AuthGuard: ({ children }: { children: ReactNode }) => <>{children}</> }));

// These load their own data and have their own tests; the header under test is the page's.
vi.mock("@/components/agents/auto-detect-button", () => ({ AutoDetectButton: () => null }));
vi.mock("@/components/agents/mcp-server-selector", () => ({ MCPServerSelector: () => null }));
vi.mock("@/components/agent/first-run-panel", () => ({ FirstRunPanel: () => null }));
vi.mock("@/components/agent/drift-score-card", () => ({ DriftScoreCard: () => null }));
vi.mock("@/components/agent/trust-score-breakdown", () => ({ TrustScoreBreakdown: () => null }));
vi.mock("@/components/modals/register-agent-modal", () => ({ RegisterAgentModal: () => null }));

const api = vi.hoisted(() => ({
  getToken: vi.fn(),
  getAgent: vi.fn(),
  verifyAgent: vi.fn(),
  getTrustScoreBreakdown: vi.fn(),
  listAgents: vi.fn(),
  listMCPServers: vi.fn(),
  getRecentVerificationEvents: vi.fn(),
  getSingleAgentActivity: vi.fn(),
  getDetectionStatus: vi.fn(),
  getAgentAlerts: vi.fn(),
  getAgentTrustScoreHistory: vi.fn(),
  getAgentMCPServers: vi.fn(),
}));
vi.mock("@/lib/api", async (importOriginal) => ({ ...(await importOriginal<object>()), api }));

const AGENT_ID = "7b1d2c7e-58a4-4f0e-9a51-0c2f6d1e8a11";

const agent = (overrides: Partial<Agent> = {}): Agent =>
  ({
    id: AGENT_ID,
    organizationId: "0d6f1a52-3c1b-4a7e-8f47-2b9c5e7d1a90",
    name: "billing-bot",
    displayName: "Billing Bot",
    description: "Reconciles invoices",
    agentType: "ai_agent",
    status: "pending",
    version: "1.0.0",
    lastActive: "2026-10-01T09:30:00Z",
    trustScore: 0.82,
    talksTo: [],
    capabilities: [],
    createdAt: "2026-09-28T12:00:00Z",
    updatedAt: "2026-10-01T09:30:00Z",
    ...overrides,
  }) as Agent;

// The page reads the signed-in role from the session token's payload.
const sessionFor = (role: string) => ["header", btoa(JSON.stringify({ role })), "signature"].join(".");

const STATUS_LABEL = /^(verified|unverified|pending|suspended|revoked|active|inactive)$/i;

// Every disabled button on the page whose whole label is a status word.
const disabledStatusButtons = (root: HTMLElement) =>
  Array.from(root.querySelectorAll<HTMLElement>("button[disabled], [role='button'][aria-disabled='true']"))
    .map((el) => (el.textContent ?? "").trim())
    .filter((label) => STATUS_LABEL.test(label));

// The row of header actions is the one holding the Delete button.
const headerActions = async () => {
  const row = (await screen.findByRole("button", { name: "Delete" })).parentElement;
  if (!row) throw new Error("the header action row did not render");
  return row;
};

const renderPage = (current: Agent, role = "admin") => {
  api.getToken.mockReturnValue(sessionFor(role));
  api.getAgent.mockResolvedValue(current);
  return render(<AgentDetailsPage params={Promise.resolve({ id: AGENT_ID })} />);
};

beforeEach(() => {
  api.getTrustScoreBreakdown.mockResolvedValue({});
  api.listAgents.mockResolvedValue({ agents: [] });
  api.listMCPServers.mockResolvedValue({ mcpServers: [] });
  api.getRecentVerificationEvents.mockResolvedValue({ events: [] });
  api.getSingleAgentActivity.mockResolvedValue({ activities: [] });
  api.getDetectionStatus.mockResolvedValue({ detectedMCPs: [] });
  api.getAgentAlerts.mockResolvedValue({ alerts: [] });
  api.getAgentTrustScoreHistory.mockResolvedValue({ history: [] });
  api.getAgentMCPServers.mockResolvedValue({ mcpServers: [] });
});

afterEach(() => {
  cleanup();
  vi.clearAllMocks();
});

describe("Agent page header, verification", () => {
  it("shows a verified agent's state as a status badge, not as a disabled button", async () => {
    const { container } = renderPage(agent({ status: "verified" }));
    const actions = await headerActions();

    const status = within(actions).getByText("Verified");
    // Not a control: no button, link or tab stop at or above the label.
    expect(status.closest("button, a, [role='button'], [tabindex]")).toBeNull();
    expect(within(actions).queryByRole("button", { name: /verif/i })).toBeNull();
    // The success tokens are the pairing the badge is read in, in both themes.
    expect(status.className).toContain("bg-success-fill");
    expect(status.className).toContain("text-success-text");

    fireEvent.click(status);
    expect(api.verifyAgent).not.toHaveBeenCalled();
    expect(disabledStatusButtons(container)).toEqual([]);
  });

  it("offers an agent that is not verified yet an enabled action", async () => {
    api.verifyAgent.mockResolvedValue({});
    const { container } = renderPage(agent({ status: "pending" }));
    const actions = await headerActions();

    expect(within(actions).queryByText("Verified")).toBeNull();
    const verify = within(actions).getByRole("button", { name: "Verify agent" }) as HTMLButtonElement;
    expect(verify.disabled).toBe(false);
    expect(disabledStatusButtons(container)).toEqual([]);

    fireEvent.click(verify);
    await waitFor(() => expect(api.verifyAgent).toHaveBeenCalledWith(AGENT_ID));
  });

  it("disables the action only while it runs, labelled with what is happening", async () => {
    api.verifyAgent.mockReturnValue(new Promise(() => {}));
    const { container } = renderPage(agent({ status: "pending" }));
    const actions = await headerActions();

    fireEvent.click(within(actions).getByRole("button", { name: "Verify agent" }));
    const busy = (await within(actions).findByRole("button", { name: "Verifying..." })) as HTMLButtonElement;
    expect(busy.disabled).toBe(true);
    expect(disabledStatusButtons(container)).toEqual([]);
  });
});
