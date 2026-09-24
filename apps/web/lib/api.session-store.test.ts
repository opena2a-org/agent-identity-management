import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { api } from "@/lib/api";

// One session store, read through: the API client keeps no copy of the
// session in the tab, sends it only as an Authorization header, never sends
// cookies, and presents the stored refresh token when it signs out.

type Call = { url: string; init: RequestInit | undefined };
const calls: Call[] = [];

function stubFetch(body: unknown = {}, status = 200) {
  calls.length = 0;
  vi.stubGlobal(
    "fetch",
    vi.fn(async (url: string, init?: RequestInit) => {
      calls.push({ url, init });
      return new Response(JSON.stringify(body), {
        status,
        headers: { "Content-Type": "application/json", "Content-Disposition": "attachment; filename=aim-sdk-python.zip" },
      });
    })
  );
}

const credentialsOf = (c: Call) => (c.init?.credentials ?? "unset") as string;
const authOf = (c: Call) => (c.init?.headers as Record<string, string> | undefined)?.Authorization ?? null;

beforeEach(() => {
  api.clearToken();
  localStorage.clear();
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("one session store, read through", () => {
  it("a tab sends the token the store holds after another tab signs in", () => {
    api.setToken("x.access.value", "x.refresh.value", "new-session");
    localStorage.setItem("auth_token", "y.access.value");
    localStorage.setItem("refresh_token", "y.refresh.value");
    expect(api.getToken()).toBe("y.access.value");
    expect(api.getRefreshToken()).toBe("y.refresh.value");
  });

  it("no request the client makes sends cookies", async () => {
    api.setToken("a.access.value", "a.refresh.value", "new-session");
    stubFetch({ accessToken: "b.access.value", refreshToken: "b.refresh.value", success: true });
    await api.getCurrentUser();
    await api.refreshAccessToken();
    await api.changePassword({ email: "seat@example.test", currentPassword: "Old-pass-1!", newPassword: "New-pass-1!" });
    await api.logout();
    api.setToken("c.access.value", "c.refresh.value", "new-session");
    await api.exportAuditLogs();
    await api.exportComplianceReport("csv", "soc2");
    await api.downloadSDK("python");
    expect(calls.length).toBeGreaterThanOrEqual(7);
    const offenders = calls.filter((c) => credentialsOf(c) !== "omit").map((c) => `${c.url} -> ${credentialsOf(c)}`);
    expect(offenders).toEqual([]);
  });

  it("sign-out presents the stored refresh token", async () => {
    api.setToken("a.access.value", "a.refresh.value", "new-session");
    stubFetch({ message: "ok", revoked: { accessToken: true, refreshToken: true } });
    await api.logout();
    const logout = calls.find((c) => c.url.endsWith("/api/v1/auth/logout"));
    expect(logout).toBeDefined();
    expect(authOf(logout!)).toBe("Bearer a.access.value");
    expect(JSON.parse(String(logout!.init?.body ?? "null"))).toEqual({ refreshToken: "a.refresh.value" });
    expect(api.getToken()).toBeNull();
    expect(api.getRefreshToken()).toBeNull();
  });

  it("after a sign-in and a refresh the client sends what the store holds", async () => {
    api.setToken("a.access.value", "a.refresh.value", "new-session");
    stubFetch({ accessToken: "b.access.value", refreshToken: "b.refresh.value" });
    await api.refreshAccessToken();
    expect(api.getToken()).toBe(localStorage.getItem("auth_token"));
    expect(api.getRefreshToken()).toBe(localStorage.getItem("refresh_token"));
    expect(api.getToken()).toBe("b.access.value");
    await api.getCurrentUser();
    expect(authOf(calls[calls.length - 1])).toBe("Bearer b.access.value");
  });
});
