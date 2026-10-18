import { afterEach, describe, expect, it, vi } from "vitest";
import { pageApiConfig } from "./runtime-config";

// The client derives its backend from the dashboard's hostname only where the page's
// connect-src policy lists that backend (lib/backend-host.ts, lib/runtime-config.ts).
// For any other hostname it calls the NEXT_PUBLIC_API_URL origin, which that policy
// lists, and with no URL named anywhere a host the policy lists as well, so the page
// never calls a host its own policy refuses.

const API_URL = "https://api.example.com";

// The origin the client sends its first request to when the dashboard is open on
// hostname, with window.__RUNTIME_CONFIG__.apiUrl set to runtimeApiUrl when given and
// the build's NEXT_PUBLIC_API_URL set to builtApiUrl (null: the build read none), and
// the URL apiBaseUrl() reports.
async function resolvedOn(
  hostname: string,
  runtimeApiUrl?: string,
  builtApiUrl: string | null = API_URL
): Promise<{ origin: string; apiBaseUrl: string }> {
  vi.resetModules();
  vi.stubEnv("NEXT_PUBLIC_API_URL", builtApiUrl ?? undefined);
  vi.stubGlobal("location", new URL(`https://${hostname}/dashboard`));
  vi.stubGlobal("__RUNTIME_CONFIG__", runtimeApiUrl ? { apiUrl: runtimeApiUrl } : undefined);
  const fetchMock = vi.fn(
    async (_url: string | URL | Request) =>
      new Response("{}", { status: 200, headers: { "Content-Type": "application/json" } })
  );
  vi.stubGlobal("fetch", fetchMock);
  vi.spyOn(console, "log").mockImplementation(() => {});

  const { api } = await import("@/lib/api");
  const { apiBaseUrl } = await import("@/lib/api-base-url");
  await api.getCurrentUser();
  return { origin: new URL(String(fetchMock.mock.calls[0][0])).origin, apiBaseUrl: apiBaseUrl() };
}

async function apiOriginOn(hostname: string): Promise<string> {
  return (await resolvedOn(hostname)).origin;
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

  it("is NEXT_PUBLIC_API_URL for a -frontend hostname the rule derives no backend for", async () => {
    // Control: the stubbed hostname is the one the client reads.
    expect(await apiOriginOn("dashboard.example.com")).toBe(API_URL);

    expect(await apiOriginOn("my_app-frontend.test")).toBe(API_URL);
    expect(await apiOriginOn("-frontend.example.com")).toBe(API_URL);
  });

  // scripts/ux-sweep.mjs sets window.__RUNTIME_CONFIG__ in its test browser.
  it("is window.__RUNTIME_CONFIG__.apiUrl when the page holds one, before the hostname rule and the built URL", async () => {
    for (const hostname of ["aim-frontend.example.com", "localhost"]) {
      expect(await resolvedOn(hostname, "http://localhost:8081"), hostname).toEqual({
        origin: "http://localhost:8081",
        apiBaseUrl: "http://localhost:8081",
      });
    }
  });

  it("is what apiBaseUrl() reports to the callers that build their own API URLs", async () => {
    expect(await resolvedOn("aim-frontend.example.com")).toEqual({
      origin: "https://aim-backend.example.com",
      apiBaseUrl: "https://aim-backend.example.com",
    });
    expect(await resolvedOn("dashboard.example.com")).toEqual({ origin: API_URL, apiBaseUrl: API_URL });
  });

  // The page names no URL, the build read none and the hostname derives no backend.
  it("is the API's default port on the dashboard's host when no URL is named, which the page's connect-src lists", async () => {
    const cases: Array<[hostname: string, origin: string, connectPolicy: string]> = [
      ["dashboard.example.com", "https://dashboard.example.com:8080", "connect-src 'self' dashboard.example.com:8080"],
      ["localhost", "https://localhost:8080", "connect-src 'self' localhost:8080"],
      ["127.0.0.1", "https://127.0.0.1:8080", "connect-src 'self' 127.0.0.1:8080"],
      // No policy host source holds an IPv6 address: the dashboard's own origin.
      ["[::1]", "https://[::1]", "connect-src 'self'"],
    ];
    for (const [hostname, origin, connectPolicy] of cases) {
      expect((await resolvedOn(hostname, undefined, null)).origin, hostname).toBe(origin);
      expect(pageApiConfig({}, `${hostname}:3000`), hostname).toEqual({ connectPolicy });
    }

    // Control: a -frontend hostname still derives its backend first.
    expect((await resolvedOn("aim-frontend.example.com", undefined, null)).origin).toBe("https://aim-backend.example.com");
  });
});
