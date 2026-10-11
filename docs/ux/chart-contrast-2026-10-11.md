# Contrast receipt: chart sweep (2026-10-11)

The charts of the overview's three lenses, the security page and the compliance page,
measured in a browser with the readability sweep. Nothing below is estimated; every
figure is copied from the instrument's result file. No finding is fixed in the first
run: it is the measurement. The last section is a second run, after the compliance
page's two chart cards were changed.

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

## Second run: the compliance chart cards on glass

The first finding above is fixed in the commit that adds this section: the two cards
that hold the compliance page's charts ("Trust Score Trend" and "Risk Distribution") are
glass cards, as on every other chart page, and the text inside them uses the ink
colours. Nothing else on the page changed.

- Command: the same, over the same five entries with the populated fixture.
- Run: started 2026-10-11T17:51:07Z, finished 2026-10-11T17:51:53Z, 20 loads, exit code 1
  (the whole-page criteria below still fail). The result file lists no request the mock
  did not recognise.

| Criterion | Entries failing, first run | Entries failing, second run |
|---|---|---|
| Chart series mark contrast | 0 of 5 | 0 of 5 |
| Chart text contrast | 1 of 5 | 0 of 5 |
| Chart without label or table control | 0 of 5 | 0 of 5 |
| Off-token paints (recorded, not gated) | 0 of 5 | 0 of 5 |
| Body text contrast, whole page | 5 of 5 | 5 of 5 |
| Large text contrast, whole page | 0 of 5 | 0 of 5 |
| Horizontal overflow | 1 of 5 | 1 of 5 (`/dashboard/compliance`, 104px at 375x812) |
| Console errors | 0 of 5 | 0 of 5 |
| Could not load | 0 of 5 | 0 of 5 |

`/dashboard/compliance`, the same in both viewports:

| Measure | Theme | First run | Second run | Foreground | Background, second run |
|---|---|---|---|---|---|
| Chart text minimum | light | 5.71 | 5.48 | rgb(102,102,107) | rgb(249,250,253) |
| Chart text minimum | dark | 4.05 | 4.79 | rgb(134,134,139) | rgb(25,26,29) |
| Mark minimum (`--chart-4`, fill) | light | 3.55 | 3.40 | rgb(255,59,48) | rgb(249,250,253) |
| Mark minimum (`--chart-4`, fill) | dark | 4.14 | 4.90 | rgb(255,59,48) | rgb(25,26,29) |

The light theme's figures fall because the glass card is a shade darker than white;
both stay over their floors. No chart text on the page is under its floor in any
variant (19 were in the first run). The other four entries measured what they did in
the first run: marks 4.50 and 3.70 (developer lens), 3.53 and 3.70 (executive lens),
3.40 and 3.70 (security page), and chart text 5.48 light and 4.79 dark on the executive
lens and the security page.

`apps/web/tests/chart-surface.test.ts` now fails when a chart is drawn on anything but
the glass card, so the surface `tests/chart-contrast.test.ts` measures is the surface
every chart has.

Not changed by this: the page's other cards keep `bg-white dark:bg-gray-800`, so the
two chart cards differ from their neighbours in fill and corner radius; the "Trust:"
label still measures 2.43 light and 2.56 dark; and the page still overflows by 104px at
375x812.

## Third run: the whole compliance page on the theme tokens

The commit that adds this section moves every card on the compliance page onto the glass
card and every colour on the page onto the theme tokens (the ink colours for text, the
success, warning, danger and brand tokens for status, `bg-glass-inset-gray`, `bg-track`
and `border-divider` for surfaces and rules). The page carried 198 lines with a palette
class; it now carries none. No other page or component of the dashboard changes in that
commit, so only `/dashboard/compliance` was measured, before and after, on the same
dashboard served by `next dev`.

- Command: `node apps/web/scripts/ux-sweep.mjs --base http://localhost:<port> --out result.json --only /dashboard/compliance --fixture populated`
- Before: started 2026-10-11T18:08:13Z, finished 2026-10-11T18:08:22Z, 4 loads, exit code 1.
- After: started 2026-10-11T18:20:27Z, finished 2026-10-11T18:20:36Z, 4 loads, exit code 1
  (the body text criterion still fails, on the sidebar link named below). Neither result
  file lists a request the mock did not recognise, and neither recorded a console error.

