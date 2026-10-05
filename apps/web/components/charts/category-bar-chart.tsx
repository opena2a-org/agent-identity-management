"use client";

import { Bar, BarChart, CartesianGrid, ResponsiveContainer, Tooltip, XAxis, YAxis } from "recharts";
import { ChartFrame } from "./chart-frame";
import { describeParts } from "./describe";
import { chartAxisTick, chartChrome, chartSeries, chartTooltipStyle, type ChartColor } from "./tokens";

export interface CategoryBarChartProps<T extends Record<string, unknown>> {
  /** What the chart measures; read first in the aria-label. */
  title: string;
  data: T[];
  /** Field holding the category name drawn on the x axis. */
  categoryKey: keyof T & string;
  /** Field holding the count drawn as the bar. */
  valueKey: keyof T & string;
  color?: ChartColor;
  className?: string;
}

export function CategoryBarChart<T extends Record<string, unknown>>({
  title,
  data,
  categoryKey,
  valueKey,
  color = chartSeries.brand,
  className,
}: CategoryBarChartProps<T>) {
  const label = describeParts(
    title,
    data.map((d) => ({ name: String(d[categoryKey]), value: String(d[valueKey]) })),
  );
  return (
    <ChartFrame label={label} className={className}>
      <ResponsiveContainer width="100%" height="100%">
        <BarChart data={data}>
          <CartesianGrid strokeDasharray="3 3" stroke={chartChrome.grid} />
          <XAxis dataKey={categoryKey} stroke={chartChrome.axisLine} tick={chartAxisTick} />
          <YAxis stroke={chartChrome.axisLine} tick={chartAxisTick} allowDecimals={false} />
          <Tooltip contentStyle={chartTooltipStyle} cursor={{ fill: chartChrome.cursor }} />
          <Bar dataKey={valueKey} fill={color} radius={[4, 4, 0, 0]} />
        </BarChart>
      </ResponsiveContainer>
    </ChartFrame>
  );
}
