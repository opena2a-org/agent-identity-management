import { api, type OnboardingClientEvent, type OnboardingTab } from "@/lib/api";
import { ONBOARDING_VIEWED_KEY } from "@/lib/onboarding-viewed";

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
 * Reports onboarding_viewed at most once per sign-in in this tab, so re-rendering or
 * revisiting the first-run screen does not inflate the funnel. Signing out or in clears
 * the marker (api.clearToken, api.setToken), so another account signing in on the same
 * tab reports its own view.
 */
export function trackOnboardingViewedOnce(): void {
  try {
    if (typeof window === "undefined" || window.sessionStorage.getItem(ONBOARDING_VIEWED_KEY)) return;
    window.sessionStorage.setItem(ONBOARDING_VIEWED_KEY, "1");
  } catch {
    // Storage blocked: report anyway, the server tolerates duplicates.
  }
  trackOnboardingEvent("onboarding_viewed");
}
