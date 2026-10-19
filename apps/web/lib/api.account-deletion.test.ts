import { afterEach, beforeEach, describe, expect, it, vi, type Mock } from "vitest";
import { readFileSync } from "fs";
import path from "path";
import { fileURLToPath } from "url";
import { toast } from "sonner";
import { api, type ApiRequestError } from "@/lib/api";
import { INVALID_PASSWORD_CODE, LAST_ACTIVE_ADMIN_CODE } from "@/lib/account-deletion";
import { SERVER_ERROR_MESSAGE } from "@/lib/error-messages";

vi.mock("sonner", () => ({ toast: { error: vi.fn(), success: vi.fn(), info: vi.fn() } }));

// POST /api/v1/auth/delete-account checks the session, then the password. Its 401 for a
// password it refuses is the route's own answer: it reaches the form as the server said
// it, with the session kept and the password sent once, not as an expired session that
// refreshes, sends the password again and signs out.

const repoRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "../../..");
const backendSource = (file: string) => readFileSync(path.join(repoRoot, "apps/backend", file), "utf8");

const ROUTE = "/api/v1/auth/delete-account";
const PASSWORD = "not-a-real-password";

const json = (status: number, body: unknown, headers: Record<string, string> = {}) =>
  new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json", ...headers },
  });

type Sent = { path: string; auth: string | null; body: unknown };

// Answers the requests with answers, in order, and records what each one sent.
function serve(...answers: Response[]): Sent[] {
  const sent: Sent[] = [];
  vi.stubGlobal(
    "fetch",
    vi.fn(async (url: string, init?: RequestInit) => {
      const headers = (init?.headers ?? {}) as Record<string, string>;
      sent.push({
        path: new URL(String(url), "http://localhost").pathname,
        auth: headers.Authorization ?? null,
        body: init?.body ? JSON.parse(String(init.body)) : null,
      });
      const answer = answers.shift();
      if (!answer) throw new Error(`no answer left for ${url}`);
      return answer;
    })
  );
  return sent;
}

// The part of the Web Locks API the client takes its refresh turn through, so a
// refresh runs without the storage lease and its timers.
function installLockManager() {
  Object.defineProperty(navigator, "locks", {
    value: { request: (_name: string, callback: () => unknown) => Promise.resolve().then(callback) },
    configurable: true,
  });
}

const accepted = {
  status: "deactivated",
  deletionRequestedAt: "2026-10-19T12:30:00Z",
  deletionDueAt: "2026-11-17T12:30:00Z",
};

const failureOf = (promise: Promise<unknown>) =>
  promise.then(
    () => null,
    (e: ApiRequestError) => e
  );

const passwordsSent = (sent: Sent[]) =>
  sent.filter((s) => (s.body as { password?: string } | null)?.password === PASSWORD).length;

const storedSession = () => ({
  accessToken: localStorage.getItem("auth_token"),
  refreshToken: localStorage.getItem("refresh_token"),
});

// Timers are faked inside a test only after its storage is set up.
beforeEach(() => {
  api.clearToken();
  localStorage.clear();
  localStorage.setItem("auth_token", "access.one");
  localStorage.setItem("refresh_token", "refresh.one");
  installLockManager();
});

afterEach(() => {
  vi.clearAllTimers();
  vi.useRealTimers();
  vi.unstubAllGlobals();
  vi.clearAllMocks();
  delete (navigator as unknown as Record<string, unknown>).locks;
});

