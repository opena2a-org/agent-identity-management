// @vitest-environment node
import { describe, expect, it } from "vitest";
import { execFileSync } from "node:child_process";
import { resolve } from "node:path";

// Planning notes under briefs/ describe work before it is done and point at
// places a reader of this repository cannot open. What a change did belongs in
// CHANGELOG.md and docs/. It lives in the frontend suite because that job runs
// on every push and pull request, but it covers the whole repository.
const PLANNING_NOTE = /^briefs\//;
const repoRoot = resolve(__dirname, "..", "..");

describe("planning notes", () => {
  it("none are tracked under briefs/", () => {
    const tracked = execFileSync("git", ["ls-files", "-z"], { cwd: repoRoot, encoding: "utf8" })
      .split("\0")
      .filter(Boolean);
    expect(tracked.length).toBeGreaterThan(0);
    expect(tracked.filter((file) => PLANNING_NOTE.test(file))).toEqual([]);
  });
});
