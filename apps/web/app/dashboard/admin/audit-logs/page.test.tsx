import { describe, it, expect, vi, afterEach, type Mock } from "vitest";
import { render, screen, fireEvent, cleanup, within } from "@testing-library/react";
import type { ReactNode } from "react";
import AuditLogsPage from "./page";
import { api } from "@/lib/api";
import { HUB_TABS } from "@/lib/hub-tabs";
import { effectiveEdgeRoles } from "@/lib/route-permissions";
import { NO_ACTOR_LABEL } from "@/lib/audit-records";

vi.mock("@/components/auth-guard", () => ({ AuthGuard: ({ children }: { children: ReactNode }) => <>{children}</> }));
vi.mock("@/lib/api", () => ({ api: { getAuditLogs: vi.fn() } }));

const getAuditLogs = api.getAuditLogs as unknown as Mock;

// What GET /api/v1/admin/audit-logs sends per record, including the members no
// list may render (network address, user agent, metadata).
function routeRecord(overrides: Record<string, unknown>) {
  return {
    id: "r",
    organizationId: "org-1",
    action: "update",
    resourceType: "agent",
    resourceId: "5c8e1f3a-2d4b-4e6f-9a7c-0b1d2e3f4a5b",
    ipAddress: "203.0.113.7",
    userAgent: "ExampleBrowser/1.0",
    metadata: { note: "metadata-must-not-render" },
    timestamp: "2026-10-01T10:00:00Z",
    ...overrides,
  };
}

const page = [
  routeRecord({ id: "agent-act", agentId: "9e2f4c11-7b3a-4d58-8c06-1a5e9f2d7b43", agentName: "example-agent", action: "attest", resourceType: "mcp_server", timestamp: "2026-10-01T12:00:00Z" }),
  routeRecord({ id: "user-act", userId: "4b1d7a52-0c9e-4f43-9a1e-2f6c8d3b5e71", userName: "Example User", action: "refresh_token_reuse", resourceType: "api_key", timestamp: "2026-10-01T13:18:21Z" }),
  routeRecord({ id: "no-actor", action: "calculate", resourceType: "trust_score", timestamp: "2026-10-01T11:00:00Z" }),
];

afterEach(() => {
  cleanup();
  vi.clearAllMocks();
});

describe("admin audit log page", () => {
  it("lists the route's records newest first with actor, action, target and absolute time", async () => {
    getAuditLogs.mockResolvedValue(page);
    render(<AuditLogsPage />);

    const rows = await screen.findAllByTestId("audit-record");
    expect(getAuditLogs).toHaveBeenCalledWith(50, 0);
    expect(rows).toHaveLength(3);

    const [first, second, third] = rows;
    expect(within(first).getByText("2026-10-01 13:18:21 UTC")).toBeTruthy();
    expect(within(first).getByText("Example User")).toBeTruthy();
    expect(within(first).getByText("Refresh token reuse")).toBeTruthy();
    expect(within(first).getByText("API key")).toBeTruthy();
    expect(within(first).getByText("5c8e1f3a-2d4b-4e6f-9a7c-0b1d2e3f4a5b")).toBeTruthy();
    expect(within(first).getByText("2026-10-01 13:18:21 UTC").getAttribute("datetime")).toBe("2026-10-01T13:18:21Z");

    expect(within(second).getByText("example-agent")).toBeTruthy();
    expect(within(second).getByText("MCP server")).toBeTruthy();

    expect(within(third).getByText(NO_ACTOR_LABEL)).toBeTruthy();
  });

  it("never shows a record's actor as system, and renders no network address, user agent or metadata", async () => {
    getAuditLogs.mockResolvedValue(page);
    const { container } = render(<AuditLogsPage />);
    await screen.findAllByTestId("audit-record");

    const text = container.textContent ?? "";
    expect(text.toLowerCase()).not.toContain("system");
    expect(text).not.toContain("203.0.113.7");
    expect(text).not.toContain("ExampleBrowser");
    expect(text).not.toContain("metadata-must-not-render");
  });

  it("pages through older records by offset", async () => {
    const full = Array.from({ length: 50 }, (_, i) =>
      routeRecord({ id: `r${i}`, userId: "4b1d7a52-0c9e-4f43-9a1e-2f6c8d3b5e71", userName: "Example User", timestamp: new Date(Date.UTC(2026, 9, 1, 12, 0, 0) - i * 1000).toISOString() })
    );
    getAuditLogs.mockResolvedValueOnce(full).mockResolvedValueOnce(page);
    render(<AuditLogsPage />);
    await screen.findAllByTestId("audit-record");
    expect(screen.getByText("Records 1-50")).toBeTruthy();

    fireEvent.click(screen.getByRole("button", { name: /older/i }));
    expect(await screen.findByText("Records 51-53")).toBeTruthy();
    expect(getAuditLogs).toHaveBeenLastCalledWith(50, 50);
    expect((screen.getByRole("button", { name: /older/i }) as HTMLButtonElement).disabled).toBe(true);
  });

  it("says when the records could not be loaded", async () => {
    getAuditLogs.mockRejectedValue(new Error("Request failed: 503"));
    render(<AuditLogsPage />);
    expect((await screen.findByRole("alert")).textContent).toContain("Request failed: 503");
    expect(screen.queryAllByTestId("audit-record")).toHaveLength(0);
  });

  it("is reachable by an admin from the Organization tabs, and only by an admin", () => {
    const tab = HUB_TABS.organization.find((t) => t.href === "/dashboard/admin/audit-logs");
    expect(tab?.roles).toEqual(["admin"]);
    expect(effectiveEdgeRoles("/dashboard/admin/audit-logs")).toEqual(["admin"]);
  });
});
