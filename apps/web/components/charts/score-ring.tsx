"use client";

import type { ReactNode } from "react";
import { Cell, Pie, PieChart, ResponsiveContainer } from "recharts";
import { ChartFrame } from "./chart-frame";
import { chartChrome, chartSeries, type ChartColor } from "./tokens";

export interface ScoreRingProps {
  /** What the score measures; read first in the aria-label. */
  title: string;
  value: number;
  /** The score's ceiling; the ring is full at this value. */
  max?: number;
  color?: ChartColor;
  /** Drawn in the middle of the ring; the label already reads the value and its ceiling. */
  children?: ReactNode;
  className?: string;
}

// One score against its ceiling, drawn as a ring filled clockwise from the top.
export function ScoreRing({ title, value, max = 100, color = chartSeries.brand, children, className }: ScoreRingProps) {
  const filled = Math.min(max, Math.max(0, value));
  const data = [
    { name: "score", value: filled },
    { name: "remaining", value: max - filled },
  ];
  return (
    <ChartFrame label={`${title}: ${value} of ${max}`} className={className} center={children}>
      <ResponsiveContainer width="100%" height="100%">
        <PieChart>
          <Pie
            data={data}
            dataKey="value"
            nameKey="name"
            cx="50%"
            cy="50%"
            startAngle={90}
            endAngle={-270}
            innerRadius="82%"
            outerRadius="98%"
            stroke="none"
            isAnimationActive={false}
          >
            <Cell fill={color} />
            <Cell fill={chartChrome.track} />
          </Pie>
        </PieChart>
      </ResponsiveContainer>
    </ChartFrame>
  );
}
