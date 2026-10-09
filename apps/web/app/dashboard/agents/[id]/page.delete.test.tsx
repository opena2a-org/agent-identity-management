import type { ReactNode } from "react";
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen, fireEvent, waitFor, cleanup, within } from "@testing-library/react";
import { api } from "@/lib/api";
import {
  AGENT_DELETE_IRREVERSIBLE_TEXT,
  AGENT_DELETE_KEPT_TEXT,
  agentDeleteRemovesText,
  agentDeletedNotice,
  takeAgentDeletedNotice,
} from "@/lib/agent-delete";
import AgentDetailsPage from "./page";

const push = vi.fn();

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push, replace: vi.fn() }),
  useSearchParams: () => new URLSearchParams(),
}));
vi.mock("@/components/auth-guard", () => ({
  AuthGuard: ({ children }: { children: ReactNode }) => <>{children}</>,
}));
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
vi.mock("@/lib/api", () => {
  const unavailable = () => vi.fn().mockRejectedValue(new Error("not used by this test"));
  return {
    api: {
      getToken: vi.fn(),
      getAgent: vi.fn(),
      deleteAgent: vi.fn(),
      getTrustScoreBreakdown: unavailable(),
      listAgents: unavailable(),
      listMCPServers: unavailable(),
      getRecentVerificationEvents: unavailable(),
      getSingleAgentActivity: unavailable(),
      getDetectionStatus: unavailable(),
      getAgentAlerts: unavailable(),
      getAgentTrustScoreHistory: unavailable(),
      getAgentMCPServers: unavailable(),
    },
  };
});

const adminToken = `x.${btoa(JSON.stringify({ role: "admin" }))}.y`;

const agent = {
  id: "agent-1",
  organizationId: "org-1",
  name: "billing-bot",
  displayName: "Billing bot",
  description: "",
  agentType: "ai_agent",
  status: "verified",
  version: "1.0.0",
  trustScore: 0.8,
  createdAt: "2026-10-01T00:00:00Z",
  updatedAt: "2026-10-01T00:00:00Z",
};

async function openDeleteDialog() {
  render(<AgentDetailsPage params={Promise.resolve({ id: "agent-1" })} />);
  fireEvent.click(await screen.findByRole("button", { name: "Delete" }));
  return screen.getByRole("alertdialog");
}

beforeEach(() => {
  sessionStorage.clear();
  push.mockReset();
  vi.mocked(api.getToken).mockReturnValue(adminToken);
  vi.mocked(api.getAgent).mockResolvedValue(agent as never);
  vi.mocked(api.deleteAgent).mockReset();
  vi.spyOn(console, "error").mockImplementation(() => {});
});

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});

describe("agent page: delete", () => {
  it("renders the same dialog text as the agents list", async () => {
    const dialog = await openDeleteDialog();
    expect(dialog.textContent).toContain(agentDeleteRemovesText("Billing bot"));
    expect(dialog.textContent).toContain(AGENT_DELETE_KEPT_TEXT);
    expect(dialog.textContent).toContain(AGENT_DELETE_IRREVERSIBLE_TEXT);
  });

  it("keeps the dialog open with the reason inside it, never a browser alert", async () => {
    vi.mocked(api.deleteAgent).mockRejectedValue(
      Object.assign(new Error("The agent was not deleted and nothing was removed. Try again, and if it fails again, contact your administrator."), { status: 500 })
    );
    const alertSpy = vi.spyOn(window, "alert").mockImplementation(() => {});
    const dialog = await openDeleteDialog();

    fireEvent.click(within(dialog).getByRole("button", { name: "Delete" }));

    expect((await within(dialog).findByRole("alert")).textContent).toContain(
      "AIM could not delete this agent."
    );
    expect(screen.getByRole("alertdialog")).toBe(dialog);
    expect(alertSpy).not.toHaveBeenCalled();
    expect(push).not.toHaveBeenCalledWith("/dashboard/agents");
  });

  it("goes to the agents list, which states the deletion, after a success", async () => {
    vi.mocked(api.deleteAgent).mockResolvedValue(undefined);
    const dialog = await openDeleteDialog();

    fireEvent.click(within(dialog).getByRole("button", { name: "Delete" }));

    await waitFor(() => expect(push).toHaveBeenCalledWith("/dashboard/agents"));
    expect(takeAgentDeletedNotice()).toBe(agentDeletedNotice("Billing bot"));
  });
});
