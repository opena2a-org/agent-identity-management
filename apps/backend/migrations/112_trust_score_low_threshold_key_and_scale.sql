-- Migration 112: a trust_score_low policy carries its threshold under
-- trust_threshold, on the trust score's 0-1 scale.
--
-- The request-time evaluator reads rules->'trust_threshold' and compares it
-- to the agent's 0-1 trust score; when the key is absent it applies 0.3.
-- Migration 015 seeded two trust_score_low policies with the value under
-- "threshold" and in percent ({"threshold": 70} for 'Low Trust Score Alert',
-- {"threshold": 50} for 'Critical Trust Score Block'), so neither value was
-- read and both policies evaluated at 0.3. The bundled seed script and the
-- policy backfill wrote {"trust_threshold": 70.0}, a percent the evaluator
-- compared to a 0-1 score, so every agent was below it.
--
-- This migration:
--   1. moves a numeric "threshold" to "trust_threshold" on a row that has no
--      trust_threshold, dividing a value above 1 (a percent) by 100;
--   2. divides a numeric trust_threshold above 1 and at most 100 by 100.
-- A trust_threshold already on the 0-1 scale is left as it is, and a value
-- that is not a number, is negative or is above 100 is not touched.
-- Idempotent: a second run matches no row.
--
-- Effect on the seeded policies: 'Critical Trust Score Block' blocks an
-- evaluable agent below 0.50 (it blocked below 0.30), and 'Low Trust Score
-- Alert' alerts below 0.70.

UPDATE security_policies
SET rules = (rules - 'threshold') || jsonb_build_object(
        'trust_threshold',
        CASE
            WHEN (rules->>'threshold')::numeric > 1
                THEN ((rules->>'threshold')::numeric / 100)::float8
            ELSE (rules->>'threshold')::float8
        END),
    updated_at = NOW()
WHERE policy_type = 'trust_score_low'
  AND NOT (rules ? 'trust_threshold')
  AND CASE
          WHEN jsonb_typeof(rules->'threshold') = 'number'
              THEN (rules->>'threshold')::numeric BETWEEN 0 AND 100
          ELSE false
      END;

UPDATE security_policies
SET rules = jsonb_set(
        rules,
        '{trust_threshold}',
        to_jsonb(((rules->>'trust_threshold')::numeric / 100)::float8)),
    updated_at = NOW()
WHERE policy_type = 'trust_score_low'
  AND CASE
          WHEN jsonb_typeof(rules->'trust_threshold') = 'number'
              THEN (rules->>'trust_threshold')::numeric > 1
               AND (rules->>'trust_threshold')::numeric <= 100
          ELSE false
      END;
