// The text a screen reader hears for a chart: what it measures, then the values
// it draws. Every primitive in this folder sets it as the chart's aria-label.

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
  const latest = series
    .map((s) => {
      const v = last[s.key];
      return `${s.name} latest ${typeof v === "number" ? formatValue(v) : "not reported"}`;
    })
    .join(", ");
  return `${title}: ${span}; ${latest}`;
}
