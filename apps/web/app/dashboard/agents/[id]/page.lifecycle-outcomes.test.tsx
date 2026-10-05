import { describe, it, expect, vi, afterEach, beforeEach } from "vitest";
import { render, screen, cleanup, within, fireEvent } from "@testing-library/react";
import type { ReactNode } from "react";
import AgentDetailsPage from "./page";
import { api } from "@/lib/api";
import { decodeJwtPayload } from "@/lib/jwt-payload";

const searchParams = new URLSearchParams();
vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: vi.fn(), replace: vi.fn(), back: vi.fn() }),
  useSearchParams: () => searchParams,
}));
vi.mock("next/link", () => ({
  default: ({ href, children, ...rest }: { href: string; children: ReactNode }) => (
    <a href={href} {...rest}>
      {children}
    </a>
  ),
}));
vi.mock("@/components/auth-guard", () => ({ AuthGuard: ({ children }: { children: ReactNode }) => <>{children}</> }));
// Sections that fetch their own data; this test is about the lifecycle action outcomes.
vi.mock("@/components/agents/auto-detect-button", () => ({ AutoDetectButton: () => null }));
vi.mock("@/components/agents/mcp-server-selector", () => ({ MCPServerSelector: () => null }));
vi.mock("@/components/agents/mcp-server-list", () => ({ MCPServerList: () => null }));
vi.mock("@/components/agents/agent-capabilities", () => ({ AgentCapabilities: () => null }));
vi.mock("@/components/modals/register-agent-modal", () => ({ RegisterAgentModal: () => null }));
vi.mock("@/components/agent/violations-tab", () => ({ ViolationsTab: () => null }));
vi.mock("@/components/agent/key-vault-tab", () => ({ KeyVaultTab: () => null }));
vi.mock("@/components/agent/api-keys-tab", () => ({ APIKeysTab: () => null }));
vi.mock("@/components/agent/trust-score-breakdown", () => ({ TrustScoreBreakdown: () => null }));
vi.mock("@/components/agent/drift-score-card", () => ({ DriftScoreCard: () => null }));
vi.mock("@/components/agent/tags-tab", () => ({ AgentTagsTab: () => null }));
vi.mock("@/lib/jwt-payload", () => ({ decodeJwtPayload: vi.fn() }));
vi.mock("@/lib/api", () => ({
  api: {
    getToken: vi.fn(() => "token"),
    getAgent: vi.fn(),
    getTrustScoreBreakdown: vi.fn(),
    listAgents: vi.fn(),
    listMCPServers: vi.fn(),
    getRecentVerificationEvents: vi.fn(),
    getSingleAgentActivity: vi.fn(),
    getDetectionStatus: vi.fn(),
    getAgentAlerts: vi.fn(),
    getAgentTrustScoreHistory: vi.fn(),
    getAgentMCPServers: vi.fn(),
    verifyAgent: vi.fn(),
    deleteAgent: vi.fn(),
    suspendAgent: vi.fn(),
    reactivateAgent: vi.fn(),
  },
}));

const AGENT_ID = "6f1c2a52-9a1e-4c39-9d0c-2b3f5f4c7a10";

const mocked = api as unknown as Record<string, ReturnType<typeof vi.fn>>;

function mockAgentPage(activities: unknown[], status = "verified") {
  mocked.getAgent.mockResolvedValue({
    id: AGENT_ID,
    organizationId: "org-1",
    name: "orders-agent",
    displayName: "Orders agent",
    description: "",
    agentType: "ai_agent",
    status,
    version: "1.0.0",
    trustScore: 0.8,
    talksTo: [],
    capabilities: [],
    createdAt: "2026-09-01T00:00:00Z",
    updatedAt: "2026-09-01T00:00:00Z",
  });
  mocked.getTrustScoreBreakdown.mockRejectedValue(new Error("not needed"));
  mocked.listAgents.mockResolvedValue({ agents: [] });
  mocked.listMCPServers.mockResolvedValue({ mcpServers: [] });
  mocked.getRecentVerificationEvents.mockResolvedValue({ events: [] });
  mocked.getSingleAgentActivity.mockResolvedValue({ activities, total: activities.length });
  mocked.getDetectionStatus.mockResolvedValue({ detectedMCPs: [] });
  mocked.getAgentAlerts.mockResolvedValue({ alerts: [] });
  mocked.getAgentTrustScoreHistory.mockResolvedValue({ history: [] });
  mocked.getAgentMCPServers.mockResolvedValue({ mcpServers: [] });
}

