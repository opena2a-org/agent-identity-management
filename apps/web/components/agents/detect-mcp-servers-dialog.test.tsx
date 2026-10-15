import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen, fireEvent, cleanup, act } from "@testing-library/react";
import { DETECT_POLL_MS, DetectMCPServersDialog, detectCommand } from "./detect-mcp-servers-dialog";
import { api } from "@/lib/api";

vi.mock("@/lib/api", () => ({
  api: { getAgent: vi.fn() },
}));

const agent = { id: "agent-1", name: "billing-bot", displayName: "Billing Bot" };

const WAITING = "Waiting for the command to report.";

function renderDialog() {
  const onOpenChange = vi.fn();
  const onDetected = vi.fn();
  const view = render(
    <DetectMCPServersDialog agent={agent} open onOpenChange={onOpenChange} onDetected={onDetected} />
  );
  return { ...view, onOpenChange, onDetected };
}

// Lets the pending read settle, then runs the next one.
const nextPoll = () => act(() => vi.advanceTimersByTimeAsync(DETECT_POLL_MS));
const settle = () => act(() => vi.advanceTimersByTimeAsync(0));

beforeEach(() => {
  vi.useFakeTimers();
  vi.mocked(api.getAgent).mockReset();
});

afterEach(() => {
  cleanup();
  vi.useRealTimers();
});

describe("DetectMCPServersDialog", () => {
  it("gives the command for the agent's machine and asks for no path", () => {
    vi.mocked(api.getAgent).mockReturnValue(new Promise(() => {}));
    renderDialog();

    const dialog = screen.getByRole("dialog");
    expect(screen.getByRole("heading", { name: "Detect MCP servers" })).toBeTruthy();
    expect(dialog.textContent).toContain(
      "Run this on the machine where Billing Bot runs. The MCP servers in its Claude Desktop configuration are registered to the agent."
    );

    const lines = (dialog.querySelector("pre code")?.textContent ?? "").split("\n");
    expect(lines).toHaveLength(2);
    expect(lines[0]).toMatch(/^aim-sdk login\b/);
    expect(lines[1]).toBe(`python -c "from aim_sdk import secure; secure('billing-bot', auto_detect_mcp=True)"`);
    expect(screen.getByRole("button", { name: "Copy commands" })).toBeTruthy();

    expect(dialog.querySelector("input, textarea")).toBeNull();
    expect(screen.queryByRole("textbox")).toBeNull();
    expect(screen.getByRole("status").textContent).toBe(WAITING);
  });

  it("says, before the commands, that the command can replace the agent's key pair", () => {
    vi.mocked(api.getAgent).mockReturnValue(new Promise(() => {}));
    renderDialog();

    const note = screen.getByRole("note");
    expect(note.textContent).toBe(
      "This can replace the agent's key pair. If the SDK on that machine has no saved credentials for this agent, the command creates a new key pair and replaces the agent's public key with it before it registers the MCP servers."
    );
    const code = screen.getByRole("dialog").querySelector("pre code") as Element;
    expect(note.compareDocumentPosition(code) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
  });

  it("escapes a name that holds quotes for both the shell and Python", () => {
    expect(detectCommand(`O'Brien "$x"`)).toBe(
      String.raw`python -c "from aim_sdk import secure; secure('O\\'Brien \"\$x\"', auto_detect_mcp=True)"`
    );
  });
});

describe("DetectMCPServersDialog listener", () => {
  it("replaces the waiting line with the MCP servers the agent gained, and refreshes the page on close", async () => {
    vi.mocked(api.getAgent)
      .mockResolvedValueOnce({ talksTo: ["existing"] } as never)
      .mockResolvedValueOnce({ talksTo: ["existing"] } as never)
      .mockResolvedValue({ talksTo: ["existing", "filesystem", "github"] } as never);
    const { onDetected, onOpenChange } = renderDialog();

    await settle();
    expect(api.getAgent).toHaveBeenCalledWith("agent-1");
    await nextPoll();
    expect(screen.getByRole("status").textContent).toBe(WAITING);

    await nextPoll();
    expect(screen.getByRole("status").textContent).toBe("Registered to the agent: filesystem, github");
    expect(screen.queryByText(WAITING)).toBeNull();

    // Listening stops once the servers are in.
    const reads = vi.mocked(api.getAgent).mock.calls.length;
    await nextPoll();
    expect(api.getAgent).toHaveBeenCalledTimes(reads);

    expect(onDetected).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole("button", { name: "Close" }));
    expect(onDetected).toHaveBeenCalledTimes(1);
    expect(onOpenChange).toHaveBeenCalledWith(false);
  });

  it("keeps waiting when a read fails, and shows nothing from the failure", async () => {
    vi.mocked(api.getAgent)
      .mockResolvedValueOnce({ talksTo: [] } as never)
      .mockRejectedValueOnce(new Error("config file not found at /Users/someone"))
      .mockResolvedValue({ talksTo: ["filesystem"] } as never);
    const { onDetected } = renderDialog();

    await settle();
    await nextPoll();
    expect(screen.getByRole("status").textContent).toBe(WAITING);
    expect(screen.getByRole("dialog").textContent).not.toContain("config file not found");

    // The next read still runs after a failed one.
    await nextPoll();
    expect(screen.getByRole("status").textContent).toBe("Registered to the agent: filesystem");
    expect(onDetected).not.toHaveBeenCalled();
  });

  it("stops reading the agent when the dialog closes, and a close with nothing new does not refresh", async () => {
    vi.mocked(api.getAgent).mockResolvedValue({ talksTo: [] } as never);
    const { rerender, onDetected } = renderDialog();
    await settle();
    await nextPoll();

    fireEvent.click(screen.getByRole("button", { name: "Close" }));
    expect(onDetected).not.toHaveBeenCalled();

    rerender(<DetectMCPServersDialog agent={agent} open={false} onOpenChange={vi.fn()} onDetected={onDetected} />);
    const reads = vi.mocked(api.getAgent).mock.calls.length;
    await nextPoll();
    await nextPoll();
    expect(api.getAgent).toHaveBeenCalledTimes(reads);
  });
});
