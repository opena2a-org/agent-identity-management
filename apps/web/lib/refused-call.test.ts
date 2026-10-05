import { describe, it, expect } from "vitest";
import { refusedCallFinding, CAPABILITY_REQUESTS_PATH, SECURITY_POLICIES_PATH } from "./refused-call";

const AGENT = "agent-1";
const refused = (denialReason: string, extra: Record<string, unknown> = {}) => ({
  action: "db:write",
  metadata: { actionType: "db:write", autoApproved: false, denialReason, ...extra },
});

describe("refusedCallFinding", () => {
  it("is null for a row with no refusal reason", () => {
    expect(refusedCallFinding({ action: "db:read", metadata: { autoApproved: true } }, AGENT)).toBeNull();
    expect(refusedCallFinding({ action: "db:write", metadata: { autoApproved: false } }, AGENT)).toBeNull();
    expect(refusedCallFinding({ action: "db:write", metadata: null }, AGENT)).toBeNull();
    expect(refusedCallFinding(refused("  "), AGENT)).toBeNull();
  });

  it("is null for an allowed row, even one that carries a reason", () => {
    expect(refusedCallFinding(refused("Action allowed by security policy 'P' (alert-only mode)", { autoApproved: true }), AGENT)).toBeNull();
  });

  it("points a missing capability at its grant, naming the capability from the reason", () => {
    const reason =
      "Capability violation blocked by security policy 'Capability Violation Detection': Agent does not have permission for capability 'files:delete' (allowed: [db:read])";
    const f = refusedCallFinding(refused(reason, { resource: "orders" }), AGENT)!;
    expect(f.kind).toBe("capability");
    expect(f.what).toBe("AIM refused db:write on orders for this agent.");
    expect(f.why).toBe(reason);
    expect(f.fix.text).toMatch(/^Grant files:delete to this agent/);
    expect(f.fix.href).toBe(CAPABILITY_REQUESTS_PATH);
    expect(f.fix.request).toBe(`POST /api/v1/agents/${AGENT}/capabilities {"capabilityType":"files:delete"}`);
  });

  it("points an agent with no granted capabilities at the grant for the refused one", () => {
    const f = refusedCallFinding(
      refused("Agent has no granted capabilities - action denied by strict mode (admin must grant capabilities first)"),
      AGENT
    )!;
    expect(f.kind).toBe("capability");
    expect(f.fix.request).toContain('{"capabilityType":"db:write"}');
  });

  it("points an unverified agent at Verify agent", () => {
    const f = refusedCallFinding(refused("Agent not verified - all actions denied"), AGENT)!;
    expect(f.kind).toBe("unverified");
    expect(f.fix.text).toMatch(/Verify agent/);
    expect(f.fix.href).toBeUndefined();
  });

  it("points a compromised agent at its security review", () => {
    const f = refusedCallFinding(refused("Agent is marked as compromised - all actions denied"), AGENT)!;
    expect(f.kind).toBe("compromised");
    expect(f.fix.href).toBeUndefined();
  });

  it("names the policy that refused the call", () => {
    const f = refusedCallFinding(
      refused("Action blocked by trust score policy 'Low Trust Block': Agent trust score too low (0.20)"),
      AGENT
    )!;
    expect(f.kind).toBe("policy");
    expect(f.fix.text).toContain("'Low Trust Block'");
    expect(f.fix.href).toBe(SECURITY_POLICIES_PATH);
  });

  it("keeps an unrecognised reason verbatim and still gives a fix", () => {
    const f = refusedCallFinding({ action: "export_all_data", metadata: { denialReason: "Verification error: timeout" } }, AGENT)!;
    expect(f.kind).toBe("other");
    expect(f.capability).toBe("export_all_data");
    expect(f.what).toBe("AIM refused export_all_data for this agent.");
    expect(f.why).toBe("Verification error: timeout");
    expect(f.fix.text).not.toBe("");
  });
});
