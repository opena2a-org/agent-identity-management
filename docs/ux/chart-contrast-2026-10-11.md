# Contrast receipt: chart sweep (2026-10-11)

The charts of the overview's three lenses, the security page and the compliance page,
measured in a browser with the readability sweep. Nothing below is estimated; every
figure is copied from the instrument's result file. No finding is fixed here: this is
the measurement.

## Instrument

- Script: `apps/web/scripts/ux-sweep.mjs`, at the commit that adds this document. That
  commit changes the script, its test, the changelog and this document only, so the
  dashboard measured is the dashboard of that commit.
- Command: `node apps/web/scripts/ux-sweep.mjs --base http://localhost:<port> --out result.json --only /dashboard,/dashboard/security,/dashboard/compliance --fixture populated`
- Dashboard: served by `next dev` (Next.js 16.3.6) from that tree. Every `/api/v1/**`
  request was answered by the instrument's route mock; the result file lists no request
  the mock did not recognise.
- Run: started 2026-10-11T16:24:40Z, finished 2026-10-11T16:25:26Z, exit code 1 (the gate
  below failed).
- Browser: Chromium through Playwright 1.59.1 (the `@playwright/test` dev dependency of
  `apps/web`), headless.

## Method

- **Routes and variants.** `/dashboard` once per lens (developer, security, executive;
  the lens is set in localStorage before each load), `/dashboard/security` and
  `/dashboard/compliance`: 5 entries, each at 375x812 and 1280x800 in the light and the
  dark theme, 20 loads.
- **Data.** The populated fixture answers the endpoints these charts read with sample
  series: six agents with trust scores 0.92, 0.85, 0.64, 0.45, 0.21 and none, two MCP
  servers, six months of verification activity, ten points of protection timeline, six
  points of trust score trend and a security score of 82. These are fixture values for
  measuring how a chart is drawn, not a measurement of any deployment.
- **What a chart is.** An element with `role="img"` that holds an `<svg>`, which is how
  `apps/web/components/charts` renders every chart.
- **Series marks.** Every painted `path`, `rect`, `circle`, `ellipse`, `polygon`,
  `polyline` or `line` in the chart whose computed fill or stroke is the colour of a
  series token (`--chart-1` to `--chart-7`, `--chart-muted`), each token resolved in the
  page's theme. The paint is composited at its fill or stroke opacity times the
  `opacity` of the shape and its svg ancestors over the chart's background, which is
  composited up the ancestor chain as for text. Floor: 3:1 (WCAG 1.4.11). Shapes in a
  chrome token (grid, axis line, track, cursor) are not series marks; shapes in no chart
  token at all are counted as off-token paints.
- **Chart text.** Every `<text>` inside the svg, measured the same way against the same
  background. Floors: 4.5:1, or 3:1 for large text.
- **Access.** Each chart is checked for a non-empty `aria-label` and for the
  `aria-controls` button that swaps it for its table.
- **Not modelled.** Contrast between adjacent marks, tooltips (drawn on hover only),
  gradient paints (the area fill under a trend line; its line is measured), HTML ancestor
  `opacity`, and backdrop blur. A paint is matched to a token by its RGB value.

## Gate

| Criterion | Entries failing | Threshold |
|---|---|---|
| Chart series mark contrast | 0 of 5 | under 3:1 in any variant |
| Chart text contrast | 1 of 5 (`/dashboard/compliance`, dark theme) | under 4.5:1 (3:1 large) in any variant |
| Chart without label or table control | 0 of 5 | any chart |
| Off-token paints (recorded, not gated) | 0 of 5 | any shape |
| Body text contrast, whole page | 5 of 5 | under 4.5:1 in at least one variant |
| Large text contrast, whole page | 0 of 5 | under 3:1 in at least one variant |
| Horizontal overflow | 1 of 5 (`/dashboard/compliance`, 104px at 375x812) | any overflow |
| Console errors | 0 of 5 | any |
| Could not load | 0 of 5 | navigation or measurement failure |

## Charts per entry

Minimums are the lowest over both viewports for that theme. "Charts" and "marks" are
the most found in one variant.

| Entry | Charts | Marks | Mark min, light | Mark min, dark | Chart text min, light | Chart text min, dark |
|---|---|---|---|---|---|---|
| /dashboard (developer lens) | 1 | 12 | 4.50 | 3.70 | no text | no text |
| /dashboard (security lens) | 0 | 0 | no chart | no chart | no chart | no chart |
| /dashboard (executive lens) | 2 | 18 | 3.53 | 3.70 | 5.48 | 4.79 |
| /dashboard/security | 2 | 23 | 3.40 | 3.70 | 5.48 | 4.79 |
| /dashboard/compliance | 2 | 5 | 3.55 | 4.14 | 5.71 | 4.05 |

Every chart found carried its `aria-label` and its table control. The developer lens
draws its verification activity without axes, so it has no chart text. The security
lens draws no chart.

## Lowest mark per entry

The same in both viewports. Colours are the composited values the ratio was computed
from.

| Entry | Theme | Token | Paint | Foreground | Background | Ratio |
|---|---|---|---|---|---|---|
| /dashboard (developer lens) | light | `--chart-1` | fill | rgb(0,113,227) | rgb(249,250,253) | 4.50 |
| /dashboard (developer lens) | dark | `--chart-1` | fill | rgb(0,113,227) | rgb(25,26,29) | 3.70 |
| /dashboard (executive lens) | light | `--chart-6` | fill | rgb(8,145,178) | rgb(249,250,253) | 3.53 |
| /dashboard (executive lens) | dark | `--chart-1` | fill | rgb(0,113,227) | rgb(25,26,29) | 3.70 |
| /dashboard/security | light | `--chart-4` | stroke | rgb(255,59,48) | rgb(249,250,253) | 3.40 |
| /dashboard/security | dark | `--chart-1` | stroke | rgb(0,113,227) | rgb(25,26,29) | 3.70 |
| /dashboard/compliance | light | `--chart-4` | fill | rgb(255,59,48) | rgb(255,255,255) | 3.55 |
| /dashboard/compliance | dark | `--chart-4` | fill | rgb(255,59,48) | rgb(31,41,55) | 4.14 |

## Findings

- **Compliance trend chart, dark theme: axis text at 4.05:1.** Every axis tick of
  "Average trust score, last 30 days" (11 at 1280x800, 8 at 375x812) is drawn in the axis
  text colour, rgb(134,134,139), on rgb(31,41,55). That background is the card around the
  chart, `bg-white dark:bg-gray-800` in `apps/web/app/dashboard/compliance/page.tsx`,
  not a glass surface. `tests/chart-contrast.test.ts` holds the axis text to 4.5:1 against
  the glass card over the page gradient and the solid glass fallback only, so it does
  not see this card. All 19 chart texts under their floor belong to this chart. The risk
  distribution donut on the same page draws its slices on the same kind of card and
  clears 3:1 (4.14 dark, 3.55 light).
- **The whole-page failures on these routes are outside the charts.** The lowest body
  text per entry: the "Trust:" label on the compliance page, 2.43 light and 2.56 dark
  (the route sweep of 2026-10-05 measured the same label at the page's earlier path,
  `/dashboard/admin/compliance`); the agent initial in the developer
  lens's agent list, 4.14 light and 4.24 dark, which renders only when the list has
  agents; the "monitored" unit in the security lens, 4.20 dark; and the active sidebar
  link, 4.38 dark, on the executive lens and the security page. The compliance page
  also overflows by 104px at 375x812.
