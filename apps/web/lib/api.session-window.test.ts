import { beforeEach, describe, expect, it } from "vitest";
import { api } from "@/lib/api";

/**
 * The absolute session window must start on a NEW SESSION and must not move on a
 * TOKEN REFRESH.
 *
 * The bug these tests exist for: `setToken` used to infer which case it was in, via
 * `!this.token && !localStorage.getItem("auth_token")`. Any login that happened while
 * a stale `auth_token` was still in localStorage was therefore treated as a refresh,
 * so `session_start` kept its old value and use-idle-timeout immediately signed the
 * user out with "Your 8-hour session ended." That logout cleared the keys, so the
 * next login worked — which is why the symptom was having to sign in exactly twice.
 *
 * The stale-token state is routine, not exotic: the 8h cap is only enforced by a
 * timer on a mounted dashboard, so closing the tab and returning the next day leaves
 * both `auth_token` and a day-old `session_start` in place.
 */

const DAY_MS = 24 * 60 * 60 * 1000;

function seedStaleSession(ageMs: number) {
  const old = String(Date.now() - ageMs);
  localStorage.setItem("auth_token", "stale.token.value");
  localStorage.setItem("session_start", old);
  localStorage.setItem("last_activity", old);
  return old;
}

describe("session window bookkeeping in setToken", () => {
  beforeEach(() => {
    localStorage.clear();
    // Drop the in-memory token too, so each case starts from a known state rather
    // than inheriting one from the previous test.
    api.clearToken();
    localStorage.clear();
  });

  it("restarts the window on login even when a stale token is still in localStorage", () => {
    const stale = seedStaleSession(DAY_MS);

    api.setToken("fresh.token.value", "fresh.refresh.value", "new-session");

    const after = localStorage.getItem("session_start");
    expect(after).not.toBeNull();
    // The load-bearing assertion. Pre-fix this stayed equal to `stale`, which is
    // what produced the immediate cap logout.
    expect(after).not.toBe(stale);
    expect(Number(after)).toBeGreaterThan(Number(stale));
    // And it must be a window that has not already expired.
    expect(Date.now() - Number(after)).toBeLessThan(60 * 1000);
  });

  it("also restarts it when no previous session is present at all", () => {
    api.setToken("fresh.token.value", "fresh.refresh.value", "new-session");
    const start = localStorage.getItem("session_start");
    expect(start).not.toBeNull();
    expect(Date.now() - Number(start)).toBeLessThan(60 * 1000);
  });

  it("does NOT move the window on a token refresh", () => {
    // A live session, 3h old: past no boundary, so a refresh must leave it alone.
    const start = seedStaleSession(3 * 60 * 60 * 1000);

    api.setToken("rotated.token.value", "rotated.refresh.value", "token-refresh");

    // If this ever equals "now", the 8h cap can never be reached, because a refresh
    // happens well inside every 8h period.
    expect(localStorage.getItem("session_start")).toBe(start);
  });

  it("defaults to starting a new window when the caller does not say", () => {
    const stale = seedStaleSession(DAY_MS);
    api.setToken("fresh.token.value");
    // The default is deliberately the safe-for-the-user direction: a caller that
    // forgets gets a usable session rather than a login loop.
    expect(localStorage.getItem("session_start")).not.toBe(stale);
  });

  it("clearToken removes the window so it cannot be inherited", () => {
    api.setToken("fresh.token.value", "fresh.refresh.value", "new-session");
    api.clearToken();
    expect(localStorage.getItem("session_start")).toBeNull();
    expect(localStorage.getItem("last_activity")).toBeNull();
    expect(localStorage.getItem("auth_token")).toBeNull();
  });
});
