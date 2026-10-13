import { afterEach, describe, expect, it, vi } from "vitest";
import { api, retryAfterSeconds, type ApiRequestError } from "@/lib/api";

// A refusal's Retry-After header is how long the server asked the client to wait. The
// client keeps it on the error, in seconds, beside the status and the reason code.

function refuseWith(body: unknown, status: number, headers: Record<string, string> = {}) {
  vi.stubGlobal(
    "fetch",
    vi.fn(
      async () =>
        new Response(JSON.stringify(body), {
          status,
          headers: { "Content-Type": "application/json", ...headers },
        })
    )
  );
}

async function refusal(): Promise<ApiRequestError> {
  try {
    await api.deleteAgent("agent-1");
  } catch (err) {
    return err as ApiRequestError;
  }
  throw new Error("the request was not refused");
}

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("Retry-After on a refusal", () => {
  it("is kept in seconds with the record path's reason and line", async () => {
    refuseWith(
      { error: "Nothing was changed.", reasonCode: "recordUnavailable", code: "recordUnavailable" },
      503,
      { "Retry-After": "300" }
    );
    const err = await refusal();
    expect(err.status).toBe(503);
    expect(err.code).toBe("recordUnavailable");
    expect(err.retryAfterSeconds).toBe(300);
    expect(err.message).toBe("Nothing was changed.");
  });

  it("is absent when the response carries no Retry-After", async () => {
    refuseWith({ error: "refused" }, 403);
    expect((await refusal()).retryAfterSeconds).toBeUndefined();
  });

  it("is read from a rate limit refusal as well", async () => {
    refuseWith({ error: "Rate limit exceeded", retryAfter: 60 }, 429, { "Retry-After": "60" });
    const err = await refusal();
    expect(err.status).toBe(429);
    expect(err.retryAfterSeconds).toBe(60);
  });
});

describe("retryAfterSeconds", () => {
  it("reads the delay-seconds form", () => {
    expect(retryAfterSeconds("300")).toBe(300);
    expect(retryAfterSeconds(" 0 ")).toBe(0);
  });

  it("reads the HTTP-date form as the seconds until that time, and 0 once it has passed", () => {
    const now = Date.parse("Mon, 12 Oct 2026 10:00:00 GMT");
    expect(retryAfterSeconds("Mon, 12 Oct 2026 10:02:00 GMT", now)).toBe(120);
    expect(retryAfterSeconds("Mon, 12 Oct 2026 09:59:00 GMT", now)).toBe(0);
  });

  it("is undefined for an absent, empty, negative or unreadable value", () => {
    expect(retryAfterSeconds(null)).toBeUndefined();
    expect(retryAfterSeconds("")).toBeUndefined();
    expect(retryAfterSeconds("-5")).toBeUndefined();
    expect(retryAfterSeconds("soon")).toBeUndefined();
  });
});
