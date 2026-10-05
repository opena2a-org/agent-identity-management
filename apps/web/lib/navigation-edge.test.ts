import { describe, expect, it } from "vitest";
import { filterNavigationByRole } from "@/lib/permissions";
import { navigationBase, resolveNavHref } from "@/lib/navigation";
import { HUB_TABS, hubTabsForPath, visibleHubTabs } from "@/lib/hub-tabs";
import { HOSTED_DEPLOYMENT } from "@/lib/deployment";
import { ALL_ROLES, ROUTE_PERMISSIONS, effectiveEdgeRoles } from "@/lib/route-permissions";
import { ROUTE_MOVES } from "@/lib/redirects";
import { tabsForRole } from "@/components/mobile-tab-bar";

/**
 * The navigation and the edge gate must agree: nothing the navigation renders for a role may
 * be a route the edge redirects that role away from. That covers the sidebar entries (with
 * their per-role resolved hrefs), every hub tab (components/hub-tabs.tsx renders from the
 * same HUB_TABS data), and the mobile tabs. The reverse (edge wider than nav) is deliberate
 * and not asserted. All structures are imported, never restated, so a new entry, tab or gate
 * entry is checked automatically.
 */
describe("navigation never shows a route the edge gate blocks", () => {
  for (const role of ALL_ROLES) {
    it(`role=${role}: every rendered nav entry resolves past the edge intersection`, () => {
      for (const entry of filterNavigationByRole(navigationBase, role)) {
        const href = resolveNavHref(entry, role);
        expect(
          effectiveEdgeRoles(href),
          `${entry.name} (${href}) is shown to ${role} but the edge blocks it`
        ).toContain(role);
      }
    });

    it(`role=${role}: every rendered hub tab passes the edge intersection`, () => {
      for (const [hub, tabs] of Object.entries(HUB_TABS)) {
        for (const tab of visibleHubTabs(tabs, role)) {
          expect(
            effectiveEdgeRoles(tab.href),
            `${hub} tab ${tab.name} (${tab.href}) is shown to ${role} but the edge blocks it`
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

  it("the IA is exactly seven flat entries, and every hub key is one of them", () => {
    expect(navigationBase).toHaveLength(7);
    const keys = navigationBase.map((e) => e.key);
    expect(new Set(keys).size).toBe(7);
    for (const hub of Object.keys(HUB_TABS)) {
      expect(keys, `hub "${hub}" has no matching nav entry`).toContain(hub);
    }
  });

  it("an ossOnly tab renders exactly when this build is not hosted", () => {
    // Hosted signups auto-approve, so the Registrations queue can never fill
    // there. This file is byte-identical in both trees; the
    // deployment flag differs, so each tree asserts its own truth.
    const names = visibleHubTabs(HUB_TABS.organization, "admin").map((t) => t.name);
    expect(names.includes("Registrations")).toBe(!HOSTED_DEPLOYMENT);
    expect(visibleHubTabs(HUB_TABS.organization, undefined)).toEqual([]);
  });

  it("the webhooks gate entry matches the backend allow-list it traces to", () => {
    // First nav exposure of /dashboard/webhooks ships with its explicit gate
    // entry in the same commit; the role set mirrors MemberMiddleware.
    expect(ROUTE_PERMISSIONS["/dashboard/webhooks"]).toEqual(["admin", "manager", "member"]);
  });

  it("compliance has its own path and gate entry, with the roles the old path had", () => {
    // /dashboard/compliance no longer inherits "/dashboard/admin", so it names its roles
    // itself. The nav entry, the gate entry and the redirected old path must all agree, and
    // the set stays admin-only (AdminMiddleware on the backend's /compliance group).
    const entry = navigationBase.find((e) => e.key === "compliance");
    expect(entry?.href).toBe("/dashboard/compliance");
    expect(ROUTE_PERMISSIONS["/dashboard/compliance"]).toEqual(entry?.roles);
    expect(effectiveEdgeRoles("/dashboard/compliance")).toEqual(["admin"]);
    expect(ROUTE_MOVES).toContainEqual({ source: "/dashboard/admin/compliance", destination: "/dashboard/compliance" });
    expect(effectiveEdgeRoles("/dashboard/admin/compliance")).toEqual(effectiveEdgeRoles("/dashboard/compliance"));
  });

  it("credentials replaces the API keys and SDK tokens tabs, with the roles both had", () => {
    // One Developers tab for both kinds, and both old paths redirect to it. Neither old path
    // had a gate entry, so the new path has none either: all three pass every role, and the
    // tab shows to the roles the two old tabs showed to.
    const tab = HUB_TABS.developers.find((t) => t.href === "/dashboard/credentials");
    expect(tab?.name).toBe("Credentials");
    expect(tab?.roles).toEqual(["admin", "manager", "member"]);
    expect(hubTabsForPath("/dashboard/credentials")).toBe(HUB_TABS.developers);
    expect(effectiveEdgeRoles("/dashboard/credentials")).toEqual(ALL_ROLES);
    for (const old of ["/dashboard/api-keys", "/dashboard/sdk-tokens"]) {
      expect(HUB_TABS.developers.map((t) => t.href)).not.toContain(old);
      expect(ROUTE_MOVES).toContainEqual({ source: old, destination: "/dashboard/credentials" });
      expect(effectiveEdgeRoles(old)).toEqual(effectiveEdgeRoles("/dashboard/credentials"));
    }
  });

  it("nothing the navigation renders points at a moved path", () => {
    // A moved path still answers, through its redirect, but the navigation links to the
    // destination so a click is one request and the active state matches the URL.
    const moved = ROUTE_MOVES.map((m) => m.source);
    for (const role of ALL_ROLES) {
      const hrefs = [
        ...filterNavigationByRole(navigationBase, role).map((e) => resolveNavHref(e, role)),
        ...Object.values(HUB_TABS).flatMap((tabs) => visibleHubTabs(tabs, role).map((t) => t.href)),
        ...tabsForRole(role).map((t) => t.href),
      ];
      for (const href of hrefs) {
        expect(moved, `${href} is shown to ${role} but has moved`).not.toContain(href.split("?")[0]);
      }
    }
  });
});
