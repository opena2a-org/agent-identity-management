// @vitest-environment node
import { describe, expect, it } from "vitest";
import { execFileSync, spawnSync } from "node:child_process";
import { mkdirSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";

// next-env.d.ts is generated: `next dev`, `next build` and `next typegen` each
// rewrite it, and the lines they write differ by command and by Next.js version.
// Tracked, a build in a clean checkout leaves the tree modified. The repo-root
// .gitignore keeps it out of a normal `git add`; this catches a force-add and a
// removed rule across the repository, and checks that the typecheck still has
// the file on a clean checkout.
const NEXT_ENV = /(^|\/)next-env\.d\.ts$/;
const repoRoot = resolve(__dirname, "..", "..");
const probe = "apps/web/next-env.d.ts";

// Returns null when `path` is ignored by a rule in the repo-root .gitignore,
// otherwise what decides it instead.
function notIgnoredByGitignore(cwd: string, path: string): string | null {
  // A personal global excludes file must not make this pass on one machine only.
  // .git/info/exclude is local to one clone too but cannot be switched off, so
  // `-v` names the rule that decides and its source is checked. `-v` also exits
  // 0 when that rule is a negation, which un-ignores the path.
  const res = spawnSync(
    "git",
    ["-c", "core.excludesFile=/dev/null", "check-ignore", "--no-index", "-v", path],
    { cwd, encoding: "utf8" },
  );
  if (res.status === 1) return `${path} is not matched by any ignore rule`;
  const rule = /^(.+?):\d+:([^\t]*)\t/.exec(res.stdout);
  if (res.status !== 0 || !rule) throw new Error(`git check-ignore failed: ${res.stderr}${res.stdout}`);
  const [decidedBy, source, pattern] = rule;
  if (source !== ".gitignore") return `${path} is ignored by ${decidedBy}, not by .gitignore`;
  if (pattern.startsWith("!")) return `${path} is un-ignored by ${decidedBy}`;
  return null;
}

function scratchRepo(files: Record<string, string>): string {
  const dir = mkdtempSync(join(tmpdir(), "next-env-"));
  execFileSync("git", ["init", "-q", dir]);
  for (const [file, body] of Object.entries(files)) {
    mkdirSync(join(dir, file, ".."), { recursive: true });
    writeFileSync(join(dir, file), body);
  }
  return dir;
}

describe("Next.js generated type declarations", () => {
  it("next-env.d.ts is not tracked anywhere in the repository", () => {
    const tracked = execFileSync("git", ["ls-files", "-z"], { cwd: repoRoot, encoding: "utf8" })
      .split("\0")
      .filter(Boolean);
    expect(tracked.length).toBeGreaterThan(0);
    expect(tracked.filter((file) => NEXT_ENV.test(file))).toEqual([]);
  });

  it("apps/web/next-env.d.ts is ignored by .gitignore", () => {
    expect(notIgnoredByGitignore(repoRoot, probe)).toBeNull();
  });

  it.each([
    ["the rule is only in .git/info/exclude", { ".gitignore": "node_modules/\n", ".git/info/exclude": "next-env.d.ts\n" }],
    ["the .gitignore rule is negated", { ".gitignore": "*.d.ts\n!next-env.d.ts\n", ".git/info/exclude": "next-env.d.ts\n" }],
    ["no rule matches", { ".gitignore": "node_modules/\n" }],
  ])("the .gitignore check fails when %s", (_case, files) => {
    const dir = scratchRepo(files);
    try {
      expect(notIgnoredByGitignore(dir, probe)).not.toBeNull();
    } finally {
      rmSync(dir, { recursive: true, force: true });
    }
  });

  it("the .gitignore check passes on a repo-root .gitignore rule", () => {
    const dir = scratchRepo({ ".gitignore": "next-env.d.ts\n", ".git/info/exclude": "next-env.d.ts\n" });
    try {
      expect(notIgnoredByGitignore(dir, probe)).toBeNull();
    } finally {
      rmSync(dir, { recursive: true, force: true });
    }
  });

  it("apps/web type-check generates next-env.d.ts before tsc reads it", () => {
    const pkg = JSON.parse(readFileSync(resolve(__dirname, "package.json"), "utf8"));
    expect(pkg.scripts["type-check"]).toMatch(/^next typegen && tsc --noEmit$/);
  });
});
