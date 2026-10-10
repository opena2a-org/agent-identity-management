import { describe, it, expect, vi, afterEach, beforeEach } from "vitest";
import { render, screen, cleanup, fireEvent } from "@testing-library/react";
import type { ReactNode } from "react";
import AlertsPage from "./page";
import { api } from "@/lib/api";
import { formatDateTime } from "@/lib/date-utils";

// jsdom computes no layout, so these tests assert the structure and classes
// that keep the alert card inside a 375px viewport, and where the page mounts
// the detail panel, rather than measured boxes.

const router = { replace: vi.fn(), push: vi.fn() };
vi.mock("next/navigation", () => ({
  useRouter: () => router,
  useSearchParams: () => new URLSearchParams(),
}));
vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn() } }));
vi.mock("@/components/auth-guard", () => ({
  AuthGuard: ({ children }: { children: ReactNode }) => <>{children}</>,
}));
// The panel's own markup has its own tests; here only its mount point matters.
vi.mock("@/components/alerts/alert-detail-panel", () => ({
  AlertDetailPanel: ({ isOpen }: { isOpen: boolean }) =>
    isOpen ? <div data-testid="alert-detail-panel" className="fixed inset-0" /> : null,
}));
vi.mock("@/lib/api", () => ({
  api: {
    getToken: vi.fn(),
    getAlerts: vi.fn(),
    acknowledgeAlert: vi.fn(),
    resolveAlert: vi.fn(),
    bulkAcknowledgeAlerts: vi.fn(),
  },
}));

const base64Url = (value: object) =>
  btoa(JSON.stringify(value)).replace(/\+/g, "-").replace(/\//g, "_").replace(/=+$/, "");
const ADMIN_TOKEN = `${base64Url({ alg: "none" })}.${base64Url({ role: "admin", sub: "admin-1" })}.sig`;

const baseAlert = {
  alertType: "security_breach",
  severity: "high" as const,
  description: "desc",
  resourceType: "user",
  resourceId: "00000000-0000-0000-0000-000000000000",
  isAcknowledged: false,
  createdAt: "2026-10-01T12:00:00Z",
};

const repeated = {
  ...baseAlert,
  id: "alert-repeated",
  title: "Account Locked: dana@example.com",
  occurrenceCount: 3,
  lastSeenAt: "2026-10-01T12:08:00Z",
};

const once = {
  ...baseAlert,
  id: "alert-once",
  title: "Raised once",
  occurrenceCount: 1,
  lastSeenAt: null,
};

function serveAlerts(alerts: object[]) {
  vi.mocked(api.getAlerts).mockResolvedValue({
    alerts,
    total: alerts.length,
    allCount: alerts.length,
    acknowledgedCount: 0,
    unacknowledgedCount: alerts.length,
    criticalCount: 0,
    highCount: alerts.length,
    mediumCount: 0,
    lowAndInfoCount: 0,
  });
}

function cardOf(title: string): HTMLElement {
  const card = screen.getByText(title).closest<HTMLElement>(".cursor-pointer");
  expect(card).not.toBeNull();
  return card!;
}

beforeEach(() => {
  vi.mocked(api.getToken).mockReturnValue(ADMIN_TOKEN);
});

afterEach(() => {
  cleanup();
  vi.clearAllMocks();
});

describe("AlertsPage alert list", () => {
  it("shows how often a repeated alert was seen, and when last", async () => {
    serveAlerts([repeated, once]);
    render(<AlertsPage />);

    const seen = await screen.findByText(
      `Seen 3 times, last ${formatDateTime(repeated.lastSeenAt)}`,
    );
    expect(cardOf(repeated.title).contains(seen)).toBe(true);
    expect(cardOf(once.title).textContent).not.toContain("Seen");
  });

  it("wraps the row that holds the repeat count", async () => {
    serveAlerts([repeated]);
    render(<AlertsPage />);

    const seen = await screen.findByText(/Seen 3 times/);
    expect(seen.parentElement?.className).toContain("flex-wrap");
  });

  it("stacks the actions under the content below the sm breakpoint, and lets the content narrow", async () => {
    serveAlerts([repeated]);
    render(<AlertsPage />);

    await screen.findByText(repeated.title);
    const card = cardOf(repeated.title);
    const acknowledge = screen.getByRole("button", { name: /Acknowledge$/ });

    const row = card.firstElementChild as HTMLElement;
    expect(row.className.split(" ")).toEqual(
      expect.arrayContaining(["flex", "flex-col", "sm:flex-row"]),
    );
    const [content, actions] = Array.from(row.children) as HTMLElement[];
    expect(actions.contains(acknowledge)).toBe(true);
    expect(content.contains(acknowledge)).toBe(false);
    expect(content.className.split(" ")).toContain("min-w-0");

    const title = screen.getByRole("heading", { name: repeated.title });
    expect(title.className.split(" ")).toEqual(expect.arrayContaining(["min-w-0", "break-words"]));
    expect(title.parentElement?.className).toContain("flex-wrap");
    // Every ancestor between the title and the card can shrink below the
    // width of the title's longest word.
    for (let el = title.parentElement; el && el !== content; el = el.parentElement) {
      if (el.className.split(" ").includes("flex-1")) {
        expect(el.className.split(" ")).toContain("min-w-0");
      }
    }
  });

  it("mounts the detail panel outside the space-y-6 container", async () => {
    serveAlerts([repeated]);
    render(<AlertsPage />);

    fireEvent.click(await screen.findByText(repeated.title));

    const panel = await screen.findByTestId("alert-detail-panel");
    expect(panel.closest(".space-y-6")).toBeNull();
  });
});
