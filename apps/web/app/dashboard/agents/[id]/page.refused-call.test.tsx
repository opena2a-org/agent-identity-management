import { describe, it, expect, vi, afterEach, beforeEach } from "vitest";
import { render, screen, cleanup, within } from "@testing-library/react";
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
// Sections that fetch their own data; this test is about the activity timeline.
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
  },
}));

const AGENT_ID = "6f1c2a52-9a1e-4c39-9d0c-2b3f5f4c7a10";
// The reason the server records for a capability the agent was not granted, as the SDK prints it.
const REASON =
  "Capability violation blocked by security policy 'Capability Violation Detection': Agent does not have permission for capability 'db:write' (allowed: [db:read])";

const mocked = api as unknown as Record<string, ReturnType<typeof vi.fn>>;

function activityRow(metadata: Record<string, unknown>, id = "a1") {
  return {
    id,
    action: "db:write",
    resourceType: "agent_action",
    resourceId: AGENT_ID,
    timestamp: "2026-09-22T10:00:00Z",
    details: "",
    metadata,
  };
}

function mockAgentPage(activities: unknown[]) {
  mocked.getAgent.mockResolvedValue({
    id: AGENT_ID,
    organizationId: "org-1",
    name: "orders-agent",
    displayName: "Orders agent",
    description: "",
    agentType: "ai_agent",
    status: "verified",
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

describe("agent page activity timeline", () => {
  beforeEach(() => {
    vi.mocked(decodeJwtPayload).mockReturnValue({ role: "admin" });
  });

  afterEach(() => {
    cleanup();
    vi.clearAllMocks();
  });

  it("shows a refused call as what was refused, the server's reason, and the grant path", async () => {
    mockAgentPage([
      activityRow({
        actionType: "db:write",
        resource: "orders",
        riskLevel: "medium",
        autoApproved: false,
        denialReason: REASON,
      }),
    ]);
    await renderPage();

    const finding = await screen.findByLabelText("Refused call: db:write");
    const f = within(finding);
    expect(f.getByText("What")).toBeTruthy();
    expect(f.getByText("AIM refused db:write on orders for this agent.")).toBeTruthy();
    expect(f.getByText("Why")).toBeTruthy();
    expect(f.getByText(REASON)).toBeTruthy();
    expect(f.getByText("Fix")).toBeTruthy();
    expect(f.getByText(/^Grant db:write to this agent/)).toBeTruthy();
    expect(f.getByText(`POST /api/v1/agents/${AGENT_ID}/capabilities {"capabilityType":"db:write"}`)).toBeTruthy();
    expect(f.getByRole("link", { name: "Open Capability requests" }).getAttribute("href")).toBe(
      "/dashboard/admin/capability-requests"
    );
  });

  it("keeps the reason and the API grant for a member, without the administrator-only link", async () => {
    vi.mocked(decodeJwtPayload).mockReturnValue({ role: "member" });
    mockAgentPage([activityRow({ actionType: "db:write", autoApproved: false, denialReason: REASON })]);
    await renderPage();

    const finding = await screen.findByLabelText("Refused call: db:write");
    expect(within(finding).getByText(REASON)).toBeTruthy();
    expect(within(finding).getByText(/^POST \/api\/v1\/agents\//)).toBeTruthy();
    expect(within(finding).queryByRole("link")).toBeNull();
  });

  it("shows no refusal for an allowed call", async () => {
    mockAgentPage([activityRow({ actionType: "db:write", riskLevel: "low", autoApproved: true })]);
    await renderPage();

    await screen.findByText(/Auto-approved/);
    expect(screen.queryByLabelText(/^Refused call/)).toBeNull();
  });
});
