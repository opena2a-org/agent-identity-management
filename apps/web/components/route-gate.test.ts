import { describe, expect, it } from "vitest";
import { gateVerdict } from "@/components/route-gate";
import { ALL_ROLES, ROUTE_PERMISSIONS, effectiveEdgeRoles } from "@/lib/route-permissions";

/**
 * The route gate's decision, exercised as the edge middleware's used to be: a
 * token in the session store either passes or names a redirect. The nav/edge
 * parity suite proves the navigation agrees with the permission map; only
 * these tests prove the gate enforces it. Mutation check: delete the "/admin"
 * entry in lib/route-permissions.ts and the viewer case goes red; restore the
 * strict atob() decode in lib/jwt-payload.ts and the base64url case goes red.
 */
const token = (payload: Record<string, unknown>, encoding: "base64" | "base64url" = "base64") =>
  `header.${Buffer.from(JSON.stringify(payload)).toString(encoding).replace(/=+$/, "")}.signature`;

const target = (to: string) => new URL(to, "http://localhost");

describe("route gate", () => {
  it("sends a signed-out visitor to login and keeps the return path", () => {
    const v = gateVerdict("/admin/registrations", null);
    expect(v.kind).toBe("login");
    const to = target(v.to ?? "");
    expect(to.pathname).toBe("/auth/login");
    expect(to.searchParams.get("returnUrl")).toBe("/admin/registrations");
  });

  it("sends a viewer who opens the registration review queue to /dashboard/forbidden", () => {
    expect(gateVerdict("/admin/registrations", token({ role: "viewer" }))).toEqual({ kind: "forbidden", to: "/dashboard/forbidden" });
  });

  it("lets an admin through to the registration review queue", () => {
    expect(gateVerdict("/admin/registrations", token({ role: "admin" })).kind).toBe("pass");
  });

  it("lets a pending account reach ungated pages and keeps it out of gated ones", () => {
    expect(gateVerdict("/dashboard", token({ role: "pending" })).kind).toBe("pass");
    expect(gateVerdict("/dashboard/security", token({ role: "pending" })).kind).toBe("forbidden");
  });

  it("sends a token whose payload is not JSON to login", () => {
    const v = gateVerdict("/dashboard", "header.not-json.signature");
    expect(v.kind).toBe("login");
    expect(target(v.to ?? "").searchParams.get("returnUrl")).toBe("/dashboard");
  });

  it("accepts the base64url alphabet real JWTs are encoded with", () => {
    const payload = { role: "admin", email: "a?b>c@example.test", sub: "~~~~" };
    expect(gateVerdict("/dashboard/admin/users", token(payload, "base64url")).kind).toBe("pass");
  });

  describe("agrees with effectiveEdgeRoles for every gated prefix and role", () => {
    for (const prefix of Object.keys(ROUTE_PERMISSIONS)) {
      for (const role of ALL_ROLES) {
        it(`${role} -> ${prefix}`, () => {
          const expected = effectiveEdgeRoles(prefix).includes(role) ? "pass" : "forbidden";
          expect(gateVerdict(prefix, token({ role })).kind).toBe(expected);
        });
      }
    }
  });
});
