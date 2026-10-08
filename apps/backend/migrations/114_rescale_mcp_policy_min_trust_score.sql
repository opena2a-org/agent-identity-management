-- Migration 114: put rules.minTrustScore on the canonical [0,1] trust scale for
-- the mcp_* security policies.
--
-- MCPPolicyEvaluator compares rules.minTrustScore against MCPServer.TrustScore,
-- which migration 104 made canonical [0,1] (CHECK 0..1). The threshold itself
-- was never rescaled: migration 052 seeds 50 and 30, and the admin page stored
-- the percentage typed into its form. Every such floor sits above every
-- representable trust score, so each policy carrying one would reject every MCP
-- server the moment the evaluator ran.
--
-- A value above 1 cannot be a [0,1] score, so it was written on the 0-100 scale
-- and is divided by 100. Values in [0,1] are already canonical and are left
-- alone; 0 (the seeded "no floor") and 1 are both unchanged. Only a JSON number
-- is rewritten: the evaluator decodes minTrustScore into a float64, so any other
-- JSON type already fails to decode and is left as it is. The type test sits in a
-- CASE because Postgres does not promise to evaluate AND operands in order, and
-- a cast of a non-numeric string would abort the migration.
--
-- The admin page now displays the percentage and stores value/100, so new rows
-- arrive on the canonical scale.
--
-- Scope: every mcp_* policy row, seeded or operator-created, because every one
-- of them is read by the same comparison. Rows of other policy types are not
-- touched. is_enabled is not touched either: migration 105 keeps the seeded
-- policies disabled, and nothing in this migration enables a policy.
--
-- Numbering note: this tree's highest migration is 112 and aim-cloud's is 113;
-- the runner tracks applied migrations by filename, so 114 is the first number
-- free in both trees and the gap at 113 here is legal.
--
-- See https://github.com/opena2a-org/agent-identity-management/issues/355.

DO $$
DECLARE
    policy_count INTEGER;
BEGIN
    SELECT COUNT(*) INTO policy_count
    FROM security_policies
    WHERE policy_type IN ('mcp_allowlist', 'mcp_blocklist', 'mcp_capabilities', 'mcp_unverified')
      AND CASE WHEN jsonb_typeof(rules -> 'minTrustScore') = 'number'
               THEN (rules ->> 'minTrustScore')::float8 > 1
               ELSE false
          END;

    RAISE NOTICE 'security_policies: % mcp_* rows with rules.minTrustScore above 1, rescaling to [0,1]',
        policy_count;
END $$;

UPDATE security_policies
SET rules = jsonb_set(
        rules,
        '{minTrustScore}',
        to_jsonb((rules ->> 'minTrustScore')::float8 / 100)
    ),
    updated_at = NOW()
WHERE policy_type IN ('mcp_allowlist', 'mcp_blocklist', 'mcp_capabilities', 'mcp_unverified')
  AND CASE WHEN jsonb_typeof(rules -> 'minTrustScore') = 'number'
           THEN (rules ->> 'minTrustScore')::float8 > 1
           ELSE false
      END;
