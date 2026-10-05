import { describe, expect, it } from "vitest";
import { readdirSync, readFileSync, statSync } from "node:fs";
import { join } from "node:path";

// The chart standard, enforced. Every chart renders through components/charts,
// so nothing else in app/ or components/ imports recharts; chart code takes its
// colours from the --chart-* tokens in app/globals.css, so a hardcoded colour
// cannot drift a chart below its contrast floor again without failing here.

const root = join(__dirname, "..");
const CHARTS_DIR = "components/charts";

// Every file, whatever its extension: a stale copy such as page.tsx.bak that
// still imports recharts is a second chart vocabulary sitting in the tree.
function files(dir: string, out: string[] = []): string[] {
  for (const entry of readdirSync(join(root, dir))) {
    const rel = join(dir, entry);
    if (statSync(join(root, rel)).isDirectory()) {
      if (entry === "node_modules" || entry === ".next") continue;
      files(rel, out);
    } else {
      out.push(rel);
    }
  }
  return out;
}

const RECHARTS_IMPORT = /from\s+["']recharts["']|import\(\s*["']recharts["']\s*\)|require\(\s*["']recharts["']\s*\)/;

// Hex colours, functional colour notation, and Tailwind palette classes.
const HEX = /#[0-9a-fA-F]{3,6}/;
const COLOR_FUNCTION = /\b(?:rgb|rgba|hsl|hsla)\(/;
const PALETTE_CLASS =
  /\b(?:text|bg|stroke|fill|border|ring|outline|from|via|to|divide|shadow|accent|decoration)-(?:slate|gray|zinc|neutral|stone|red|orange|amber|yellow|lime|green|emerald|teal|cyan|sky|blue|indigo|violet|purple|fuchsia|pink|rose)-\d{2,3}\b/;

function rawColours(text: string): string[] {
  const found: string[] = [];
  text.split("\n").forEach((line, i) => {
    for (const pattern of [HEX, COLOR_FUNCTION, PALETTE_CLASS]) {
      const m = line.match(pattern);
      if (m) found.push(`${i + 1}: ${m[0]}`);
    }
  });
  return found;
}

describe("chart standard", () => {
  const chartFiles = files(CHARTS_DIR);

  it("has a chart layer to enforce", () => {
    expect(chartFiles.some((f) => f.endsWith("index.ts"))).toBe(true);
  });

  it("imports recharts only inside components/charts", () => {
    const outside = ["app", "components"]
      .flatMap((d) => files(d))
      .filter((rel) => !rel.startsWith(`${CHARTS_DIR}/`))
      .filter((rel) => RECHARTS_IMPORT.test(readFileSync(join(root, rel), "utf8")));
    expect(outside).toEqual([]);
  });

  it("carries no hex, rgb/hsl or palette-class colour in chart code", () => {
    const offenders = chartFiles.flatMap((rel) =>
      rawColours(readFileSync(join(root, rel), "utf8")).map((hit) => `${rel}:${hit}`),
    );
    expect(offenders).toEqual([]);
  });

  it("declares every --chart-* token the layer reads in globals.css", () => {
    const css = readFileSync(join(root, "app/globals.css"), "utf8");
    const used = new Set(
      chartFiles.flatMap((rel) => [...readFileSync(join(root, rel), "utf8").matchAll(/var\((--chart-[a-z0-9-]+)\)/g)].map((m) => m[1])),
    );
    expect(used.size).toBeGreaterThan(0);
    const undeclared = [...used].filter((token) => !new RegExp(`^\\s*${token}\\s*:`, "m").test(css));
    expect(undeclared).toEqual([]);
  });

  it("recognises the drift it exists to catch", () => {
    expect(rawColours('stroke="#10b981"')).toHaveLength(1);
    expect(rawColours('className="text-yellow-600"')).toHaveLength(1);
    expect(rawColours('fill="hsl(45 93% 47%)"')).toHaveLength(1);
    expect(rawColours('stroke="var(--chart-1)"')).toEqual([]);
  });
});
