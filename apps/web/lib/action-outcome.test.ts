import { describe, expect, it } from "vitest";
import { actionSuccess, describeActionFailure, type ActionFailureContext } from "./action-outcome";

// What the API client throws for a non-2xx response: the server's text, its status, and its code if stated.
function requestError(message: string, status?: number, code?: string) {
  return Object.assign(new Error(message), { status, code });
}

// The text the server sends when a delete trips a foreign key: true, and useless to the reader.
const RAW_FK =
  'pq: update or delete on table "agents" violates foreign key constraint "verification_events_agent_id_fkey"';

const context: ActionFailureContext = {
  action: "delete this agent",
  missing: {
    reason: "This agent no longer exists in your organization.",
    link: { label: "Back to agents", href: "/dashboard/agents" },
  },
};

describe("describeActionFailure", () => {
  it("never shows the server's text: an unmapped failure is a generic reason and a next step", () => {
    const failure = describeActionFailure(requestError(RAW_FK, 500), context);
    expect(failure).toEqual({
      kind: "failure",
      reason: "AIM could not delete this agent.",
      nextStep: "Try again. If it fails again, an administrator can find the cause in the AIM server log.",
    });
    expect(JSON.stringify(failure)).not.toContain("foreign key");
  });

  it("maps a stated code to the reason the endpoint gives for it, ahead of the status", () => {
    const failure = describeActionFailure(requestError(RAW_FK, 409, "agentHasDependents"), {
      ...context,
      codes: {
        agentHasDependents: {
          reason: "This agent still has records that depend on it.",
          nextStep: "Suspend it instead.",
        },
      },
    });
    expect(failure).toEqual({
      kind: "failure",
      reason: "This agent still has records that depend on it.",
      nextStep: "Suspend it instead.",
    });
  });

  it("falls back to the status when the stated code is not one the page maps", () => {
    const failure = describeActionFailure(requestError(RAW_FK, 500, "somethingElse"), { ...context, codes: {} });
    expect(failure.reason).toBe("AIM could not delete this agent.");
  });

  it("does not treat an inherited property name as a stated code", () => {
    const failure = describeActionFailure(requestError("x", 500, "toString"), { ...context, codes: {} });
    expect(failure.reason).toBe("AIM could not delete this agent.");
  });

  it("names the role as the reason for a 403", () => {
    const failure = describeActionFailure(requestError("Insufficient permissions", 403), context);
    expect(failure.reason).toBe("Your role does not allow you to delete this agent.");
    expect(failure.nextStep).toBe("Ask an organization administrator or manager to do it, or to change your role.");
  });

  it("says the resource is gone for a 404 and links back to the list", () => {
    const failure = describeActionFailure(requestError("not found", 404), context);
    expect(failure).toEqual({
      kind: "failure",
      reason: "This agent no longer exists in your organization.",
      nextStep: "Open the list to see the current state.",
      link: { label: "Back to agents", href: "/dashboard/agents" },
    });
  });

  it("uses the generic reason for a 404 when the page names no list to return to", () => {
    const failure = describeActionFailure(requestError("not found", 404), { action: "delete this agent" });
    expect(failure.reason).toBe("AIM could not delete this agent.");
  });

  it("says the server could not be reached when the request never left", () => {
    const failure = describeActionFailure(new TypeError("Failed to fetch"), context);
    expect(failure.reason).toBe("AIM could not be reached, so it did not delete this agent.");
    expect(failure.nextStep).toBe("Check your network connection, then try again.");
  });

  it("handles a thrown value that is not an Error", () => {
    expect(describeActionFailure(undefined, context).reason).toBe("AIM could not delete this agent.");
    expect(describeActionFailure("boom", context).reason).toBe("AIM could not delete this agent.");
  });
});

describe("actionSuccess", () => {
  it("carries the message", () => {
    expect(actionSuccess("Agent suspended.")).toEqual({ kind: "success", message: "Agent suspended." });
  });
});
