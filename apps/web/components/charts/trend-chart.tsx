"use client";

import { useId, type ReactNode } from "react";
import {
  Area,
  CartesianGrid,
  ComposedChart,
  Legend,
  Line,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";
import { ChartFrame } from "./chart-frame";
import { describeTrend } from "./describe";
import { chartAxisTick, chartChrome, chartTooltipStyle, type ChartColor } from "./tokens";

export interface TrendSeries {
  /** Field of each point holding this series' value. */
  key: string;
  name: string;
  color: ChartColor;
  /** "area" fills below the line; "line" draws the line alone. */
  kind?: "area" | "line";
  /** Mark every point, for sparse series. */
  dots?: boolean;
}

export interface TrendChartProps<T extends Record<string, unknown>> {
  /** What the chart measures and over which window; read first in the aria-label. */
  title: string;
  data: T[];
  /** Field holding the x-axis value (a date or time label). */
  xKey: keyof T & string;
  series: TrendSeries[];
  yDomain?: [number, number];
  /** Unit drawn along the y axis. */
  yLabel?: string;
  /** How a value reads in the aria-label (default: the number). */
  formatValue?: (value: number) => string;
  legend?: boolean;
  /** Replaces the default tooltip body for the hovered point. */
  renderTooltip?: (point: T) => ReactNode;
  className?: string;
}

const AREA_TOP_OPACITY = 0.2;

export function TrendChart<T extends Record<string, unknown>>({
  title,
  data,
  xKey,
  series,
  yDomain,
  yLabel,
  formatValue,
  legend = false,
  renderTooltip,
  className,
}: TrendChartProps<T>) {
  // url(#...) needs an id without the punctuation useId may produce.
  const gradientBase = `chart-area-${useId().replace(/[^a-zA-Z0-9_-]/g, "")}`;
  const label = describeTrend(title, data, xKey, series, formatValue);
  return (
    <ChartFrame label={label} className={className}>
      <ResponsiveContainer width="100%" height="100%">
        <ComposedChart data={data} margin={yLabel ? { top: 5, right: 30, left: 20, bottom: 5 } : undefined}>
          <defs>
            {series.map((s, i) =>
              (s.kind ?? "area") === "area" ? (
                <linearGradient key={s.key} id={`${gradientBase}-${i}`} x1="0" y1="0" x2="0" y2="1">
                  <stop offset="5%" stopColor={s.color} stopOpacity={AREA_TOP_OPACITY} />
                  <stop offset="95%" stopColor={s.color} stopOpacity={0} />
                </linearGradient>
              ) : null,
            )}
          </defs>
          <CartesianGrid strokeDasharray="3 3" stroke={chartChrome.grid} />
          <XAxis dataKey={xKey} stroke={chartChrome.axisLine} tick={chartAxisTick} interval="preserveStartEnd" />
          <YAxis
            stroke={chartChrome.axisLine}
            tick={chartAxisTick}
            domain={yDomain}
            allowDecimals={false}
            label={
              yLabel
                ? { value: yLabel, angle: -90, position: "insideLeft", fill: chartChrome.axisText, fontSize: 12 }
                : undefined
            }
          />
          {renderTooltip ? (
            <Tooltip
              content={({ active, payload }: { active?: boolean; payload?: Array<{ payload: T }> }) =>
                active && payload && payload.length ? renderTooltip(payload[0].payload) : null
              }
            />
          ) : (
            <Tooltip contentStyle={chartTooltipStyle} />
          )}
          {legend && (
            <Legend formatter={(value: string) => <span className="text-ink-secondary text-sm">{value}</span>} />
          )}
          {series.map((s, i) =>
            (s.kind ?? "area") === "area" ? (
              <Area
                key={s.key}
                type="monotone"
                dataKey={s.key}
                name={s.name}
                stroke={s.color}
                strokeWidth={2}
                fill={`url(#${gradientBase}-${i})`}
                dot={s.dots ? { r: 3, fill: s.color } : false}
              />
            ) : (
              <Line
                key={s.key}
                type="monotone"
                dataKey={s.key}
                name={s.name}
                stroke={s.color}
                strokeWidth={2}
                dot={s.dots ? { r: 3, fill: s.color } : false}
                activeDot={{ r: 5 }}
              />
            ),
          )}
        </ComposedChart>
      </ResponsiveContainer>
    </ChartFrame>
  );
}
