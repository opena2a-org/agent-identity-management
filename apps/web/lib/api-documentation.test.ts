import { describe, expect, it } from "vitest";
import { apiDocumentation } from "@/lib/api-documentation";

const endpoint = (method: string, path: string) => {
  const found = apiDocumentation
    .flatMap((category) => category.endpoints)
    .filter((e) => e.method === method && e.path === path);
  expect(found).toHaveLength(1);
  return found[0];
};

// POST /api/v1/public/agents/register sits in the /public route group, but its
// handler registers an agent only for a signed-in user and answers every other
// caller 401. The developers page reads `requiresAuth` to decide whether the
// request it builds carries the session token, so an entry that says `false`
// documents, and sends, a call that cannot succeed.
describe("API reference: POST /api/v1/public/agents/register", () => {
  const register = () => endpoint("POST", "/api/v1/public/agents/register");

  it("is marked as needing a user token", () => {
    const entry = register();
    expect(entry.requiresAuth).toBe(true);
    expect(entry.auth).toBe("Bearer Token (JWT)");
    expect(entry.tags).not.toContain("public");
  });

  it("says what an unauthenticated caller gets and where an API key works", () => {
    const entry = register();
    expect(entry.description).toContain("401");
    expect(entry.description).toContain("POST /api/v1/agents");
    expect(entry.description).toContain("X-API-Key");
  });

  it("documents the four fields the handler requires, under the names it binds", () => {
    const entry = register();
    const required = Object.entries(entry.requestSchema?.properties ?? {})
      .filter(([, property]) => property.required)
      .map(([name]) => name)
      .sort();
    expect(required).toEqual(["agentType", "description", "displayName", "name"]);
    expect(Object.keys(JSON.parse(entry.example)).sort()).toEqual(required);
  });
});

// POST /api/v1/agents binds CreateAgentRequest
// (apps/backend/internal/application/agent_service.go), and CreateAgent refuses a
// request without name, displayName or agentType. The example used to send
// `type` and `tags`, names the request does not bind, and neither displayName
// nor agentType, so the server answered it 500.
describe("API reference: POST /api/v1/agents", () => {
  const create = () => endpoint("POST", "/api/v1/agents");

  it("marks name, displayName and agentType as required, under the names the handler binds", () => {
    const properties = create().requestSchema?.properties ?? {};
    const required = Object.entries(properties)
      .filter(([, property]) => property.required)
      .map(([name]) => name)
      .sort();
    expect(required).toEqual(["agentType", "displayName", "name"]);
    expect(Object.keys(properties)).not.toContain("type");
    expect(Object.keys(properties)).not.toContain("tags");
  });

  it("gives an example that sends every required field, and only documented ones", () => {
    const entry = create();
    const sent = Object.keys(JSON.parse(entry.example));
    expect(sent).toEqual(expect.arrayContaining(["agentType", "displayName", "name"]));
    expect(Object.keys(entry.requestSchema?.properties ?? {})).toEqual(expect.arrayContaining(sent));
  });
});

// POST /api/v1/admin/security-policies binds CreatePolicyRequest
// (apps/backend/internal/interfaces/http/handlers/security_policy_handler.go). The
// example used to send `enabled`, a name the handler does not bind, left out the
// policy type, enforcement action, severity, scope and priority, and set
// rules.action, which no evaluator reads.
describe("API reference: POST /api/v1/admin/security-policies", () => {
  const create = () => endpoint("POST", "/api/v1/admin/security-policies");
  const bound = [
    "appliesTo",
    "description",
    "enforcementAction",
    "isEnabled",
    "name",
    "policyType",
    "priority",
    "rules",
    "severityThreshold",
  ];
  const requiredByTag = ["appliesTo", "enforcementAction", "name", "policyType", "priority", "severityThreshold"];

  it("documents the fields the handler binds, and marks the ones its struct tags require", () => {
    const properties = create().requestSchema?.properties ?? {};
    expect(Object.keys(properties).sort()).toEqual(bound);
    const required = Object.entries(properties)
      .filter(([, property]) => property.required)
      .map(([name]) => name)
      .sort();
    expect(required).toEqual(requiredByTag);
  });

  it("gives an example that sends every bound field and rules a trust_score_low policy reads", () => {
    const example = JSON.parse(create().example);
    expect(Object.keys(example).sort()).toEqual(bound);
    expect(example.policyType).toBe("trust_score_low");
    expect(example.rules).toEqual({ trust_threshold: 0.5 });
  });
});