| Measure | Variant | Before | After |
|---|---|---|---|
| Body text minimum | 375x812 light | 2.43 ("Trust:") | 4.67 ("View ABOM") |
| Body text minimum | 1280x800 light | 2.43 ("Trust:") | 4.62 (active sidebar link) |
| Body text minimum | 375x812 dark | 2.56 ("Trust:") | 4.38 (active sidebar link) |
| Body text minimum | 1280x800 dark | 2.56 ("Trust:") | 4.59 (active sidebar link) |
| Horizontal overflow | 375x812, both themes | 104px | 0px |
| Horizontal overflow | 1280x800, both themes | 0px | 0px |
| Chart text minimum | light / dark | 5.48 / 4.79 | 5.48 / 4.79 |
| Series mark minimum | light / dark | 3.40 / 4.90 | 3.40 / 4.90 |

- **The overflow.** Each event in the audit trail is a `flex-1` box beside its timeline
  dot. A flex item is never narrower than its content unless it carries `min-w-0`, so
  the box was as wide as its whole description line before truncation (a registration
  event's URL): the widest box ended at 479px on a 375px screen. The box now carries
  `min-w-0`, and the description is cut with an ellipsis inside it.
- **Text under 4.5:1.** Before, the sweep listed text under the floor in all four
  variants (its list stops at 25 entries per variant, and the light variants reached it):
  the "Trust:" label with its score and the event status ("verified", "pending",
  "suspended"), all `text-gray-400` at 2.43 light and 2.56 dark; the page subtitle at 4.15
  and the ABOM description at 4.32 in the light theme; and the indigo "registration" badge
  at 4.10 in the dark theme. After, the only text under the floor in any variant is the
  active sidebar link at 375x812 dark. The label and the status are now the secondary ink
  colour.
- **Badges inside the event box.** A first pass filled the event box with
  `bg-glass-inset-gray`; the brand-tinted "registration" badge on that second tint
  measured 4.28 light and 4.00 dark. The event box is now outlined with `border-divider`
  rather than filled, so a badge sits on one tint over the glass card.
- **Left under the floor:** the active sidebar link, 4.38 in the dark theme at 375x812.
  It belongs to the shared navigation, not to this page, and was recorded on the
  executive lens and the security page in the first run.

`apps/web/tests/page-standard.test.ts` holds the overview's lenses, the security page and
the compliance page to two rules read from the source: no palette class, and no growing
flex item (`flex-1`, `grow`) around a `truncate` line without `min-w-0` or a clipping
overflow. Before this commit it reported the 198 palette lines and the event box at the
line the browser measured.

## Fourth run: the last text under the floor on the five entries

The commit that adds this section changes the three kinds of text the earlier runs left
under 4.5:1 on these entries: the active link of the shared navigation, the agent initials
in the developer lens's agent list, and the unit beside a figure in the security lens's
inset tiles. Nothing else on the dashboard changes in that commit.

- Command: the same as the second run, over the same five entries with the populated
  fixture, on the dashboard served by `next dev` before and after the change.
- Before: started 2026-10-11T18:31:14Z, finished 2026-10-11T18:32:00Z, 20 loads, exit code 1.
- After: started 2026-10-11T18:37:24Z, finished 2026-10-11T18:38:10Z, 20 loads, exit code 0
  (`GATE: PASS`). Neither result file lists a request the mock did not recognise, and
  neither recorded a console error.

| Criterion | Entries failing, before | Entries failing, after |
|---|---|---|
| Body text contrast, whole page | 5 of 5 | 0 of 5 |
| Every other gated criterion | 0 of 5 | 0 of 5 |

Body text minimum per variant. Text under the floor, summed over the 20 variants: 21
before, 0 after.

| Entry | Variant | Before | After |
|---|---|---|---|
| /dashboard (developer lens) | 375x812 light | 4.14 (agent initial) | 4.70 |
| /dashboard (developer lens) | 1280x800 light | 4.14 (agent initial) | 4.70 |
| /dashboard (developer lens) | 375x812 dark | 4.24 (agent initial) | 4.59 |
| /dashboard (developer lens) | 1280x800 dark | 4.24 (agent initial) | 4.70 |
| /dashboard (security lens) | 375x812 light | 4.85 | 5.28 |
| /dashboard (security lens) | 1280x800 light | 4.62 (active sidebar link) | 4.70 |
| /dashboard (security lens) | 375x812 dark | 4.20 ("monitored") | 4.59 |
| /dashboard (security lens) | 1280x800 dark | 4.20 ("monitored") | 4.70 |
| /dashboard (executive lens) | 375x812 light | 4.70 | 4.70 |
| /dashboard (executive lens) | 1280x800 light | 4.62 (active sidebar link) | 4.70 |
| /dashboard (executive lens) | 375x812 dark | 4.38 (active sidebar link) | 4.59 |
| /dashboard (executive lens) | 1280x800 dark | 4.59 (active sidebar link) | 4.70 |
| /dashboard/compliance | 375x812 light | 4.67 | 4.67 |
| /dashboard/compliance | 1280x800 light | 4.62 (active sidebar link) | 4.67 |
| /dashboard/compliance | 375x812 dark | 4.38 (active sidebar link) | 4.59 |
| /dashboard/compliance | 1280x800 dark | 4.59 (active sidebar link) | 4.59 |
| /dashboard/security | 375x812 light | 4.67 | 4.67 |
| /dashboard/security | 1280x800 light | 4.62 (active sidebar link) | 4.67 |
| /dashboard/security | 375x812 dark | 4.38 (active sidebar link) | 4.59 |
| /dashboard/security | 1280x800 dark | 4.59 (active sidebar link) | 4.59 |

After the change the lowest text in each variant is unchanged text the sweep already held
over the floor: white on the brand button (4.70), brand text on its tint (4.67 light, 4.59
dark), and the "Viewing as" label in the phone drawer (4.59 dark).

- **Active sidebar link.** It was drawn in `--brand-text` on `--nav-active-fill`:
  rgb(41,151,255) on rgb(32,49,68) in the dark phone drawer (4.38), and rgb(0,102,204) on
  rgb(223,235,250) over the light chrome (4.62). It now has its own token,
  `--nav-active-text`. Computed from the tokens over every surface the navigation sits on
  (the chrome over each page stop, bare and under either wash, the solid chrome, and the
  drawer's opaque overlay), the lowest ratio is 4.30 dark and 4.40 light for `--brand-text`
  and 4.93 dark and 5.21 light for the new token.
- **Agent initials.** The four tints were hex gradients written into the overview and the
  check-in panel; the initial was measured against the gradient's second stop: rgb(21,128,61)
  on rgb(187,247,208) (4.14 light) and rgb(253,164,175) on rgb(159,18,57) (4.24 dark), with
  the sky and rose initials at 4.47 and 4.46 light. The tints are now `--avatar-1` to `-4`
  in `app/globals.css`, drawn by `.avatar-tint-1` to `-4`, with the same gradients and an
  initial one step deeper (light) or lighter (dark) where it fell short. Lowest computed
  ratio on either stop: 5.12 light and 4.87 dark.
- **Inset tile unit.** "monitored", "verified" and "incidents" were the tertiary ink,
  rgb(134,134,139) on rgb(37,38,40) (4.20 dark). They are now the secondary ink. Computed
  over the inset tile on the glass card: tertiary 3.94 dark and 4.81 light, secondary 5.55
  dark and 5.60 light.

Not changed: the sweep's ungated counts of text under 14px and targets under 44x44 still
list all five entries, as in every earlier run.

`apps/web/tests/text-contrast.test.ts` holds the three pairings to 4.5:1 from the tokens
in both themes, and `apps/web/tests/page-standard.test.ts` now also fails on a hex value or
colour function on these screens. Before this commit they reported the missing tokens and
the five hex lines.

## Fifth run: target size on the five entries

Until the commit that adds this section, the sweep counted targets under 44x44 and text
under 14px and failed on neither. (The fourth run above first named them as text under
12px and targets under 24px; the sweep's thresholds were 14px and 44x44 then as now, and
that line is corrected.) The commit changes the instrument in two ways:

- **Targets under 24x24 fail the gate.** 24x24 CSS px is the minimum target of WCAG
  2.5.8. A link inside running text is exempt, as in 2.5.8. The spacing exception of
  2.5.8 is not applied, so an undersized target counts wherever it sits and the gate is
  stricter than 2.5.8.
- **A chart's "view as table" control under 44x44 fails the gate**, the floor the chart
  standard sets for chart controls. Every chart found in both runs below has a control of
  at least 44x44.

Targets under 44x44 and text under 14px stay counted and ungated.

- Command: the same as the second run, over the same five entries with the populated
  fixture, on the dashboard served by `next dev`.
- Before: the changed instrument over the page sources of the previous commit. Started
  2026-10-11T18:56:30Z, finished 2026-10-11T18:57:16Z, 20 loads, exit code 1.
- After: started 2026-10-11T18:57:26Z, finished 2026-10-11T18:58:11Z, 20 loads, exit code 0
  (`GATE: PASS`). Neither result file lists a request the mock did not recognise, and
  neither recorded a console error.

| Criterion | Entries failing, before | Entries failing, after |
|---|---|---|
| Target under 24x24 | 4 of 5 | 0 of 5 |
| Chart table control under 44x44 | 0 of 5 | 0 of 5 |
| Every other gated criterion | 0 of 5 | 0 of 5 |
| Targets under 44x44 (counted, not gated) | 5 of 5 | 5 of 5 |
| Text under 14px (counted, not gated) | 5 of 5 | 5 of 5 |

Targets under 24x24 before the change, the same in both themes. Summed over the 20
variants: 30 before, 0 after.

| Entry | Target | 375x812 | 1280x800 |
|---|---|---|---|
| /dashboard (developer lens) | "See all" in the agents card | 39x16 | 39x16 |
| /dashboard (developer lens) | SDK language tabs (Python, TypeScript, Java) | 21.6 tall | 21.6 tall |
| /dashboard (security lens) | "Open Security" | 301x16 | 318x16 |
| /dashboard (executive lens) | "Open compliance" | 309x16 | 201x16 |
| /dashboard/security | "View all" | 69.3x20, one link | 69.3x20, two links |

Each of these controls now has a minimum height of 24px. The links that were inline also
became inline-flex, since an inline box ignores a minimum height. Nothing else on the
entries moved: every other per-entry figure in the two result files (body, large and
chart text minimums, series mark minimum, overflow, the number of charts, marks and
interactive targets) is identical.

`apps/web/tests/page-standard.test.ts` now reads the height of every link and button on
these screens from its classes (`tests/target-size.ts`) and fails on one under 24px.
Before the change it reported 11 controls in the source: the six behind the table above
(the two "View all" links are two controls), and five the populated fixture does not draw,
which get the same change: the security page's "Review agent" and "Investigate"
links and the "View all" beside blocked actions (shown past ten), and the compliance
page's "View all alerts" (shown past eight alerts) and "Reset page" buttons.
`apps/web/tests/ux-sweep.test.ts` holds the new criteria: a target page in the browser
with targets on both sides of each floor, and the gate.

## Sixth run: the A2A, supply chain and agent detail pages

The A2A page, the supply chain page and the agent detail page draw their charts through
the same chart layer, but the populated fixture left them empty: swept with it at the
previous commit (started 2026-10-11T19:04:56Z, 12 loads), the A2A page's two charts drew
0 marks, the agent detail page drew no chart (its trust score history was empty), and the
supply chain page's two charts drew 9 marks from two MCP servers with no confidence score.

The commit that adds this section extends the populated fixture to the endpoints those
charts read: three A2A agent cards, eight A2A tasks in five states, A2A trust scores of
0.94, 0.78, 0.55 and 0.32, confidence scores of 96 and 58 on the two MCP servers, seven
days of attestations with three agent to MCP server connections, and six trust score
history points from 0.62 to 0.82. These are fixture values for measuring how a chart is
drawn, not a measurement of any deployment. The five entries of the earlier runs read
none of these endpoints, apart from the MCP server list, whose confidence score they do
not draw.

- Command: the same as the second run with
  `--only /dashboard/a2a,/dashboard/mcp/supply-chain,/dashboard/agents/00000000-0000-4000-8000-000000000001`,
  on the dashboard served by `next dev`.
- Before: the changed fixture over the page sources of the previous commit. Started
  2026-10-11T19:06:37Z, finished 2026-10-11T19:07:04Z, 12 loads, exit code 1.
- After: started 2026-10-11T19:14:09Z, finished 2026-10-11T19:14:36Z, 12 loads, exit code 0
  (`GATE: PASS`).
- All eight entries (the five above and these three) after the change: started
  2026-10-11T19:14:53Z, finished 2026-10-11T19:16:06Z, 32 loads, exit code 0. No result
  file lists a request the mock did not recognise, and none recorded a console error.

| Criterion | Entries failing, before | Entries failing, after |
|---|---|---|
| Body text contrast, whole page | 2 of 3 | 0 of 3 |
| Large text contrast, whole page | 1 of 3 | 0 of 3 |
| Target under 24x24 | 2 of 3 | 0 of 3 |
| Horizontal overflow | 2 of 3 | 0 of 3 |
| Chart series mark, chart text, label and table control | 0 of 3 | 0 of 3 |
| Targets under 44x44 (counted, not gated) | 3 of 3 | 3 of 3 |
| Text under 14px (counted, not gated) | 3 of 3 | 3 of 3 |

What failed before the change, the same in both viewports unless named:

| Entry | Finding | Before |
|---|---|---|
| /dashboard/a2a | The header's "Refresh" and "A2A spec" stay beside the title at 375x812 | 25px overflow |
| /dashboard/mcp/supply-chain | The header's "Refresh" and "Export report" stay beside the title at 375x812 | 27px overflow |
| /dashboard/mcp/supply-chain | "Active connections" figure in `--brand-sky`, large text, light | 2.05 |
| /dashboard/mcp/supply-chain | "Bill of Materials" tab label, brand ink on the brand tint, light | 4.21 |
| /dashboard/mcp/supply-chain | Eye button in each MCP server row, no accessible name | 16x16, 2 buttons |
| /dashboard/agents/:id | Trust score badges in the activity timeline, a badge tint on a tinted event card | 4.21 light, 3.58 dark |
| /dashboard/agents/:id | Trust score history table headers, tertiary ink on the inset tint, dark | 4.20 |
| /dashboard/agents/:id | "About drift score" button | 16x16 |
| /dashboard/agents/:id | Info button beside each of the nine trust factors, no accessible name | 14x14, 9 buttons |

Targets under 24x24 summed over the 12 variants: 48 before, 0 after. The changes: below
640px both headers put their buttons under the title, and the buttons wrap; the
connections figure and its icon
use the brand ink; the tab label uses the neutral badge of the tab beside it; the eye
button is 32x32 and named for its server; the timeline's event cards are outlined
rather than filled, as on the compliance page; the table headers use the secondary ink;
the drift and factor info buttons are 24x24, and each factor's is named for its factor.

Chart figures after the change. Minimums are the lowest over both viewports for that
theme.

| Entry | Charts | Marks | Mark min, light | Mark min, dark | Chart text min, light | Chart text min, dark |
|---|---|---|---|---|---|---|
| /dashboard/a2a | 2 | 14 | 3.40 | 3.70 | 5.48 | 4.79 |
| /dashboard/mcp/supply-chain | 2 | 11 | 3.40 | 3.70 | 5.48 | 4.79 |
| /dashboard/agents/:id | 1 | 14 | 4.50 | 3.70 | 5.48 | 4.79 |

The lowest body text after the change is 4.67 light and 4.59 dark on the A2A and agent
detail pages, 4.70 light and 4.59 dark on the supply chain page.

`apps/web/tests/page-standard.test.ts` now holds these three pages, and the trust score
and drift score cards the agent detail page draws, to the same source rules as the five
entries above. Before the change it reported 19 raw colours, 4 growing flex items around
truncated text and 1 link under 24px. Of the colours, 14 were in the supply chain page's
printable report, a standalone document that loads no theme: its stylesheet moves to
`apps/web/lib/abom-print-style.ts` with the same rules, and the one inline style in the
report becomes a class there. One was an issue number in a comment read as a hex value,
and four were palette classes in the drift score card: its baseline state, which the
fixture does not draw, its error icon and the hover colour of its info icon. Of the flex
items, two were `w-0 flex-1` stat cards, which shrink (a set width caps the automatic
minimum), so the rule now accepts `w-0`; the other two were the supply chain page's
connection rows, which now carry `min-w-0`. The link is the A2A page's agent card URL,
now at least 24px tall. `apps/web/tests/ux-sweep.test.ts` holds the fixture to drawing
more than one point or slice in each of these charts, and the default fixture to leaving
them empty.
