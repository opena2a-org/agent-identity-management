// Chart colours. Every value names a --chart-* custom property declared once in
// app/globals.css for the light and dark themes, so a chart follows the theme and
// a contrast correction made there reaches every chart. Chart code takes its
// colours from here and nowhere else: no hex, rgb/hsl or raw palette class.

export const chartSeries = {
  brand: "var(--chart-1)",
  green: "var(--chart-2)",
  amber: "var(--chart-3)",
  red: "var(--chart-4)",
  indigo: "var(--chart-5)",
  sky: "var(--chart-6)",
  orange: "var(--chart-7)",
  muted: "var(--chart-muted)",
} as const;

export type ChartColor = (typeof chartSeries)[keyof typeof chartSeries];

export const chartChrome = {
  grid: "var(--chart-grid)",
  axisLine: "var(--chart-axis-line)",
  axisText: "var(--chart-axis-text)",
  cursor: "var(--chart-cursor)",
} as const;

export const chartTooltipStyle = {
  backgroundColor: "var(--chart-tooltip-bg)",
  border: "1px solid var(--chart-tooltip-border)",
  borderRadius: "12px",
  boxShadow: "var(--chart-tooltip-shadow)",
  color: "var(--chart-tooltip-text)",
} as const;

export const chartAxisTick = { fill: chartChrome.axisText, fontSize: 12 } as const;
