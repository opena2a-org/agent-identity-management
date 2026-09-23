import { describe, expect, it } from "vitest";
import { readdirSync, readFileSync, statSync } from "node:fs";
import { join, relative } from "node:path";

// Tripwire: the session store is read and written in lib/api.ts only and no
// dashboard request sends cookies. The edge-file cell lives in ../routing.test.ts.
// The behaviour is pinned by the shell and store cells; this catches a new
// reader or writer at review time.

const root = join(__dirname, "..");

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

function hits(pattern: RegExp, exclude: (rel: string) => boolean = () => false): string[] {
  const found: string[] = [];
  for (const rel of files) {
    if (exclude(rel)) continue;
    readFileSync(join(root, rel), "utf8").split("\n").forEach((line, i) => {
      if (pattern.test(line)) found.push(`${relative(root, join(root, rel))}:${i + 1}`);
    });
  }
  return found;
}

describe("session sources", () => {
  it("scans the dashboard sources", () => {
    expect(files.length).toBeGreaterThan(50);
  });

  it("only lib/api.ts reads or writes the session keys in localStorage", () => {
    const pattern = /localStorage\.(getItem|setItem|removeItem)\(\s*["'](auth_token|refresh_token|token)["']/;
    expect(hits(pattern, (rel) => rel === join("lib", "api.ts"))).toEqual([]);
  });

  it("no dashboard request sends cookies", () => {
    expect(hits(/credentials:\s*["']include["']/)).toEqual([]);
  });
});
