// @vitest-environment node
import { describe, expect, it } from "vitest";
import { execFileSync, spawnSync } from "node:child_process";
import { resolve } from "node:path";

// TypeScript's incremental build cache (tsconfig.tsbuildinfo, written because
// tsconfig.json sets "incremental": true) is rewritten as one long line by every
// typecheck. Tracked, it puts that churn into unrelated commits and pushes their
// diffs over review budgets. The repo-root .gitignore keeps it out of a normal
// `git add`; this catches a force-add and a removed rule across the repository.
const BUILD_CACHE = /\.tsbuildinfo$/;
const repoRoot = resolve(__dirname, "..", "..");

describe("TypeScript build cache files", () => {
  it("none are tracked anywhere in the repository", () => {
    const tracked = execFileSync("git", ["ls-files", "-z"], { cwd: repoRoot, encoding: "utf8" })
      .split("\0")
      .filter(Boolean);
    expect(tracked.length).toBeGreaterThan(0);
    expect(tracked.filter((file) => BUILD_CACHE.test(file))).toEqual([]);
  });

  it.each(["apps/web/tsconfig.tsbuildinfo", "sdk/typescript/tsconfig.tsbuildinfo"])(
    "%s is ignored by .gitignore",
    (probe) => {
      // A personal global excludes file must not make this pass on one machine only.
      const res = spawnSync(
        "git",
        ["-c", "core.excludesFile=/dev/null", "check-ignore", "--no-index", "-q", probe],
        { cwd: repoRoot },
      );
      expect(res.status, `${probe} is not matched by any ignore rule`).toBe(0);
    },
  );
});
