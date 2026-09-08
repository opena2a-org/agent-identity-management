import { describe, expect, it } from "vitest";
import { normalizeViolation } from "@/lib/violations";

// The exact wire shape of domain.CapabilityViolation (apps/backend/internal/domain/capability.go).
const wire = {
  id: "9c1f0c2e-1b7a-4c8e-9d1e-0a1b2c3d4e5f",
  agent_id: "0a1b2c3d-4e5f-4a6b-8c7d-9e8f7a6b5c4d",
  agent_name: "deploy-runner",
  attempted_capability: "fs:write",
  registered_capabilities: { "fs:read": true, "net:http": true },
  severity: "high",
  trust_score_impact: -5,
  is_blocked: true,
  source_ip: "10.0.0.7",
  request_metadata: { path: "/etc/passwd" },
  created_at: "2026-08-24T20:00:00Z",
};

describe("normalizeViolation", () => {
  it("maps the backend's snake_case wire shape to the camelCase shape consumers read", () => {
    const v = normalizeViolation(wire);
    expect(v).toEqual({
      id: wire.id,
      agentId: wire.agent_id,
      agentName: "deploy-runner",
      attemptedCapability: "fs:write",
      registeredCapabilities: ["fs:read", "net:http"],
      severity: "high",
      trustScoreImpact: -5,
      isBlocked: true,
      sourceIp: "10.0.0.7",
      requestMetadata: { path: "/etc/passwd" },
      createdAt: "2026-08-24T20:00:00Z",
    });
  });

  it("keeps a blocked violation blocked (the field the hero card decides on)", () => {
    expect(normalizeViolation({ ...wire, is_blocked: false }).isBlocked).toBe(false);
    expect(normalizeViolation(wire).isBlocked).toBe(true);
  });

  it("passes an already camelCase payload through unchanged", () => {
    const camel = normalizeViolation(wire);
    expect(normalizeViolation(camel as unknown as Record<string, unknown>)).toEqual(camel);
  });

  it("accepts a capability list as well as a keyed object", () => {
    expect(normalizeViolation({ ...wire, registered_capabilities: ["a", "b"] }).registeredCapabilities).toEqual(["a", "b"]);
    expect(normalizeViolation({ ...wire, registered_capabilities: undefined }).registeredCapabilities).toEqual([]);
  });
});
