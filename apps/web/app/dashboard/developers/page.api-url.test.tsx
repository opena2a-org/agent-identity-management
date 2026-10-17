import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import DevelopersPage from "./page";

// The API documentation page runs requests against the API and writes cURL, JavaScript
// and Python samples that name its address. On a "-frontend" hostname the client signs in
// to the "-backend" host (lib/backend-host.ts), and the page uses that address too, not
// the NEXT_PUBLIC_API_URL the dashboard was built with.

// The value the published image is built with.
const BUILT_API_URL = "http://localhost:8080";
const LOGIN_URL = "https://aim-backend.example.com/api/v1/public/login";

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
  vi.unstubAllEnvs();
  vi.restoreAllMocks();
});

// Radix tabs switch on a primary-button mouse down.
async function openTab(name: RegExp) {
  fireEvent.mouseDown(await screen.findByRole("tab", { name }), { button: 0 });
}

describe("the API documentation page", () => {
  it("runs requests against, and writes its samples for, the API the client resolved", async () => {
    vi.stubEnv("NEXT_PUBLIC_API_URL", BUILT_API_URL);
    vi.stubGlobal("location", new URL("https://aim-frontend.example.com/dashboard/developers"));
    const fetchMock = vi.fn(
      async (_url: string | URL | Request, _init?: RequestInit) =>
        new Response(JSON.stringify({ error: "invalid credentials" }), {
          status: 401,
          headers: { "Content-Type": "application/json" },
        }),
    );
    vi.stubGlobal("fetch", fetchMock);
    vi.spyOn(console, "log").mockImplementation(() => {});

    render(<DevelopersPage />);

    // The first endpoint of the first category is selected on load.
    await openTab(/Try it out/);
    fireEvent.click(await screen.findByRole("button", { name: /Execute request/ }));
    await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(1));
    expect(String(fetchMock.mock.calls[0][0])).toBe(LOGIN_URL);

    await openTab(/Code/);
    const samples = (await screen.findByRole("tabpanel")).textContent ?? "";
    expect(samples).toContain(`curl -X POST '${LOGIN_URL}'`);
    expect(samples).toContain(`fetch('${LOGIN_URL}'`);
    expect(samples).toContain(`url = '${LOGIN_URL}'`);
    expect(samples).not.toContain(BUILT_API_URL);
  });
});
