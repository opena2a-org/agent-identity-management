-- Migration 111: one name for the fga_policies onUnavailable CHECK constraint.
--
-- Migration 110 adds a CHECK on fga_policies.context_rules->>'onUnavailable'.
-- One deployment applied an earlier variant of 110 that created the same
-- constraint under the name fga_policies_context_rules_on_unavailable; the
-- published 110 names it fga_policies_context_rules_on_unavailable_check.
-- The expression is identical in both. This migration converges the name so
-- a later migration can refer to the constraint without knowing which 110 ran.
--
-- Idempotent: renames only when the old name exists on fga_policies, and is a
-- no-op everywhere the published 110 ran. It creates nothing.

DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'fga_policies_context_rules_on_unavailable'
          AND conrelid = 'fga_policies'::regclass
    ) THEN
        ALTER TABLE fga_policies
            RENAME CONSTRAINT fga_policies_context_rules_on_unavailable
            TO fga_policies_context_rules_on_unavailable_check;
    END IF;
END
$$;