describe("api.requestAccountDeletion", () => {
  it("posts to the route main.go mounts, and reads the codes the handler answers", () => {
    expect(backendSource("cmd/server/main.go")).toContain(
      'authProtected.Post("/delete-account", h.AccountDeletion.RequestDeletion)'
    );
    const handler = backendSource("internal/interfaces/http/handlers/account_deletion_handler.go");
    expect(handler).toContain(`InvalidPasswordCode = "${INVALID_PASSWORD_CODE}"`);
    expect(handler).toContain(`LastActiveAdminCode = "${LAST_ACTIVE_ADMIN_CODE}"`);
  });

  it("sends the password with the session, returns the request's times and clears the session", async () => {
    const sent = serve(json(202, accepted));

    await expect(api.requestAccountDeletion(PASSWORD)).resolves.toEqual(accepted);

    expect(sent).toEqual([{ path: ROUTE, auth: "Bearer access.one", body: { password: PASSWORD } }]);
    expect(storedSession()).toEqual({ accessToken: null, refreshToken: null });
  });

  it("throws a refused password as the server said it, sent once, with the session kept", async () => {
    vi.useFakeTimers({ toFake: ["setTimeout"] });
    const sent = serve(json(401, { error: "Invalid password", code: INVALID_PASSWORD_CODE }));

    const err = await failureOf(api.requestAccountDeletion(PASSWORD));

    expect(err?.message).toBe("Invalid password");
    expect(err?.status).toBe(401);
    expect(err?.code).toBe(INVALID_PASSWORD_CODE);
    expect(sent.map((s) => s.path)).toEqual([ROUTE]);
    expect(toast.error).not.toHaveBeenCalled();
    expect(vi.getTimerCount()).toBe(0);
    expect(storedSession()).toEqual({ accessToken: "access.one", refreshToken: "refresh.one" });
  });

  it("refreshes a refused session once and sends the request again with the new token", async () => {
    const sent = serve(
      json(401, { error: "Invalid or expired token" }),
      json(200, { accessToken: "access.two", refreshToken: "refresh.two" }),
      json(202, accepted)
    );

    await expect(api.requestAccountDeletion(PASSWORD)).resolves.toEqual(accepted);

    expect(sent).toEqual([
      { path: ROUTE, auth: "Bearer access.one", body: { password: PASSWORD } },
      { path: "/api/v1/auth/refresh", auth: null, body: { refreshToken: "refresh.one" } },
      { path: ROUTE, auth: "Bearer access.two", body: { password: PASSWORD } },
    ]);
    expect(storedSession()).toEqual({ accessToken: null, refreshToken: null });
  });

  it("throws a refused password that answers the request sent after a refresh", async () => {
    const sent = serve(
      json(401, { error: "Invalid or expired token" }),
      json(200, { accessToken: "access.two", refreshToken: "refresh.two" }),
      json(401, { error: "Invalid password", code: INVALID_PASSWORD_CODE })
    );

    const err = await failureOf(api.requestAccountDeletion(PASSWORD));

    expect(err?.code).toBe(INVALID_PASSWORD_CODE);
    expect(sent).toHaveLength(3);
    expect(toast.error).not.toHaveBeenCalled();
    expect(storedSession()).toEqual({ accessToken: "access.two", refreshToken: "refresh.two" });
  });

  it("signs out, as request() does, when the refresh is refused, and does not send the password again", async () => {
    vi.useFakeTimers({ toFake: ["setTimeout"] });
    const sent = serve(
      json(401, { error: "Invalid or expired token" }),
      json(401, { error: "Invalid or expired refresh token" }),
      json(401, { error: "No authentication token provided" })
    );

    await expect(api.requestAccountDeletion(PASSWORD)).rejects.toThrow("Unauthorized");

    expect(sent.map((s) => s.path)).toEqual([ROUTE, "/api/v1/auth/refresh", ROUTE]);
    expect(passwordsSent(sent)).toBe(1);
    expect((toast.error as Mock).mock.calls[0][0]).toBe("Session expired");
    expect(vi.getTimerCount()).toBe(1);
    expect(storedSession()).toEqual({ accessToken: null, refreshToken: null });
  });

  it("signs out without a refresh when no refresh token is stored", async () => {
    localStorage.removeItem("refresh_token");
    vi.useFakeTimers({ toFake: ["setTimeout"] });
    const sent = serve(
      json(401, { error: "Token has been revoked: the account is deactivated" }),
      json(401, { error: "Token has been revoked: the account is deactivated" })
    );

    await expect(api.requestAccountDeletion(PASSWORD)).rejects.toThrow("Unauthorized");

    expect(sent.map((s) => s.path)).toEqual([ROUTE, ROUTE]);
    expect(passwordsSent(sent)).toBe(1);
    expect((toast.error as Mock).mock.calls[0][0]).toBe("Session expired");
    expect(vi.getTimerCount()).toBe(1);
    expect(storedSession().accessToken).toBeNull();
  });

  it("throws the only active admin's refusal with its code, with the session kept", async () => {
    serve(json(409, { error: "You are the only active admin of your organization", code: LAST_ACTIVE_ADMIN_CODE }));

    const err = await failureOf(api.requestAccountDeletion(PASSWORD));

    expect(err?.status).toBe(409);
    expect(err?.code).toBe(LAST_ACTIVE_ADMIN_CODE);
    expect(err?.message).toBe("You are the only active admin of your organization");
    expect(storedSession()).toEqual({ accessToken: "access.one", refreshToken: "refresh.one" });
  });

  it("carries the seconds of a rate limit's Retry-After", async () => {
    serve(json(429, { error: "Rate limit exceeded. Please try again later." }, { "Retry-After": "30" }));

    const err = await failureOf(api.requestAccountDeletion(PASSWORD));

    expect(err?.status).toBe(429);
    expect(err?.retryAfterSeconds).toBe(30);
  });

  it("throws a 5xx, and a success without the request's times, as the server error line, with the session kept", async () => {
    serve(json(500, { error: "database is down" }));
    const serverFailure = await failureOf(api.requestAccountDeletion(PASSWORD));
    expect(serverFailure?.message).toBe(SERVER_ERROR_MESSAGE);
    expect(serverFailure?.status).toBe(500);

    serve(new Response("<html>a proxy's page</html>", { status: 200, headers: { "Content-Type": "text/html" } }));
    const malformed = await failureOf(api.requestAccountDeletion(PASSWORD));
    expect(malformed?.message).toBe(SERVER_ERROR_MESSAGE);
    expect(storedSession()).toEqual({ accessToken: "access.one", refreshToken: "refresh.one" });
  });
});
