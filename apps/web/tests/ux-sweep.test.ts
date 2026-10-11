// @vitest-environment node
import { existsSync } from "node:fs";
import { join } from "node:path";
import { chromium, type Browser } from "@playwright/test";
import { afterAll, beforeAll, describe, expect, it } from "vitest";
import {
  MEASURE_ARGS,
  entryName,
  enumerateRoutes,
  findMock,
  gateFailures,
  measureInPage,
  mocksFor,
  selectRoutes,
  summarize,
} from "../scripts/ux-sweep.mjs";

// The readability sweep (scripts/ux-sweep.mjs) is the browser instrument the
// chart standard's contrast floors are measured with. These tests hold the parts
// that decide what it measures: one overview load per lens, the populated fixture
// that makes charts draw marks, the chart measurement itself, and which failures
// fail the gate.

const appDir = join(__dirname, "..", "app");

describe("ux-sweep routes", () => {
  it("loads the overview once per lens and every other route once", async () => {
    const routes = await enumerateRoutes(appDir);
    const overview = routes.filter((r: { route: string }) => r.route === "/dashboard");
    expect(overview.map((r: { lens: string | null }) => r.lens)).toEqual(["developer", "security", "executive"]);
    expect(overview.map(entryName)).toEqual([
      "/dashboard (developer lens)",
      "/dashboard (security lens)",
      "/dashboard (executive lens)",
    ]);
    const security = routes.filter((r: { route: string }) => r.route === "/dashboard/security");
    expect(security).toHaveLength(1);
    expect(security[0].lens).toBeNull();
    expect(entryName(security[0])).toBe("/dashboard/security");
  });

  it("limits the sweep to the routes --only names and refuses a name that is no route", async () => {
    const routes = await enumerateRoutes(appDir);
    const picked = selectRoutes(routes, ["/dashboard", "/dashboard/compliance"]);
    expect(picked.map(entryName)).toEqual([
      "/dashboard (developer lens)",
      "/dashboard (security lens)",
      "/dashboard (executive lens)",
      "/dashboard/compliance",
    ]);
    expect(() => selectRoutes(routes, ["/dashboard/not-a-page"])).toThrow(/\/dashboard\/not-a-page/);
    expect(selectRoutes(routes, null)).toBe(routes);
  });
});

describe("ux-sweep fixtures", () => {
  const populated = mocksFor("populated");
  const empty = mocksFor("empty");
  const body = (mocks: ReturnType<typeof mocksFor>, path: string) => findMock("GET", path, mocks) as Record<string, unknown>;

  it("gives every overview, security and compliance chart more than one point to draw", () => {
    const activity = body(populated, "/api/v1/analytics/verification-activity").activity as unknown[];
    expect(activity.length).toBeGreaterThan(1);
    const timeline = body(populated, "/api/v1/security/metrics").protectionTimeline as unknown[];
    expect(timeline.length).toBeGreaterThan(1);
    expect(body(populated, "/api/v1/security/metrics").securityScore).toBeGreaterThan(0);
    const trend = (body(populated, "/api/v1/compliance/metrics").metrics as { trustScoreTrend: unknown[] }).trustScoreTrend;
    expect(trend.length).toBeGreaterThan(1);
  });

  it("fills every trust bucket the executive lens draws, including an agent with no score", () => {
    const agents = body(populated, "/api/v1/agents").agents as { trustScore: number | null }[];
    const scores = agents.map((a) => a.trustScore);
    expect(scores.some((s) => s !== null && s >= 0.8)).toBe(true);
    expect(scores.some((s) => s !== null && s >= 0.5 && s < 0.8)).toBe(true);
    expect(scores.some((s) => s !== null && s < 0.5)).toBe(true);
    expect(scores.some((s) => s === null)).toBe(true);
  });

  it("leaves the default fixture empty, so the empty states stay measured", () => {
    expect(body(empty, "/api/v1/analytics/verification-activity").activity).toEqual([]);
    expect(body(empty, "/api/v1/agents").agents).toHaveLength(1);
    expect(body(empty, "/api/v1/security/metrics").protectionTimeline).toEqual([]);
  });
});

