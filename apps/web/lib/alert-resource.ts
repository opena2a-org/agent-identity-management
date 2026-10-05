/**
 * How the alert detail panel presents what an alert is about.
 *
 * Alerts are raised on agents and on dashboard user accounts (account_locked,
 * auth_failure_pattern, refresh_token_reuse). Only an agent has a detail page,
 * a trust score and a verification history, so a user alert must not be shown
 * as an agent: that rendered "Agent name Unknown" and a "View agent" link to a
 * page that does not exist.
 */

const NIL_UUID = "00000000-0000-0000-0000-000000000000";

/** Where the dashboard lists user accounts; a user alert links here. */
export const USERS_PAGE_HREF = "/dashboard/admin/users";

export function isUserResource(resourceType: string | undefined): boolean {
  return resourceType === "user";
}

/**
 * Whether an alert names a resource. The authentication-failure alerts are
 * raised without a user ID and carry the nil UUID, which identifies nothing.
 */
export function hasResourceId(resourceId: string | undefined): boolean {
  return !!resourceId && resourceId !== NIL_UUID;
}

/** clientMatch on a refresh-token reuse: who presented the retired token. */
const CLIENT_MATCH_WORDS: Record<string, string> = {
  sameClient: "The same client that last used this token",
  differentClient: "A different client from the one that last used this token",
  unknown: "Could not be determined",
};

const CONTEXT_LABELS: Record<string, string> = {
  clientMatch: "Presented by",
};

/** Label for an alert metadata key in the panel's context block. */
export function contextLabel(key: string): string {
  return CONTEXT_LABELS[key] ?? key.replace(/([A-Z])/g, " $1").trim();
}

/** Display text for an alert metadata value in the panel's context block. */
export function contextValue(key: string, value: unknown): string {
  if (key === "clientMatch" && typeof value === "string" && Object.hasOwn(CLIENT_MATCH_WORDS, value)) {
    return CLIENT_MATCH_WORDS[value];
  }
  return typeof value === "object" ? JSON.stringify(value) : String(value);
}
