import { describe, it, expect, vi, beforeEach, afterEach, type Mock } from "vitest";
import { render, screen, act, cleanup } from "@testing-library/react";
import type { ReactNode } from "react";
import { LiveCheckinPanel, CHECKIN_POLL_MS } from "./live-checkin-panel";
import { api, type Agent } from "@/lib/api";

vi.mock("next/link", () => ({
  default: ({ href, children, ...rest }: { href: string; children: ReactNode }) => (
    <a href={href} {...rest}>
      {children}
    </a>
  ),
}));
vi.mock("@/lib/api", () => ({ api: { listAgents: vi.fn() } }));

const agent = (id: string, createdAt: string, status: Agent["status"] = "pending"): Agent =>
  ({ id, name: id, displayName: id, status, trustScore: 0.82, createdAt }) as Agent;

/** Flush the microtask queue so an in-flight poll's promise settles. */
const flush = () => act(async () => {});
const tickOnce = () =>
  act(async () => {
    vi.advanceTimersByTime(CHECKIN_POLL_MS);
  });

beforeEach(() => {
  // Only the interval is faked; promises still settle on the real microtask queue.
  vi.useFakeTimers({ toFake: ["setInterval", "clearInterval"] });
});

afterEach(() => {
  vi.useRealTimers();
  vi.clearAllMocks();
  cleanup();
});

describe("LiveCheckinPanel", () => {
  it("listens on the poll cadence while nothing has checked in", async () => {
    (api.listAgents as Mock).mockResolvedValue({ agents: [] });
    const onArrived = vi.fn();
    render(<LiveCheckinPanel arrived={null} onArrived={onArrived} />);
    await flush();

    expect(screen.getByText("Listening for the first check-in")).toBeTruthy();
    expect(screen.getByText(`Checks every ${CHECKIN_POLL_MS / 1000} seconds.`)).toBeTruthy();
    expect(api.listAgents).toHaveBeenCalledTimes(1);

    // Nothing fires before the cadence elapses; the tick lands exactly on it.
    await act(async () => {
      vi.advanceTimersByTime(CHECKIN_POLL_MS - 1);
    });
    expect(api.listAgents).toHaveBeenCalledTimes(1);
    await act(async () => {
      vi.advanceTimersByTime(1);
    });
    expect(api.listAgents).toHaveBeenCalledTimes(2);
    expect(onArrived).not.toHaveBeenCalled();
  });

  it("hands the newest agent to onArrived and stops polling once the parent records it", async () => {
    (api.listAgents as Mock).mockResolvedValueOnce({ agents: [] });
    const older = agent("a-old", "2026-08-26T10:00:00Z");
    const newer = agent("a-new", "2026-08-26T11:00:00Z");
    (api.listAgents as Mock).mockResolvedValue({ agents: [older, newer] });
    const onArrived = vi.fn();
    const view = render(<LiveCheckinPanel arrived={null} onArrived={onArrived} />);
    await flush();
    expect(onArrived).not.toHaveBeenCalled();

    await tickOnce();
    expect(onArrived).toHaveBeenCalledTimes(1);
    expect(onArrived).toHaveBeenCalledWith(newer);

    view.rerender(<LiveCheckinPanel arrived={newer} onArrived={onArrived} />);
    const calls = (api.listAgents as Mock).mock.calls.length;
    await tickOnce();
    await tickOnce();
    expect(api.listAgents).toHaveBeenCalledTimes(calls);
  });

  it("shows the agent's identity and the open link once arrived", async () => {
    const a = agent("a1", "2026-08-26T10:00:00Z");
    render(<LiveCheckinPanel arrived={a} onArrived={vi.fn()} />);

    expect(screen.getByText("Your agent checked in.")).toBeTruthy();
    expect(screen.getByText(/trust score 0\.82/)).toBeTruthy();
    expect(screen.getByRole("link", { name: /Open the agent/ }).getAttribute("href")).toBe("/dashboard/agents/a1");
    expect(api.listAgents).not.toHaveBeenCalled();
  });

  it("names the verified state when the first agent arrives already verified", () => {
    render(<LiveCheckinPanel arrived={agent("a1", "2026-08-26T10:00:00Z", "verified")} onArrived={vi.fn()} />);
    expect(screen.getByText("Your agent checked in and is verified.")).toBeTruthy();
  });

  it("shows the retry line on a failed poll and recovers on the next good one", async () => {
    (api.listAgents as Mock).mockRejectedValueOnce(new Error("network"));
    render(<LiveCheckinPanel arrived={null} onArrived={vi.fn()} />);
    await flush();
    expect(screen.getByText("Cannot reach the API right now; retrying.")).toBeTruthy();

    (api.listAgents as Mock).mockResolvedValue({ agents: [] });
    await tickOnce();
    expect(screen.getByText("Listening for the first check-in")).toBeTruthy();
  });

  it("offers the in-browser path after two minutes of silence", async () => {
    (api.listAgents as Mock).mockResolvedValue({ agents: [] });
    render(<LiveCheckinPanel arrived={null} onArrived={vi.fn()} />);
    await flush();
    expect(screen.queryByRole("link", { name: "secure it in the browser" })).toBeNull();

    for (let i = 0; i < 40; i++) await tickOnce();
    expect(screen.getByRole("link", { name: "secure it in the browser" }).getAttribute("href")).toBe("/dashboard/agents?register=1");
  });
});
