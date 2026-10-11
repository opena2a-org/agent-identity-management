import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";
import type { ReactNode } from "react";
import type { Agent } from "@/lib/api";
import { ExecutiveLens } from "./executive-lens";
import type { LensData, StatsData, VerificationActivityMonth } from "./types";

vi.mock("next/link", () => ({
  default: ({ href, children, ...rest }: { href: string; children: ReactNode }) => (
    <a href={href} {...rest}>
      {children}
    </a>
  ),
}));

// ResponsiveContainer observes its size; jsdom has no ResizeObserver.
globalThis.ResizeObserver ??= class {
  observe() {}
  unobserve() {}
  disconnect() {}
};

afterEach(cleanup);

const stats: StatsData = {
  totalAgents: 4,
  verifiedAgents: 3,
  pendingAgents: 1,
  verificationRate: 75,
  avgTrustScore: 0.82,
  totalMcpServers: 2,
  activeMcpServers: 2,
  totalUsers: 3,
  activeUsers: 3,
  activeAlerts: 0,
  criticalAlerts: 0,
  securityIncidents: 0,
  organizationId: "org-1",
};

// Scores arrive on both scales; null means the agent has not been scored.
const agents = [
  { id: "a1", name: "a1", status: "verified", trustScore: 0.91 },
  { id: "a2", name: "a2", status: "verified", trustScore: 85 },
  { id: "a3", name: "a3", status: "verified", trustScore: 0.6 },
  { id: "a4", name: "a4", status: "pending", trustScore: null },
] as unknown as Agent[];

const activity: VerificationActivityMonth[] = [
  { month: "Sep", verified: 4, pending: 1, monthYear: "2026-09" },
  { month: "Oct", verified: 7, pending: 2, monthYear: "2026-10" },
];

const lens: LensData = { security: null, violations: null, events: null, compliance: null };

function renderLens(overrides: Partial<StatsData> = {}) {
  render(
    <ExecutiveLens
      stats={{ ...stats, ...overrides }}
      verification={null}
      activity={activity}
      agents={agents}
      lens={lens}
      role="admin"
    />,
  );
}

describe("ExecutiveLens data displays", () => {
  it("draw the trust distribution through the chart layer, reading every bucket and the average", () => {
    renderLens();
    expect(
      screen.getByRole("img", {
        name: "Agents by trust score: High (0.80+) 2, Medium (0.50–0.79) 1, Low (below 0.50) 0, Not scored 1; Avg trust 82",
      }),
    ).toBeTruthy();
  });

  it("withhold the average rather than draw a placeholder when the stats carry none", () => {
    renderLens({ avgTrustScore: Number.NaN });
    expect(
      screen.getByRole("img", {
        name: "Agents by trust score: High (0.80+) 2, Medium (0.50–0.79) 1, Low (below 0.50) 0, Not scored 1",
      }),
    ).toBeTruthy();
  });

  it("draw verification activity through the chart layer, titled by the window the endpoint returned", () => {
    renderLens();
    expect(
      screen.getByRole("img", {
        name: "Verifications per month, last 2 months: 2 points from Sep to Oct; Verified latest 7, Pending latest 2",
      }),
    ).toBeTruthy();
  });
});