function variant(overrides: Record<string, unknown>) {
  return {
    route: "/dashboard/security",
    lens: null,
    file: "app/dashboard/security/page.tsx",
    finalUrl: "http://localhost:3000/dashboard/security",
    redirected: false,
    loadError: null,
    consoleErrors: [],
    minBodyContrast: 7,
    minLargeContrast: 7,
    smallTextNodes: 0,
    smallTargets: 0,
    targetsUnderMinimum: 0,
    hasMain: true,
    overflowPx: 0,
    chartCount: 1,
    chartMarks: 4,
    minChartMarkContrast: 4,
    minChartBodyContrast: 5,
    minChartLargeContrast: null,
    chartBelowFloor: [],
    chartsUnlabelled: 0,
    chartsWithoutTable: 0,
    chartControlsUnderFloor: 0,
    offTokenPaints: 0,
    ...overrides,
  };
}

describe("ux-sweep gate", () => {
  const routes = [{ route: "/dashboard/security", lens: null, file: "app/dashboard/security/page.tsx" }];

  it("passes a chart whose marks and text clear their floors", () => {
    const { fails } = summarize([variant({})], routes);
    expect(gateFailures(fails)).toEqual([]);
  });

  it("fails a series mark under 3:1 and chart text under its floor", () => {
    const { rows, fails } = summarize(
      [
        variant({
          minChartMarkContrast: 2.04,
          chartBelowFloor: [
            { kind: "mark", floor: 3, ratio: 2.04 },
            { kind: "text", floor: 4.5, ratio: 3.1 },
          ],
        }),
      ],
      routes,
    );
    expect(rows[0].minChartMarkContrast).toBe(2.04);
    expect(fails.chartMarkContrast).toEqual(["/dashboard/security"]);
    expect(fails.chartTextContrast).toEqual(["/dashboard/security"]);
    expect(gateFailures(fails)).toEqual(["chartMarkContrast", "chartTextContrast"]);
  });

  it("fails a chart with no aria-label or no table control", () => {
    expect(gateFailures(summarize([variant({ chartsUnlabelled: 1 })], routes).fails)).toEqual(["chartAccess"]);
    expect(gateFailures(summarize([variant({ chartsWithoutTable: 1 })], routes).fails)).toEqual(["chartAccess"]);
  });

  it("fails a target under 24x24, and counts one under 44x44 or small text without failing", () => {
    const counted = summarize([variant({ smallTargets: 6, smallTextNodes: 12 })], routes);
    expect(counted.fails.smallTargets).toEqual(["/dashboard/security"]);
    expect(counted.fails.smallText).toEqual(["/dashboard/security"]);
    expect(gateFailures(counted.fails)).toEqual([]);
    const { rows, fails } = summarize([variant({ smallTargets: 6, targetsUnderMinimum: 2 }), variant({ targetsUnderMinimum: 1 })], routes);
    expect(rows[0].targetsUnderMinimum).toBe(2);
    expect(fails.targetSize).toEqual(["/dashboard/security"]);
    expect(gateFailures(fails)).toEqual(["targetSize"]);
  });

  it("fails a chart whose table control is under 44x44", () => {
    const { fails } = summarize([variant({ chartControlsUnderFloor: 1 })], routes);
    expect(fails.chartControlSize).toEqual(["/dashboard/security"]);
    expect(gateFailures(fails)).toEqual(["chartControlSize"]);
  });

  it("reports a paint outside the chart tokens without failing the gate", () => {
    const { fails } = summarize([variant({ offTokenPaints: 3 })], routes);
    expect(fails.offTokenPaints).toEqual(["/dashboard/security"]);
    expect(gateFailures(fails)).toEqual([]);
  });

  it("keeps one row per lens of the overview", () => {
    const lensRoutes = ["developer", "executive"].map((lens) => ({ route: "/dashboard", lens, file: "app/dashboard/page.tsx" }));
    const { rows, fails } = summarize(
      [
        variant({ route: "/dashboard", lens: "developer" }),
        variant({ route: "/dashboard", lens: "executive", chartBelowFloor: [{ kind: "mark", floor: 3, ratio: 1.87 }] }),
      ],
      lensRoutes,
    );
    expect(rows.map((r: { route: string }) => r.route)).toEqual(["/dashboard (developer lens)", "/dashboard (executive lens)"]);
    expect(fails.chartMarkContrast).toEqual(["/dashboard (executive lens)"]);
  });
});

