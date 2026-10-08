import { afterEach, beforeEach, describe, expect, it, vi, type Mock } from "vitest";
import { toast } from "sonner";
import { api, type ApiRequestError } from "@/lib/api";

vi.mock("sonner", () => ({ toast: { error: vi.fn(), success: vi.fn(), info: vi.fn() } }));

// A public route does not check the session. Its 401 is that route's own answer, so a
// wrong password at sign-in must reach the form as the server said it, not as an
// expired session that refreshes tokens, toasts and reloads the page.

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

// Timers are faked inside each test, after its storage is set up: jsdom's Storage
// dispatches its storage event through setTimeout, so a token written under fake
// timers would be counted as a pending timer of the client.
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

describe("a 401 from the sign-in route", () => {
  it("is the server's refusal, not an expired session", async () => {
    // A refresh token left over from an earlier session must not be spent on it.
    localStorage.setItem("refresh_token", "earlier.refresh.value");
    vi.useFakeTimers({ toFake: ["setTimeout"] });
    const fetchMock = answer(401, { success: false, error: "Invalid email or password" });

    const err = await api
      .loginWithPassword({ email: "person@example.test", password: "wrong" })
      .then(
        () => null,
        (e: ApiRequestError) => e
      );

    expect(err?.message).toBe("Invalid email or password");
    expect(err?.status).toBe(401);
    expect(fetchMock).toHaveBeenCalledTimes(1);
    expect(String(fetchMock.mock.calls[0][0])).toMatch(/\/api\/v1\/public\/login$/);
    expect(toast.error).not.toHaveBeenCalled();
    expect(vi.getTimerCount()).toBe(0);
    expect(localStorage.getItem("refresh_token")).toBe("earlier.refresh.value");
  });
});

describe("a 401 from a route that checks the session", () => {
  it("still ends the session", async () => {
    vi.useFakeTimers({ toFake: ["setTimeout"] });
    answer(401, { error: "Unauthorized" });

    await expect(api.getCurrentUser()).rejects.toThrow("Unauthorized");
    expect((toast.error as Mock).mock.calls[0][0]).toBe("Session expired");
    expect(vi.getTimerCount()).toBe(1);
  });
});
