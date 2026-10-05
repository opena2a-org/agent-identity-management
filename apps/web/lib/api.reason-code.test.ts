import { afterEach, describe, expect, it, vi } from "vitest";
import { api, type ApiRequestError } from "@/lib/api";

// A refused request names its machine-readable reason in reasonCode, the API's
// refusal-code member. code is the older member the same refusals still carry;
// the client reads it only when reasonCode is absent.

function refuseWith(body: unknown, status = 403) {
  vi.stubGlobal(
    "fetch",
    vi.fn(
      async () =>
        new Response(JSON.stringify(body), {
          status,
          headers: { "Content-Type": "application/json" },
        })
    )
  );
}

async function refusal(): Promise<ApiRequestError> {
  try {
    await api.getCurrentUser();
  } catch (err) {
    return err as ApiRequestError;
  }
  throw new Error("the request was not refused");
}

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("refusal reason", () => {
  it("is read from reasonCode", async () => {
    refuseWith({ error: "refused", reasonCode: "noAdministrators" }, 503);
    const err = await refusal();
    expect(err.code).toBe("noAdministrators");
    expect(err.status).toBe(503);
    expect(err.message).toBe("refused");
  });

  it("is read from reasonCode first when both members are present", async () => {
    refuseWith({ error: "refused", reasonCode: "fromReasonCode", code: "fromCode" });
    expect((await refusal()).code).toBe("fromReasonCode");
  });

  it("falls back to code when reasonCode is absent or not a string", async () => {
    refuseWith({ error: "refused", code: "executionOutcomeNotAccepted" });
    expect((await refusal()).code).toBe("executionOutcomeNotAccepted");
    refuseWith({ error: "refused", reasonCode: 7, code: "enforcementModeUnavailable" }, 503);
    expect((await refusal()).code).toBe("enforcementModeUnavailable");
  });

  it("is absent when the body carries none", async () => {
    refuseWith({ error: "refused" });
    expect((await refusal()).code).toBeUndefined();
  });
});
