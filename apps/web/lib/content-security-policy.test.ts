import { describe, expect, it } from "vitest";
import { contentSecurityPolicy, dashboardSecurityHeaders } from "./content-security-policy";

function directives(policy: string): Record<string, string[]> {
  return Object.fromEntries(
    policy.split(";").map((d) => {
      const [name, ...sources] = d.trim().split(/\s+/);
      return [name, sources];
    }),
  );
}

describe("contentSecurityPolicy", () => {
  it("pins the production policy", () => {
    expect(contentSecurityPolicy({ apiUrl: "https://api.example.com/v1/" })).toBe(
      "default-src 'self'; " +
        "script-src 'self' 'unsafe-inline'; " +
        "style-src 'self' 'unsafe-inline'; " +
        "img-src 'self' data: blob: https:; " +
        "font-src 'self'; " +
        "connect-src 'self' https://api.example.com; " +
        "object-src 'none'; " +
        "base-uri 'self'; " +
        "form-action 'self'; " +
        "frame-ancestors 'none'",
    );
  });

  it("allows eval only under next dev", () => {
    expect(directives(contentSecurityPolicy({ development: true }))["script-src"]).toContain("'unsafe-eval'");
    expect(directives(contentSecurityPolicy())["script-src"]).not.toContain("'unsafe-eval'");
  });

  it("allows the API URL's origin, port included, and nothing for a relative or non-HTTP URL", () => {
    expect(directives(contentSecurityPolicy({ apiUrl: "http://localhost:8080" }))["connect-src"]).toEqual([
      "'self'",
      "http://localhost:8080",
    ]);
    for (const apiUrl of ["/", "", "javascript:alert(1)", "not a url"]) {
      expect(directives(contentSecurityPolicy({ apiUrl }))["connect-src"]).toEqual(["'self'"]);
    }
  });
});

describe("dashboardSecurityHeaders", () => {
  it("captures the halves of a Host around its first -frontend", () => {
    const rule = dashboardSecurityHeaders().find((r) => r.has);
    const pattern = new RegExp(`^${rule!.has![0].value}$`);

    expect("aim-frontend.blue-1.eastus.azurecontainerapps.io".match(pattern)?.groups).toEqual({
      frontendHostPrefix: "aim",
      frontendHostSuffix: ".blue-1.eastus.azurecontainerapps.io",
    });
    expect("a-frontend-frontend.example.com".match(pattern)?.groups).toEqual({
      frontendHostPrefix: "a",
      frontendHostSuffix: "-frontend.example.com",
    });
    expect("dashboard.example.com".match(pattern)).toBeNull();
    // A Host carrying policy syntax does not match, so it never reaches the value.
    expect("x-frontend.example.com; script-src *".match(pattern)).toBeNull();
  });
});
