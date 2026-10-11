"use client";

import { Cell, Legend, Pie, PieChart, ResponsiveContainer, Tooltip } from "recharts";
import { ChartFrame } from "./chart-frame";
import { describeParts } from "./describe";
import { chartTooltipStyle, type ChartColor } from "./tokens";

export interface DonutSlice {
  name: string;
  value: number;
  color: ChartColor;
}

export interface DonutChartProps {
  /** What the chart measures; read first in the aria-label. */
  title: string;
  data: DonutSlice[];
  /** How a slice value reads in the aria-label (default: the number). */
  formatValue?: (value: number) => string;
  /** Text drawn beside each slice; omit for no slice labels. */
  sliceLabel?: (slice: { name: string; value: number; percent: number }) => string;
  tooltip?: boolean;
  legend?: boolean;
  /** A summary drawn in the hole and read after the slices, e.g. an average. */
  center?: { value: string; caption: string };
  innerRadius?: number;
  outerRadius?: number;
  paddingAngle?: number;
  className?: string;
}

export function DonutChart({
  title,
  data,
  formatValue = String,
  sliceLabel,
  tooltip = true,
  legend = false,
  center,
  innerRadius = 60,
  outerRadius = 80,
  paddingAngle = 5,
  className,
}: DonutChartProps) {
  const parts = describeParts(
    title,
    data.map((d) => ({ name: d.name, value: formatValue(d.value) })),
  );
  const label = center ? `${parts}; ${center.caption} ${center.value}` : parts;
  return (
    <ChartFrame
      label={label}
      className={className}
      center={
        center ? (
          <>
            <span className="text-[22px] font-bold leading-none text-ink">{center.value}</span>
            <span className="mt-1 text-2xs text-ink-tertiary">{center.caption}</span>
          </>
        ) : undefined
      }
    >
      <ResponsiveContainer width="100%" height="100%">
        <PieChart>
          <Pie
            data={data}
            cx="50%"
            cy="50%"
            innerRadius={innerRadius}
            outerRadius={outerRadius}
            paddingAngle={paddingAngle}
            dataKey="value"
            nameKey="name"
            label={sliceLabel ? (p: { name: string; value: number; percent: number }) => sliceLabel(p) : false}
          >
            {data.map((slice) => (
              <Cell key={slice.name} fill={slice.color} />
            ))}
          </Pie>
          {tooltip && <Tooltip contentStyle={chartTooltipStyle} />}
          {legend && (
            <Legend
              wrapperStyle={{ paddingTop: "20px" }}
              formatter={(value: string) => <span className="text-ink-secondary text-sm">{value}</span>}
            />
          )}
        </PieChart>
      </ResponsiveContainer>
    </ChartFrame>
  );
}
