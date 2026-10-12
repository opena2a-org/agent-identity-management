import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";
import type { ReactNode } from "react";
import { drawnTextColour, insetTileGrounds, shortfalls } from "@/tests/text-colour";
import { SecurityLens } from "./security-lens";
import type { LensData, SecurityMetrics, StatsData } from "./types";

vi.mock("next/link", () => ({
  default: ({ href, children, ...rest }: { href: string; children: ReactNode }) => (
    <a href={href} {...rest}>
      {children}
    </a>
  ),
}));

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

const security: SecurityMetrics = {
  securityScore: 80,
  securityStatus: "good",
  lastIncidentAt: null,
  actionsBlocked: 3,
  actionsBlockedToday: 1,
  agentsMonitored: 4,
  agentsTrusted: 3,
  trustPercentage: 75,
  actionsToday: 12,
  requiresAttention: 1,
  averageTrustScore: 0.82,
  mcpServersTotal: 2,
  mcpServersVerified: 2,
  totalThreats: 3,
  activeThreats: 0,
  blockedThreats: 3,
  highSeverityCount: 1,
  openIncidents: 2,
  riskByCategory: [],
  recentBlockedActions: [],
};

const lens: LensData = { security, violations: { violations: [], total: 0 }, events: [], compliance: null };

describe("the security lens's policy tiles", () => {
  // The unit beside each figure read 4.20:1 in the dark theme in the tertiary ink.
  it("draw the unit beside each figure at 4.5:1 or more on the inset tile in both themes", () => {
    render(<SecurityLens stats={stats} verification={null} lens={lens} role="admin" />);
    const units = ["monitored", "verified", "incidents"].map((unit) => screen.getByText(unit, { exact: true }));
    expect(units.map((u) => u.closest("p")?.textContent)).toEqual(["3 of 4 monitored", "2 of 2 verified", "2 incidents"]);
    for (const unit of units) {
      expect(shortfalls(drawnTextColour(unit), insetTileGrounds), unit.textContent ?? "").toEqual([]);
    }
  });
});
