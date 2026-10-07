import { describe, it, expect, vi, afterEach, beforeEach } from "vitest";
import { render, screen, cleanup } from "@testing-library/react";
import type { ReactNode } from "react";
import type { Agent } from "@/lib/api";
import AgentDetailsPage from "./page";

// The agent page renders the first-run panel for an agent that has never connected and leaves
// it out once the agent has, and a malformed trust-score answer leaves the rest of the page up.
// The panel's own content is covered by components/agent/first-run-panel.test.tsx.

vi.mock("next/navigation", () => ({
  useRouter: () => ({ replace: vi.fn(), push: vi.fn(), prefetch: vi.fn(), back: vi.fn() }),
  usePathname: () => `/dashboard/agents/${AGENT_ID}`,
  useSearchParams: () => new URLSearchParams(),
}));
vi.mock("@/components/auth-guard", () => ({ AuthGuard: ({ children }: { children: ReactNode }) => <>{children}</> }));

// Every API method the page and its overview cards call answers {} unless a test says otherwise;
// each test starts from fresh mocks.
const { api, calls } = vi.hoisted(() => {
  const calls: Record<string, ReturnType<typeof vi.fn>> = {};
  const api = new Proxy(calls, {
    get: (target, key: string) => (target[key] ??= vi.fn().mockResolvedValue({})),
  });
  return { api, calls };
});
vi.mock("@/lib/api", async (importOriginal) => ({ ...(await importOriginal<object>()), api }));

const AGENT_ID = "7b1d2c7e-58a4-4f0e-9a51-0c2f6d1e8a11";
const PANEL = "Agent registered. Next, connect it from your code";

const agent = (overrides: Partial<Agent> = {}): Agent =>
  ({
    id: AGENT_ID,
    name: "billing-bot",
    displayName: "Billing Bot",
    description: "",
    agentType: "custom",
    status: "pending",
    version: "1.0.0",
    publicKey: "mC4sAbPp0Vt1x2Yq3Zr4Ws5Xt6Yu7Zv8Aw9Bx0Cy1D=",
    lastActive: null,
    trustScore: 0.5,
    capabilities: [],
    talksTo: [],
    createdAt: "2026-10-01T09:00:00Z",
    updatedAt: "2026-10-01T09:00:00Z",
    ...overrides,
  }) as Agent;

const openPage = () => render(<AgentDetailsPage params={Promise.resolve({ id: AGENT_ID })} />);

beforeEach(() => {
  api.getToken.mockReturnValue(null);
  api.getTrustScoreBreakdown.mockResolvedValue({
    overall: 0.5,
    confidence: 0.8,
    calculatedAt: "2026-10-01T09:00:00Z",
    factors: { verificationStatus: 0.5 },
    weights: { verificationStatus: 1 },
    contributions: { verificationStatus: 0.5 },
  });
  api.getAgentTrustScoreHistory.mockResolvedValue({ agentId: AGENT_ID, history: [] });
});

afterEach(() => {
  cleanup();
  for (const key of Object.keys(calls)) delete calls[key];
});

describe("Agent page, first-run panel", () => {
  it("shows the panel for an agent that has never connected", async () => {
    api.getAgent.mockResolvedValue(agent());
    openPage();
    expect(await screen.findByRole("heading", { level: 1, name: "billing-bot" })).toBeTruthy();
    expect(screen.getByRole("region", { name: PANEL })).toBeTruthy();
    expect(api.getAgent).toHaveBeenCalledWith(AGENT_ID);
  });

  it("leaves the panel out for an agent that has connected", async () => {
    api.getAgent.mockResolvedValue(agent({ status: "verified", lastActive: "2026-10-01T09:30:00Z" }));
    openPage();
    expect(await screen.findByRole("heading", { level: 1, name: "billing-bot" })).toBeTruthy();
    expect(screen.queryByRole("region", { name: PANEL })).toBeNull();
  });
});

describe("Agent page, trust-score breakdown", () => {
  it("still loads when the breakdown answer has no factors, and the card shows its empty state", async () => {
    api.getAgent.mockResolvedValue(agent());
    api.getTrustScoreBreakdown.mockResolvedValue({});
    openPage();
    expect(await screen.findByRole("heading", { level: 1, name: "billing-bot" })).toBeTruthy();
    expect(await screen.findByText("No trust score data available")).toBeTruthy();
    expect(screen.getByRole("region", { name: PANEL })).toBeTruthy();
  });

  it("still loads when the history answer has no list, and the history shows its empty state", async () => {
    api.getAgent.mockResolvedValue(agent());
    api.getAgentTrustScoreHistory.mockResolvedValue({});
    openPage();
    expect(await screen.findByRole("heading", { level: 1, name: "billing-bot" })).toBeTruthy();
    expect(await screen.findByText("No historical data available yet")).toBeTruthy();
    expect(screen.getByText("Factor breakdown")).toBeTruthy();
  });
});
