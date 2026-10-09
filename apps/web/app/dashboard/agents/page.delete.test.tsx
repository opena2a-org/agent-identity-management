import { StrictMode } from "react";
import type { ReactNode } from "react";
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen, fireEvent, waitFor, cleanup, within } from "@testing-library/react";
import { api } from "@/lib/api";
import {
  AGENT_DELETE_IRREVERSIBLE_TEXT,
  AGENT_DELETE_KEPT_TEXT,
  agentDeleteRemovesText,
  agentDeletedNotice,
  rememberAgentDeleted,
} from "@/lib/agent-delete";
import AgentsPage from "./page";

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: vi.fn(), replace: vi.fn() }),
  useSearchParams: () => new URLSearchParams(),
}));
vi.mock("@/components/auth-guard", () => ({
  AuthGuard: ({ children }: { children: ReactNode }) => <>{children}</>,
}));
vi.mock("@/components/modals/register-agent-modal", () => ({ RegisterAgentModal: () => null }));
vi.mock("@/components/modals/agent-detail-modal", () => ({ AgentDetailModal: () => null }));
vi.mock("@/lib/api", () => ({
  api: {
    getToken: vi.fn(),
    listAgents: vi.fn(),
    deleteAgent: vi.fn(),
  },
}));

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

function requestError(message: string, status: number) {
  return Object.assign(new Error(message), { status });
}

async function openDeleteDialog() {
  render(<AgentsPage />);
  const deleteButtons = await screen.findAllByTitle("Delete agent");
  fireEvent.click(deleteButtons[0]);
  return screen.getByRole("alertdialog");
}

beforeEach(() => {
  sessionStorage.clear();
  vi.mocked(api.getToken).mockReturnValue(adminToken);
  vi.mocked(api.listAgents).mockResolvedValue({ agents: [agent] } as never);
  vi.mocked(api.deleteAgent).mockReset();
  vi.spyOn(console, "error").mockImplementation(() => {});
});

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});

describe("agents list: delete", () => {
  it("says what the delete removes, what is kept, and that it cannot be undone", async () => {
    const dialog = await openDeleteDialog();
    expect(dialog.textContent).toContain(agentDeleteRemovesText("Billing bot"));
    expect(dialog.textContent).toContain(AGENT_DELETE_KEPT_TEXT);
    expect(dialog.textContent).toContain(AGENT_DELETE_IRREVERSIBLE_TEXT);
  });

  it("keeps the dialog open with the reason when the delete fails, and keeps the agent", async () => {
    vi.mocked(api.deleteAgent).mockRejectedValue(requestError("The agent was not deleted and nothing was removed. Try again, and if it fails again, contact your administrator.", 500));
    const dialog = await openDeleteDialog();

    fireEvent.click(within(dialog).getByRole("button", { name: "Delete" }));

    expect((await within(dialog).findByRole("alert")).textContent).toContain(
      "AIM could not delete this agent."
    );
    expect(screen.getByRole("alertdialog")).toBe(dialog);
    expect(screen.getAllByTitle("Delete agent").length).toBeGreaterThan(0);
  });

  it("never shows the server's raw text for another failure", async () => {
    vi.mocked(api.deleteAgent).mockRejectedValue(
      requestError('pq: update or delete on table "agents" violates foreign key constraint', 500)
    );
    const dialog = await openDeleteDialog();

    fireEvent.click(within(dialog).getByRole("button", { name: "Delete" }));

    expect((await within(dialog).findByRole("alert")).textContent).toContain(
      "AIM could not delete this agent."
    );
    expect(document.body.textContent).not.toContain("foreign key");
  });

  it("removes the agent and states that it was deleted after a success", async () => {
    vi.mocked(api.deleteAgent).mockResolvedValue(undefined);
    const dialog = await openDeleteDialog();

    fireEvent.click(within(dialog).getByRole("button", { name: "Delete" }));

    await waitFor(() =>
      expect(screen.getByRole("status").textContent).toContain(agentDeletedNotice("Billing bot"))
    );
    expect(screen.queryByRole("alertdialog")).toBeNull();
    expect(screen.queryAllByTitle("Delete agent")).toHaveLength(0);
  });

  it("states a delete made from the agent page", async () => {
    rememberAgentDeleted("Billing bot");
    vi.mocked(api.listAgents).mockResolvedValue({ agents: [] } as never);
    render(<AgentsPage />);

    await waitFor(() =>
      expect(screen.getByRole("status").textContent).toContain(agentDeletedNotice("Billing bot"))
    );
  });

  // React runs mount effects twice under StrictMode (the dashboard's development build).
  // The second run finds the notice already taken and must not clear it.
  it("states a delete made from the agent page when its effects run twice", async () => {
    rememberAgentDeleted("Billing bot");
    vi.mocked(api.listAgents).mockResolvedValue({ agents: [] } as never);
    render(
      <StrictMode>
        <AgentsPage />
      </StrictMode>
    );

    await waitFor(() =>
      expect(screen.getByRole("status").textContent).toContain(agentDeletedNotice("Billing bot"))
    );
  });
});
