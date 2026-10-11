import { describe, expect, it } from "vitest";
import { readdirSync, readFileSync, statSync } from "node:fs";
import { join } from "node:path";
import { unshrinkableFlexItems } from "./flex-shrink";
import { paletteClasses, rawColours } from "./raw-colours";

// Two rules for the screens that hold the charts, read from the source.
//
// A chart screen takes its colours from the theme tokens. The tokens are what
// tests/chart-contrast.test.ts, tests/text-contrast.test.ts and the browser
// sweep measure in both themes; a palette class or a hex value beside them is a
// colour whose contrast nobody measured (the compliance page's "Trust:" label
// read 2.43:1, the overview's agent initials on hex tints 4.14:1).
//
// A growing flex item around truncated text can shrink, so a long line is cut
// with an ellipsis instead of pushing the page wider than a phone (the
// compliance page's audit trail overflowed 375px by 104px).

const root = join(__dirname, "..");

function files(dir: string): string[] {
  return readdirSync(join(root, dir)).map((entry) => join(dir, entry));
}

// The screens the chart standard covers: the overview (its three lenses live in
// components/overview), the security page and the compliance page.
const CHART_SCREENS = [
  "app/dashboard/page.tsx",
  "app/dashboard/security/page.tsx",
  "app/dashboard/compliance/page.tsx",
  ...files("components/overview").filter((f) => f.endsWith(".tsx") && !f.endsWith(".test.tsx")),
];

describe("chart screens", () => {
  it("covers the overview's lenses, the security page and the compliance page", () => {
    expect(CHART_SCREENS).toContain("components/overview/executive-lens.tsx");
    expect(CHART_SCREENS).toContain("components/overview/security-lens.tsx");
    for (const rel of CHART_SCREENS) expect(statSync(join(root, rel)).isFile()).toBe(true);
  });

  it("paint nothing from the raw palette, a hex value or a colour function", () => {
    const offenders = CHART_SCREENS.flatMap((rel) =>
      rawColours(readFileSync(join(root, rel), "utf8")).map((hit) => `${rel}:${hit}`),
    );
    expect(offenders).toEqual([]);
  });

  it("let every growing flex item around truncated text shrink", () => {
    const offenders = CHART_SCREENS.flatMap((rel) =>
      unshrinkableFlexItems(rel, readFileSync(join(root, rel), "utf8")).map(
        (item) => `${item.file}:${item.line} (truncated text at ${item.truncatedAt}): ${item.classes}`,
      ),
    );
    expect(offenders).toEqual([]);
  });
});

describe("palette classes", () => {
  it("are read wherever a page keeps them", () => {
    expect(paletteClasses('const tone = { ok: "text-green-600 dark:text-green-400" };')).toHaveLength(1);
    expect(paletteClasses('<div className="bg-white dark:bg-gray-800 rounded-xl" />')).toHaveLength(1);
    expect(paletteClasses('<span className="text-ink-secondary bg-success-fill" />')).toEqual([]);
  });
});

describe("unshrinkable flex items", () => {
  it("are found around truncated text when the growing box has no min-w-0", () => {
    const text = [
      "function Row() {",
      "  return (",
      '    <div className="flex gap-4">',
      '      <div className="flex-1 p-4">',
      '        <div className="min-w-0 flex-1">',
      '          <p className="truncate">{description}</p>',
      "        </div>",
      "      </div>",
      "    </div>",
      "  );",
      "}",
    ].join("\n");
    expect(unshrinkableFlexItems("row.tsx", text)).toEqual([{ file: "row.tsx", line: 4, truncatedAt: 6, classes: "flex-1 p-4" }]);
  });

  it("are not reported with min-w-0 or a clipping overflow, or above a clipping ancestor", () => {
    const shrinks = '<div className="flex"><div className="flex-1 min-w-0"><p className="truncate">{t}</p></div></div>';
    const clipsItself = '<div className="flex"><div className="flex-1 overflow-hidden"><p className="truncate">{t}</p></div></div>';
    const clippedAbove =
      '<div className="flex-1"><div className="overflow-x-auto"><div className="flex"><p className="truncate">{t}</p></div></div></div>';
    expect(unshrinkableFlexItems("a.tsx", shrinks)).toEqual([]);
    expect(unshrinkableFlexItems("a.tsx", clipsItself)).toEqual([]);
    expect(unshrinkableFlexItems("a.tsx", clippedAbove)).toEqual([]);
  });

  it("are followed into rows rendered in a callback, not past a component's declaration", () => {
    const inMap = '<div className="flex">{items.map((i) => <div className="grow"><span className="truncate">{i}</span></div>)}</div>';
    expect(unshrinkableFlexItems("a.tsx", inMap)).toHaveLength(1);
    const otherComponent = [
      '<div className="flex-1"><Label /></div>;',
      'function Label() { return <span className="truncate">x</span>; }',
    ].join("\n");
    expect(unshrinkableFlexItems("a.tsx", otherComponent)).toEqual([]);
  });
});
