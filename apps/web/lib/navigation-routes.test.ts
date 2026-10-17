import { afterAll, describe, expect, it } from "vitest";
import { mkdirSync, mkdtempSync, readdirSync, rmSync, statSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { dirname, join } from "node:path";
import { navigationBase } from "@/lib/navigation";
import { HUB_TABS } from "@/lib/hub-tabs";
import { ROUTE_MOVES, type RouteMove } from "@/lib/redirects";

/**
 * Every link the navigation renders opens a page on the tree it runs in. navigation.ts and
 * hub-tabs.ts can reach a tree before the pages they name, so each sidebar entry's href, each
 * of its hrefByRole values and each hub tab's href is checked against the page files under
 * app/ of this checkout. A link passes when a page file serves exactly its path, or when its
 * path is the source of a lib/redirects.ts row whose destination a page file serves.
 *
 * An ossOnly tab is checked like the rest. If a hosted tree drops the page behind one, exempt
 * that href by name on that tree, never the ossOnly flag as a whole.
 */

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
    for (const tab of tabs) links.push({ label: `${hub} tab ${tab.name}`, href: tab.href });
  }
  return links;
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
    expect(problems(links, pages, ROUTE_MOVES)).toEqual([]);
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
