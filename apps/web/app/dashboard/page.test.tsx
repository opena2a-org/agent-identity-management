import { describe, it, expect, vi, afterEach, type Mock } from "vitest";
import { render, screen, fireEvent, cleanup } from "@testing-library/react";
import type { ReactNode } from "react";
import DashboardPage from "./page";
import { api } from "@/lib/api";

// One stable instance, as Next returns: a fresh object per render would change the
// page's load callback identity and refetch (back to the skeleton) on every click.
const searchParams = new URLSearchParams();
vi.mock("next/navigation", () => ({ useSearchParams: () => searchParams }));
vi.mock("next/link", () => ({
  default: ({ href, children, ...rest }: { href: string; children: ReactNode }) => (
    <a href={href} {...rest}>
      {children}
    </a>
  ),
}));
vi.mock("@/components/auth-guard", () => ({ AuthGuard: ({ children }: { children: ReactNode }) => <>{children}</> }));
vi.mock("@/components/analytics/activity-timeline", () => ({ ActivityTimeline: () => null }));
vi.mock("@/components/overview/security-lens", () => ({ SecurityLens: () => null }));
vi.mock("@/components/overview/executive-lens", () => ({ ExecutiveLens: () => null }));
vi.mock("@/lib/api", () => ({
  api: {
    getToken: () => null,
    setToken: vi.fn(),
    getDashboardStats: vi.fn(),
    getVerificationStatistics: vi.fn(),
    getVerificationActivity: vi.fn(),
    listAgents: vi.fn(),
    getCurrentUser: vi.fn(),
  },
}));

const stats = (totalAgents: number) => ({
  totalAgents,
  verifiedAgents: totalAgents,
  pendingAgents: 0,
  verificationRate: 0,
  avgTrustScore: 0.9,
  totalMcpServers: 0,
  activeMcpServers: 0,
  totalUsers: 1,
  activeUsers: 1,
  activeAlerts: 0,
  criticalAlerts: 0,
  securityIncidents: 0,
  organizationId: "org-1",
});

const agent = { id: "a1", name: "billing-bot", displayName: "billing-bot", status: "verified", trustScore: 0.9, updatedAt: new Date().toISOString() };

function arrange(totalAgents: number) {
  (api.getDashboardStats as Mock).mockResolvedValue(stats(totalAgents));
  (api.getVerificationStatistics as Mock).mockResolvedValue({ totalVerifications: 0, successCount: 0, failedCount: 0, pendingCount: 0, uniqueAgentsVerified: 0 });
  (api.getVerificationActivity as Mock).mockResolvedValue({ activity: [] });
  (api.listAgents as Mock).mockResolvedValue({ agents: totalAgents ? [agent] : [] });
  (api.getCurrentUser as Mock).mockResolvedValue({ name: "Seat Three", email: "seat@example.test", role: "admin" });
}

const installBlocks = () => document.querySelectorAll('pre[aria-label="Commands"]');

afterEach(cleanup);

describe("dashboard quickstart", () => {
  it("shows one tabbed quickstart, and no side panel, before the first agent exists", async () => {
    arrange(0);
    render(<DashboardPage />);
    await screen.findByText("Secure your first agent");

    expect(screen.getAllByRole("tablist")).toHaveLength(1);
    expect(screen.getAllByRole("tab").map((t) => t.textContent)).toEqual(["Python", "TypeScript", "Java"]);
    expect(installBlocks()).toHaveLength(1);
    expect(screen.queryByRole("heading", { name: "Quickstart" })).toBeNull();
  });

  it("switches the install line and example with the selected language", async () => {
    arrange(0);
    render(<DashboardPage />);
    await screen.findByText("Secure your first agent");
    expect(installBlocks()[0].textContent).toContain("pip install aim-sdk");

    fireEvent.click(screen.getByRole("tab", { name: "TypeScript" }));
    expect(installBlocks()[0].textContent).toContain("npm install @opena2a/aim-sdk");
    expect(screen.getByRole("tabpanel").textContent).toContain("new AIMClient");

    fireEvent.click(screen.getByRole("tab", { name: "Java" }));
    expect(installBlocks()[0].textContent).toContain("mvn -f agent-identity-management/sdk/java");
  });

  it("moves the quickstart to the side panel once an agent exists", async () => {
    arrange(1);
    render(<DashboardPage />);
    await screen.findByRole("heading", { name: "Quickstart" });

    expect(screen.queryByText("Secure your first agent")).toBeNull();
    expect(screen.getByText("billing-bot")).toBeTruthy();
    expect(screen.getAllByRole("tablist")).toHaveLength(1);
    expect(screen.getAllByRole("tab")).toHaveLength(3);
    expect(installBlocks()).toHaveLength(1);
  });
});
