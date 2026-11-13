// @vitest-environment node
import { describe, expect, it } from "vitest";
import { spawnSync } from "node:child_process";
import { createRequire } from "node:module";
import { dirname, join, resolve } from "node:path";
import config from "./vitest.config";

// The runner settings this suite depends on. Root-level on purpose, like
// next-config.test.ts.

const webDir = resolve(__dirname);
const vitestCli = join(dirname(createRequire(import.meta.url).resolve("vitest/package.json")), "vitest.mjs");

// Runs this directory's configuration in its own process with a file filter
// that matches no file and no command-line flag that changes the outcome. The
// timeout ends a stuck child before the per-test bound, because a synchronous
// spawn holds the worker until the child exits.
function runCollectingNoFile() {
  const env: NodeJS.ProcessEnv = { ...process.env, NO_COLOR: "1" };
  delete env.FORCE_COLOR;
  for (const key of Object.keys(env)) {
    if (key.startsWith("VITEST")) delete env[key];
  }
  return spawnSync(
    process.execPath,
    [vitestCli, "run", "--config", join(webDir, "vitest.config.ts"), "--root", webDir, "no-test-file-matches-this-filter"],
    { cwd: webDir, env, encoding: "utf8", timeout: 15_000 },
  );
}

describe("apps/web vitest configuration", () => {
  it("fails a run that collects no test file", () => {
    const res = runCollectingNoFile();
    const output = `${res.stdout ?? ""}${res.stderr ?? ""}`;
    expect(res.error).toBeUndefined();
    // The child reached collection and found nothing; only the exit code is
    // left to decide.
    expect(output).toContain("No test files found");
    expect(res.status, output).toBe(1);
  });

  it("bounds each test and each hook at 20000 ms", () => {
    expect(config.test?.testTimeout).toBe(20_000);
    expect(config.test?.hookTimeout).toBe(20_000);
  });

  it("sets no reporter and no higher slow-test threshold, so the default reporter's per-test duration lines stay", () => {
    expect(config.test?.reporters).toBeUndefined();
    expect(config.test?.slowTestThreshold ?? 300).toBeLessThanOrEqual(300);
  });
});
