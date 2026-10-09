import { describe, it, expect, beforeEach } from "vitest";
import {
  agentDeleteFailure,
  agentDeletedNotice,
  rememberAgentDeleted,
  takeAgentDeletedNotice,
} from "./agent-delete";

function requestError(message: string, status?: number) {
  const err = new Error(message) as Error & { status?: number };
  err.status = status;
  return err;
}

describe("agentDeleteFailure", () => {
  it("never shows the server's text, even on a 500", () => {
    const dbText =
      'pq: update or delete on table "agents" violates foreign key constraint "a2a_tasks_client_agent_id_fkey" on table "a2a_tasks"';
    const failure = agentDeleteFailure(requestError(dbText, 500));
    expect(failure.reason).toBe("AIM could not delete this agent.");
    expect(JSON.stringify(failure)).not.toContain("a2a_tasks");
  });

  it("names the role on a 403", () => {
    expect(agentDeleteFailure(requestError("Insufficient permissions", 403)).reason).toBe(
      "Your role does not allow you to delete this agent."
    );
  });

  it("states that the agent is gone, with a link to the list, on a 404", () => {
    const failure = agentDeleteFailure(requestError("Resource not found", 404));
    expect(failure.reason).toBe("This agent no longer exists in your organization.");
    expect(failure.link).toEqual({ label: "Back to agents", href: "/dashboard/agents" });
  });

  it("says AIM could not be reached when the request never left", () => {
    expect(agentDeleteFailure(new TypeError("Failed to fetch")).reason).toBe(
      "AIM could not be reached, so it did not delete this agent."
    );
  });
});

describe("deleted notice carried to the agents list", () => {
  beforeEach(() => sessionStorage.clear());

  it("is read once", () => {
    rememberAgentDeleted("billing-bot");
    expect(takeAgentDeletedNotice()).toBe(agentDeletedNotice("billing-bot"));
    expect(takeAgentDeletedNotice()).toBeNull();
  });

  it("is absent when nothing was deleted", () => {
    expect(takeAgentDeletedNotice()).toBeNull();
  });
});
