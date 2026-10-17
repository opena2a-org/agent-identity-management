import { afterEach, describe, expect, it, vi } from "vitest";
import { downloadSDK } from "./agent-sdk";

// downloadSDK requests the archive from the API the client signs in to: on a
// "-frontend" hostname that is the "-backend" host (lib/backend-host.ts), not the
// NEXT_PUBLIC_API_URL the dashboard was built with.

// The value the published image is built with.
const BUILT_API_URL = "http://localhost:8080";

afterEach(() => {
  localStorage.clear();
  vi.unstubAllGlobals();
  vi.unstubAllEnvs();
  vi.restoreAllMocks();
});

describe("downloadSDK", () => {
  it("requests the SDK from the API the client resolved", async () => {
    vi.stubEnv("NEXT_PUBLIC_API_URL", BUILT_API_URL);
    vi.stubGlobal("location", new URL("https://aim-frontend.example.com/dashboard/agents/agent-1"));
    localStorage.setItem("auth_token", "not-a-real-token");
    const fetchMock = vi.fn(
      async (_url: string | URL | Request, _init?: RequestInit) =>
        new Response("zip", {
          status: 200,
          headers: { "Content-Disposition": 'attachment; filename="aim-sdk-python-1.0.0.zip"' },
        }),
    );
    vi.stubGlobal("fetch", fetchMock);
    vi.spyOn(console, "log").mockImplementation(() => {});
    // jsdom has no object URLs and does not navigate; the archive's save is not under test.
    const objectUrls = { createObjectURL: URL.createObjectURL, revokeObjectURL: URL.revokeObjectURL };
    Object.assign(URL, { createObjectURL: vi.fn(() => "blob:sdk"), revokeObjectURL: vi.fn() });
    vi.spyOn(HTMLAnchorElement.prototype, "click").mockImplementation(() => {});

    try {
      await downloadSDK("agent-1", "Agent One", "python");
    } finally {
      Object.assign(URL, objectUrls);
    }

    expect(fetchMock).toHaveBeenCalledTimes(1);
    expect(String(fetchMock.mock.calls[0][0])).toBe(
      "https://aim-backend.example.com/api/v1/agents/agent-1/sdk?lang=python",
    );
  });
});
