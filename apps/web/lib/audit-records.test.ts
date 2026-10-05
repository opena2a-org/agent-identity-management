import { describe, expect, it } from "vitest";
import {
  NO_ACTOR_LABEL,
  auditActionLabel,
  auditActor,
  auditTargetLabel,
  formatAuditTime,
  newestFirst,
  type AuditRecord,
} from "./audit-records";

const USER_ID = "4b1d7a52-0c9e-4f43-9a1e-2f6c8d3b5e71";
const AGENT_ID = "9e2f4c11-7b3a-4d58-8c06-1a5e9f2d7b43";

function record(overrides: Partial<AuditRecord> = {}): AuditRecord {
  return {
    id: "r1",
    action: "update",
    resourceType: "agent",
    resourceId: "5c8e1f3a-2d4b-4e6f-9a7c-0b1d2e3f4a5b",
    timestamp: "2026-10-01T13:18:21Z",
    ...overrides,
  };
}

describe("auditActor", () => {
  it("names the user the route sends for a user act", () => {
    expect(auditActor(record({ userId: USER_ID, userName: "Example User" }))).toEqual({
      kind: "user",
      id: USER_ID,
      label: "Example User",
    });
  });

  it("still attributes a user act to a user when the route sends no name", () => {
    expect(auditActor(record({ userId: USER_ID }))).toEqual({ kind: "user", id: USER_ID, label: "User 4b1d7a52" });
  });

  it("names the agent for an agent act", () => {
    expect(auditActor(record({ agentId: AGENT_ID, agentName: "example-agent" }))).toEqual({
      kind: "agent",
      id: AGENT_ID,
      label: "example-agent",
    });
    expect(auditActor(record({ agentId: AGENT_ID })).label).toBe("Agent 9e2f4c11");
  });

  it("says no user or agent was recorded instead of calling the record system", () => {
    for (const r of [
      record(),
      record({ userId: null, agentId: null }),
      record({ userId: "00000000-0000-0000-0000-000000000000" }),
    ]) {
      const actor = auditActor(r);
      expect(actor).toEqual({ kind: "none", label: NO_ACTOR_LABEL });
      expect(actor.label.toLowerCase()).not.toContain("system");
    }
  });
});

describe("row text", () => {
  it("turns route identifiers into words", () => {
    expect(auditActionLabel("refresh_token_reuse")).toBe("Refresh token reuse");
    expect(auditActionLabel("update")).toBe("Update");
    expect(auditTargetLabel("api_key")).toBe("API key");
    expect(auditTargetLabel("mcp_server")).toBe("MCP server");
    expect(auditTargetLabel("audit_logs")).toBe("Audit logs");
  });

  it("prints an absolute UTC time, and an unparseable time as sent", () => {
    expect(formatAuditTime("2026-10-01T13:18:21Z")).toBe("2026-10-01 13:18:21 UTC");
    expect(formatAuditTime("2026-10-01T07:18:21-06:00")).toBe("2026-10-01 13:18:21 UTC");
    expect(formatAuditTime("not a time")).toBe("not a time");
  });
});

describe("newestFirst", () => {
  it("orders by time descending without mutating the input", () => {
    const input = [
      record({ id: "old", timestamp: "2026-09-30T00:00:00Z" }),
      record({ id: "new", timestamp: "2026-10-01T00:00:00Z" }),
      record({ id: "bad", timestamp: "" }),
      record({ id: "mid", timestamp: "2026-09-30T12:00:00Z" }),
    ];
    expect(newestFirst(input).map((r) => r.id)).toEqual(["new", "mid", "old", "bad"]);
    expect(input.map((r) => r.id)).toEqual(["old", "new", "bad", "mid"]);
  });
});
