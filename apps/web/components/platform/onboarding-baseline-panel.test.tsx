import { describe, it, expect, vi, afterEach, type Mock } from "vitest";
import { render, screen, fireEvent, cleanup, within } from "@testing-library/react";
import { OnboardingBaselinePanel, formatDuration } from "./onboarding-baseline-panel";
import { api, type OnboardingBaseline, type TimeToFirstAgentStats } from "@/lib/api";

vi.mock("@/lib/api", () => ({
  api: { getOnboardingMetrics: vi.fn() },
}));

const buckets = (counts: number[]) =>
  ["under 1 minute", "1 to 5 minutes", "5 to 60 minutes", "1 to 24 hours", "1 to 7 days", "over 7 days"].map((label, i) => ({
    label,
    upperSeconds: [60, 300, 3600, 86400, 604800, null][i],
    count: counts[i],
  }));

const stats = (over: Partial<TimeToFirstAgentStats>): TimeToFirstAgentStats => ({
  organizations: 0,
  organizationsWithAgent: 0,
  conversionRate: 0,
  medianSeconds: null,
  p75Seconds: null,
  p90Seconds: null,
  fastestSeconds: null,
  buckets: buckets([0, 0, 0, 0, 0, 0]),
  excludedNegative: 0,
  ...over,
});

const baseline: OnboardingBaseline = {
  generatedAt: "2026-10-08T12:00:00Z",
  windowDays: 30,
  allTime: stats({
    organizations: 40,
    organizationsWithAgent: 10,
    conversionRate: 0.25,
    medianSeconds: 7200,
    p75Seconds: 90000,
    p90Seconds: 700000,
    fastestSeconds: 30,
    buckets: buckets([1, 2, 1, 3, 2, 1]),
    excludedNegative: 2,
  }),
  recent: stats({ organizations: 4, organizationsWithAgent: 1, conversionRate: 0.25, medianSeconds: 312, p75Seconds: 312, p90Seconds: 312, fastestSeconds: 312, buckets: buckets([0, 0, 1, 0, 0, 0]) }),
  events: [
    { event: "onboarding_viewed", organizations: 4, total: 9 },
    { event: "tab_selected", organizations: 2, total: 3 },
    { event: "token_minted", organizations: 0, total: 0 },
    { event: "token_exchanged", organizations: 0, total: 0 },
    { event: "first_agent_registered", organizations: 1, total: 1 },
    { event: "onboarding_completed", organizations: 0, total: 0 },
    { event: "onboarding_skipped", organizations: 0, total: 0 },
  ],
};

afterEach(() => cleanup());

describe("formatDuration", () => {
  it("uses the largest one or two units", () => {
    expect(formatDuration(null)).toBe("–");
    expect(formatDuration(-5)).toBe("–");
    expect(formatDuration(0)).toBe("0s");
    expect(formatDuration(45)).toBe("45s");
    expect(formatDuration(60)).toBe("1m");
    expect(formatDuration(312)).toBe("5m 12s");
    expect(formatDuration(7200)).toBe("2h");
    expect(formatDuration(12000)).toBe("3h 20m");
    expect(formatDuration(187200)).toBe("2d 4h");
  });
});

describe("OnboardingBaselinePanel", () => {
  it("shows the recent window first and switches to all time", async () => {
    (api.getOnboardingMetrics as Mock).mockResolvedValue(baseline);
    render(<OnboardingBaselinePanel />);
    await screen.findByRole("heading", { name: "Time to first agent" });

    expect(screen.getAllByText("5m 12s").length).toBeGreaterThan(0);
    expect(screen.getByText("1 of 4")).toBeTruthy();

    fireEvent.click(screen.getByRole("button", { name: "All time" }));
    expect(screen.getByRole("button", { name: "All time" }).getAttribute("aria-pressed")).toBe("true");
    expect(screen.getByText("2h")).toBeTruthy();
    expect(screen.getByText("10 of 40")).toBeTruthy();
    expect(screen.getByText("25%")).toBeTruthy();
    expect(screen.getByText(/2 organizations have an agent older than the organization/)).toBeTruthy();

    const list = screen.getByRole("list", { name: "Organizations by time to first agent" });
    expect(within(list).getAllByRole("listitem")).toHaveLength(6);
  });

  it("lists every onboarding event with organization and event counts", async () => {
    (api.getOnboardingMetrics as Mock).mockResolvedValue(baseline);
    render(<OnboardingBaselinePanel />);
    const row = (await screen.findByRole("rowheader", { name: "Saw the first-run screen" })).closest("tr")!;
    expect(within(row).getAllByRole("cell").map((c) => c.textContent)).toEqual(["4", "9"]);
    expect(screen.getAllByRole("row")).toHaveLength(1 + baseline.events.length);
  });

  it("explains the access rule on a 403 and offers no retry", async () => {
    (api.getOnboardingMetrics as Mock).mockRejectedValue(Object.assign(new Error("Platform admin access required"), { status: 403 }));
    render(<OnboardingBaselinePanel />);
    expect(await screen.findByText(/limited to platform admins/)).toBeTruthy();
    expect(screen.queryByRole("button", { name: "Try again" })).toBeNull();
  });

  it("offers a retry on other failures", async () => {
    (api.getOnboardingMetrics as Mock).mockRejectedValueOnce(Object.assign(new Error("HTTP 500"), { status: 500 })).mockResolvedValueOnce(baseline);
    render(<OnboardingBaselinePanel />);
    fireEvent.click(await screen.findByRole("button", { name: "Try again" }));
    expect(await screen.findByText("1 of 4")).toBeTruthy();
  });
});
