-- Disable the seeded MCP security policies, because nothing evaluates them
-- Purpose: migration 052 seeded six MCP policies with is_enabled = true, two of them
--          enforcement_action 'block_and_alert' at severity 'critical'. No code path evaluates
--          any mcp_* policy type. SecurityPolicyService -- the service wired in
--          cmd/server/main.go -- reaches only the six non-MCP types through
--          policyRepo.GetByType, and MCPPolicyEvaluator, which is the sole implementation,
--          has no non-test call site in either repo.
--
--          The admin security-policy page is mounted in production and renders these rows with
--          an enable toggle and their enforcement action, so an administrator sees enabled
--          blocking policies that have never run. A control that is not running must not
--          present as running.
-- Migration: 105_disable_unenforced_mcp_policies.sql
-- Date: 2026-08-06
--
-- Scope: only the six rows migration 052 seeded, matched on policy type AND the exact seeded
-- name. A policy an operator created, or renamed, keeps whatever state the operator set. This
-- resets our own defaults; it does not manage anyone else's configuration.
--
-- Do NOT simply revert this when the evaluator is wired. Re-enabling these rows as they stand
-- would reject every MCP server, for two independent reasons:
--
--   1. rules.minTrustScore is seeded 50 and 30 on a 0-100 scale, while the evaluator compares it
--      against MCPServer.TrustScore, which migration 104 made canonical [0,1].
--   2. 'allowedDomains': ['*'] matches no host at all. matchDomainPattern only special-cases the
--      '*.' prefix and otherwise falls through to an equality test, so a bare '*' means "allow
--      nothing", not "allow everything" -- and 'High-Risk MCP Server Block' is block_and_alert.
--
-- Both are prerequisites of the wiring unit: https://github.com/opena2a-org/agent-identity-management/issues/355.

UPDATE security_policies
SET is_enabled = false,
    updated_at = NOW()
WHERE policy_type IN ('mcp_allowlist', 'mcp_blocklist', 'mcp_capabilities', 'mcp_unverified')
  AND name IN (
      'Trusted MCP Server Domains',
      'Blocked MCP Domains',
      'MCP Capability Requirements',
      'Unverified MCP Server Restrictions',
      'MCP Minimum Trust Score',
      'High-Risk MCP Server Block'
  )
  AND is_enabled = true;

COMMENT ON COLUMN security_policies.policy_type IS
    'Type of policy: capability_violation, trust_score_low, unusual_activity, unauthorized_access, data_exfiltration, config_drift. The mcp_* types (mcp_allowlist, mcp_blocklist, mcp_capabilities, mcp_unverified) are stored and editable but NOT evaluated by any running code path as of migration 105; see https://github.com/opena2a-org/agent-identity-management/issues/355.';
