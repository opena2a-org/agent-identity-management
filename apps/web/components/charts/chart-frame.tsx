"use client";

import { useId, useState, type ReactNode } from "react";
import { cn } from "@/lib/utils";

/** The values a chart draws, as rows: the chart's "view as table" alternative. */
export interface ChartTable {
  /** Column headings; the first names what each row is. */
  columns: string[];
  rows: string[][];
  /** Rows read after the data, such as the average a donut draws in its hole. */
  summary?: string[][];
}

// The element every chart renders inside: one image to assistive technology,
// labelled with the values it draws, and a control that swaps the image for a
// table of the same values. The table is the keyboard path to the data and reads
// without colour. The frame fills its parent unless sized here; the control takes
// 44px of that height above the plot.
// `center` is drawn over the middle of the plot (a ring's value); it is part of
// the image, so whatever it says must also be in the label.
export function ChartFrame({
  title,
  label,
  table,
  className,
  center,
  children,
}: {
  /** What the chart measures: the table's caption. */
  title: string;
  label: string;
  table: ChartTable;
  className?: string;
  center?: ReactNode;
  children: ReactNode;
}) {
  const [asTable, setAsTable] = useState(false);
  const viewId = useId();
  return (
    <div className={cn("flex h-full w-full flex-col", className)}>
      <button
        type="button"
        aria-controls={viewId}
        onClick={() => setAsTable((v) => !v)}
        className="h-11 min-w-11 flex-shrink-0 self-end rounded-inset px-2 text-sm font-medium text-brand-text hover:underline focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
      >
        {asTable ? "View as chart" : "View as table"}
      </button>
      <div id={viewId} className="relative min-h-0 flex-1">
        {asTable ? (
          <div className="h-full overflow-auto">
            <ChartDataTable title={title} table={table} />
          </div>
        ) : (
          <div role="img" aria-label={label} className="relative h-full w-full">
            {children}
            {center != null && (
              <div className="pointer-events-none absolute inset-0 flex flex-col items-center justify-center text-center">
                {center}
              </div>
            )}
          </div>
        )}
      </div>
    </div>
  );
}

function ChartDataTable({ title, table }: { title: string; table: ChartTable }) {
  const row = (cells: string[], key: string) => (
    <tr key={key} className="border-t border-divider">
      {cells.map((cell, i) =>
        i === 0 ? (
          <th key={i} scope="row" className="py-1.5 pr-3 text-left font-normal text-ink-secondary">
            {cell}
          </th>
        ) : (
          <td key={i} className="py-1.5 pr-3 text-right tabular-nums text-ink">
            {cell}
          </td>
        ),
      )}
    </tr>
  );
  return (
    <table className="w-full border-collapse text-sm">
      <caption className="sr-only">{title}</caption>
      <thead>
        <tr>
          {table.columns.map((column, i) => (
            <th
              key={column}
              scope="col"
              className={cn("pb-1.5 pr-3 font-semibold text-ink", i === 0 ? "text-left" : "text-right")}
            >
              {column}
            </th>
          ))}
        </tr>
      </thead>
      <tbody>
        {table.rows.length === 0 ? (
          <tr className="border-t border-divider">
            <td colSpan={table.columns.length} className="py-1.5 text-ink-secondary">
              No data
            </td>
          </tr>
        ) : (
          table.rows.map((cells, i) => row(cells, `r${i}`))
        )}
      </tbody>
      {table.summary && table.summary.length > 0 && (
        <tfoot>{table.summary.map((cells, i) => row(cells, `s${i}`))}</tfoot>
      )}
    </table>
  );
}
