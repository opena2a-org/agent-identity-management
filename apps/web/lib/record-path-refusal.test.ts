import { describe, expect, it } from "vitest";
import {
  RECORD_UNAVAILABLE_CODE,
  recordPathRefusal,
  retryAfterText,
} from "@/lib/record-path-refusal";

function refusalError(code: string | undefined, retryAfterSeconds?: number) {
  return Object.assign(new Error("server line"), { status: 503, code, retryAfterSeconds });
}

describe("recordPathRefusal", () => {
  it("describes a recordUnavailable refusal with the action and when to try again", () => {
    const refusal = recordPathRefusal(refusalError(RECORD_UNAVAILABLE_CODE, 300), "delete this agent");
    expect(refusal).toEqual({
      title: "Nothing was changed",
      description:
        "AIM did not delete this agent: it writes an audit record of this change together with the change, " +
        "and the record could not be written. Nothing you entered caused this. Try again in 5 minutes. " +
        "If it fails again, an administrator can find the cause in the AIM server log.",
    });
  });

  it("never shows the server's line", () => {
    const refusal = recordPathRefusal(refusalError(RECORD_UNAVAILABLE_CODE, 300), "delete this agent");
    expect(JSON.stringify(refusal)).not.toContain("server line");
  });

  it("promises no time when the refusal carried no Retry-After", () => {
    const refusal = recordPathRefusal(refusalError(RECORD_UNAVAILABLE_CODE), "delete this agent");
    expect(refusal?.description).toContain("Try again in a few minutes.");
  });

  it("is null for any other refusal", () => {
    expect(recordPathRefusal(refusalError("noAdministrators", 300), "delete this agent")).toBeNull();
    expect(recordPathRefusal(refusalError(undefined), "delete this agent")).toBeNull();
    expect(recordPathRefusal(new TypeError("Failed to fetch"), "delete this agent")).toBeNull();
    expect(recordPathRefusal(null, "delete this agent")).toBeNull();
  });
});

describe("retryAfterText", () => {
  it("states seconds under a minute and whole minutes, rounded up, from a minute on", () => {
    expect(retryAfterText(undefined)).toBe("Try again in a few minutes.");
    expect(retryAfterText(0)).toBe("Try again now.");
    expect(retryAfterText(1)).toBe("Try again in 1 second.");
    expect(retryAfterText(45)).toBe("Try again in 45 seconds.");
    expect(retryAfterText(60)).toBe("Try again in 1 minute.");
    expect(retryAfterText(61)).toBe("Try again in 2 minutes.");
    expect(retryAfterText(300)).toBe("Try again in 5 minutes.");
  });
});
