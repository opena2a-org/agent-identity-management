import { describe, expect, it } from "vitest";
import nextConfig from "./next.config.js";

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
