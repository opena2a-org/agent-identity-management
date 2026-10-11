// The text a screen reader hears for a chart: what it measures, then the values
// it draws. Every primitive in this folder sets it as the chart's aria-label,
// and the chart's table carries the same values.

import type { ChartTable } from "./chart-frame";

export interface DescribedPart {
  name: string;
  value: string;
}

export function describeParts(title: string, parts: DescribedPart[]): string {
  if (parts.length === 0) return `${title}: no data`;
  return `${title}: ${parts.map((p) => `${p.name} ${p.value}`).join(", ")}`;
}

export function describeTrend(
  title: string,
  data: ReadonlyArray<Record<string, unknown>>,
  xKey: string,
  series: ReadonlyArray<{ key: string; name: string }>,
  formatValue: (value: number) => string = String,
): string {
  if (data.length === 0) return `${title}: no data`;
  const first = String(data[0][xKey]);
  const last = data[data.length - 1];
  const span =
    data.length === 1 ? `1 point, ${first}` : `${data.length} points from ${first} to ${String(last[xKey])}`;
  const latest = series.map((s) => `${s.name} latest ${pointValue(last[s.key], formatValue)}`).join(", ");
  return `${title}: ${span}; ${latest}`;
}

// A point that does not carry a series' value reads as such, never as zero.
function pointValue(v: unknown, formatValue: (value: number) => string): string {
  return typeof v === "number" ? formatValue(v) : "not reported";
}

/** A field name as a column heading: "month" reads "Month". */
export function columnHeading(key: string): string {
  return key.charAt(0).toUpperCase() + key.slice(1);
}

/** A trend's table: one row per point, one column per series. */
export function trendTable(
  data: ReadonlyArray<Record<string, unknown>>,
  xKey: string,
  xLabel: string,
  series: ReadonlyArray<{ key: string; name: string }>,
  formatValue: (value: number) => string = String,
): ChartTable {
  return {
    columns: [xLabel, ...series.map((s) => s.name)],
    rows: data.map((d) => [String(d[xKey]), ...series.map((s) => pointValue(d[s.key], formatValue))]),
  };
}
