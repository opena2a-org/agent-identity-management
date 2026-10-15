-- Migration 130: the seeded "Trusted MCP Server Domains" allowlist admits the
-- logging and completions capabilities.
--
-- Migration 052 seeded this mcp_allowlist policy with allowedCapabilities
-- ["tools", "resources", "prompts"]. MCPPolicyEvaluator holds every server that
-- the policy's allowedDomains admit to that list, so a server on an allowed
-- domain that declares logging or completions, two more capabilities an MCP
-- server can declare, would be reported as "Capability not in allowlist". The
-- policy is disabled (migration 105) and the evaluator is not wired, so nothing
-- is reported today; this keeps the seed from rejecting such a server once both
-- change. See https://github.com/opena2a-org/agent-identity-management/issues/355.
--
-- Scope: only a row with the seeded name and type whose list is still exactly
-- the seeded one. An administrator who edited the list keeps the edit.
-- is_enabled is not touched.

UPDATE security_policies
SET rules = jsonb_set(
        rules,
        '{allowedCapabilities}',
        '["tools", "resources", "prompts", "logging", "completions"]'::jsonb
    ),
    updated_at = NOW()
WHERE policy_type = 'mcp_allowlist'
  AND name = 'Trusted MCP Server Domains'
  AND rules -> 'allowedCapabilities' = '["tools", "resources", "prompts"]'::jsonb;
