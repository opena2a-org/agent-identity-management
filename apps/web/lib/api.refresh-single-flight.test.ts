import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

/**
 * One token refresh at a time, across every tab of the dashboard.
 *
 * A login refresh token is single-use: the API rotates it on every refresh, and
 * presenting one that was already rotated ends the whole sign-in. Two tabs share
 * one session store, so when the access token expires both are refused with the
 * same token and both go to refresh. Each used to post the stored refresh token
 * on its own; the second post presented a token the first had just rotated, was
 * refused, and cleared the pair the first tab had just stored, which sent every
 * tab to the login page.
 *
 * A tab here is a separate instance of the client module: its own in-memory
 * state, the same localStorage and the same lock manager, which is what two
 * documents of one origin share in a browser.
 */

type Tab = (typeof import("@/lib/api"))["api"];

async function openTab(): Promise<Tab> {
  vi.resetModules();
  return (await import("@/lib/api")).api;
}

type Call = { url: string; method: string; auth: string | null; body: any };

/**
 * An API with single-use refresh tokens. A request is decided when it arrives
 * and answered one round trip later, so two requests sent together overlap.
 */
function startServer() {
  const state = {
    generation: 1,
    // The access token the store starts with has expired: nothing is valid
    // until the first refresh.
    access: null as string | null,
    refresh: "r1" as string | null,
    ended: false,
  };
  const calls: Call[] = [];
  const held: Promise<void>[] = [];

  const roundTrip = () => new Promise<void>((resolve) => setTimeout(resolve, 5));

  vi.stubGlobal(
    "fetch",
    vi.fn(async (url: string, init?: RequestInit) => {
      const method = init?.method ?? "GET";
      const auth = (init?.headers as Record<string, string> | undefined)?.Authorization ?? null;
      const body = init?.body ? JSON.parse(String(init.body)) : null;
      calls.push({ url, method, auth, body });

      let status = 401;
      let payload: unknown = { error: "Unauthorized" };
      if (url.endsWith("/api/v1/auth/refresh")) {
        if (!state.ended && body?.refreshToken === state.refresh) {
          state.generation += 1;
          state.access = `a${state.generation}`;
          state.refresh = `r${state.generation}`;
          status = 200;
          payload = { accessToken: state.access, refreshToken: state.refresh };
        } else {
          // A refresh token that is not the current one ends the sign-in.
          state.ended = true;
          payload = { error: "Invalid or expired refresh token" };
        }
      } else if (!state.ended && state.access !== null && auth === `Bearer ${state.access}`) {
        status = 200;
        payload = { id: "user-1" };
      }

      await (held.shift() ?? roundTrip());
      return new Response(JSON.stringify(payload), {
        status,
        headers: { "Content-Type": "application/json" },
      });
    })
  );

  return {
    state,
    calls,
    refreshPosts: () => calls.filter((c) => c.url.endsWith("/api/v1/auth/refresh")),
    /** Keeps the answer to the next request back until the returned function is called. */
    holdNext(): () => void {
      let release!: () => void;
      held.push(new Promise<void>((resolve) => (release = resolve)));
      return release;
    },
  };
}

/** The part of the Web Locks API the client uses: one holder per name, first come first served. */
function installLockManager() {
  const tails = new Map<string, Promise<unknown>>();
  setLocks({
    request(name: string, callback: () => unknown) {
      const run = (tails.get(name) ?? Promise.resolve()).then(() => callback());
      tails.set(
        name,
        run.catch(() => undefined)
      );
      return run;
    },
  });
}

function setLocks(value: unknown) {
  Object.defineProperty(navigator, "locks", { value, configurable: true });
}

const storedPair = () => ({
  accessToken: localStorage.getItem("auth_token"),
  refreshToken: localStorage.getItem("refresh_token"),
});

beforeEach(() => {
  localStorage.clear();
  localStorage.setItem("auth_token", "a1");
  localStorage.setItem("refresh_token", "r1");
});

