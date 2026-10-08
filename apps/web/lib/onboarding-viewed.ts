/**
 * sessionStorage key that records this tab has reported onboarding_viewed. It belongs
 * to one sign-in: the API client clears it when a session ends or a new one starts, so
 * the next account to sign in on this tab reports its own first-run view.
 */
export const ONBOARDING_VIEWED_KEY = "aim.onboarding.viewedReported";

/** Clears the onboarding_viewed marker for this tab. */
export function forgetOnboardingViewed(): void {
  try {
    if (typeof window !== "undefined") window.sessionStorage.removeItem(ONBOARDING_VIEWED_KEY);
  } catch {
    // Storage blocked: there is no marker to clear.
  }
}
