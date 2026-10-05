/**
 * A call AIM refused, as a finding: what was refused, why, and where the fix
 * lives. Built from one row of `GET /api/v1/agents/{id}/activity`; the server
 * writes `metadata.denialReason` on a refused verification, and it is the same
 * reason string the SDK prints in its `ActionDeniedError`.
 */

export interface RefusedCallFix {
  text: string;
  /** Dashboard page holding the control. Every page linked here is administrator-only. */
  href?: string;
  linkLabel?: string;
  /** An API request that applies the fix, for roles without the dashboard page. */
  request?: string;
}

export type RefusedCallKind = "capability" | "unverified" | "compromised" | "policy" | "other";

export interface RefusedCallFinding {
  kind: RefusedCallKind;
  /** The capability the agent asked for, as recorded on the row. */
  capability: string;
  resource: string | null;
  what: string;
  /** The server's reason, verbatim. */
  why: string;
  fix: RefusedCallFix;
}

export interface ActivityRow {
  action?: string;
  metadata?: Record<string, unknown> | null;
}

export const CAPABILITY_REQUESTS_PATH = "/dashboard/admin/capability-requests";
export const SECURITY_POLICIES_PATH = "/dashboard/admin/security-policies";

const MISSING_CAPABILITY = /does not have permission for capability '([^']+)'/;
const NO_GRANTED_CAPABILITIES = /has no granted capabilities/i;
const NAMED_POLICY = /polic(?:y|ies) '([^']+)'/i;

function str(value: unknown): string | null {
  return typeof value === "string" && value.trim() !== "" ? value.trim() : null;
}

function grantFix(capability: string, agentId: string): RefusedCallFix {
  return {
    text: `Grant ${capability} to this agent: an administrator approves the agent's pending request for it under Capability requests, and a manager or administrator can grant it through the API. If this agent should not hold ${capability}, nothing needs to change.`,
    href: CAPABILITY_REQUESTS_PATH,
    linkLabel: "Open Capability requests",
    request: `POST /api/v1/agents/${agentId}/capabilities {"capabilityType":"${capability}"}`,
  };
}

/** Returns the finding for a refused call, or null when the row records no refusal. */
export function refusedCallFinding(row: ActivityRow, agentId: string): RefusedCallFinding | null {
  const meta = row.metadata ?? {};
  const reason = str(meta.denialReason);
  if (!reason || meta.autoApproved === true) return null;

  const capability = str(meta.actionType) ?? str(row.action) ?? "this capability";
  const resource = str(meta.resource);
  const what = `AIM refused ${capability}${resource ? ` on ${resource}` : ""} for this agent.`;
  const base = { capability, resource, what, why: reason };

  const missing = MISSING_CAPABILITY.exec(reason);
  if (missing || NO_GRANTED_CAPABILITIES.test(reason)) {
    return { ...base, kind: "capability", fix: grantFix(missing?.[1] ?? capability, agentId) };
  }
  if (/^Agent not verified/i.test(reason)) {
    return {
      ...base,
      kind: "unverified",
      fix: {
        text: "An administrator or manager verifies the agent with Verify agent at the top of this page; a suspended agent is restored with Reactivate. Until then AIM refuses every call it makes, whatever it was granted.",
      },
    };
  }
  if (/marked as compromised/i.test(reason)) {
    return {
      ...base,
      kind: "compromised",
      fix: {
        text: "Review the agent's alerts in this timeline and its violations under Security on this page. AIM refuses every call from an agent marked compromised, whatever it was granted, until the mark is cleared.",
      },
    };
  }
  const policy = NAMED_POLICY.exec(reason);
  if (policy) {
    return {
      ...base,
      kind: "policy",
      fix: {
        text: `Review the policy '${policy[1]}' under Security policies: it is the rule that refused this call.`,
        href: SECURITY_POLICIES_PATH,
        linkLabel: "Open Security policies",
      },
    };
  }
  return {
    ...base,
    kind: "other",
    fix: {
      text: "Check this agent's status and capabilities on this page, and the organization's policies under Security policies.",
      href: SECURITY_POLICIES_PATH,
      linkLabel: "Open Security policies",
    },
  };
}
