import { describe, expect, it } from "vitest";
import { filterNavigationByRole, type UserRole } from "@/lib/permissions";
import { navigationBase, orderNavigationForPersona } from "@/lib/navigation";
import { PERSONAS, personaFromSignupRole, type Persona } from "@/lib/persona";

const ROLES: (UserRole | undefined)[] = ["admin", "manager", "member", "viewer", undefined];
const hrefs = (sections: ReturnType<typeof filterNavigationByRole>) =>
  sections.flatMap((s) => s.items.map((i) => i.href)).sort();

describe("persona lens never changes authorization", () => {
  for (const role of ROLES) {
    for (const { value: persona } of PERSONAS) {
      it(`role=${role ?? "none"} persona=${persona}: same hrefs as the role filter`, () => {
        const authorized = filterNavigationByRole(navigationBase, role);
        const lensed = orderNavigationForPersona(authorized, persona);
        expect(hrefs(lensed)).toEqual(hrefs(authorized));
        // Every item keeps its roles array untouched (no lens-side widening).
        for (const section of lensed) {
          for (const item of section.items) {
            expect(role === undefined || item.roles.includes(role)).toBe(true);
          }
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
