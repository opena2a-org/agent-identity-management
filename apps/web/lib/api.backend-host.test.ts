import { afterEach, describe, expect, it, vi } from "vitest";

// The client derives its backend from the dashboard's hostname only where the
// Content-Security-Policy header lists that backend (lib/backend-host.ts). For any
// other hostname it calls the NEXT_PUBLIC_API_URL origin, which the header always
// lists, so the page never calls a host its own policy refuses.

const API_URL = "https://api.example.com";

// The origin the client sends its first request to when the dashboard is open on hostname.
async function apiOriginOn(hostname: string): Promise<string> {
  vi.resetModules();
  vi.stubEnv("NEXT_PUBLIC_API_URL", API_URL);
  vi.stubGlobal("location", new URL(`https://${hostname}/dashboard`));
  const fetchMock = vi.fn(
    async (_url: string | URL | Request) =>
      new Response("{}", { status: 200, headers: { "Content-Type": "application/json" } })
  );
  vi.stubGlobal("fetch", fetchMock);
  vi.spyOn(console, "log").mockImplementation(() => {});

  const { api } = await import("@/lib/api");
  await api.getCurrentUser();
  return new URL(String(fetchMock.mock.calls[0][0])).origin;
}

afterEach(() => {
  vi.unstubAllGlobals();
  vi.unstubAllEnvs();
  vi.restoreAllMocks();
});

describe("the API origin the client resolves from the dashboard's hostname", () => {
  it("is the -backend host for a -frontend hostname", async () => {
    expect(await apiOriginOn("aim-frontend.blue-1.eastus.azurecontainerapps.io")).toBe(
      "https://aim-backend.blue-1.eastus.azurecontainerapps.io"
    );
    expect(await apiOriginOn("a-frontend-frontend.example.com")).toBe("https://a-backend-frontend.example.com");
  });

  it("is NEXT_PUBLIC_API_URL for a -frontend hostname the header derives no backend for", async () => {
    // Control: the stubbed hostname is the one the client reads.
    expect(await apiOriginOn("dashboard.example.com")).toBe(API_URL);

    expect(await apiOriginOn("my_app-frontend.test")).toBe(API_URL);
    expect(await apiOriginOn("-frontend.example.com")).toBe(API_URL);
  });
});
