// @vitest-environment node
import { describe, expect, it } from "vitest";
import { NextRequest } from "next/server";
import { middleware } from "@/middleware";
import { ALL_ROLES, ROUTE_PERMISSIONS, effectiveEdgeRoles } from "@/lib/route-permissions";

/**
 * The edge gate exercised end to end: a request carrying a role cookie either passes
 * (NextResponse.next) or is redirected. The nav/edge parity suite proves the navigation
 * agrees with the permission map; only these tests prove the middleware enforces it.
 * Mutation check: delete the "/admin" entry in lib/route-permissions.ts and the viewer case
 * goes red; restore the strict atob() decode and the base64url case goes red.
 */
const ORIGIN = "http://localhost:3000";

const token = (payload: Record<string, unknown>, encoding: "base64" | "base64url" = "base64") =>
  `header.${Buffer.from(JSON.stringify(payload)).toString(encoding).replace(/=+$/, "")}.signature`;

function request(pathname: string, cookie?: string) {
  return new NextRequest(
    new URL(pathname, ORIGIN),
    cookie ? { headers: { cookie: `access_token=${cookie}` } } : undefined
  );
}

function verdict(res: Response) {
  const location = res.headers.get("location");
  if (location) return { kind: "redirect" as const, to: new URL(location).pathname };
  return { kind: "pass" as const, to: null };
}

describe("edge gate", () => {
  it("redirects an unauthenticated request to login and keeps the return path", () => {
    const res = middleware(request("/admin/registrations"));
    expect(res.status).toBe(307);
    const to = new URL(res.headers.get("location") ?? "");
    expect(to.pathname).toBe("/auth/login");
    expect(to.searchParams.get("returnUrl")).toBe("/admin/registrations");
  });

  it("sends a viewer who opens the registration review queue to /dashboard/forbidden", () => {
    expect(verdict(middleware(request("/admin/registrations", token({ role: "viewer" }))))).toEqual({
      kind: "redirect",
      to: "/dashboard/forbidden",
    });
  });

  it("lets an admin through to the registration review queue", () => {
    expect(verdict(middleware(request("/admin/registrations", token({ role: "admin" })))).kind).toBe("pass");
  });

  it("lets a pending account reach ungated pages and keeps it out of gated ones", () => {
    expect(verdict(middleware(request("/dashboard", token({ role: "pending" })))).kind).toBe("pass");
    expect(verdict(middleware(request("/dashboard/security", token({ role: "pending" }))))).toEqual({
      kind: "redirect",
      to: "/dashboard/forbidden",
    });
  });

  it("redirects a token whose payload is not JSON to login", () => {
    expect(verdict(middleware(request("/dashboard", "not.a-jwt.at-all")))).toEqual({
      kind: "redirect",
      to: "/auth/login",
    });
  });

  it("accepts the base64url alphabet real JWTs are encoded with", () => {
    // A '?' in a claim encodes to the url-safe '_' when it lands on the last byte of a
    // triple; a strict atob() rejects that alphabet and would bounce the user to login.
    let t = "";
    for (let n = 1; n <= 3 && !/[-_]/.test(t); n++) {
      t = token({ role: "admin", name: "?".repeat(n) }, "base64url");
    }
    expect(t).toMatch(/[-_]/);
    expect(verdict(middleware(request("/dashboard/agents", t))).kind).toBe("pass");
  });

  it("leaves public and backend-proxied routes alone without a cookie", () => {
    for (const p of ["/auth/login", "/auth/register", "/auth/reset-password", "/api/v1/agents", "/health"]) {
      expect(verdict(middleware(request(p))).kind, p).toBe("pass");
    }
  });

  describe("agrees with effectiveEdgeRoles for every gated prefix and role", () => {
    for (const prefix of Object.keys(ROUTE_PERMISSIONS)) {
      for (const role of ALL_ROLES) {
        it(`${role} -> ${prefix}`, () => {
          const allowed = effectiveEdgeRoles(prefix).includes(role);
          const v = verdict(middleware(request(prefix, token({ role }))));
          expect(v.kind).toBe(allowed ? "pass" : "redirect");
          if (!allowed) expect(v.to).toBe("/dashboard/forbidden");
        });
      }
    }
  });
});
