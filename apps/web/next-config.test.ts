import { describe, expect, it, vi } from "vitest";
import { existsSync, mkdirSync, mkdtempSync, readdirSync, rmSync, statSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import loadCustomRoutes from "next/dist/lib/load-custom-routes";
import { normalizeConfig } from "next/dist/server/config-shared";
import { buildCustomRoute } from "next/dist/server/lib/router-utils/filesystem";
import { PHASE_PRODUCTION_SERVER } from "next/dist/shared/lib/constants";
import { getPathMatch } from "next/dist/shared/lib/router/utils/path-match";
import { compileNonPath, matchHas } from "next/dist/shared/lib/router/utils/prepare-destination";
import nextConfig from "./next.config.js";
import { headerContentSecurityPolicy } from "./lib/content-security-policy";
import { UNDECODABLE_PATH_DESTINATION } from "./lib/undecodable-path";

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

// Route of every page under app/, as Next.js serves it: route groups drop out, dynamic
// segments keep their brackets ("/dashboard/agents/[id]").
function pageRoutes(dir = "app", segments: string[] = [], out: string[] = []): string[] {
  for (const entry of readdirSync(join(__dirname, dir))) {
    const rel = join(dir, entry);
    if (statSync(join(__dirname, rel)).isDirectory()) {
      pageRoutes(rel, /^\(.*\)$/.test(entry) ? segments : [...segments, entry], out);
    } else if (entry === "page.tsx") {
      out.push(`/${segments.join("/")}`);
    }
  }
  return out;
}

// A route's path with every dynamic segment set to value.
function withSegments(route: string, value: string): string {
  return route
    .split("/")
    .map((s) => (/^\[.*\]$/.test(s) ? value : s))
    .join("/");
}

// The page route that serves a path, if any: a dynamic segment matches one segment, a
// catch-all one or more, an optional catch-all any number.
function servedBy(path: string, routes: string[]): string | undefined {
  const literal = (s: string) => s.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
  return routes.find((route) => {
    const pattern = route
      .split("/")
      .filter(Boolean)
      .map((s) =>
        /^\[\[\.\.\..+\]\]$/.test(s)
          ? "(?:/[^/]+)*"
          : /^\[\.\.\..+\]$/.test(s)
            ? "(?:/[^/]+)+"
            : /^\[.+\]$/.test(s)
              ? "/[^/]+"
              : `/${literal(s)}`,
      )
      .join("");
    return new RegExp(`^${pattern || "/"}$`).test(path);
  });
}

// The rewrites of next.config.ts, built with the server's own route code the way the
// server builds them.
async function builtRewrites() {
  const resolved = await normalizeConfig(PHASE_PRODUCTION_SERVER, nextConfig);
  const { rewrites } = await loadCustomRoutes(resolved);
  return {
    beforeFiles: rewrites.beforeFiles.map((rule) => buildCustomRoute("before_files_rewrite", rule)),
    afterFiles: rewrites.afterFiles.map((rule) => buildCustomRoute("rewrite", rule)),
  };
}

// Where the first rule whose source matches sends a path, and the parameters the server
// adds to the rewritten request's query; null when no rule matches.
function rewrittenBy(rules: Awaited<ReturnType<typeof builtRewrites>>["afterFiles"], path: string) {
  for (const rule of rules) {
    const params = rule.match(path);
    if (params) return { destination: rule.destination, params };
  }
  return null;
}

// Next.js decodes a page's dynamic segments while it resolves the route. A path whose
// dynamic segment does not decode is rewritten before that (lib/undecodable-path.ts).
describe("dashboard paths with an escape that does not decode", () => {
  it("are rewritten, wherever a page has a dynamic segment, to a path no page serves", async () => {
    const { beforeFiles, afterFiles } = await builtRewrites();
    const routes = pageRoutes();
    const dynamic = routes.filter((route) => route.includes("["));
    // Control: the walk finds both detail pages.
    expect(dynamic).toEqual(expect.arrayContaining(["/dashboard/agents/[id]", "/dashboard/mcp/[id]"]));

    for (const route of dynamic) {
      expect(route.startsWith("/dashboard/"), route).toBe(true);
      for (const value of ["%E0%A4%A", "%", "%ff"]) {
        const path = withSegments(route, value);
        expect(rewrittenBy(beforeFiles, path), path).toEqual({ destination: UNDECODABLE_PATH_DESTINATION, params: {} });
      }
    }

    expect(servedBy(UNDECODABLE_PATH_DESTINATION, routes)).toBeUndefined();
    // Control: servedBy finds the page that serves a dynamic path.
    expect(servedBy("/dashboard/agents/x", routes)).toBe("/dashboard/agents/[id]");
    expect(existsSync(join(__dirname, "public", UNDECODABLE_PATH_DESTINATION))).toBe(false);
    expect(rewrittenBy(afterFiles, UNDECODABLE_PATH_DESTINATION)).toBeNull();
  });

  it("leave a page path that decodes, and a path relayed to the backend, as they are", async () => {
    const { beforeFiles, afterFiles } = await builtRewrites();
    for (const route of pageRoutes()) {
      for (const value of ["00000000-0000-4000-8000-000000000001", "%C3%A9"]) {
        const path = withSegments(route, value);
        expect(rewrittenBy(beforeFiles, path), path).toBeNull();
      }
    }
    for (const path of ["/api/v1/agents/%E0%A4%A", "/health/%ff"]) {
      expect(rewrittenBy(beforeFiles, path), path).toBeNull();
      // Control: the rewrite to the backend takes the path.
      expect(rewrittenBy(afterFiles, path), path).not.toBeNull();
    }
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
function pagePaths(): string[] {
  return pageRoutes().map((route) => withSegments(route, "x"));
}

function connectSources(policy: string): string[] {
  const directive = policy.split(";").map((d) => d.trim()).find((d) => d.startsWith("connect-src "));
  return directive ? directive.split(/\s+/).slice(1) : [];
}

describe("dashboard Content-Security-Policy", () => {
  it("is served on every page", async () => {
    const pages = pagePaths();
    // Control: the walk finds the shell's pages.
    expect(pages).toEqual(expect.arrayContaining(["/", "/dashboard", "/dashboard/agents/x"]));

    for (const page of pages) {
      const served = await servedHeaders(page, "localhost");
      expect(served["content-security-policy"], page).toBe(headerContentSecurityPolicy());
    }
  });

  // The header is fixed when the dashboard is built. The API origin, and the backend a
  // -frontend host derives, are listed by the connect-src policy each page carries
  // (lib/runtime-config.ts, app/layout.tsx).
  it("is the same on every Host, with a connect-src the page's policy narrows", async () => {
    const hosts = [
      "localhost",
      "aim-frontend.blue-1.eastus.azurecontainerapps.io",
      "aim-frontend",
      "dashboard.example.com",
      "x-frontend.example.com; script-src *",
    ];
    for (const host of hosts) {
      const served = await servedHeaders("/dashboard", host);
      expect(served["content-security-policy"], host).toBe(headerContentSecurityPolicy());
      expect(connectSources(served["content-security-policy"]), host).toEqual(["'self'", "http:", "https:"]);
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
      expect(development).toHaveLength(1);
      for (const sources of development) expect(sources).toContain("'unsafe-eval'");

      for (const mode of ["production", "test"]) {
        vi.stubEnv("NODE_ENV", mode);
        const served = await scriptSources();
        expect(served).toHaveLength(1);
        for (const sources of served) expect(sources, mode).not.toContain("'unsafe-eval'");
      }
    } finally {
      vi.unstubAllEnvs();
    }
  });
});

// next build writes the NEXT_PUBLIC_API_URL it reads into the browser's code. With none,
// the code resolves the API's address when it runs (getApiUrl in lib/api.ts).
describe("the dashboard build", () => {
  it("gives the browser's code no NEXT_PUBLIC_API_URL when the build reads none", async () => {
    try {
      vi.resetModules();
      vi.stubEnv("NEXT_PUBLIC_API_URL", undefined);
      const { default: built } = await import("./next.config.js");
      // Control: the configuration was evaluated again, with the variable unset.
      expect(built).not.toBe(nextConfig);
      expect(built.env?.NEXT_PUBLIC_APP_NAME).toBe("Agent Identity Management");

      expect(built.env ?? {}).not.toHaveProperty("NEXT_PUBLIC_API_URL");
    } finally {
      vi.unstubAllEnvs();
    }
  });

  it("fails after compiling when the browser's code holds http://localhost:8080 and the build was not given it", async () => {
    const hook = nextConfig.compiler?.runAfterProductionCompile;
    expect(hook).toBeTypeOf("function");
    const dir = mkdtempSync(join(tmpdir(), "aim-built-"));
    try {
      mkdirSync(join(dir, "static", "chunks"), { recursive: true });
      writeFileSync(join(dir, "static", "chunks", "a.js"), '"http://localhost:8080"');
      const afterCompile = () => hook!({ distDir: dir, projectDir: __dirname });

      vi.stubEnv("NEXT_PUBLIC_API_URL", undefined);
      await expect(afterCompile()).rejects.toThrow("given no NEXT_PUBLIC_API_URL that holds it: static/chunks/a.js");
      vi.stubEnv("NEXT_PUBLIC_API_URL", "https://api.example.com");
      await expect(afterCompile()).rejects.toThrow("static/chunks/a.js");

      vi.stubEnv("NEXT_PUBLIC_API_URL", "http://localhost:8080");
      await expect(afterCompile()).resolves.toBeUndefined();
    } finally {
      vi.unstubAllEnvs();
      rmSync(dir, { recursive: true, force: true });
    }
  });
});
