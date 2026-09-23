import { describe, expect, it } from "vitest";
import { existsSync, readdirSync, readFileSync, statSync } from "node:fs";
import { join, relative } from "node:path";

// Routing tripwires for the dashboard. This file is root-level on purpose: the
// private-fork sync copies named directories and named root files only, and the
// fork keeps its own edge file, so these cells must not travel there.
//
// After the route gate moved into the dashboard shell, a page is public exactly
// when it renders outside that shell. There is no allow-list of public paths left
// to drift from the pages that exist, and these cells keep it that way.

const root = __dirname;

function sources(dir: string, out: string[] = []): string[] {
  for (const entry of readdirSync(join(root, dir))) {
    const rel = join(dir, entry);
    const abs = join(root, rel);
    if (statSync(abs).isDirectory()) {
      if (entry === "node_modules" || entry === ".next" || entry === "e2e") continue;
      sources(rel, out);
    } else if (/\.(ts|tsx)$/.test(entry) && !/\.(test|spec)\.tsx?$/.test(entry) && !/\.d\.ts$/.test(entry)) {
      out.push(rel);
    }
  }
  return out;
}

const files = ["app", "components", "hooks", "lib"].flatMap((d) => sources(d));

// Every non-test source file that imports the named module.
function importers(module: string): string[] {
  const pattern = new RegExp(`from\\s+["'][^"']*/${module}["']`);
  return files
    .filter((rel) => pattern.test(readFileSync(join(root, rel), "utf8")))
    .map((rel) => relative(root, join(root, rel)))
    .sort();
}

describe("dashboard routing", () => {
  it("no edge file makes routing decisions", () => {
    const edge = ["middleware.ts", "middleware.js", "proxy.ts", "proxy.js"].filter((f) => existsSync(join(root, f)));
    expect(edge).toEqual([]);
  });

  it("the deactivation check and the idle timer run only inside the dashboard shell", () => {
    // Both hooks dropped their public-route skip lists on this premise: nothing
    // outside the gated shell mounts them, so every pathname they see is gated.
    expect(importers("use-deactivation-check")).toEqual([join("app", "dashboard", "layout.tsx")]);
    expect(importers("idle-timeout-guard")).toEqual([join("app", "dashboard", "layout.tsx")]);
    expect(importers("use-idle-timeout")).toEqual([join("components", "idle-timeout-guard.tsx")]);
  });
});
