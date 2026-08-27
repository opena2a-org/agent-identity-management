import { describe, expect, it } from "vitest";
import { filterNavigationByRole, type UserRole } from "@/lib/permissions";
import { navigationBase, orderNavigationForPersona, resolveNavHref, type NavEntry } from "@/lib/navigation";
import { PERSONAS, personaFromSignupRole, type Persona } from "@/lib/persona";

const ROLES: (UserRole | undefined)[] = ["admin", "manager", "member", "viewer", undefined];
const hrefs = (entries: NavEntry[], role?: UserRole) =>
  entries.map((e) => resolveNavHref(e, role)).sort();

describe("persona lens never changes authorization", () => {
  for (const role of ROLES) {
    for (const { value: persona } of PERSONAS) {
      it(`role=${role ?? "none"} persona=${persona}: same hrefs as the role filter`, () => {
        const authorized = filterNavigationByRole(navigationBase, role);
        const lensed = orderNavigationForPersona(authorized, persona);
        expect(hrefs(lensed, role)).toEqual(hrefs(authorized, role));
        // Every entry keeps its roles array untouched (no lens-side widening).
        for (const entry of lensed) {
          expect(role === undefined || entry.roles.includes(role)).toBe(true);
        }
      });
    }
  }

  it("a lens for an unknown persona value falls back to the developer order", () => {
    const authorized = filterNavigationByRole(navigationBase, "admin");
    expect(orderNavigationForPersona(authorized, "nope" as Persona)).toEqual(
      orderNavigationForPersona(authorized, "developer")
    );
  });
});

describe("entry-level persona ordering", () => {
  const names = (persona: Persona, role: UserRole) =>
    orderNavigationForPersona(filterNavigationByRole(navigationBase, role), persona).map((e) => e.name);

  it("developer: Overview first, then Agents, Developers, MCP servers, Security, Compliance, Organization last", () => {
    expect(names("developer", "admin")).toEqual([
      "Overview", "Agents", "Developers", "MCP servers", "Security", "Compliance", "Organization",
    ]);
  });

  it("security: Overview first, then Security, Agents, MCP servers, Compliance, Developers, Organization last", () => {
    expect(names("security", "admin")).toEqual([
      "Overview", "Security", "Agents", "MCP servers", "Compliance", "Developers", "Organization",
    ]);
  });

  it("executive: Overview first, then Compliance, Security, Agents, MCP servers, Developers, Organization last", () => {
    expect(names("executive", "admin")).toEqual([
      "Overview", "Compliance", "Security", "Agents", "MCP servers", "Developers", "Organization",
    ]);
  });

  it("pinning survives the role filter: a member still gets Overview first and Organization last", () => {
    const ordered = names("developer", "member");
    expect(ordered[0]).toBe("Overview");
    expect(ordered[ordered.length - 1]).toBe("Organization");
    expect(ordered).toEqual(["Overview", "Agents", "Developers", "MCP servers", "Organization"]);
  });

  it("the Organization entry resolves per role: admins land on Users, others on Tags", () => {
    const organization = navigationBase.find((e) => e.key === "organization")!;
    expect(resolveNavHref(organization, "admin")).toBe("/dashboard/admin/users");
    expect(resolveNavHref(organization, "manager")).toBe("/dashboard/tags");
    expect(resolveNavHref(organization, "member")).toBe("/dashboard/tags");
  });
});

describe("personaFromSignupRole", () => {
  it("maps the signup roles onto the three lenses", () => {
    expect(personaFromSignupRole("developer")).toBe("developer");
    expect(personaFromSignupRole("student-or-researcher")).toBe("developer");
    expect(personaFromSignupRole("security-engineer")).toBe("security");
    expect(personaFromSignupRole("founder-or-exec")).toBe("executive");
    expect(personaFromSignupRole("other")).toBe("developer");
    expect(personaFromSignupRole(undefined)).toBe("developer");
  });
});