afterEach(() => {
  vi.unstubAllGlobals();
  delete (navigator as unknown as Record<string, unknown>).locks;
});

describe("token refresh across tabs", () => {
  it("two tabs refused with the same expired token send one refresh and both carry on", async () => {
    installLockManager();
    const server = startServer();
    const first = await openTab();
    const second = await openTab();

    const users = await Promise.all([first.getCurrentUser(), second.getCurrentUser()]);

    expect(server.refreshPosts().map((c) => c.body)).toEqual([{ refreshToken: "r1" }]);
    expect(users).toEqual([{ id: "user-1" }, { id: "user-1" }]);
    expect(server.state.ended).toBe(false);
    expect(storedPair()).toEqual({ accessToken: "a2", refreshToken: "r2" });
  });

  it("a tab refused with a token the store no longer holds adopts the stored pair without refreshing", async () => {
    installLockManager();
    const server = startServer();
    const first = await openTab();
    const second = await openTab();

    // The second tab's request is on the wire with the expired token while the
    // first tab refreshes.
    const release = server.holdNext();
    const pending = second.getCurrentUser();
    await first.refreshAccessToken();
    expect(storedPair()).toEqual({ accessToken: "a2", refreshToken: "r2" });
    release();

    await expect(pending).resolves.toEqual({ id: "user-1" });
    expect(server.refreshPosts().map((c) => c.body)).toEqual([{ refreshToken: "r1" }]);
    expect(server.calls[server.calls.length - 1].auth).toBe("Bearer a2");
    expect(storedPair()).toEqual({ accessToken: "a2", refreshToken: "r2" });
  });

  it("a tab that waited while another tab's refresh was refused presents nothing", async () => {
    installLockManager();
    const server = startServer();
    server.state.refresh = "expired-on-the-server";
    const first = await openTab();
    const second = await openTab();

    const results = await Promise.all([first.refreshAccessToken(), second.refreshAccessToken()]);

    expect(results).toEqual([null, null]);
    expect(server.refreshPosts()).toHaveLength(1);
    expect(storedPair()).toEqual({ accessToken: null, refreshToken: null });
  });
});

describe("token refresh where the browser has no Web Locks", () => {
  beforeEach(() => {
    setLocks(undefined);
  });

  it("one tab sends one refresh for requests refused together", async () => {
    const server = startServer();
    const tab = await openTab();

    const answers = await Promise.all([
      tab.getCurrentUser(),
      tab.getCurrentOrganization(),
      tab.getCurrentUser(),
    ]);

    expect(server.refreshPosts().map((c) => c.body)).toEqual([{ refreshToken: "r1" }]);
    expect(answers).toHaveLength(3);
    expect(server.state.ended).toBe(false);
    expect(storedPair()).toEqual({ accessToken: "a2", refreshToken: "r2" });
  });

  it("a refused refresh leaves a pair it did not present in the store", async () => {
    const server = startServer();
    server.state.refresh = "expired-on-the-server";
    const first = await openTab();
    const second = await openTab();

    const release = server.holdNext();
    const pending = second.refreshAccessToken();
    await vi.waitFor(() => expect(server.refreshPosts()).toHaveLength(1));
    // Another tab signs in while the refusal is on its way back.
    first.setToken("b1", "s1", "new-session");
    release();

    await expect(pending).resolves.toEqual({ accessToken: "b1", refreshToken: "s1" });
    expect(storedPair()).toEqual({ accessToken: "b1", refreshToken: "s1" });
  });

  it("a lock manager that refuses the request does not stop the refresh", async () => {
    setLocks({
      request: () => Promise.reject(new DOMException("The request was denied.", "SecurityError")),
    });
    const server = startServer();
    const tab = await openTab();

    await expect(tab.refreshAccessToken()).resolves.toEqual({ accessToken: "a2", refreshToken: "r2" });
    expect(server.refreshPosts()).toHaveLength(1);
    expect(storedPair()).toEqual({ accessToken: "a2", refreshToken: "r2" });
  });
});
