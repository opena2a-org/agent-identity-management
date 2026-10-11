import type { ReactElement } from "react";
import { afterEach, describe, expect, it } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { CategoryBarChart, DonutChart, ScoreRing, TrendChart, chartSeries } from "./index";
import { describeParts, describeTrend } from "./describe";
import { HIT_AREA_FLOOR_PX, hitAreaPx } from "@/tests/readability";

// ResponsiveContainer observes its size; jsdom has no ResizeObserver and lays
// nothing out, so the frame and its label render while the plot stays empty.
globalThis.ResizeObserver ??= class {
  observe() {}
  unobserve() {}
  disconnect() {}
};

afterEach(cleanup);

describe("chart primitives", () => {
  it("render as one labelled image that reads the donut's slices", () => {
    render(
      <DonutChart
        title="Risk distribution"
        data={[
          { name: "Low risk", value: 75, color: chartSeries.green },
          { name: "Critical", value: 25, color: chartSeries.red },
        ]}
        formatValue={(v) => `${v}%`}
      />,
    );
    expect(screen.getByRole("img", { name: "Risk distribution: Low risk 75%, Critical 25%" })).toBeTruthy();
  });

  it("read each bar's category and count", () => {
    render(
      <CategoryBarChart
        title="Trust score distribution"
        data={[
          { range: "90-100%", count: 3 },
          { range: "<50%", count: 0 },
        ]}
        categoryKey="range"
        valueKey="count"
      />,
    );
    expect(screen.getByRole("img", { name: "Trust score distribution: 90-100% 3, <50% 0" })).toBeTruthy();
  });

  it("read a trend's window and each series' latest value", () => {
    render(
      <TrendChart
        title="Protection timeline, last 30 days"
        data={[
          { date: "Sep 1", actions: 4, blocked: 1 },
          { date: "Sep 2", actions: 6, blocked: 0 },
        ]}
        xKey="date"
        series={[
          { key: "actions", name: "Actions", color: chartSeries.brand },
          { key: "blocked", name: "Blocked", color: chartSeries.red, kind: "line" },
        ]}
      />,
    );
    expect(
      screen.getByRole("img", {
        name: "Protection timeline, last 30 days: 2 points from Sep 1 to Sep 2; Actions latest 6, Blocked latest 0",
      }),
    ).toBeTruthy();
  });

  it("read a donut's centre summary after its slices", () => {
    render(
      <DonutChart
        title="Agents by trust score"
        data={[{ name: "High", value: 3, color: chartSeries.brand }]}
        center={{ value: "82", caption: "Avg trust" }}
      />,
    );
    expect(screen.getByRole("img", { name: "Agents by trust score: High 3; Avg trust 82" })).toBeTruthy();
  });

  it("read a stacked bar trend the same way as its line and area trends", () => {
    render(
      <TrendChart
        title="Verifications per month, last 2 months"
        data={[
          { month: "Sep", verified: 4, pending: 1 },
          { month: "Oct", verified: 7, pending: 2 },
        ]}
        xKey="month"
        axes={false}
        series={[
          { key: "verified", name: "Verified", color: chartSeries.brand, kind: "bar", stack: "v" },
          { key: "pending", name: "Pending", color: chartSeries.muted, kind: "bar", stack: "v" },
        ]}
      />,
    );
    expect(
      screen.getByRole("img", {
        name: "Verifications per month, last 2 months: 2 points from Sep to Oct; Verified latest 7, Pending latest 2",
      }),
    ).toBeTruthy();
  });

  it("read a score against its ceiling", () => {
    render(
      <ScoreRing title="Security posture score" value={82}>
        <span>82</span>
      </ScoreRing>,
    );
    expect(screen.getByRole("img", { name: "Security posture score: 82 of 100" })).toBeTruthy();
  });

  it("say so when there is nothing to draw", () => {
    expect(describeParts("Task state distribution", [])).toBe("Task state distribution: no data");
    expect(describeTrend("Attestation activity", [], "date", [{ key: "n", name: "Attestations" }])).toBe(
      "Attestation activity: no data",
    );
  });

  it("do not invent a value a point does not carry", () => {
    expect(
      describeTrend("Trust score history", [{ t: "Oct 1" }], "t", [{ key: "score", name: "Trust score" }]),
    ).toBe("Trust score history: 1 point, Oct 1; Trust score latest not reported");
  });
});

