// @vitest-environment node
import { afterEach, describe, expect, it } from "vitest";
import { mkdtempSync, mkdirSync, rmSync, writeFileSync } from "node:fs";
import { createRequire } from "node:module";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { TAILWIND_FONT_SIZES } from "./readability";
import { TEXT_SIZES } from "./target-size";
import { RAMP_KEYS, scanTypeScale, sourceFiles, typeScaleViolations } from "./type-scale";

// One type ramp: six steps in tailwind.config.js, 12px for captions and 14px
// upward for reading text. The config replaces Tailwind's scale, so a size
// class outside the six keys renders at the inherited size with no error from
// the build; this file is where such a class fails.

const root = join(__dirname, "..");
const SOURCE_DIRS = ["app", "components", "lib", "hooks"];

// The six steps, font size over line height, as the config must carry them.
const RAMP: Record<string, [string, { lineHeight: string }]> = {
  xs: ["12px", { lineHeight: "16px" }],
  sm: ["14px", { lineHeight: "20px" }],
  base: ["16px", { lineHeight: "22px" }],
  xl: ["20px", { lineHeight: "26px" }],
  "2xl": ["24px", { lineHeight: "32px" }],
  "3xl": ["32px", { lineHeight: "40px" }],
};

// The network graph's node and legend labels keep their arbitrary sizes until
// the graph gets a sizing rule of its own. The list only shrinks: a file that
// no longer carries an off-ramp class is removed here by the test below.
const AWAITING_OWN_RULE = ["components/mcp/network-graph.tsx"];

describe("tailwind.config.js", () => {
  it("carries the six steps of the ramp and nothing else", () => {
    const config = createRequire(__filename)(join(root, "tailwind.config.js"));
    expect(config.theme.fontSize).toEqual(RAMP);
    expect(Object.keys(config.theme.fontSize)).toEqual([...RAMP_KEYS]);
    expect(config.theme.extend.fontSize).toBeUndefined();
  });

  it("is the table the readability and target-size helpers read sizes from", () => {
    const fonts = Object.fromEntries(Object.entries(RAMP).map(([key, [size]]) => [key, parseFloat(size)]));
    const fontsAndLines = Object.fromEntries(
      Object.entries(RAMP).map(([key, [size, { lineHeight }]]) => [key, [parseFloat(size), parseFloat(lineHeight)]]),
    );
    expect(TAILWIND_FONT_SIZES).toEqual(fonts);
    expect(TEXT_SIZES).toEqual(fontsAndLines);
  });
});

describe("off-ramp size classes", () => {
  let seeded: string | null = null;
  afterEach(() => {
    if (seeded) rmSync(seeded, { recursive: true, force: true });
    seeded = null;
  });

  it("are found in a seeded file, with their variants and line numbers", () => {
    seeded = mkdtempSync(join(tmpdir(), "type-scale-"));
    mkdirSync(join(seeded, "components"));
    writeFileSync(
      join(seeded, "components", "seed.tsx"),
      [
        '<p className="text-xs text-ink">caption</p>',
        '<p className="text-2xs">gone</p>',
        '<h3 className="text-lg font-semibold sm:text-4xl">off</h3>',
        '<span className={cn("text-[13px]", on && "max-sm:text-[0.8rem]")}>px</span>',
        '<span className="text-[color:var(--text-code-prompt)] text-[#fff]">colour</span>',
        "const size = 'text-base';",
      ].join("\n"),
    );
    writeFileSync(join(seeded, "components", "seed.test.tsx"), '<p className="text-lg">a test fixture</p>');
    expect(sourceFiles(seeded, ["components"])).toEqual(["components/seed.tsx"]);
    expect(scanTypeScale(seeded, ["components"])).toEqual([
      { file: "components/seed.tsx", line: 2, cls: "text-2xs" },
      { file: "components/seed.tsx", line: 3, cls: "text-lg" },
      { file: "components/seed.tsx", line: 3, cls: "sm:text-4xl" },
      { file: "components/seed.tsx", line: 4, cls: "text-[13px]" },
      { file: "components/seed.tsx", line: 4, cls: "max-sm:text-[0.8rem]" },
    ]);
  });

  it("leave the six ramp keys and their variants alone", () => {
    const text = '<p className="text-xs sm:text-sm text-base md:text-xl text-2xl lg:text-3xl text-xs/5 text-ink-secondary">a</p>';
    expect(typeScaleViolations("ok.tsx", text)).toEqual([]);
  });

  it("appear in no source file outside the ones awaiting their own rule", () => {
    const files = sourceFiles(root, SOURCE_DIRS);
    expect(files.length).toBeGreaterThan(100);
    expect(files).toContain("app/dashboard/page.tsx");
    const offenders = scanTypeScale(root, SOURCE_DIRS)
      .filter((v) => !AWAITING_OWN_RULE.includes(v.file))
      .map((v) => `${v.file}:${v.line} ${v.cls}`);
    expect(offenders).toEqual([]);
  });

  it("still exist in every file listed as awaiting its own rule, so the list only shrinks", () => {
    const byFile = new Set(scanTypeScale(root, SOURCE_DIRS).map((v) => v.file));
    expect(AWAITING_OWN_RULE.filter((file) => !byFile.has(file))).toEqual([]);
  });
});
