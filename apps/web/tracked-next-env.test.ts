// @vitest-environment node
import { describe, expect, it } from "vitest";
import { execFileSync, spawnSync } from "node:child_process";
import { readFileSync } from "node:fs";
import { resolve } from "node:path";

// next-env.d.ts is generated: `next dev`, `next build` and `next typegen` each
// rewrite it, and the lines they write differ by command and by Next.js version.
// Tracked, a build in a clean checkout leaves the tree modified. The repo-root
// .gitignore keeps it out of a normal `git add`; this catches a force-add and a
// removed rule across the repository, and checks that the typecheck still has
// the file on a clean checkout.
const NEXT_ENV = /(^|\/)next-env\.d\.ts$/;
const repoRoot = resolve(__dirname, "..", "..");

describe("Next.js generated type declarations", () => {
  it("next-env.d.ts is not tracked anywhere in the repository", () => {
    const tracked = execFileSync("git", ["ls-files", "-z"], { cwd: repoRoot, encoding: "utf8" })
      .split("\0")
      .filter(Boolean);
    expect(tracked.length).toBeGreaterThan(0);
    expect(tracked.filter((file) => NEXT_ENV.test(file))).toEqual([]);
  });

  it("apps/web/next-env.d.ts is ignored by .gitignore", () => {
    const probe = "apps/web/next-env.d.ts";
    // A personal global excludes file must not make this pass on one machine only.
    const res = spawnSync(
      "git",
      ["-c", "core.excludesFile=/dev/null", "check-ignore", "--no-index", "-q", probe],
      { cwd: repoRoot },
    );
    expect(res.status, `${probe} is not matched by any ignore rule`).toBe(0);
  });

  it("apps/web type-check generates next-env.d.ts before tsc reads it", () => {
    const pkg = JSON.parse(readFileSync(resolve(__dirname, "package.json"), "utf8"));
    expect(pkg.scripts["type-check"]).toMatch(/^next typegen && tsc --noEmit$/);
  });
});