// WCAG 2.x contrast of two opaque sRGB colours, for the expected values below.
function contrast(a: number[], b: number[]) {
  const lum = (c: number[]) =>
    c
      .map((v) => v / 255)
      .map((v) => (v <= 0.03928 ? v / 12.92 : Math.pow((v + 0.055) / 1.055, 2.4)))
      .reduce((sum, v, i) => sum + v * [0.2126, 0.7152, 0.0722][i], 0);
  const [hi, lo] = [lum(a), lum(b)].sort((x, y) => y - x);
  return (hi + 0.05) / (lo + 0.05);
}

const BLUE = [37, 99, 235];
const YELLOW = [250, 204, 21];
const GREY_TEXT = [75, 85, 99];
const WHITE = [255, 255, 255];
const rgb = (c: number[]) => `rgb(${c.join(", ")})`;

// Two charts as the chart layer renders them. The first carries its label and its
// table control and draws: a blue bar (passes 3:1), a yellow bar (fails), a blue bar
// at 30% fill opacity (fails once composited), a grid line in a chrome token (not a
// series mark, so never held to 3:1), a magenta legend swatch that is no chart token,
// an axis tick in the axis text token (passes 4.5:1) and a label drawn in the yellow
// series colour (fails 4.5:1). The second has neither a label nor a table control.
const CHART_PAGE = `<!doctype html>
<html><head><style>
  :root {
    --chart-1: ${rgb(BLUE)};
    --chart-2: ${rgb(YELLOW)};
    --chart-grid: rgb(229, 231, 235);
    --chart-axis-text: ${rgb(GREY_TEXT)};
  }
  body { margin: 0; background: ${rgb(WHITE)}; font-family: sans-serif; }
</style></head><body><main>
  <div>
    <button type="button" aria-controls="view-1">View as table</button>
    <div id="view-1">
      <div role="img" aria-label="Verifications per month: Jan 3, Feb 5">
        <svg width="200" height="120">
          <line x1="0" y1="100" x2="200" y2="100" stroke="var(--chart-grid)" stroke-width="1"></line>
          <rect x="10" y="40" width="20" height="60" fill="var(--chart-1)"></rect>
          <rect x="40" y="20" width="20" height="80" fill="var(--chart-2)"></rect>
          <rect x="70" y="30" width="20" height="70" fill="var(--chart-1)" fill-opacity="0.3"></rect>
          <rect x="100" y="5" width="10" height="10" fill="rgb(255, 0, 255)"></rect>
          <text x="10" y="115" fill="var(--chart-axis-text)" font-size="12">Jan</text>
          <text x="40" y="115" fill="var(--chart-2)" font-size="12">Feb</text>
        </svg>
      </div>
    </div>
  </div>
  <div>
    <div id="view-2">
      <div role="img">
        <svg width="50" height="50"><rect x="0" y="0" width="50" height="50" fill="var(--chart-1)"></rect></svg>
      </div>
    </div>
  </div>
</main></body></html>`;

// Targets as a page draws them: a 44px button (clears both floors), a 30px button
// (under 44, over 24), a button 20px tall (under 24), a link inside running text
// (exempt from both, as in WCAG 2.5.8), a standalone link 16px tall (under 24),
// and two charts whose table controls are 44px and 28px tall.
const box = (w: string, h: string) => `box-sizing: border-box; padding: 0; border: 0; width: ${w}; height: ${h};`;
const TARGET_PAGE = `<!doctype html>
<html><head><style>body { margin: 0; font-family: sans-serif; }</style></head><body><main>
  <button type="button" style="${box("44px", "44px")}">A</button>
  <button type="button" style="${box("30px", "30px")}">B</button>
  <button type="button" style="${box("60px", "20px")}">C</button>
  <p>Read the <a href="#guide">policy guide</a> before you start.</p>
  <div><a href="#agents" style="display: inline-block; font-size: 12px; line-height: 16px;">See all</a></div>
  <div>
    <button type="button" aria-controls="view-a" style="${box("auto", "44px")} min-width: 44px;">View as table</button>
    <div id="view-a"><div role="img" aria-label="Verifications: Jan 3"><svg width="50" height="50"></svg></div></div>
  </div>
  <div>
    <button type="button" aria-controls="view-b" style="${box("auto", "28px")}">View as table</button>
    <div id="view-b"><div role="img" aria-label="Attestations: Jan 2"><svg width="50" height="50"></svg></div></div>
  </div>
</main></body></html>`;

