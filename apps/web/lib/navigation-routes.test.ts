import { afterAll, describe, expect, it } from "vitest";
import { mkdirSync, mkdtempSync, readdirSync, rmSync, statSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { dirname, join } from "node:path";
import { navigationBase } from "@/lib/navigation";
import { HUB_TABS, type HubTab } from "@/lib/hub-tabs";
import { HOSTED_DEPLOYMENT } from "@/lib/deployment";
import { ROUTE_MOVES, type RouteMove } from "@/lib/redirects";

/**
 * Every link the navigation renders opens a page on the tree it runs in. navigation.ts and
 * hub-tabs.ts can reach a tree before the pages they name, so each sidebar entry's href, each
 * of its hrefByRole values and each hub tab's href is checked against the page files under
 * app/ of this checkout. A link passes when a page file serves exactly its path, or when its
 * path is the source of a lib/redirects.ts row whose destination a page file serves.
 *
 * An ossOnly tab is checked like the rest. If a hosted tree drops the page behind one, add
 * that href to HOSTED_EXEMPT_HREFS below, never a skip of the ossOnly flag as a whole.
 */

// Hosted exemptions, one href per row. A row names the href of an ossOnly hub tab whose page a
// hosted tree does not ship: visibleHubTabs, which the tab strip lists its tabs through, drops
// an ossOnly tab where HOSTED_DEPLOYMENT is true. A row takes effect only where
// HOSTED_DEPLOYMENT is true, and only for ossOnly tabs with that href; the open-source tree
// checks every link. The flag comes from lib/deployment.ts, which each tree keeps its own copy
// of; the rows are written here, so both trees carry the same list.
const HOSTED_EXEMPT_HREFS: readonly string[] = [];

// Every page file under app/, keyed by the path Next.js serves it at. A route group folder
// ("(marketing)") adds no path segment, and a private folder ("_components") serves nothing.
// A dynamic segment keeps its brackets ("/dashboard/mcp/[id]"), so a dynamic page never
// answers a link: without its own page, /dashboard/mcp/discovery would open the
// app/dashboard/mcp/[id] page with the id "discovery".
function pageFiles(dir: string, segments: string[] = [], rel = "app", out = new Map<string, string>()) {
  for (const entry of readdirSync(dir).sort()) {
    const abs = join(dir, entry);
    if (statSync(abs).isDirectory()) {
      if (entry.startsWith("_")) continue;
      const next = /^\(.+\)$/.test(entry) ? segments : [...segments, entry];
      pageFiles(abs, next, `${rel}/${entry}`, out);
    } else if (/^page\.(tsx|ts|jsx|js)$/.test(entry)) {
      out.set("/" + segments.join("/"), `${rel}/${entry}`);
    }
  }
  return out;
}

interface NavLink {
  label: string;
  href: string;
  ossOnly?: boolean;
}

function navigationLinks(): NavLink[] {
  const links: NavLink[] = [];
  for (const entry of navigationBase) {
    links.push({ label: `nav entry ${entry.name}`, href: entry.href });
    for (const [role, href] of Object.entries(entry.hrefByRole ?? {})) {
      if (href !== undefined) links.push({ label: `nav entry ${entry.name} for ${role}`, href });
    }
  }
  for (const [hub, tabs] of Object.entries(HUB_TABS)) {
    for (const tab of tabs) links.push({ label: `${hub} tab ${tab.name}`, href: tab.href, ossOnly: tab.ossOnly });
  }
  return links;
}

// The links to check on this tree. On a hosted tree, an ossOnly tab whose href has a row in
// the exemption list is left out. A sidebar entry or a tab without the flag is kept, even with
// an exempted href, and the open-source tree keeps every link.
function linksToCheck(links: NavLink[], hosted: boolean, exempt: readonly string[]): NavLink[] {
  if (!hosted) return links;
  return links.filter((l) => !(l.ossOnly && exempt.includes(l.href)));
}

// One line per exemption row that names no ossOnly hub tab: a row may only exempt a tab the
// hosted build never renders.
function exemptionProblems(exempt: readonly string[], hubTabs: Record<string, HubTab[]>): string[] {
  const ossOnly = new Set(Object.values(hubTabs).flat().filter((t) => t.ossOnly).map((t) => t.href));
  return exempt
    .filter((href) => !ossOnly.has(href))
    .map((href) => `hosted exemption ${href} names no ossOnly hub tab: only a tab the hosted build hides may be exempt`);
}

// One line per link no page serves, naming the href and the page file it needs. The path is
// the href before any "?" or "#". A redirect row is matched first, as Next.js runs redirects
// before it reads the filesystem.
function problems(links: NavLink[], pages: Map<string, string>, moves: readonly RouteMove[]): string[] {
  const found: string[] = [];
  for (const { label, href } of links) {
    const path = href.split(/[?#]/)[0];
    const move = moves.find((m) => m.source === path);
    const target = move ? move.destination : path;
    if (pages.has(target)) continue;
    const expected = `app${target === "/" ? "" : target}/page.tsx`;
    found.push(
      move
        ? `${label} links to ${href}, which lib/redirects.ts sends to ${target}, and no page serves ${target}: expected ${expected}`
        : `${label} links to ${href}, and no page serves it and lib/redirects.ts has no row for it: expected ${expected}`
    );
  }
  return found;
}

describe("every navigation link opens a page on this tree", () => {
  const pages = pageFiles(join(__dirname, "..", "app"));
  const links = navigationLinks();

  it("reads the pages from app/", () => {
    expect(pages.get("/dashboard")).toBe("app/dashboard/page.tsx");
    expect(pages.get("/dashboard/agents/[id]")).toBe("app/dashboard/agents/[id]/page.tsx");
  });

  it("walks each sidebar href, each per-role href and each hub tab, ossOnly tabs included", () => {
    const hrefs = links.map((l) => l.href);
    for (const entry of navigationBase) {
      expect(hrefs).toContain(entry.href);
      for (const href of Object.values(entry.hrefByRole ?? {})) expect(hrefs).toContain(href);
    }
    for (const tabs of Object.values(HUB_TABS)) {
      for (const tab of tabs) expect(hrefs).toContain(tab.href);
    }
    expect(Object.values(HUB_TABS).flat().some((t) => t.ossOnly)).toBe(true);
  });

  it("every link resolves to a page file, directly or through one redirect row", () => {
    expect(problems(linksToCheck(links, HOSTED_DEPLOYMENT, HOSTED_EXEMPT_HREFS), pages, ROUTE_MOVES)).toEqual([]);
  });

  it("each hosted exemption names an ossOnly hub tab", () => {
    expect(exemptionProblems(HOSTED_EXEMPT_HREFS, HUB_TABS)).toEqual([]);
  });
});

describe("a hosted exemption", () => {
  const links: NavLink[] = [
    { label: "nav entry Organization for admin", href: "/admin/registrations" },
    { label: "organization tab Registrations", href: "/admin/registrations", ossOnly: true },
    { label: "organization tab Invitations", href: "/admin/invitations", ossOnly: true },
    { label: "organization tab Audit log", href: "/dashboard/admin/audit-logs" },
  ];
  const labels = (kept: NavLink[]) => kept.map((l) => l.label);

  it("leaves every link checked on the open-source tree", () => {
    expect(linksToCheck(links, false, ["/admin/registrations"])).toEqual(links);
  });

  it("leaves out only the ossOnly tab it names on a hosted tree", () => {
    expect(labels(linksToCheck(links, true, ["/admin/registrations"]))).toEqual([
      "nav entry Organization for admin",
      "organization tab Invitations",
      "organization tab Audit log",
    ]);
  });

  it("never leaves out a link without the ossOnly flag", () => {
    expect(linksToCheck(links, true, ["/dashboard/admin/audit-logs"])).toEqual(links);
  });

  it("rejects a row that names a tab shown on both builds, or no tab", () => {
    expect(exemptionProblems(["/admin/registrations", "/dashboard/admin/audit-logs", "/dashboard/gone"], HUB_TABS)).toEqual([
      "hosted exemption /dashboard/admin/audit-logs names no ossOnly hub tab: only a tab the hosted build hides may be exempt",
      "hosted exemption /dashboard/gone names no ossOnly hub tab: only a tab the hosted build hides may be exempt",
    ]);
  });
});

describe("the link check", () => {
  const pages = new Map([
    ["/dashboard", "app/dashboard/page.tsx"],
    ["/dashboard/credentials", "app/dashboard/credentials/page.tsx"],
    ["/dashboard/mcp/[id]", "app/dashboard/mcp/[id]/page.tsx"],
  ]);
  const moves: RouteMove[] = [
    { source: "/dashboard/api-keys", destination: "/dashboard/credentials" },
    { source: "/dashboard/old-audit", destination: "/dashboard/admin/audit-logs" },
  ];
  const link = (href: string): NavLink[] => [{ label: "organization tab Audit log", href }];

  it("accepts a link a page serves, with or without a query", () => {
    expect(problems(link("/dashboard/credentials"), pages, moves)).toEqual([]);
    expect(problems(link("/dashboard/credentials?tab=sdk"), pages, moves)).toEqual([]);
  });

  it("accepts a link a redirect row sends to a page", () => {
    expect(problems(link("/dashboard/api-keys"), pages, moves)).toEqual([]);
  });

  it("rejects a link no page serves, naming the href and the file it expected", () => {
    expect(problems(link("/dashboard/admin/audit-logs"), pages, moves)).toEqual([
      "organization tab Audit log links to /dashboard/admin/audit-logs, and no page serves it and lib/redirects.ts has no row for it: expected app/dashboard/admin/audit-logs/page.tsx",
    ]);
  });

  it("rejects a link a redirect row sends to a path no page serves", () => {
    expect(problems(link("/dashboard/old-audit"), pages, moves)).toEqual([
      "organization tab Audit log links to /dashboard/old-audit, which lib/redirects.ts sends to /dashboard/admin/audit-logs, and no page serves /dashboard/admin/audit-logs: expected app/dashboard/admin/audit-logs/page.tsx",
    ]);
  });

  it("rejects a link only a dynamic page would answer", () => {
    expect(problems(link("/dashboard/mcp/discovery"), pages, moves).join("\n")).toContain(
      "expected app/dashboard/mcp/discovery/page.tsx"
    );
  });
});

describe("the page walk", () => {
  const root = mkdtempSync(join(tmpdir(), "nav-routes-"));
  afterAll(() => rmSync(root, { recursive: true, force: true }));
  for (const file of [
    "page.tsx",
    "(console)/dashboard/admin/audit-logs/page.tsx",
    "dashboard/mcp/[id]/page.tsx",
    "dashboard/_parts/page.tsx",
    "dashboard/tags/page.test.tsx",
  ]) {
    mkdirSync(dirname(join(root, file)), { recursive: true });
    writeFileSync(join(root, file), "export default function Page() { return null; }\n");
  }

  it("drops route group folders, skips private folders and test files, and keeps dynamic brackets", () => {
    expect([...pageFiles(root)]).toEqual([
      ["/dashboard/admin/audit-logs", "app/(console)/dashboard/admin/audit-logs/page.tsx"],
      ["/dashboard/mcp/[id]", "app/dashboard/mcp/[id]/page.tsx"],
      ["/", "app/page.tsx"],
    ]);
  });
});
