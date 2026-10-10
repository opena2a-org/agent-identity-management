import { describe, it, expect, vi, afterEach, type Mock } from "vitest";
import { render, screen, act, cleanup } from "@testing-library/react";
import type { ReactNode } from "react";
import { AlertDetailPanel } from "./alert-detail-panel";
import { api } from "@/lib/api";

vi.mock("next/link", () => ({
  default: ({ href, children, ...rest }: { href: string; children: ReactNode }) => (
    <a href={href} {...rest}>
      {children}
    </a>
  ),
}));
vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn() } }));
vi.mock("@/lib/api", () => ({
  api: {
    getAuditLogById: vi.fn(),
    getAgentVerificationHistory: vi.fn(),
    getTrustScoreBreakdown: vi.fn(),
    getAgent: vi.fn(),
    revokeSDKToken: vi.fn(),
  },
}));

type PanelAlert = Parameters<typeof AlertDetailPanel>[0]["alert"];

const NIL_UUID = "00000000-0000-0000-0000-000000000000";
const AGENT_ID = "6f1c2a9e-3b4d-4e5f-8a7b-1c2d3e4f5a6b";
const USER_ID = "9a8b7c6d-5e4f-4a3b-8c2d-1e0f9a8b7c6d";

const baseAlert = {
  severity: "high" as const,
  description: "desc",
  isAcknowledged: false,
  createdAt: "2026-10-01T12:00:00Z",
};

const accountLocked: PanelAlert = {
  ...baseAlert,
  id: "alert-locked",
  alertType: "account_locked",
  title: "Account Locked: dana@example.com",
  resourceType: "user",
  resourceId: NIL_UUID,
  metadata: {
    email: "dana@example.com",
    failureCount: 5,
    eventType: "account_locked",
  },
};

const tokenReuse = (clientMatch: string): PanelAlert => ({
  ...baseAlert,
  id: `alert-reuse-${clientMatch}`,
  alertType: "refresh_token_reuse",
  title: "Refresh token reused",
  resourceType: "user",
  resourceId: USER_ID,
  metadata: { clientMatch, familyRevoked: true },
});

const agentAlert: PanelAlert = {
  ...baseAlert,
  id: "alert-agent",
  alertType: "trust_score_low",
  title: "Trust score low",
  resourceType: "agent",
  resourceId: AGENT_ID,
  agentName: "billing-bot",
};

async function renderPanel(alert: PanelAlert) {
  render(
    <AlertDetailPanel
      alert={alert}
      isOpen
      onClose={() => {}}
      onAcknowledge={() => {}}
      onResolve={() => {}}
    />,
  );
  // Let the panel's fetches settle.
  await act(async () => {});
}

afterEach(() => {
  vi.clearAllMocks();
  cleanup();
});

describe("AlertDetailPanel on a user-resource alert", () => {
  it("presents the account instead of an unknown agent", async () => {
    await renderPanel(accountLocked);

    expect(screen.queryByText("Agent information")).toBeNull();
    expect(screen.queryByText("Agent name")).toBeNull();
    expect(screen.queryByText("Verification history")).toBeNull();
    expect(screen.getByText("Account")).toBeTruthy();
    expect(screen.getByText("dana@example.com")).toBeTruthy();
    // A nil resource ID is not an identifier; it is not shown as one.
    expect(screen.queryByText(NIL_UUID)).toBeNull();
  });

  it("links to the users page, never to an agent page", async () => {
    await renderPanel(accountLocked);

    expect(screen.queryByText("View agent")).toBeNull();
    const hrefs = Array.from(document.querySelectorAll("a")).map((a) => a.getAttribute("href"));
    expect(hrefs.some((h) => h?.startsWith("/dashboard/agents/"))).toBe(false);
    expect(screen.getByText("View users").closest("a")?.getAttribute("href")).toBe(
      "/dashboard/admin/users",
    );
  });

  it("does not ask the agent endpoints about a user", async () => {
    await renderPanel(accountLocked);

    expect(api.getAgent).not.toHaveBeenCalled();
    expect(api.getAgentVerificationHistory).not.toHaveBeenCalled();
    expect(api.getTrustScoreBreakdown).not.toHaveBeenCalled();
  });

  it("shows the user ID when the alert names one", async () => {
    await renderPanel(tokenReuse("differentClient"));

    expect(screen.getByText("User ID")).toBeTruthy();
    expect(screen.getByText(USER_ID)).toBeTruthy();
  });

  it.each([
    ["sameClient", "The same client that last used this token"],
    ["differentClient", "A different client from the one that last used this token"],
    ["unknown", "Could not be determined"],
  ])("states clientMatch %s in words", async (clientMatch, words) => {
    await renderPanel(tokenReuse(clientMatch));

    expect(screen.getByText("Presented by")).toBeTruthy();
    expect(screen.getByText(words)).toBeTruthy();
    expect(screen.queryByText(clientMatch)).toBeNull();
  });
});

describe("AlertDetailPanel on a repeated alert", () => {
  it("shows how many times it was raised and when it was last seen", async () => {
    await renderPanel({
      ...accountLocked,
      occurrenceCount: 7,
      lastSeenAt: "2026-10-01T12:09:00Z",
    });

    expect(screen.getByText("Occurrences")).toBeTruthy();
    expect(screen.getByText("7")).toBeTruthy();
    expect(screen.getByText("Last seen")).toBeTruthy();
  });

  it("shows neither for an alert raised once", async () => {
    await renderPanel({ ...accountLocked, occurrenceCount: 1, lastSeenAt: null });

    expect(screen.queryByText("Occurrences")).toBeNull();
    expect(screen.queryByText("Last seen")).toBeNull();
  });
});

describe("AlertDetailPanel on an agent alert", () => {
  it("keeps the agent view, its fetches and its agent link", async () => {
    (api.getAgentVerificationHistory as Mock).mockResolvedValue([]);
    (api.getTrustScoreBreakdown as Mock).mockResolvedValue({ overall: 0.42 });
    (api.getAgent as Mock).mockResolvedValue({ id: AGENT_ID });
    await renderPanel(agentAlert);

    expect(screen.getByText("Agent information")).toBeTruthy();
    expect(screen.getByText("billing-bot")).toBeTruthy();
    expect(screen.getByText("Verification history")).toBeTruthy();
    expect(screen.getByText("View agent").closest("a")?.getAttribute("href")).toBe(
      `/dashboard/agents/${AGENT_ID}`,
    );
    expect(api.getAgent).toHaveBeenCalledWith(AGENT_ID);
    expect(api.getAgentVerificationHistory).toHaveBeenCalledWith(AGENT_ID, 5);
    expect(api.getTrustScoreBreakdown).toHaveBeenCalledWith(AGENT_ID);
  });
});
