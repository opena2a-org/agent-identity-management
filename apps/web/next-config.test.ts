import { describe, expect, it } from "vitest";
import { readdirSync, statSync } from "node:fs";
import { join } from "node:path";
import loadCustomRoutes from "next/dist/lib/load-custom-routes";
import { normalizeConfig } from "next/dist/server/config-shared";
import { PHASE_PRODUCTION_SERVER } from "next/dist/shared/lib/constants";
import { getPathMatch } from "next/dist/shared/lib/router/utils/path-match";
import { compileNonPath, matchHas } from "next/dist/shared/lib/router/utils/prepare-destination";
import nextConfig from "./next.config.js";
import { contentSecurityPolicy } from "./lib/content-security-policy";

// The backend serves the Prometheus exposition on its own listener, loopback
// by default. A dashboard rewrite to the backend's /metrics would publish it
// on the dashboard origin, so none may exist. Root-level on purpose, like
// routing.test.ts.

type Rewrite = { source: string; destination: string };

const config = nextConfig as unknown as {
  rewrites: () => Promise<{ afterFiles: Rewrite[] }>;
};

// A rule proxies /metrics when it matches /metrics, when it sends a request to
// the backend's /metrics, or when it forwards every root path.
function proxiesMetrics(r: Rewrite): boolean {
  const dest = new URL(r.destination).pathname;
  return (
    r.source === "/metrics" ||
    r.source.startsWith("/metrics/") ||
    dest === "/metrics" ||
    dest.startsWith("/metrics/") ||
    /^\/:[A-Za-z_]+[*+?]?$/.test(r.source)
  );
}

describe("dashboard rewrites", () => {
  it("do not proxy /metrics to the backend", async () => {
    const { afterFiles } = await config.rewrites();

    // Control: the rewrites to the backend are being read.
    expect(afterFiles.map((r) => r.source)).toContain("/health");

    expect(afterFiles.filter(proxiesMetrics)).toEqual([]);
  });
});

// The headers the server sends for a request, worked out with the server's own
// route code the way it applies header rules: every rule whose source and `has`
// match, in order, with the path and `has` captures substituted into the value,
// and a later rule replacing an earlier one's value.
async function servedHeaders(pathname: string, host: string): Promise<Record<string, string>> {
  const resolved = await normalizeConfig(PHASE_PRODUCTION_SERVER, nextConfig);
  const { headers: rules } = await loadCustomRoutes(resolved);
  const served: Record<string, string> = {};
  for (const rule of rules) {
    let params = getPathMatch(rule.source, { strict: true, removeUnnamedParams: true })(pathname);
    if (params && (rule.has || rule.missing)) {
      const captured = matchHas({ headers: { host } } as never, {}, rule.has, rule.missing);
      params = captured ? Object.assign(params, captured) : false;
    }
    if (!params) continue;
    const substitute = Object.keys(params).length > 0;
    for (const { key, value } of rule.headers) {
      served[key.toLowerCase()] = substitute ? compileNonPath(value, params) : value;
    }
  }
  return served;
}

// Every page under app/, with a placeholder for each dynamic segment.
function pagePaths(dir = "app", out: string[] = []): string[] {
  for (const entry of readdirSync(join(__dirname, dir))) {
    const rel = join(dir, entry);
    if (statSync(join(__dirname, rel)).isDirectory()) pagePaths(rel, out);
    else if (entry === "page.tsx") {
      const segments = dir
        .split("/")
        .slice(1)
        .filter((s) => !/^\(.*\)$/.test(s))
        .map((s) => (/^\[.*\]$/.test(s) ? "x" : s));
      out.push(`/${segments.join("/")}`);
    }
  }
  return out;
}

function connectSources(policy: string): string[] {
  const directive = policy.split(";").map((d) => d.trim()).find((d) => d.startsWith("connect-src "));
  return directive ? directive.split(/\s+/).slice(1) : [];
}

describe("dashboard Content-Security-Policy", () => {
  const apiUrl = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";

  it("is served on every page", async () => {
    const pages = pagePaths();
    // Control: the walk finds the shell's pages.
    expect(pages).toEqual(expect.arrayContaining(["/", "/dashboard", "/dashboard/agents/x"]));

    for (const page of pages) {
      const served = await servedHeaders(page, "localhost");
      expect(served["content-security-policy"], page).toBe(contentSecurityPolicy({ apiUrl }));
    }
  });

  it("lets a -frontend host connect to the backend the client derives from it", async () => {
    const served = await servedHeaders("/dashboard/agents", "aim-frontend.blue-1.eastus.azurecontainerapps.io");
    expect(connectSources(served["content-security-policy"])).toEqual([
      "'self'",
      new URL(apiUrl).origin,
      "aim-backend.blue-1.eastus.azurecontainerapps.io",
    ]);

    // A host with no domain part, as on a container network.
    const bare = await servedHeaders("/", "aim-frontend");
    expect(connectSources(bare["content-security-policy"])).toContain("aim-backend");
  });
});
