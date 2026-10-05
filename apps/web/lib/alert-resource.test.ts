import { describe, it, expect } from "vitest";
import { contextLabel, contextValue, hasResourceId, isUserResource } from "./alert-resource";

describe("alert resource", () => {
  it("treats only the user resource type as a user", () => {
    expect(isUserResource("user")).toBe(true);
    expect(isUserResource("agent")).toBe(false);
    expect(isUserResource("mcp_server")).toBe(false);
    expect(isUserResource(undefined)).toBe(false);
  });

  it("does not count the nil UUID as a resource ID", () => {
    expect(hasResourceId("00000000-0000-0000-0000-000000000000")).toBe(false);
    expect(hasResourceId("")).toBe(false);
    expect(hasResourceId(undefined)).toBe(false);
    expect(hasResourceId("9a8b7c6d-5e4f-4a3b-8c2d-1e0f9a8b7c6d")).toBe(true);
  });
});

describe("alert context", () => {
  it("labels clientMatch in words and other keys from their name", () => {
    expect(contextLabel("clientMatch")).toBe("Presented by");
    expect(contextLabel("failureCount")).toBe("failure Count");
  });

  it("prints an unrecognised clientMatch as it was recorded", () => {
    expect(contextValue("clientMatch", "someFutureValue")).toBe("someFutureValue");
    expect(contextValue("clientMatch", "constructor")).toBe("constructor");
  });

  it("leaves other values as before", () => {
    expect(contextValue("failureCount", 5)).toBe("5");
    expect(contextValue("eventType", "differentClient")).toBe("differentClient");
    expect(contextValue("details", { a: 1 })).toBe('{"a":1}');
  });
});
