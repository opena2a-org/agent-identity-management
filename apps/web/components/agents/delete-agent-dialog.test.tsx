import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen, fireEvent, waitFor, cleanup } from "@testing-library/react";
import { DeleteAgentDialog } from "./delete-agent-dialog";
import { api } from "@/lib/api";
import {
  AGENT_DELETE_IRREVERSIBLE_TEXT,
  AGENT_DELETE_KEPT_TEXT,
  agentDeleteRemovesText,
} from "@/lib/agent-delete";

vi.mock("@/lib/api", () => ({
  api: { deleteAgent: vi.fn() },
}));

const agent = { id: "agent-1", name: "billing-bot" };

function requestError(message: string, status: number) {
  return Object.assign(new Error(message), { status });
}

function renderDialog() {
  const onOpenChange = vi.fn();
  const onDeleted = vi.fn();
  render(
    <DeleteAgentDialog agent={agent} open onOpenChange={onOpenChange} onDeleted={onDeleted} />
  );
  return { onOpenChange, onDeleted };
}

beforeEach(() => {
  vi.mocked(api.deleteAgent).mockReset();
  vi.spyOn(console, "error").mockImplementation(() => {});
});

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});

describe("DeleteAgentDialog", () => {
  it("says what the delete removes, what is kept, and that it cannot be undone", () => {
    renderDialog();
    const dialog = screen.getByRole("alertdialog");
    expect(dialog.textContent).toContain(agentDeleteRemovesText("billing-bot"));
    expect(dialog.textContent).toContain(AGENT_DELETE_KEPT_TEXT);
    expect(dialog.textContent).toContain(AGENT_DELETE_IRREVERSIBLE_TEXT);
  });

  it("keeps the dialog open and states the reason and next step inside it when the delete fails", async () => {
    vi.mocked(api.deleteAgent).mockRejectedValue(
      requestError(
        "The agent was not deleted and nothing was removed. Try again, and if it fails again, contact your administrator.",
        500
      )
    );
    const alertSpy = vi.spyOn(window, "alert").mockImplementation(() => {});
    const { onOpenChange, onDeleted } = renderDialog();

    fireEvent.click(screen.getByRole("button", { name: "Delete" }));

    const dialog = screen.getByRole("alertdialog");
    const failure = await screen.findByRole("alert");
    expect(failure.textContent).toContain("AIM could not delete this agent.");
    expect(failure.textContent).toContain(
      "Try again. If it fails again, an administrator can find the cause in the AIM server log."
    );
    expect(dialog.contains(failure)).toBe(true);
    expect(dialog.contains(document.activeElement)).toBe(true);
    expect(screen.getByRole("button", { name: "Delete" })).toBeTruthy();
    expect(onOpenChange).not.toHaveBeenCalledWith(false);
    expect(onDeleted).not.toHaveBeenCalled();
    expect(alertSpy).not.toHaveBeenCalled();
  });

  it("never shows the server's text", async () => {
    vi.mocked(api.deleteAgent).mockRejectedValue(
      requestError('pq: update or delete on table "agents" violates foreign key constraint', 500)
    );
    renderDialog();

    fireEvent.click(screen.getByRole("button", { name: "Delete" }));

    expect((await screen.findByRole("alert")).textContent).toContain(
      "AIM could not delete this agent."
    );
    expect(screen.getByRole("alertdialog").textContent).not.toContain("foreign key");
  });

  it("names the role when the delete is refused", async () => {
    vi.mocked(api.deleteAgent).mockRejectedValue(requestError("Insufficient permissions", 403));
    renderDialog();

    fireEvent.click(screen.getByRole("button", { name: "Delete" }));

    expect((await screen.findByRole("alert")).textContent).toContain(
      "Your role does not allow you to delete this agent."
    );
  });

  it("hands the deleted agent to the caller after a success", async () => {
    vi.mocked(api.deleteAgent).mockResolvedValue(undefined);
    const { onDeleted } = renderDialog();

    fireEvent.click(screen.getByRole("button", { name: "Delete" }));

    await waitFor(() => expect(onDeleted).toHaveBeenCalledWith(agent));
    expect(api.deleteAgent).toHaveBeenCalledWith("agent-1");
    expect(screen.queryByRole("alert")).toBeNull();
  });

  it("cannot be dismissed while the delete is in flight", async () => {
    let finish: () => void = () => {};
    vi.mocked(api.deleteAgent).mockReturnValue(
      new Promise<void>((resolve) => {
        finish = resolve;
      })
    );
    const { onOpenChange, onDeleted } = renderDialog();

    fireEvent.click(screen.getByRole("button", { name: "Delete" }));
    await waitFor(() => expect(screen.getByRole("button", { name: "Deleting..." })).toBeTruthy());
    fireEvent.click(screen.getByRole("button", { name: "Cancel" }));
    fireEvent.keyDown(screen.getByRole("alertdialog"), { key: "Escape" });
    expect(onOpenChange).not.toHaveBeenCalledWith(false);

    finish();
    await waitFor(() => expect(onDeleted).toHaveBeenCalledTimes(1));
  });
});
