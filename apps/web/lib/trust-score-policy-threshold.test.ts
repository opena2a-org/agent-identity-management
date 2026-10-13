import { describe, expect, it } from "vitest";
import {
  DEFAULT_TRUST_SCORE_POLICY_THRESHOLD,
  TRUST_THRESHOLD_RULE_KEY,
  appliedTrustThreshold,
  describeAppliedTrustThreshold,
} from "./trust-score-policy-threshold";

describe("appliedTrustThreshold", () => {
  it("reads trust_threshold and falls back to 0.3, as the evaluator does", () => {
    expect(TRUST_THRESHOLD_RULE_KEY).toBe("trust_threshold");
    expect(DEFAULT_TRUST_SCORE_POLICY_THRESHOLD).toBe(0.3);
  });

  it("returns the numeric trust_threshold the rules carry", () => {
    expect(appliedTrustThreshold({ trust_threshold: 0.5, auto_disable: true })).toEqual({ value: 0.5, source: "rule" });
    expect(appliedTrustThreshold({ trust_threshold: 0.7 })).toEqual({ value: 0.7, source: "rule" });
    expect(appliedTrustThreshold({ trust_threshold: 0 })).toEqual({ value: 0, source: "rule" });
    // A percent is applied as stored: the evaluator does not convert it.
    expect(appliedTrustThreshold({ trust_threshold: 50 })).toEqual({ value: 50, source: "rule" });
  });

  it("returns the default when the rules carry no numeric trust_threshold", () => {
    const fallback = { value: 0.3, source: "default" };
    // The key migration 015 first seeded, which the evaluator never read.
    expect(appliedTrustThreshold({ threshold: 50, auto_disable: true })).toEqual(fallback);
    expect(appliedTrustThreshold({ trustThreshold: 0.5 })).toEqual(fallback);
    expect(appliedTrustThreshold({ trust_threshold: "0.5" })).toEqual(fallback);
    expect(appliedTrustThreshold({ trust_threshold: null })).toEqual(fallback);
    expect(appliedTrustThreshold({})).toEqual(fallback);
    expect(appliedTrustThreshold(null)).toEqual(fallback);
    expect(appliedTrustThreshold(undefined)).toEqual(fallback);
  });
});

describe("describeAppliedTrustThreshold", () => {
  it("states a threshold on the [0,1] scale as a percentage", () => {
    expect(describeAppliedTrustThreshold({ trust_threshold: 0.5 })).toBe("50%");
    expect(describeAppliedTrustThreshold({ trust_threshold: 0.7 })).toBe("70%");
    expect(describeAppliedTrustThreshold({ trust_threshold: 0.3 })).toBe("30%");
    expect(describeAppliedTrustThreshold({ trust_threshold: 0.725 })).toBe("72.5%");
    expect(describeAppliedTrustThreshold({ trust_threshold: 1 })).toBe("100%");
  });

  it("says when the default applies and names the key the rules lack", () => {
    const text = "30% (default: these rules set no numeric trust_threshold)";
    expect(describeAppliedTrustThreshold({ threshold: 50 })).toBe(text);
    expect(describeAppliedTrustThreshold({})).toBe(text);
  });

  it("says when the stored value sits outside the scale of a trust score", () => {
    expect(describeAppliedTrustThreshold({ trust_threshold: 50 })).toBe(
      "5000% (trust_threshold 50 is above the 0-1 scale: every score is below it)",
    );
    expect(describeAppliedTrustThreshold({ trust_threshold: 0 })).toBe("0% (no score is below it)");
    expect(describeAppliedTrustThreshold({ trust_threshold: -0.1 })).toBe("-10% (no score is below it)");
  });
});
