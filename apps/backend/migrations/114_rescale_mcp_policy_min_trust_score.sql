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
-- A JSON number above 1 cannot be a [0,1] score, so it was written on the 0-100
-- scale. A number above 1 and up to 100 is divided by 100. A number above 100
-- is no percentage at all and is set to 1, the strictest floor, the value the
-- admin page saves for any percentage of 100 or more. Every result is at most 1,
-- so applying this a second time changes nothing.
--
-- A number from 0 to 1 is already on the canonical scale and is left alone; 0
-- (the seeded "no floor") and 1 are both unchanged. That includes a floor the
-- old admin form stored as a percentage of 1 or less: a stored 1 meant 1% and
-- now reads as 100%, and a stored 0.5 meant 0.5% and now reads as 50%. The
-- migration cannot tell the two apart, so a NOTICE line names every mcp_* policy
-- whose floor lies in (0,1] before the rewrite, for an administrator to check.
--
-- Only a JSON number is rewritten. A JSON null decodes into the evaluator's
-- float64 without error and reads as no floor; it stays null. A string or
-- boolean does not decode into a float64 and is left as it is, and a NOTICE line
-- names each such policy. The type test sits in a CASE because Postgres does not
-- promise to evaluate AND operands in order, and a cast of a non-numeric string
-- would abort the migration.
--
-- The admin page now displays the percentage and stores value/100, so new rows
-- arrive on the canonical scale.
--
-- Scope: every mcp_* policy row, seeded or operator-created. Only the
-- mcp_allowlist evaluator reads minTrustScore (MCPAllowlistRules is the one rule
-- struct that carries it), but the update route takes the policy type and the
-- rules from the request, so a client that changes a policy's type to
-- mcp_allowlist and sends its stored rules back carries their floor into a row
-- the evaluator reads. Rescaling every mcp_* row means a floor carried that way
-- is already on the [0,1] scale. Rows of other policy types are not touched.
-- is_enabled is not touched either: migration 105 keeps the seeded policies
-- disabled, and nothing in this migration enables a policy.
--
-- Numbering note: this tree's highest migration was 112 and aim-cloud's 113 when
-- this was written; the runner tracks applied migrations by filename, so 114 was
-- the first number free in both trees and the gap at 113 here is legal.
--
-- See https://github.com/opena2a-org/agent-identity-management/issues/355.

DO $$
DECLARE
    rescaled INTEGER;
    capped INTEGER;
    kept RECORD;
BEGIN
    SELECT COUNT(*) FILTER (WHERE floor > 1 AND floor <= 100),
           COUNT(*) FILTER (WHERE floor > 100)
    INTO rescaled, capped
    FROM (
        SELECT CASE WHEN jsonb_typeof(rules -> 'minTrustScore') = 'number'
                    THEN (rules ->> 'minTrustScore')::float8
               END AS floor
        FROM security_policies
        WHERE policy_type IN ('mcp_allowlist', 'mcp_blocklist', 'mcp_capabilities', 'mcp_unverified')
    ) AS floors;

    RAISE NOTICE 'security_policies: % mcp_* rows with rules.minTrustScore above 1 and up to 100, dividing by 100; % above 100, setting to 1',
        rescaled, capped;

    FOR kept IN
        SELECT id, name, rules -> 'minTrustScore' AS floor
        FROM security_policies
        WHERE policy_type IN ('mcp_allowlist', 'mcp_blocklist', 'mcp_capabilities', 'mcp_unverified')
          AND CASE WHEN jsonb_typeof(rules -> 'minTrustScore') = 'number'
                   THEN (rules ->> 'minTrustScore')::float8 > 0
                        AND (rules ->> 'minTrustScore')::float8 <= 1
                   ELSE false
              END
        ORDER BY name, id
    LOOP
        RAISE NOTICE 'security_policies: policy % (%) keeps rules.minTrustScore % as a fraction of 1 (0.5 is a 50%% floor); if it was saved as a percentage, set the floor again',
            kept.id, kept.name, kept.floor;
    END LOOP;

    FOR kept IN
        SELECT id, name, rules -> 'minTrustScore' AS floor
        FROM security_policies
        WHERE policy_type IN ('mcp_allowlist', 'mcp_blocklist', 'mcp_capabilities', 'mcp_unverified')
          AND jsonb_typeof(rules -> 'minTrustScore') IN ('string', 'boolean', 'array', 'object')
        ORDER BY name, id
    LOOP
        RAISE NOTICE 'security_policies: policy % (%) has rules.minTrustScore %, which is not a number; it is left unchanged',
            kept.id, kept.name, kept.floor;
    END LOOP;
END $$;

UPDATE security_policies
SET rules = jsonb_set(
        rules,
        '{minTrustScore}',
        to_jsonb(LEAST((rules ->> 'minTrustScore')::float8, 100) / 100)
    ),
    updated_at = NOW()
WHERE policy_type IN ('mcp_allowlist', 'mcp_blocklist', 'mcp_capabilities', 'mcp_unverified')
  AND CASE WHEN jsonb_typeof(rules -> 'minTrustScore') = 'number'
           THEN (rules ->> 'minTrustScore')::float8 > 1
           ELSE false
      END;
