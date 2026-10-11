import { describe, expect, it } from "vitest";
import { readFileSync } from "node:fs";
import { join } from "node:path";
import { chartChrome, chartSeries, chartTooltipStyle } from "@/components/charts/tokens";
import {
  colorsIn,
  contrastRatio,
  over,
  parseColor,
  resolveValue,
  themeTokens,
  type Rgba,
  type Theme,
} from "./theme-contrast";

// The chart standard's contrast floors, measured on the --chart-* tokens in
// app/globals.css in both themes: every series colour holds 3:1 (WCAG 1.4.11,
// graphical objects) and the text a chart draws holds 4.5:1 (WCAG 1.4.3)
// against every surface a chart is drawn on. Grid lines, axis lines, the score
// ring's track and the hover cursor carry no value a reader needs (the axis
// labels and the table do), so no floor applies to them.

const GRAPHIC_FLOOR = 3;
const TEXT_FLOOR = 4.5;
const THEMES: Theme[] = ["light", "dark"];

const css = readFileSync(join(__dirname, "..", "app", "globals.css"), "utf8");

function color(tokens: Map<string, string>, value: string): Rgba {
  const resolved = resolveValue(tokens, value);
  const c = parseColor(resolved);
  if (!c) throw new Error(`${value} resolves to ${resolved}, not a colour`);
  return c;
}

// A chart sits on a glass card (tests/chart-surface.test.ts fails for one that
// does not). The card is translucent over the page, so its
// colour is the glass composited over each stop of the page gradient, bare and
// under the peak of either radial wash; where the glass is drawn solid (reduced
// transparency, no backdrop-filter) it is the solid fill.
function chartSurfaces(tokens: Map<string, string>): Rgba[] {
  const page = [...colorsIn(resolveValue(tokens, "var(--bg-page)")), color(tokens, "var(--bg-page-solid)")];
  const washes = ["var(--wash-1)", "var(--wash-2)"].map((w) => color(tokens, w));
  const grounds = page.flatMap((p) => [p, ...washes.map((w) => over(w, p))]);
  const glass = color(tokens, "var(--glass-fill)");
  return [...grounds.map((g) => over(glass, g)), color(tokens, "var(--glass-fill-solid)")];
}

function lowest(fg: Rgba, backgrounds: Rgba[]): number {
  return Math.min(...backgrounds.map((bg) => contrastRatio(over(fg, bg), bg)));
}

function below(floor: number, checks: Array<{ theme: Theme; name: string; ratio: number }>): string[] {
  return checks.filter((c) => c.ratio < floor).map((c) => `${c.theme} ${c.name} ${c.ratio.toFixed(2)}:1`);
}

describe("chart contrast", () => {
  it("measures the way WCAG does", () => {
    expect(contrastRatio([0, 0, 0, 1], [255, 255, 255, 1])).toBeCloseTo(21, 5);
    expect(contrastRatio(parseColor("#767676")!, parseColor("#ffffff")!)).toBeCloseTo(4.54, 2);
    // The drift the standard exists for: yellow-600 on white sits under the graphics floor.
    expect(contrastRatio(parseColor("#ca8a04")!, parseColor("#ffffff")!)).toBeLessThan(GRAPHIC_FLOOR);
    expect(over([0, 0, 0, 0.5], [255, 255, 255, 1])).toEqual([127.5, 127.5, 127.5, 1]);
  });

  it("finds the surfaces a chart is drawn on in both themes", () => {
    for (const theme of THEMES) {
      const surfaces = chartSurfaces(themeTokens(css, theme));
      expect(surfaces.length).toBeGreaterThan(3);
    }
    const lightCard = chartSurfaces(themeTokens(css, "light"))[0];
    const darkCard = chartSurfaces(themeTokens(css, "dark"))[0];
    expect(contrastRatio(lightCard, darkCard)).toBeGreaterThan(10);
  });

  it("draws every series at 3:1 or more against the card, light and dark", () => {
    const checks = THEMES.flatMap((theme) => {
      const tokens = themeTokens(css, theme);
      const surfaces = chartSurfaces(tokens);
      return Object.entries(chartSeries).map(([name, value]) => ({
        theme,
        name: `${name} (${value})`,
        ratio: lowest(color(tokens, value), surfaces),
      }));
    });
    expect(checks.length).toBe(Object.keys(chartSeries).length * THEMES.length);
    expect(below(GRAPHIC_FLOOR, checks)).toEqual([]);
  });

  it("draws axis and tooltip text at 4.5:1 or more, light and dark", () => {
    const checks = THEMES.flatMap((theme) => {
      const tokens = themeTokens(css, theme);
      const surfaces = chartSurfaces(tokens);
      const tooltipBg = color(tokens, chartTooltipStyle.backgroundColor);
      return [
        { theme, name: "axis text", ratio: lowest(color(tokens, chartChrome.axisText), surfaces) },
        {
          theme,
          name: "tooltip text",
          ratio: lowest(
            color(tokens, chartTooltipStyle.color),
            surfaces.map((s) => over(tooltipBg, s)),
          ),
        },
      ];
    });
    expect(below(TEXT_FLOOR, checks)).toEqual([]);
  });
});
