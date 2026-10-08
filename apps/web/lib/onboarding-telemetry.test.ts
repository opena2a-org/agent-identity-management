import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { api } from "@/lib/api";
import { trackOnboardingViewedOnce } from "@/lib/onboarding-telemetry";

// onboarding_viewed is reported once per sign-in in a tab. The marker that
// suppresses repeats must not outlive the sign-in: when one account signs out
// and another signs in on the same tab, the second account's first-run view
// is reported too.

const urls: string[] = [];

function viewedReports(): number {
  return urls.filter((u) => u.endsWith("/api/v1/onboarding/events")).length;
}

async function settle(): Promise<void> {
  // The report is not awaited by its caller; let its request reach fetch.
  for (let i = 0; i < 5; i++) await new Promise((r) => setTimeout(r, 0));
}

beforeEach(() => {
  urls.length = 0;
  vi.stubGlobal(
    "fetch",
    vi.fn(async (url: string) => {
      urls.push(url);
      return new Response("{}", { status: 200, headers: { "Content-Type": "application/json" } });
    })
  );
  api.clearToken();
  localStorage.clear();
  window.sessionStorage.clear();
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("onboarding_viewed is reported once per sign-in", () => {
  it("is reported once however often the first-run screen renders", async () => {
    api.setToken("a.access.value", "a.refresh.value", "new-session");
    trackOnboardingViewedOnce();
    trackOnboardingViewedOnce();
    await settle();
    expect(viewedReports()).toBe(1);
  });

  it("is reported again after a sign-out and a sign-in to another organization in the same tab", async () => {
    api.setToken("a.access.value", "a.refresh.value", "new-session");
    trackOnboardingViewedOnce();
    await settle();
    expect(viewedReports()).toBe(1);

    await api.logout();
    api.setToken("b.access.value", "b.refresh.value", "new-session");
    trackOnboardingViewedOnce();
    await settle();
    expect(viewedReports()).toBe(2);
  });

  it("is cleared by a forced sign-out that skips the logout request", async () => {
    api.setToken("a.access.value", "a.refresh.value", "new-session");
    trackOnboardingViewedOnce();
    await settle();

    api.clearToken();
    trackOnboardingViewedOnce();
    await settle();
    expect(viewedReports()).toBe(2);
  });

  it("survives a token refresh, which is the same sign-in", async () => {
    api.setToken("a.access.value", "a.refresh.value", "new-session");
    trackOnboardingViewedOnce();
    await settle();

    api.setToken("a.access.next", "a.refresh.next", "token-refresh");
    trackOnboardingViewedOnce();
    await settle();
    expect(viewedReports()).toBe(1);
  });
});
