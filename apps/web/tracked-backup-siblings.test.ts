// @vitest-environment node
import { describe, expect, it } from "vitest";
import { execFileSync, spawnSync } from "node:child_process";
import { resolve } from "node:path";

// Hand-made backup siblings of source files (page.tsx.bak, .bkp, .final, .fmt)
// are stale copies that scanners and readers take for live code, and the
// frontend image builds copy apps/web/ wholesale. The repo-root .gitignore keeps
// new ones out of a normal `git add`; this catches a force-add and a removed rule.
// It lives in the frontend suite because that job runs on every push and pull
// request, but it covers the whole repository.
const BACKUP_SIBLING = /\.(bak|bkp|final|fmt)$/;
const repoRoot = resolve(__dirname, "..", "..");

describe("backup sibling files", () => {
  it("none are tracked anywhere in the repository", () => {
    const tracked = execFileSync("git", ["ls-files", "-z"], { cwd: repoRoot, encoding: "utf8" })
      .split("\0")
      .filter(Boolean);
    expect(tracked.length).toBeGreaterThan(0);
    expect(tracked.filter((file) => BACKUP_SIBLING.test(file))).toEqual([]);
  });

  it.each(["bak", "bkp", "final", "fmt"])("*.%s is ignored by .gitignore", (ext) => {
    const probe = `apps/web/app/dashboard/page.tsx.${ext}`;
    // A personal global excludes file must not make this pass on one machine only.
    const res = spawnSync(
      "git",
      ["-c", "core.excludesFile=/dev/null", "check-ignore", "--no-index", "-q", probe],
      { cwd: repoRoot },
    );
    expect(res.status, `${probe} is not matched by any ignore rule`).toBe(0);
  });
});