// The table a chart swaps in, as the text of each row: headings first, then the
// data, then any summary row.
function openTable(): string[][] {
  fireEvent.click(screen.getByRole("button", { name: "View as table" }));
  return Array.from(screen.getByRole("table").querySelectorAll("tr")).map((tr) =>
    Array.from(tr.children).map((cell) => cell.textContent ?? ""),
  );
}

describe("chart table alternative", () => {
  const everyPrimitive: Array<[string, ReactElement]> = [
    ["DonutChart", <DonutChart key="d" title="Tasks by state" data={[{ name: "Completed", value: 4, color: chartSeries.green }]} />],
    ["CategoryBarChart", <CategoryBarChart key="c" title="Agents by trust score range" data={[{ range: "90-100%", count: 3 }]} categoryKey="range" valueKey="count" />],
    ["TrendChart", <TrendChart key="t" title="Attestations, last 7 days" data={[{ date: "Oct 1", n: 2 }]} xKey="date" series={[{ key: "n", name: "Attestations", color: chartSeries.brand }]} />],
    ["ScoreRing", <ScoreRing key="s" title="Security posture score" value={82} />],
  ];

  it.each(everyPrimitive)("is offered by %s through a 44px control", (_name, chart) => {
    render(chart);
    const toggle = screen.getByRole("button", { name: "View as table" });
    const area = hitAreaPx(toggle);
    expect(area.height).toBeGreaterThanOrEqual(HIT_AREA_FLOOR_PX);
    expect(area.width).toBeGreaterThanOrEqual(HIT_AREA_FLOOR_PX);
    expect(toggle.getAttribute("aria-controls")).toBeTruthy();
    expect(document.getElementById(toggle.getAttribute("aria-controls")!)?.contains(screen.getByRole("img"))).toBe(true);
  });

  it("carries a donut's slices and its centre summary, captioned with what it measures", () => {
    render(
      <DonutChart
        title="Risk distribution, share of agents and MCP servers"
        data={[
          { name: "Low risk", value: 75, color: chartSeries.green },
          { name: "Critical", value: 25, color: chartSeries.red },
        ]}
        formatValue={(v) => `${v}%`}
        columns={["Risk", "Share"]}
        center={{ value: "82", caption: "Avg trust" }}
      />,
    );
    expect(openTable()).toEqual([
      ["Risk", "Share"],
      ["Low risk", "75%"],
      ["Critical", "25%"],
      ["Avg trust", "82"],
    ]);
    expect(screen.getByRole("table", { name: "Risk distribution, share of agents and MCP servers" })).toBeTruthy();
  });

  it("swaps the image out and back", () => {
    render(<ScoreRing title="Security posture score" value={82} />);
    expect(openTable()).toEqual([
      ["Measure", "Value"],
      ["Security posture score", "82"],
      ["Out of", "100"],
    ]);
    expect(screen.queryByRole("img")).toBeNull();
    fireEvent.click(screen.getByRole("button", { name: "View as chart" }));
    expect(screen.queryByRole("table")).toBeNull();
    expect(screen.getByRole("img", { name: "Security posture score: 82 of 100" })).toBeTruthy();
  });

  it("carries every point of a trend, one column per series, without inventing a missing value", () => {
    render(
      <TrendChart
        title="Agent actions and blocked actions, last 30 days"
        data={[
          { date: "Sep 1", actions: 4, blocked: 1 },
          { date: "Sep 2", actions: 6 },
        ]}
        xKey="date"
        series={[
          { key: "actions", name: "Actions", color: chartSeries.brand },
          { key: "blocked", name: "Blocked", color: chartSeries.red, kind: "line" },
        ]}
      />,
    );
    expect(openTable()).toEqual([
      ["Date", "Actions", "Blocked"],
      ["Sep 1", "4", "1"],
      ["Sep 2", "6", "not reported"],
    ]);
  });

  it("reads each bar under the field names unless given headings", () => {
    render(
      <CategoryBarChart
        title="Agents by trust score range"
        data={[
          { range: "90-100%", count: 3 },
          { range: "<50%", count: 0 },
        ]}
        categoryKey="range"
        valueKey="count"
      />,
    );
    expect(openTable()).toEqual([
      ["Range", "Count"],
      ["90-100%", "3"],
      ["<50%", "0"],
    ]);
  });

  it("says so when there is nothing to tabulate", () => {
    render(<TrendChart title="Attestations, last 7 days" data={[]} xKey="date" series={[{ key: "n", name: "Attestations", color: chartSeries.brand }]} />);
    expect(openTable()).toEqual([["Date", "Attestations"], ["No data"]]);
  });
});
