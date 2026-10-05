import { describe, expect, it } from "vitest";
import { readdirSync, statSync } from "node:fs";
import { join } from "node:path";
import { ROUTE_MOVES, dashboardRedirects, type RouteMove } from "@/lib/redirects";

/**
 * The redirect table's contract, checked against the pages in app/. A row is a finished move:
 * it starts at a dashboard path no page serves any more and ends, in one hop, at a page that
 * exists. The checks are written once and run over the real table and over hand-made bad
 * rows, so they are known to bite while the table is still empty.
 */

// Route of every page under app/, as Next.js serves it: route groups drop out, dynamic
// segments keep their brackets ("/dashboard/agents/[id]").
function pageRoutes(dir = join(__dirname, "..", "app"), segments: string[] = [], out: string[] = []): string[] {
  for (const entry of readdirSync(dir)) {
    const abs = join(dir, entry);
    if (statSync(abs).isDirectory()) {
      pageRoutes(abs, /^\(.+\)$/.test(entry) ? segments : [...segments, entry], out);
    } else if (/^page\.(tsx|ts|jsx|js)$/.test(entry)) {
      out.push("/" + segments.join("/"));
    }
  }
  return out;
}

// The page that answers a table path, if any. ":id" in the path matches only a dynamic page
// segment; a literal segment matches its own name or a dynamic one.
function servedBy(path: string, pages: string[]): string | undefined {
  const want = path.split("/").filter(Boolean);
  return pages.find((page) => {
    const have = page.split("/").filter(Boolean);
    for (let i = 0; i < have.length; i++) {
      if (/^\[\[?\.\.\./.test(have[i])) return i < want.length || have[i].startsWith("[[");
      if (i >= want.length) return false;
      if (!/^\[.+\]$/.test(have[i]) && have[i] !== want[i]) return false;
    }
    return have.length === want.length;
  });
}

// A path under /dashboard with no host, query, hash or pattern modifier.
const DASHBOARD_PATH = /^\/dashboard(\/(:[A-Za-z]\w*|[a-z0-9-]+))*$/;
const params = (path: string) => path.split("/").filter((s) => s.startsWith(":"));

function problems(moves: readonly RouteMove[], pages: string[]): string[] {
  const found: string[] = [];
  const sources = moves.map((m) => m.source);
  for (const { source, destination } of moves) {
    for (const path of [source, destination]) {
      if (!DASHBOARD_PATH.test(path)) found.push(`${path} is not a plain dashboard path`);
    }
    if (sources.indexOf(source) !== sources.lastIndexOf(source)) found.push(`${source} is listed twice`);
    if (sources.includes(destination)) found.push(`${source} -> ${destination} is a chain: ${destination} moved again`);
    const unnamed = params(destination).filter((p) => !params(source).includes(p));
    if (unnamed.length) found.push(`${destination} uses ${unnamed.join(", ")}, which ${source} does not name`);
    if (!servedBy(destination, pages)) found.push(`${source} -> ${destination}: no page serves ${destination}`);
    const live = servedBy(source, pages);
    if (live) found.push(`${source} is still served by app${live}/page, so the redirect would hide it`);
  }
  return found;
}

describe("dashboard redirect table", () => {
  const pages = pageRoutes();

  it("reads the pages from app/", () => {
    expect(pages).toContain("/dashboard");
    expect(pages).toContain("/dashboard/agents/[id]");
  });

  it("every row is a finished move: retired source, one hop, live destination", () => {
    expect(problems(ROUTE_MOVES, pages)).toEqual([]);
  });

  it("serves every row as a permanent redirect", () => {
    expect(dashboardRedirects()).toEqual(ROUTE_MOVES.map((m) => ({ ...m, permanent: true })));
  });
});

describe("the table checks", () => {
  const pages = ["/dashboard", "/dashboard/agents/[id]", "/dashboard/api-keys", "/dashboard/developers"];
  const finished: RouteMove = { source: "/dashboard/agents/:id/success", destination: "/dashboard/agents/:id" };

  it("accept finished moves", () => {
    expect(problems([finished, { source: "/dashboard/sdk-tokens", destination: "/dashboard/developers" }], pages)).toEqual([]);
  });

  const bad: [string, RouteMove, string][] = [
    ["a destination no page serves", { source: "/dashboard/sdk-tokens", destination: "/dashboard/credentials" }, "no page serves /dashboard/credentials"],
    ["a source a page still serves", { source: "/dashboard/api-keys", destination: "/dashboard/developers" }, "still served by app/dashboard/api-keys/page"],
    ["a source outside the dashboard", { source: "/api/v1/agents", destination: "/dashboard" }, "/api/v1/agents is not a plain dashboard path"],
    ["an off-site destination", { source: "/dashboard/old", destination: "https://example.com/dashboard" }, "https://example.com/dashboard is not a plain dashboard path"],
    ["a query in the path", { source: "/dashboard/old", destination: "/dashboard/developers?tab=sdk" }, "?tab=sdk is not a plain dashboard path"],
    ["a pattern modifier", { source: "/dashboard/old/:rest*", destination: "/dashboard" }, ":rest* is not a plain dashboard path"],
    ["a parameter the source does not name", { source: "/dashboard/old", destination: "/dashboard/agents/:id" }, "uses :id, which /dashboard/old does not name"],
  ];
  it.each(bad)("reject %s", (_name, row, problem) => {
    expect(problems([row], pages).join("\n")).toContain(problem);
  });

  it("reject a duplicate source and a two-hop chain", () => {
    expect(problems([finished, finished], pages).join("\n")).toContain("/dashboard/agents/:id/success is listed twice");
    const chain: RouteMove[] = [
      { source: "/dashboard/older", destination: "/dashboard/old" },
      { source: "/dashboard/old", destination: "/dashboard/developers" },
    ];
    expect(problems(chain, pages).join("\n")).toContain("/dashboard/older -> /dashboard/old is a chain");
  });
});
