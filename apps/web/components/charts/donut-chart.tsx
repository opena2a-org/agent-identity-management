"use client";

import { Cell, Legend, Pie, PieChart, ResponsiveContainer, Tooltip } from "recharts";
import { ChartFrame } from "./chart-frame";
import { describeParts } from "./describe";
import { chartChrome, chartTooltipStyle, type ChartColor } from "./tokens";

// What recharts passes a slice label: the slice, and where its label goes. The
// label is drawn as text in the axis text colour; recharts would otherwise fill it
// with the slice colour, which holds the graphics floor but not the text floor.
interface SliceLabelProps {
  name: string;
  value: number;
  percent: number;
  x: number;
  y: number;
  textAnchor: "start" | "middle" | "end";
}

export interface DonutSlice {
  name: string;
  value: number;
  color: ChartColor;
}

export interface DonutChartProps {
  /** What the chart measures; read first in the aria-label. */
  title: string;
  data: DonutSlice[];
  /** How a slice value reads in the aria-label and the table (default: the number). */
  formatValue?: (value: number) => string;
  /** The table's column headings: what a slice is, then what its value counts. */
  columns?: [string, string];
  /** Text drawn beside each slice, in the axis text colour; omit for no slice labels. */
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
  columns = ["Category", "Value"],
  sliceLabel,
  tooltip = true,
  legend = false,
  center,
  innerRadius = 60,
  outerRadius = 80,
  paddingAngle = 5,
  className,
}: DonutChartProps) {
  const parts = data.map((d) => ({ name: d.name, value: formatValue(d.value) }));
  const described = describeParts(title, parts);
  const label = center ? `${described}; ${center.caption} ${center.value}` : described;
  const table = {
    columns,
    rows: parts.map((p) => [p.name, p.value]),
    summary: center ? [[center.caption, center.value]] : undefined,
  };
  return (
    <ChartFrame
      title={title}
      label={label}
      table={table}
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
            label={
              sliceLabel
                ? (p: SliceLabelProps) => (
                    <text
                      x={p.x}
                      y={p.y}
                      textAnchor={p.textAnchor}
                      dominantBaseline="central"
                      fill={chartChrome.axisText}
                      fontSize={12}
                    >
                      {sliceLabel(p)}
                    </text>
                  )
                : false
            }
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