// Runs where a Chromium build for this Playwright is installed (`npx playwright
// install chromium`), as on any machine the sweep itself runs on.
const browserInstalled = existsSync(chromium.executablePath());

describe.skipIf(!browserInstalled)("ux-sweep chart measurement in a browser", () => {
  let browser: Browser;
  beforeAll(async () => {
    browser = await chromium.launch();
  }, 60_000);
  afterAll(async () => {
    await browser?.close();
  });

  type BelowFloor = { kind: "mark" | "text"; ratio: number; token?: string; text?: string };

  async function measure() {
    const page = await browser.newPage({ viewport: { width: 400, height: 400 } });
    await page.setContent(CHART_PAGE);
    const result = await page.evaluate(measureInPage, MEASURE_ARGS);
    await page.close();
    return { ...result, chartBelowFloor: result.chartBelowFloor as BelowFloor[] };
  }

  it("holds every series mark to 3:1 against the chart's background, at its composited opacity", async () => {
    const r = await measure();
    expect(r.chartCount).toBe(2);
    // Three bars in the first chart and one in the second; the grid line and the
    // off-token swatch are not series marks.
    expect(r.chartMarks).toBe(4);
    const faded = BLUE.map((c) => c * 0.3 + 255 * 0.7);
    const expectedMin = Math.min(contrast(YELLOW, WHITE), contrast(faded, WHITE));
    expect(r.minChartMarkContrast).toBeCloseTo(expectedMin, 1);
    const marks = r.chartBelowFloor.filter((f) => f.kind === "mark");
    expect(marks.map((f) => f.token).sort()).toEqual(["--chart-1", "--chart-2"]);
    for (const f of marks) expect(f.ratio).toBeLessThan(3);
    expect(contrast(BLUE, WHITE)).toBeGreaterThan(3);
  });

  it("holds text drawn inside a chart to the text floor", async () => {
    const r = await measure();
    expect(r.chartTextNodes).toBe(2);
    expect(r.minChartBodyContrast).toBeCloseTo(contrast(YELLOW, WHITE), 1);
    const text = r.chartBelowFloor.filter((f) => f.kind === "text");
    expect(text.map((f) => f.text)).toEqual(["Feb"]);
    expect(contrast(GREY_TEXT, WHITE)).toBeGreaterThan(4.5);
  });

  it("counts a chart without an aria-label or a table control, and a paint outside the tokens", async () => {
    const r = await measure();
    expect(r.chartsUnlabelled).toBe(1);
    expect(r.chartsWithoutTable).toBe(1);
    expect(r.offTokenPaints).toBe(1);
    expect(r.offTokenSamples[0].color).toEqual([255, 0, 255]);
  });

  it("holds every target to 24x24 and counts those under 44x44, exempting a link in running text", async () => {
    const page = await browser.newPage({ viewport: { width: 400, height: 400 } });
    await page.setContent(TARGET_PAGE);
    const r = await page.evaluate(measureInPage, MEASURE_ARGS);
    await page.close();
    type Target = { text: string; widthPx: number; heightPx: number };
    expect(r.interactiveTargets).toBe(6);
    expect(r.smallTargets).toBe(4);
    expect(r.targetsUnderMinimum).toBe(2);
    expect((r.targetMinimumSamples as Target[]).map((t) => [t.text, t.heightPx])).toEqual([
      ["C", 20],
      ["See all", 16],
    ]);
    expect(r.chartCount).toBe(2);
    expect(r.chartControlsUnderFloor).toBe(1);
    expect(r.chartControlSamples[0]).toMatchObject({ chart: "Attestations: Jan 2", heightPx: 28 });
  });
});
