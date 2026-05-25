-- Migration 096: Recompute mcp_servers.capabilities (JSONB cache)
-- from mcp_server_capabilities (detail table) on every mutation
-- of the detail table.
--
-- The audit doc § 6 observed agent cards showing 3 capabilities
-- while the MCP detail page showed 10 tools for the SAME MCP —
-- two stores with no sync. `mcp_servers.capabilities` is a JSONB
-- list documented as the high-level capability types (see
-- `internal/domain/mcp_server.go`: `Capabilities []string // e.g.,
-- ["tools", "prompts", "resources"]`). `mcp_server_capabilities`
-- holds one row per discovered/declared capability with rich
-- schema (capability_type, schema, last_verified_at).
--
-- Source-of-truth choice (conservative):
--
-- Once discovery has produced detail rows for an MCP server, the
-- JSONB cache is derived from `SELECT DISTINCT capability_type`
-- (filtered to is_active = TRUE) so the agent-card chips and the
-- detail page agree on which capability TYPES are present.
--
-- When the detail table is empty for a given MCP server — for
-- example, a brand-new server whose discovery has not yet run —
-- the trigger preserves whatever the registering agent claimed,
-- so the agent card does not regress to "no capabilities" before
-- discovery happens.
--
-- This is intentionally narrower than "the detail table IS the
-- truth." The agent-claimed JSONB remains the initial fallback;
-- discovered detail rows override once present.
--
-- Closes #164.
--
-- Per the [CHIEF-CA] decision in
-- `todo/2026-05-24-counter-drift-cluster-chief-ca.md`: denormalized
-- cache from a detail table uses an AFTER INSERT/UPDATE/DELETE
-- trigger on the detail table. Same family as migrations 091
-- (capability_violation_count), 093 (agents.trust_score), 094
-- (mcp_servers.trust_score), 095 (a2a peer aggregates).

CREATE OR REPLACE FUNCTION recompute_mcp_capabilities_cache()
RETURNS TRIGGER AS $$
DECLARE
    affected_server UUID;
    new_cache JSONB;
BEGIN
    IF TG_OP = 'DELETE' THEN
        affected_server := OLD.mcp_server_id;
    ELSE
        affected_server := NEW.mcp_server_id;
    END IF;

    -- Compute distinct active capability types as a JSONB array.
    -- Returns NULL if no active rows exist for this server.
    SELECT jsonb_agg(DISTINCT capability_type ORDER BY capability_type)
    INTO new_cache
    FROM mcp_server_capabilities
    WHERE mcp_server_id = affected_server
      AND is_active = TRUE;

    -- Empty-table guard: preserve the agent's original claim if
    -- discovery has produced no active rows. A new MCP that has
    -- not yet been scanned would otherwise have its claimed
    -- capabilities wiped to [].
    IF new_cache IS NULL THEN
        IF TG_OP = 'UPDATE' AND OLD.mcp_server_id IS DISTINCT FROM NEW.mcp_server_id THEN
            -- Fall through to also handle OLD below.
            NULL;
        ELSE
            RETURN COALESCE(NEW, OLD);
        END IF;
    ELSE
        UPDATE mcp_servers
        SET capabilities = new_cache,
            updated_at = NOW()
        WHERE id = affected_server
          AND capabilities IS DISTINCT FROM new_cache;
    END IF;

    -- UPDATE moving a row across mcp_server_ids: recompute the OLD
    -- server too. Rare (the unique constraint
    -- UNIQUE(mcp_server_id, name, capability_type) would block
    -- most cases) but possible.
    IF TG_OP = 'UPDATE' AND OLD.mcp_server_id IS DISTINCT FROM NEW.mcp_server_id THEN
        SELECT jsonb_agg(DISTINCT capability_type ORDER BY capability_type)
        INTO new_cache
        FROM mcp_server_capabilities
        WHERE mcp_server_id = OLD.mcp_server_id
          AND is_active = TRUE;

        IF new_cache IS NOT NULL THEN
            UPDATE mcp_servers
            SET capabilities = new_cache,
                updated_at = NOW()
            WHERE id = OLD.mcp_server_id
              AND capabilities IS DISTINCT FROM new_cache;
        END IF;
    END IF;

    RETURN COALESCE(NEW, OLD);
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_recompute_mcp_capabilities_cache ON mcp_server_capabilities;

CREATE TRIGGER trg_recompute_mcp_capabilities_cache
    AFTER INSERT OR UPDATE OR DELETE ON mcp_server_capabilities
    FOR EACH ROW
    EXECUTE FUNCTION recompute_mcp_capabilities_cache();

-- One-shot backfill: reconcile mcp_servers.capabilities to the
-- derived set of active capability types for any server with
-- detail-table rows. Servers with no detail rows keep their
-- existing JSONB (the agent's claim) — same preserve-on-empty
-- semantic as the trigger.
UPDATE mcp_servers m
SET capabilities = sub.derived,
    updated_at = NOW()
FROM (
    SELECT mcp_server_id,
           jsonb_agg(DISTINCT capability_type ORDER BY capability_type) AS derived
    FROM mcp_server_capabilities
    WHERE is_active = TRUE
    GROUP BY mcp_server_id
) sub
WHERE m.id = sub.mcp_server_id
  AND m.capabilities IS DISTINCT FROM sub.derived;
