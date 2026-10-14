import { describe, expect, it } from "vitest";
import { readdirSync, readFileSync, statSync } from "node:fs";
import { join } from "node:path";
import { unshrinkableFlexItems } from "./flex-shrink";
import { paletteClasses, rawColours } from "./raw-colours";
import { minHeightPx, undersizedControls } from "./target-size";

// Three rules for the screens that hold the charts, read from the source.
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
//
// A link or button is at least 24px tall, the minimum target of WCAG 2.5.8
// (the overview's "See all" and "Open Security" links measured 16px, its SDK
// language tabs 21.6px, the security page's "View all" links 20px).

const root = join(__dirname, "..");

function files(dir: string): string[] {
  return readdirSync(join(root, dir)).map((entry) => join(dir, entry));
}

// The screens the chart standard covers: the overview (its three lenses live in
// components/overview), the security page and the compliance page; then the A2A
// page, the supply chain page and the agent detail page with the trust score and
// drift score cards it draws.
const CHART_SCREENS = [
  "app/dashboard/page.tsx",
  "app/dashboard/security/page.tsx",
  "app/dashboard/compliance/page.tsx",
  ...files("components/overview").filter((f) => f.endsWith(".tsx") && !f.endsWith(".test.tsx")),
  "app/dashboard/a2a/page.tsx",
  "app/dashboard/mcp/supply-chain/page.tsx",
  "app/dashboard/agents/[id]/page.tsx",
  "components/agent/trust-score-breakdown.tsx",
  "components/agent/drift-score-card.tsx",
];

describe("chart screens", () => {
  it("covers the overview's lenses, the security, compliance, A2A and supply chain pages and the agent detail page", () => {
    expect(CHART_SCREENS).toContain("components/overview/executive-lens.tsx");
    expect(CHART_SCREENS).toContain("components/overview/security-lens.tsx");
    expect(CHART_SCREENS).toContain("components/agent/trust-score-breakdown.tsx");
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

  it("draw no link or button under 24px tall", () => {
    const offenders = CHART_SCREENS.flatMap((rel) =>
      undersizedControls(rel, readFileSync(join(root, rel), "utf8")).map(
        (control) => `${control.file}:${control.line} (${control.heightPx}px): ${control.classes}`,
      ),
    );
    expect(offenders).toEqual([]);
  });
});

describe("control heights", () => {
  it("are the larger of a set height and one line of text with its padding", () => {
    expect(minHeightPx(["text-xs", "font-semibold"])).toBe(16);
    expect(minHeightPx(["rounded-pill", "px-3", "py-0.5", "text-xs"])).toBe(20);
    expect(minHeightPx(["inline-flex", "min-h-6", "items-center", "text-xs"])).toBe(24);
    expect(minHeightPx(["h-11", "px-2", "text-sm"])).toBe(44);
    expect(minHeightPx(["px-4", "py-2", "text-sm"])).toBe(36);
    expect(minHeightPx(["pt-1", "pb-3", "py-0.5", "text-sm"])).toBe(36);
    expect(minHeightPx(["text-sm", "leading-none"])).toBe(14);
  });

  it("take the smaller arm of a conditional and ignore variants", () => {
    expect(minHeightPx(["text-xs", "py-3", "py-0.5"])).toBe(20);
    expect(minHeightPx(["text-xs", "sm:py-4", "hover:min-h-11"])).toBe(16);
  });

  it("are not read when the classes set neither a height nor a text size", () => {
    expect(minHeightPx(["p-1", "rounded"])).toBeNull();
    expect(minHeightPx(["size-6"])).toBe(24);
  });

  it("exempt a link inside running text and follow cn() and both arms", () => {
    const text = [
      "function Card() {",
      "  return (",
      "    <div>",
      '      <p>Read the <a href="/g" className="text-xs underline">guide</a> first.</p>',
      '      <Link href="/a" className="text-xs font-semibold">See all</Link>',
      '      <button className={cn("rounded-pill px-3 py-0.5 text-xs", on ? "bg-brand" : "")}>Python</button>',
      '      <Link href="/b" className="inline-flex min-h-6 items-center text-xs">Open</Link>',
      '      <Link href="/c" className="mt-1 min-h-6 text-xs">Investigate</Link>',
      '      <button className="min-h-6 text-xs">Reset page</button>',
      '      <Link href="/d" className="hidden h-10 items-center text-sm sm:inline-flex">Wide only</Link>',
      "    </div>",
      "  );",
      "}",
    ].join("\n");
    // A link with no display class is inline and ignores min-h-6; a button does not.
    // A link hidden at 375px has no target there and is not read.
    expect(undersizedControls("card.tsx", text).map((c) => [c.line, c.heightPx])).toEqual([
      [5, 16],
      [6, 20],
      [8, 16],
    ]);
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

  it("are not reported with min-w-0, a zero width or a clipping overflow, or above a clipping ancestor", () => {
    const shrinks = '<div className="flex"><div className="flex-1 min-w-0"><p className="truncate">{t}</p></div></div>';
    const zeroWidth = '<div className="flex"><div className="ml-5 w-0 flex-1"><dt className="truncate">{t}</dt></div></div>';
    const clipsItself = '<div className="flex"><div className="flex-1 overflow-hidden"><p className="truncate">{t}</p></div></div>';
    const clippedAbove =
      '<div className="flex-1"><div className="overflow-x-auto"><div className="flex"><p className="truncate">{t}</p></div></div></div>';
    expect(unshrinkableFlexItems("a.tsx", shrinks)).toEqual([]);
    expect(unshrinkableFlexItems("a.tsx", zeroWidth)).toEqual([]);
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