async function renderPage() {
  render(<AgentDetailsPage params={Promise.resolve({ id: AGENT_ID })} />);
  await screen.findByText("Activity Timeline");
}

// What the API client throws for a non-2xx response: the server's text and its status.
function requestError(message: string, status: number) {
  return Object.assign(new Error(message), { status });
}

describe("agent page lifecycle outcomes", () => {
  let browserAlert: ReturnType<typeof vi.spyOn>;

  beforeEach(() => {
    vi.mocked(decodeJwtPayload).mockReturnValue({ role: "admin" });
    browserAlert = vi.spyOn(window, "alert").mockImplementation(() => {});
  });

  afterEach(() => {
    cleanup();
    vi.clearAllMocks();
    browserAlert.mockRestore();
  });

  it("reports a failed delete beside the actions as a reason and a next step, without the server's text", async () => {
    mockAgentPage([]);
    mocked.deleteAgent.mockRejectedValue(
      requestError('pq: update or delete on table "agents" violates foreign key constraint "x_agent_id_fkey"', 500)
    );
    await renderPage();

    fireEvent.click(screen.getByRole("button", { name: "Delete" }));
    fireEvent.click(await screen.findByRole("button", { name: "Delete" }));

    const outcome = await screen.findByRole("alert");
    expect(within(outcome).getByText("AIM could not delete this agent.")).toBeTruthy();
    expect(
      within(outcome).getByText(
        "Try again. If it fails again, an administrator can find the cause in the AIM server log."
      )
    ).toBeTruthy();
    expect(screen.queryByText(/foreign key/)).toBeNull();
    expect(browserAlert).not.toHaveBeenCalled();
  });

  it("names the role when verifying is refused, and can be dismissed", async () => {
    mockAgentPage([], "pending");
    mocked.verifyAgent.mockRejectedValue(requestError("Insufficient permissions", 403));
    await renderPage();

    fireEvent.click(screen.getByRole("button", { name: "Verify agent" }));

    const outcome = await screen.findByRole("alert");
    expect(within(outcome).getByText("Your role does not allow you to verify this agent.")).toBeTruthy();
    fireEvent.click(within(outcome).getByRole("button", { name: "Dismiss" }));
    expect(screen.queryByRole("alert")).toBeNull();
    expect(browserAlert).not.toHaveBeenCalled();
  });

  it("links back to the list when the agent no longer exists", async () => {
    mockAgentPage([], "suspended");
    mocked.reactivateAgent.mockRejectedValue(requestError("not found", 404));
    await renderPage();

    fireEvent.click(screen.getByRole("button", { name: "Reactivate" }));

    const outcome = await screen.findByRole("alert");
    expect(within(outcome).getByText("This agent no longer exists in your organization.")).toBeTruthy();
    expect(within(outcome).getByRole("link", { name: "Back to agents" }).getAttribute("href")).toBe(
      "/dashboard/agents"
    );
    expect(browserAlert).not.toHaveBeenCalled();
  });

  it("reports a suspend that succeeded as a status beside the actions", async () => {
    mockAgentPage([]);
    mocked.suspendAgent.mockResolvedValue({ success: true, message: "Agent suspended successfully" });
    await renderPage();

    fireEvent.click(screen.getByRole("button", { name: "Suspend" }));
    fireEvent.click(await screen.findByRole("button", { name: "Suspend" }));

    const outcome = await screen.findByRole("status");
    expect(
      within(outcome).getByText("Agent suspended. It cannot authenticate or act until you reactivate it.")
    ).toBeTruthy();
    expect(browserAlert).not.toHaveBeenCalled();
  });
});
