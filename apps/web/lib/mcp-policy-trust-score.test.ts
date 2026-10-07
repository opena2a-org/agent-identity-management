import { describe, expect, it } from "vitest";
import { minTrustScoreToPercent, percentToMinTrustScore } from "./mcp-policy-trust-score";

describe("percentToMinTrustScore", () => {
  it("stores the percentage on the canonical [0,1] scale", () => {
    expect(percentToMinTrustScore(70)).toBe(0.7);
    expect(percentToMinTrustScore(30)).toBe(0.3);
    expect(percentToMinTrustScore(72.5)).toBe(0.725);
    expect(percentToMinTrustScore(0)).toBe(0);
    expect(percentToMinTrustScore(100)).toBe(1);
  });

  it("never stores a value outside [0,1]", () => {
    expect(percentToMinTrustScore(150)).toBe(1);
    expect(percentToMinTrustScore(-5)).toBe(0);
    expect(percentToMinTrustScore(Number.NaN)).toBe(0);
  });
});

describe("minTrustScoreToPercent", () => {
  it("shows a stored [0,1] value as a percentage", () => {
    expect(minTrustScoreToPercent(0.7)).toBe(70);
    expect(minTrustScoreToPercent(0.3)).toBe(30);
    expect(minTrustScoreToPercent(0.725)).toBe(72.5);
    expect(minTrustScoreToPercent(1)).toBe(100);
  });

  it("shows 0 when no floor is stored", () => {
    expect(minTrustScoreToPercent(undefined)).toBe(0);
    expect(minTrustScoreToPercent(null)).toBe(0);
    expect(minTrustScoreToPercent("50")).toBe(0);
  });

  it("round-trips what the form saves", () => {
    for (const percent of [0, 1, 25, 33.33, 50, 72.5, 99.99, 100]) {
      expect(minTrustScoreToPercent(percentToMinTrustScore(percent))).toBe(percent);
    }
  });
});
