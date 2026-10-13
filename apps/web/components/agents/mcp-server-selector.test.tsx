import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen, fireEvent, waitFor, cleanup } from "@testing-library/react";
import { MCPServerSelector } from "./mcp-server-selector";
import { api } from "@/lib/api";

vi.mock("@/lib/api", () => ({
  api: { listMCPServers: vi.fn(), addMCPServersToAgent: vi.fn() },
}));

const servers = [
  { id: "srv-1", name: "memory", url: "https://memory.example.test", status: "active", createdAt: "2026-10-01T00:00:00Z" },
  { id: "srv-2", name: "github", url: "https://github.example.test", status: "active", createdAt: "2026-10-01T00:00:00Z" },
];

function requestError(message: string, status: number, extra: Record<string, unknown> = {}) {
  return Object.assign(new Error(message), { status }, extra);
}

const recordRefusal = () =>
  requestError("server line", 503, { code: "recordUnavailable", retryAfterSeconds: 300 });

// Opens the dialog, selects "memory" and returns the dialog and its Add button.
async function openAndSelect(onSelectionComplete = vi.fn()) {
  render(<MCPServerSelector agentId="agent-1" currentMCPServers={[]} onSelectionComplete={onSelectionComplete} />);
  fireEvent.click(screen.getByRole("button", { name: "Add MCP servers" }));
  const dialog = await screen.findByRole("dialog");
  fireEvent.click(await screen.findByText("memory"));
  const add = screen.getByRole("button", { name: "Add (1)" });
  return { dialog, add, onSelectionComplete };
}

beforeEach(() => {
  vi.mocked(api.listMCPServers).mockReset().mockResolvedValue({ mcpServers: servers } as never);
  vi.mocked(api.addMCPServersToAgent).mockReset();
  vi.spyOn(console, "error").mockImplementation(() => {});
});

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});

describe("MCPServerSelector", () => {
  it("states an addition the record path refused once, in a neutral alert inside the dialog, with when to try again", async () => {
    vi.mocked(api.addMCPServersToAgent).mockRejectedValueOnce(recordRefusal());
    const { dialog, add, onSelectionComplete } = await openAndSelect();
    add.focus();

    fireEvent.click(add);

    const refusal = await screen.findByRole("alert");
    expect(screen.getAllByRole("alert")).toHaveLength(1);
    expect(dialog.contains(refusal)).toBe(true);
    expect(refusal.querySelector("h5")?.textContent).toBe("Nothing was changed");
    expect(refusal.textContent).toContain("AIM did not add the selected MCP servers to this agent");
    expect(refusal.textContent).toContain("the record could not be written");
    expect(refusal.textContent).toContain("Try again in 5 minutes.");
    expect(dialog.textContent).not.toContain("server line");
    // The default variant: the person did nothing wrong.
    expect(refusal.className).not.toContain("destructive");
    expect(dialog.contains(document.activeElement)).toBe(true);
    expect(onSelectionComplete).not.toHaveBeenCalled();
    // The selection is kept, so the next attempt is one click.
    expect(screen.getByRole("button", { name: "Add (1)" })).toBe(add);

    // The alert stays until the next attempt, which clears it.
    vi.mocked(api.addMCPServersToAgent).mockResolvedValueOnce({
      message: "Successfully added 1 MCP server(s)",
      talksTo: ["memory"],
      addedServers: ["memory"],
      totalCount: 1,
    });
    fireEvent.click(add);
    await waitFor(() => expect(onSelectionComplete).toHaveBeenCalledTimes(1));
    expect(screen.queryByRole("alert")).toBeNull();
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(api.addMCPServersToAgent).toHaveBeenCalledTimes(2);
    expect(api.addMCPServersToAgent).toHaveBeenLastCalledWith("agent-1", { mcpServerIds: ["memory"] });
  });

  it("keeps focus in the dialog while the addition runs and does not close until it answers", async () => {
    let answer: (value: never) => void = () => {};
    vi.mocked(api.addMCPServersToAgent).mockReturnValueOnce(
      new Promise((resolve) => {
        answer = resolve as (value: never) => void;
      })
    );
    const { dialog, add } = await openAndSelect();

    fireEvent.click(add);

    // aria-disabled rather than disabled: a disabled button drops focus out of the dialog.
    await waitFor(() => expect(add.getAttribute("aria-disabled")).toBe("true"));
    expect(add.hasAttribute("disabled")).toBe(false);
    fireEvent.click(screen.getByRole("button", { name: "Cancel" }));
    fireEvent.keyDown(dialog, { key: "Escape" });
    expect(screen.getByRole("dialog")).toBe(dialog);
    fireEvent.click(add);
    expect(api.addMCPServersToAgent).toHaveBeenCalledTimes(1);

    answer({ message: "ok", talksTo: ["memory"], addedServers: ["memory"], totalCount: 1 } as never);
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
  });

  it("clears the refusal when the dialog is closed and opened again", async () => {
    vi.mocked(api.addMCPServersToAgent).mockRejectedValueOnce(recordRefusal());
    const { add } = await openAndSelect();
    fireEvent.click(add);
    await screen.findByRole("alert");

    fireEvent.click(screen.getByRole("button", { name: "Cancel" }));
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    fireEvent.click(screen.getByRole("button", { name: "Add MCP servers" }));
    await screen.findByRole("dialog");
    await screen.findByText("memory");

    expect(screen.queryByRole("alert")).toBeNull();
    expect(screen.queryByText("Nothing was changed")).toBeNull();
  });

  it("states any other failure as before, without the record path's alert", async () => {
    vi.mocked(api.addMCPServersToAgent).mockRejectedValueOnce(
      requestError("Agent not found", 404)
    );
    const { dialog, add, onSelectionComplete } = await openAndSelect();

    fireEvent.click(add);

    await waitFor(() => expect(dialog.textContent).toContain("Agent not found"));
    expect(screen.queryByText("Nothing was changed")).toBeNull();
    expect(screen.queryByRole("alert")).toBeNull();
    expect(onSelectionComplete).not.toHaveBeenCalled();
  });
});
