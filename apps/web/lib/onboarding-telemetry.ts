import { api, type OnboardingClientEvent, type OnboardingTab } from "@/lib/api";

const VIEWED_KEY = "aim.onboarding.viewedReported";

/**
 * Reports an onboarding step for the signed-in user's organization. Telemetry never
 * interrupts the screen it describes: the call is not awaited, and a failure (offline,
 * an older backend without the route, a refused event) is dropped silently.
 */
export function trackOnboardingEvent(event: OnboardingClientEvent, tab?: OnboardingTab): void {
  try {
    void api.recordOnboardingEvent(event, tab).catch(() => undefined);
  } catch {
    // api is unavailable (tests, server render): nothing to report to.
  }
}

/**
 * Reports onboarding_viewed at most once per browser session, so re-rendering or
 * revisiting the first-run screen does not inflate the funnel.
 */
export function trackOnboardingViewedOnce(): void {
  try {
    if (typeof window === "undefined" || window.sessionStorage.getItem(VIEWED_KEY)) return;
    window.sessionStorage.setItem(VIEWED_KEY, "1");
  } catch {
    // Storage blocked: report anyway, the server tolerates duplicates.
  }
  trackOnboardingEvent("onboarding_viewed");
}
