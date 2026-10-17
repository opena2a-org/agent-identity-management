import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, render, waitFor } from "@testing-library/react";
import { MCPDetailModal } from "./mcp-detail-modal";

// The dialog fetches an MCP server's attestations and audit log itself. It reaches the
// API the client signs in to: on a "-frontend" hostname that is the "-backend" host
// (lib/backend-host.ts), not the NEXT_PUBLIC_API_URL the dashboard was built with.

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: vi.fn(), replace: vi.fn(), prefetch: vi.fn() }),
}));

// The value the published image is built with.
const BUILT_API_URL = "http://localhost:8080";

const mcp = {
  id: "mcp-1",
  name: "Files",
  url: "https://mcp.example.test",
  status: "verified" as const,
  createdAt: "2026-10-01T00:00:00Z",
  verificationMethod: "agent_attestation",
  attestationCount: 1,
};

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
  vi.unstubAllEnvs();
  vi.restoreAllMocks();
});

describe("the MCP server detail dialog", () => {
  it("fetches attestations and the audit log from the API the client resolved", async () => {
    vi.stubEnv("NEXT_PUBLIC_API_URL", BUILT_API_URL);
    vi.stubGlobal("location", new URL("https://aim-frontend.example.com/dashboard/mcp"));
    const fetchMock = vi.fn(
      async (_url: string | URL | Request) =>
        new Response(JSON.stringify({ attestations: [], logs: [] }), {
          status: 200,
          headers: { "Content-Type": "application/json" },
        }),
    );
    vi.stubGlobal("fetch", fetchMock);
    vi.spyOn(console, "log").mockImplementation(() => {});

    render(<MCPDetailModal isOpen onClose={() => {}} mcp={mcp} />);
    await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(2));

    expect(fetchMock.mock.calls.map(([url]) => String(url)).sort()).toEqual([
      "https://aim-backend.example.com/api/v1/mcp-servers/mcp-1/attestations",
      "https://aim-backend.example.com/api/v1/mcp-servers/mcp-1/audit-logs?limit=50",
    ]);
  });
});
