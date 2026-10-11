import { describe, expect, it, vi } from "vitest";
import { readdirSync, statSync } from "node:fs";
import { join } from "node:path";
import loadCustomRoutes from "next/dist/lib/load-custom-routes";
import { normalizeConfig } from "next/dist/server/config-shared";
import { PHASE_PRODUCTION_SERVER } from "next/dist/shared/lib/constants";
import { getPathMatch } from "next/dist/shared/lib/router/utils/path-match";
import { compileNonPath, matchHas } from "next/dist/shared/lib/router/utils/prepare-destination";
import nextConfig from "./next.config.js";
import { derivedBackendHost } from "./lib/backend-host";
import { contentSecurityPolicy } from "./lib/content-security-policy";

// The backend serves the Prometheus exposition on its own listener, loopback
// by default. A dashboard rewrite to the backend's /metrics would publish it
// on the dashboard origin, so none may exist. Root-level on purpose, like
// routing.test.ts.

type Rewrite = { source: string; destination: string };

const config = nextConfig as unknown as {
  rewrites: () => Promise<{ afterFiles: Rewrite[] }>;
  headers: () => Promise<Array<{ source: string; headers: Array<{ key: string; value: string }> }>>;
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

// The headers the server sends for a page, worked out with the server's own route
// code the way it applies header rules: every rule whose source and `has` match, in
// order, with the path and `has` captures substituted into the value, and a later
// rule replacing an earlier one's value. It models pages only: it also reports a
// header for a path the server redirects or rewrites to the backend, where the
// server does not set one, and it matches a source strictly, where the built server
// also sets a "/:path*" rule's header on "/".
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

  // The client and the header read one rule (lib/backend-host.ts). Where the client
  // derives a backend the header lists it; where the client derives none it calls the
  // API origin, and the header lists nothing more than that.
  it("lists a derived backend for exactly the hosts the client derives one for", async () => {
    const base = ["'self'", new URL(apiUrl).origin];
    const hosts = [
      "aim-frontend.blue-1.eastus.azurecontainerapps.io",
      "a-frontend-frontend.example.com",
      "aim-frontend",
      "dashboard.example.com",
      "my_app-frontend.test",
      "-frontend.example.com",
      "x-frontend.example.com; script-src *",
    ];
    // Control: the table holds hosts of both kinds.
    expect(hosts.map(derivedBackendHost)).toEqual(expect.arrayContaining([null, "aim-backend"]));

    for (const host of hosts) {
      const served = await servedHeaders("/dashboard", host);
      const derived = derivedBackendHost(host);
      expect(connectSources(served["content-security-policy"]), host).toEqual(derived ? [...base, derived] : base);
    }
  });

  // next dev runs with NODE_ENV=development, and React's development build needs eval.
  it("allows eval under next dev and in no other mode", async () => {
    const scriptSources = async () =>
      (await config.headers()).map(({ headers }) => {
        const policy = headers.find((h) => h.key === "Content-Security-Policy")!.value;
        return policy.split(";").map((d) => d.trim()).find((d) => d.startsWith("script-src "))!.split(/\s+/);
      });

    try {
      vi.stubEnv("NODE_ENV", "development");
      const development = await scriptSources();
      expect(development).toHaveLength(2);
      for (const sources of development) expect(sources).toContain("'unsafe-eval'");

      for (const mode of ["production", "test"]) {
        vi.stubEnv("NODE_ENV", mode);
        const served = await scriptSources();
        expect(served).toHaveLength(2);
        for (const sources of served) expect(sources, mode).not.toContain("'unsafe-eval'");
      }
    } finally {
      vi.unstubAllEnvs();
    }
  });
});
