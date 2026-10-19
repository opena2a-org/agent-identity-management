import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { toast } from "sonner";
import { api, type ApiRequestError } from "@/lib/api";

vi.mock("sonner", () => ({ toast: { error: vi.fn(), success: vi.fn(), info: vi.fn() } }));

// Sign-in through an organization's identity provider: the start and the callback read
// no session, so a 401 from the callback is the server's refusal of that sign-in, never
// an expired session that refreshes tokens, toasts and reloads the page.

const ORG = "2f1c9a4e-8b7d-4c3a-9e1f-0a2b3c4d5e6f";

function answer(status: number, body: unknown) {
  const fetchMock = vi.fn<typeof fetch>(
    async () =>
      new Response(JSON.stringify(body), {
        status,
        headers: { "Content-Type": "application/json" },
      })
  );
  vi.stubGlobal("fetch", fetchMock);
  return fetchMock;
}

// Timers are faked inside a test, after its storage is set up: jsdom's Storage
// dispatches its storage event through setTimeout.
beforeEach(() => {
  api.clearToken();
  localStorage.clear();
});

afterEach(() => {
  vi.clearAllTimers();
  vi.useRealTimers();
  vi.unstubAllGlobals();
  vi.clearAllMocks();
});

describe("starting a sign-in through the organization's identity provider", () => {
  it("posts to the organization's login route and answers the authorization URL and the flow", async () => {
    const start = {
      authorizationUrl: "https://idp.example.test/authorize?state=s",
      flow: `${ORG}.sealed-flow`,
      expiresAt: "2026-10-19T08:10:00Z",
    };
    const fetchMock = answer(200, start);

    await expect(api.startOidcSignIn(ORG)).resolves.toEqual(start);

    expect(fetchMock).toHaveBeenCalledTimes(1);
    const [url, init] = fetchMock.mock.calls[0];
    expect(String(url)).toMatch(new RegExp(`/api/v1/auth/oidc/${ORG}/login$`));
    expect(init?.method).toBe("POST");
    expect(init?.credentials).toBe("omit");
    expect(api.getToken()).toBeNull();
  });
});

describe("completing a sign-in through the organization's identity provider", () => {
  it("posts the code, state and flow, and keeps the answered pair as a new session", async () => {
    localStorage.setItem("session_start", "1");
    const fetchMock = answer(200, {
      accessToken: "oidc.access.value",
      refreshToken: "oidc.refresh.value",
      user: { id: "u-1", email: "person@example.test", name: "Person", role: "viewer", organizationId: ORG },
    });

    const result = await api.completeOidcSignIn({ code: "the-code", state: "the-state", flow: `${ORG}.sealed-flow` });

    expect(result.user.email).toBe("person@example.test");
    const [url, init] = fetchMock.mock.calls[0];
    expect(String(url)).toMatch(/\/api\/v1\/auth\/oidc\/callback$/);
    expect(init?.method).toBe("POST");
    expect(JSON.parse(String(init?.body))).toEqual({ code: "the-code", state: "the-state", flow: `${ORG}.sealed-flow` });
    expect(api.getToken()).toBe("oidc.access.value");
    expect(api.getRefreshToken()).toBe("oidc.refresh.value");
    expect(localStorage.getItem("session_start")).not.toBe("1");
  });

  it("hands a 401 to the caller as the server's refusal, with its code", async () => {
    // A refresh token left over from an earlier session must not be spent on it.
    localStorage.setItem("refresh_token", "earlier.refresh.value");
    vi.useFakeTimers({ toFake: ["setTimeout"] });
    const fetchMock = answer(401, { error: "Sign-in through the identity provider was refused", code: "no_user" });

    const err = await api
      .completeOidcSignIn({ code: "the-code", state: "the-state", flow: `${ORG}.sealed-flow` })
      .then(
        () => null,
        (e: ApiRequestError) => e
      );

    expect(err?.status).toBe(401);
    expect(err?.code).toBe("no_user");
    expect(err?.message).toBe("Sign-in through the identity provider was refused");
    expect(fetchMock).toHaveBeenCalledTimes(1);
    expect(toast.error).not.toHaveBeenCalled();
    expect(vi.getTimerCount()).toBe(0);
    expect(localStorage.getItem("refresh_token")).toBe("earlier.refresh.value");
    expect(api.getToken()).toBeNull();
  });
});
