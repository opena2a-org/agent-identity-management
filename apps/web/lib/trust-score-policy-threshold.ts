// A trust_score_low security policy triggers for an agent whose trust score is
// below rules.trust_threshold, a number on the trust score's [0,1] scale. The
// backend evaluator applies 0.3 when the rules carry no numeric trust_threshold
// and reads no other key, "threshold" included. The policy card states the value
// the evaluator applies, so the two constants below repeat the backend's; a
// backend test reads this file and fails when they differ.

/** The rules key the evaluator reads a trust_score_low policy's threshold from. */
export const TRUST_THRESHOLD_RULE_KEY = "trust_threshold";

/** The threshold the evaluator applies when the rules carry no numeric trust_threshold. */
export const DEFAULT_TRUST_SCORE_POLICY_THRESHOLD = 0.3;

export interface AppliedTrustThreshold {
  /** The value the evaluator compares to an agent's trust score, unconverted. */
  value: number;
  /** "rule" when the rules carry a numeric trust_threshold, "default" when they do not. */
  source: "rule" | "default";
}

/** Returns the threshold the evaluator applies for a trust_score_low policy's rules. */
export function appliedTrustThreshold(rules: unknown): AppliedTrustThreshold {
  const stored =
    typeof rules === "object" && rules !== null
      ? (rules as Record<string, unknown>)[TRUST_THRESHOLD_RULE_KEY]
      : undefined;
  if (typeof stored === "number" && Number.isFinite(stored)) {
    return { value: stored, source: "rule" };
  }
  return { value: DEFAULT_TRUST_SCORE_POLICY_THRESHOLD, source: "default" };
}

/**
 * Says, as the percentage the dashboard shows trust scores in, which scores a
 * trust_score_low policy triggers below, and why when that is not a value the
 * rules set on the [0,1] scale.
 */
export function describeAppliedTrustThreshold(rules: unknown): string {
  const { value, source } = appliedTrustThreshold(rules);
  const percent = `${Math.round(value * 10000) / 100}%`;
  if (source === "default") {
    return `${percent} (default: these rules set no numeric ${TRUST_THRESHOLD_RULE_KEY})`;
  }
  if (value > 1) {
    return `${percent} (${TRUST_THRESHOLD_RULE_KEY} ${value} is above the 0-1 scale: every score is below it)`;
  }
  if (value <= 0) {
    return `${percent} (no score is below it)`;
  }
  return percent;
}
