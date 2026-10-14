/**
 * The type ramp as the source uses it: the six font-size keys of
 * tailwind.config.js theme.fontSize (xs, sm, base, xl, 2xl, 3xl). That block
 * replaces Tailwind's scale instead of extending it, so any other size class
 * sets no size: text-2xs, text-lg and text-4xl are not generated, and an
 * arbitrary text-[13px] steps off the ramp. This module finds such classes in
 * source text; type-scale.test.ts runs it over app/, components/, lib/ and
 * hooks/ and proves it on a seeded file first.
 */
import { readdirSync, readFileSync, statSync } from "node:fs";
import { join, relative } from "node:path";

export const RAMP_KEYS = ["xs", "sm", "base", "xl", "2xl", "3xl"] as const;

export interface TypeScaleViolation {
  file: string;
  line: number;
  cls: string;
}

// A text-* class, with any variants in front of it, whose suffix is a Tailwind
// size name outside the ramp or an arbitrary length. text-[color:...],
// text-[var(--x)] and text-[#fff] set a colour, not a size, and pass.
const OFF_RAMP = /(?<![\w-])((?:[\w-]+:)*text-(?:2xs|lg|[4-9]xl|\[[\d.]+(?:px|rem|em)\]))(?![\w-])/g;

/** Every off-ramp size class in `text`, with the 1-based line it sits on. */
export function typeScaleViolations(file: string, text: string): TypeScaleViolation[] {
  const found: TypeScaleViolation[] = [];
  text.split("\n").forEach((lineText, i) => {
    for (const m of lineText.matchAll(OFF_RAMP)) found.push({ file, line: i + 1, cls: m[1] });
  });
  return found;
}

/** The .ts and .tsx source files under `dirs` (relative to `root`), test files left out. */
export function sourceFiles(root: string, dirs: readonly string[]): string[] {
  const out: string[] = [];
  const walk = (dir: string) => {
    for (const entry of readdirSync(dir)) {
      if (entry === "node_modules") continue;
      const path = join(dir, entry);
      if (statSync(path).isDirectory()) walk(path);
      else if (/\.tsx?$/.test(entry) && !/\.test\.tsx?$/.test(entry) && !entry.endsWith(".d.ts")) {
        out.push(relative(root, path));
      }
    }
  };
  for (const dir of dirs) walk(join(root, dir));
  return out.sort();
}

/** Off-ramp size classes across the source files under `dirs`. */
export function scanTypeScale(root: string, dirs: readonly string[]): TypeScaleViolation[] {
  return sourceFiles(root, dirs).flatMap((file) => typeScaleViolations(file, readFileSync(join(root, file), "utf8")));
}
