import { describe, expect, it } from "vitest";
import { filterNavigationByRole } from "@/lib/permissions";
import { navigationBase } from "@/lib/navigation";
import { ALL_ROLES, ROUTE_PERMISSIONS, effectiveEdgeRoles } from "@/lib/route-permissions";
import { tabsForRole } from "@/components/mobile-tab-bar";

/**
 * The navigation and the edge gate must agree: nothing the navigation renders for a role may
 * be a route the edge redirects that role away from. The reverse (edge wider than nav) is
 * deliberate and not asserted. Both structures are imported, never restated, so a new nav
 * item or gate entry is checked automatically.
 */
describe("navigation never shows a route the edge gate blocks", () => {
  for (const role of ALL_ROLES) {
    it(`role=${role}: every rendered nav href passes the edge intersection`, () => {
      const sections = filterNavigationByRole(navigationBase, role);
      for (const section of sections) {
        for (const item of section.items) {
          expect(
            effectiveEdgeRoles(item.href),
            `${item.name} (${item.href}) is shown to ${role} but the edge blocks it`
          ).toContain(role);
        }
      }
    });

    it(`role=${role}: every mobile tab resolves for the role`, () => {
      for (const tab of tabsForRole(role)) {
        const path = tab.href.split("?")[0];
        expect(
          effectiveEdgeRoles(path),
          `tab ${tab.name} (${tab.href}) is shown to ${role} but the edge blocks it`
        ).toContain(role);
      }
    });
  }

  it("documents the intersection hazard: a specific entry cannot widen a broader prefix", () => {
    // /dashboard/admin/alerts lists managers, but /dashboard/admin narrows it to admin.
    expect(ROUTE_PERMISSIONS["/dashboard/admin/alerts"]).toContain("manager");
    expect(effectiveEdgeRoles("/dashboard/admin/alerts")).toEqual(["admin"]);
  });
});
