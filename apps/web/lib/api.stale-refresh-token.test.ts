import { beforeEach, describe, expect, it } from "vitest";
import { api } from "@/lib/api";

/**
 * A new session must never keep the previous account's refresh token.
 *
 * `setToken` writes `refresh_token` only when it is given one, so a caller that
 * starts a new session with an access token alone leaves the refresh token of
 * whoever was signed in before in localStorage, and the next silent refresh
 * runs as that previous account. A token refresh, by contrast, may keep the
 * current refresh token when the answer carries none.
 */
describe("the refresh token across setToken calls", () => {
  beforeEach(() => {
    api.clearToken();
    localStorage.clear();
  });

  it("a new session without a refresh token clears the previous one", () => {
    api.setToken("previous.access.value", "previous.refresh.value", "new-session");
    expect(localStorage.getItem("refresh_token")).toBe("previous.refresh.value");
    api.setToken("next.access.value", undefined, "new-session");
    expect(localStorage.getItem("refresh_token")).toBeNull();
    expect(localStorage.getItem("auth_token")).toBe("next.access.value");
  });

  it("a new session with a refresh token stores it", () => {
    api.setToken("previous.access.value", "previous.refresh.value", "new-session");
    api.setToken("next.access.value", "next.refresh.value", "new-session");
    expect(localStorage.getItem("refresh_token")).toBe("next.refresh.value");
  });

  it("a token refresh without a refresh token keeps the current one", () => {
    api.setToken("access.value", "current.refresh.value", "new-session");
    api.setToken("newer.access.value", undefined, "token-refresh");
    expect(localStorage.getItem("refresh_token")).toBe("current.refresh.value");
  });
});
