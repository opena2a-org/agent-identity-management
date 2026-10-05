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
