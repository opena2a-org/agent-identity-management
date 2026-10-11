import { afterEach, describe, expect, it } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";
import { CategoryBarChart, DonutChart, ScoreRing, TrendChart, chartSeries } from "./index";
import { describeParts, describeTrend } from "./describe";

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
