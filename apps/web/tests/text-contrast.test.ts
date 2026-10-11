import { describe, expect, it } from "vitest";
import { readFileSync } from "node:fs";
import { join } from "node:path";
import {
  colorsIn,
  contrastRatio,
  over,
  parseColor,
  resolveValue,
  themeTokens,
  tokenColor,
  translucentSurfaces,
  type Rgba,
  type Theme,
} from "./theme-contrast";

// Text on the chart screens that the readability sweep measured under 4.5:1,
// held to the floor from the tokens in app/globals.css in both themes, on every
// surface it is drawn on:
// - the active link of the shared navigation (4.38:1 in the dark drawer),
// - the agent initials in the overview's agent list (4.14:1 light, 4.24:1 dark),
// - the unit beside a figure in an inset tile (4.20:1 dark).

const TEXT_FLOOR = 4.5;
const THEMES: Theme[] = ["light", "dark"];
const AVATAR_TINTS = [1, 2, 3, 4];

const webRoot = join(__dirname, "..");
const css = readFileSync(join(webRoot, "app", "globals.css"), "utf8");

function lowest(fg: Rgba, backgrounds: Rgba[]): number {
  return Math.min(...backgrounds.map((bg) => contrastRatio(over(fg, bg), bg)));
}

function below(checks: Array<{ theme: Theme; name: string; ratio: number }>): string[] {
  return checks.filter((c) => c.ratio < TEXT_FLOOR).map((c) => `${c.theme} ${c.name} ${c.ratio.toFixed(2)}:1`);
}

describe("text contrast on the chart screens", () => {
  it("reads the shadcn hsl triplets the overlays are painted with", () => {
    expect(parseColor("hsl(0 0% 100%)")).toEqual([255, 255, 255, 1]);
    const popover = tokenColor(themeTokens(css, "dark"), "hsl(var(--popover))");
    expect(popover.slice(0, 3).map(Math.round)).toEqual([30, 30, 32]);
  });

  it("draws the active navigation link at 4.5:1 or more on every surface the navigation sits on", () => {
    const checks = THEMES.flatMap((theme) => {
      const tokens = themeTokens(css, theme);
      // The sidebar is glass chrome over the page; the phone drawer is an opaque overlay.
      const grounds = [
        ...translucentSurfaces(tokens, "var(--glass-chrome-fill)", "var(--glass-chrome-solid)"),
        tokenColor(tokens, "hsl(var(--popover))"),
      ];
      const fill = tokenColor(tokens, "var(--nav-active-fill)");
      return [
        {
          theme,
          name: "active navigation link",
          ratio: lowest(
            tokenColor(tokens, "var(--nav-active-text)"),
            grounds.map((g) => over(fill, g)),
          ),
        },
      ];
    });
    expect(checks).toHaveLength(THEMES.length);
    expect(below(checks)).toEqual([]);
  });

  it("draws every agent initial at 4.5:1 or more on both stops of its tint", () => {
    const checks = THEMES.flatMap((theme) => {
      const tokens = themeTokens(css, theme);
      return AVATAR_TINTS.map((n) => {
        const stops = colorsIn(resolveValue(tokens, `var(--avatar-${n})`));
        expect(stops, `--avatar-${n} in ${theme}`).toHaveLength(2);
        return {
          theme,
          name: `avatar tint ${n}`,
          ratio: lowest(tokenColor(tokens, `var(--avatar-${n}-text)`), stops),
        };
      });
    });
    expect(below(checks)).toEqual([]);
  });

  it("declares every avatar tint the overview paints", () => {
    const sources = ["app/dashboard/page.tsx", "components/overview/live-checkin-panel.tsx"].map((rel) =>
      readFileSync(join(webRoot, rel), "utf8"),
    );
    const used = new Set(sources.flatMap((s) => [...s.matchAll(/\bavatar-tint-(\d+)\b/g)].map((m) => Number(m[1]))));
    expect([...used].sort()).toEqual(AVATAR_TINTS);
    for (const n of used) expect(css).toMatch(new RegExp(`\\.avatar-tint-${n}\\s*\\{`));
  });

  it("draws the unit in an inset tile in the secondary ink at 4.5:1 or more", () => {
    const checks = THEMES.flatMap((theme) => {
      const tokens = themeTokens(css, theme);
      const inset = tokenColor(tokens, "var(--surface-inset-gray)");
      const tiles = translucentSurfaces(tokens, "var(--glass-fill)", "var(--glass-fill-solid)").map((g) => over(inset, g));
      return [{ theme, name: "secondary ink on an inset tile", ratio: lowest(tokenColor(tokens, "var(--text-secondary)"), tiles) }];
    });
    expect(below(checks)).toEqual([]);
  });
});
