// Recharts v2 class components are incompatible with React 19 JSX types.
// This declaration patches the affected components to work as JSX elements.
// Remove this file when upgrading to recharts v3 which supports React 19.
import type { ComponentType } from "react";

declare module "recharts" {
  export const BarChart: ComponentType<any>;
  export const Bar: ComponentType<any>;
  export const XAxis: ComponentType<any>;
  export const YAxis: ComponentType<any>;
  export const CartesianGrid: ComponentType<any>;
  export const Tooltip: ComponentType<any>;
  export const ResponsiveContainer: ComponentType<any>;
  export const PieChart: ComponentType<any>;
  export const Pie: ComponentType<any>;
  export const Cell: ComponentType<any>;
  export const Legend: ComponentType<any>;
  export const AreaChart: ComponentType<any>;
  export const Area: ComponentType<any>;
  export const LineChart: ComponentType<any>;
  export const Line: ComponentType<any>;
}
