import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { api, type ApiRequestError } from "@/lib/api";

vi.mock("sonner", () => ({ toast: { error: vi.fn(), success: vi.fn(), info: vi.fn() } }));

// The dashboard shell, the sidebar, the header and the deactivation check each ask for
// the signed-in user when a page loads, and /api/v1/auth/me shares the strict rate limit
// of the sign-in route. Callers that ask while a request for their token is on the wire
// share its answer; the next call after it settles, or a call with another token, sends
// its own request.

type Answer = { status: number; body: unknown };

function serve(answers: Answer[]) {
  const pending: Array<() => void> = [];
  let served = 0;
  const fetchMock = vi.fn<typeof fetch>(
    () =>
      new Promise<Response>((resolve) => {
        const answer = answers[Math.min(served++, answers.length - 1)];
        pending.push(() =>
          resolve(
            new Response(JSON.stringify(answer.body), {
              status: answer.status,
              headers: { "Content-Type": "application/json" },
            }),
          ),
        );
      }),
  );
  vi.stubGlobal("fetch", fetchMock);
  const release = () => {
    for (const answer of pending.splice(0)) answer();
  };
  return { fetchMock, release };
}

const authHeader = (call: unknown[]) => ((call[1] as RequestInit).headers as Record<string, string>)["Authorization"];

beforeEach(() => {
  api.clearToken();
  localStorage.clear();
  localStorage.setItem("auth_token", "a1");
});

afterEach(() => {
  vi.unstubAllGlobals();
  vi.clearAllMocks();
});

describe("the signed-in user's profile request", () => {
  it("is sent once for the callers that ask together, and again for the next caller", async () => {
    const { fetchMock, release } = serve([{ status: 200, body: { id: "user-1", email: "person@example.test" } }]);

    const together = Promise.all([api.getCurrentUser(), api.getCurrentUser(), api.getCurrentUser()]);
    expect(fetchMock).toHaveBeenCalledTimes(1);
    expect(String(fetchMock.mock.calls[0][0])).toMatch(/\/api\/v1\/auth\/me$/);
    release();
    const users = await together;
    expect(users).toEqual([
      { id: "user-1", email: "person@example.test" },
      { id: "user-1", email: "person@example.test" },
      { id: "user-1", email: "person@example.test" },
    ]);

    const later = api.getCurrentUser();
    expect(fetchMock).toHaveBeenCalledTimes(2);
    release();
    await expect(later).resolves.toEqual({ id: "user-1", email: "person@example.test" });
  });

  it("reaches every caller with the server's refusal, and the caller after it asks again", async () => {
    const { fetchMock, release } = serve([
      { status: 429, body: { error: "Rate limit exceeded. Please try again later." } },
      { status: 200, body: { id: "user-1" } },
    ]);

    const first = api.getCurrentUser();
    const second = api.getCurrentUser();
    expect(fetchMock).toHaveBeenCalledTimes(1);
    release();
    const refusals = await Promise.all([first.catch((e) => e), second.catch((e) => e)]);
    for (const refusal of refusals as ApiRequestError[]) {
      expect(refusal.status).toBe(429);
      expect(refusal.message).toBe("Rate limit exceeded. Please try again later.");
    }

    const retry = api.getCurrentUser();
    expect(fetchMock).toHaveBeenCalledTimes(2);
    release();
    await expect(retry).resolves.toEqual({ id: "user-1" });
  });

  it("is sent on its own for a caller whose session token differs from the one on the wire", async () => {
    const { fetchMock, release } = serve([{ status: 200, body: { id: "user-1" } }]);

    const withFirstToken = api.getCurrentUser();
    localStorage.setItem("auth_token", "a2");
    const withSecondToken = api.getCurrentUser();
    expect(fetchMock).toHaveBeenCalledTimes(2);
    expect(authHeader(fetchMock.mock.calls[0])).toBe("Bearer a1");
    expect(authHeader(fetchMock.mock.calls[1])).toBe("Bearer a2");
    release();
    await expect(Promise.all([withFirstToken, withSecondToken])).resolves.toHaveLength(2);
  });
});
