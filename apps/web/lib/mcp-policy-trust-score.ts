// rules.minTrustScore on the mcp_* security policies is stored on the canonical
// [0,1] trust scale, the scale of an MCP server's trust score since migration 104.
// The admin form edits it as a percentage, so it converts at load and at save.
// Storing the percentage as typed put every floor above every trust score.

/** Converts a stored [0,1] minTrustScore into the percentage the admin form shows. */
export function minTrustScoreToPercent(stored: unknown): number {
  const value = typeof stored === "number" && Number.isFinite(stored) ? stored : 0;
  return Math.round(value * 10000) / 100;
}

/** Converts the admin form's percentage into the [0,1] minTrustScore the API stores. */
export function percentToMinTrustScore(percent: number): number {
  if (!Number.isFinite(percent)) return 0;
  const clamped = Math.min(Math.max(percent, 0), 100);
  return Math.round(clamped * 100) / 10000;
}
