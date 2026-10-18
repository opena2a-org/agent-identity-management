import { describe, expect, it } from "vitest";
import { connectSourcePolicy, dashboardHeaderRules, headerContentSecurityPolicy } from "./content-security-policy";

function directives(policy: string): Record<string, string[]> {
  return Object.fromEntries(
    policy.split(";").map((d) => {
      const [name, ...sources] = d.trim().split(/\s+/);
      return [name, sources];
    }),
  );
}

describe("headerContentSecurityPolicy", () => {
  it("pins the production policy", () => {
    expect(headerContentSecurityPolicy()).toBe(
      "default-src 'self'; " +
        "script-src 'self' 'unsafe-inline'; " +
        "style-src 'self' 'unsafe-inline'; " +
        "img-src 'self' data: blob: https:; " +
        "font-src 'self'; " +
        "connect-src 'self' http: https:; " +
        "object-src 'none'; " +
        "base-uri 'self'; " +
        "form-action 'self'; " +
        "frame-ancestors 'none'",
    );
  });

  it("allows eval only under next dev", () => {
    expect(directives(headerContentSecurityPolicy({ development: true }))["script-src"]).toContain("'unsafe-eval'");
    expect(directives(headerContentSecurityPolicy())["script-src"]).not.toContain("'unsafe-eval'");
  });
});

describe("connectSourcePolicy", () => {
  it("allows the API URL's origin, port included", () => {
    expect(connectSourcePolicy({ apiUrl: "http://localhost:8080" })).toBe("connect-src 'self' http://localhost:8080");
    expect(connectSourcePolicy({ apiUrl: "https://api.example.com/v1/" })).toBe("connect-src 'self' https://api.example.com");
  });

  it("allows nothing more than 'self' for a relative or non-HTTP URL", () => {
    for (const apiUrl of ["/", "", "javascript:alert(1)", "not a url", undefined]) {
      expect(connectSourcePolicy({ apiUrl }), String(apiUrl)).toBe("connect-src 'self'");
    }
  });

  it("appends the extra sources after the API origin", () => {
    expect(connectSourcePolicy({ apiUrl: "https://api.example.com", extraConnectSources: ["aim-backend.example.com"] })).toBe(
      "connect-src 'self' https://api.example.com aim-backend.example.com",
    );
  });

  it("holds connect-src only", () => {
    expect(Object.keys(directives(connectSourcePolicy({ apiUrl: "https://api.example.com" })))).toEqual(["connect-src"]);
  });
});

describe("dashboardHeaderRules", () => {
  it("sets the header's policy on every path, with no condition on the request", () => {
    expect(dashboardHeaderRules()).toEqual([
      { source: "/(.*)", headers: [{ key: "Content-Security-Policy", value: headerContentSecurityPolicy() }] },
    ]);
    expect(dashboardHeaderRules({ development: true })[0].headers[0].value).toBe(
      headerContentSecurityPolicy({ development: true }),
    );
  });
});
