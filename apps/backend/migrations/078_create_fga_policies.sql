-- Migration 078: Fine-Grained Authorization (FGA) Policies
-- Extends AIM's capability-based access control with 9-level scope hierarchy.
-- Policies are additive: agents with no FGA policies = zero behavior change.

CREATE TABLE IF NOT EXISTS fga_policies (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    capability TEXT NOT NULL,

    -- Level 4: Attribute scope
    allowed_attributes TEXT[] DEFAULT '{}',   -- field-level allow list (empty = all)
    denied_attributes TEXT[] DEFAULT '{}',    -- field-level deny list (takes precedence)

    -- Level 3: Object scope
    allowed_objects TEXT[] DEFAULT '{}',      -- object path patterns (empty = all)
    allowed_actions TEXT[] DEFAULT '{}',      -- action types (read, write, delete, execute)

    -- Level 5: Row predicate (JSONB for flexible conditions)
    row_predicate JSONB DEFAULT '{}'::jsonb,  -- e.g., {"org_id": "abc", "env": "production"}

    -- Level 7: Context rules
    context_rules JSONB DEFAULT '{}'::jsonb,  -- e.g., {"max_drift_score": 0.5, "require_scan_clean": true}

    -- Level 6: Chain rules (multi-agent orchestration)
    chain_rules JSONB DEFAULT '{}'::jsonb,    -- e.g., {"max_calls_per_hour": 100, "deny_after_capability": "admin:*"}

    -- Level 8: Intent check configuration
    intent_check JSONB DEFAULT '{}'::jsonb,   -- e.g., {"required": true, "min_confidence": 0.8}

    -- Risk level determines authorization strictness
    risk_level TEXT DEFAULT 'LOW' CHECK (risk_level IN ('HIGH', 'MEDIUM', 'LOW')),

    -- Metadata
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    created_by UUID,

    UNIQUE (agent_id, capability)
);

CREATE INDEX IF NOT EXISTS idx_fga_policies_agent ON fga_policies (agent_id);
CREATE INDEX IF NOT EXISTS idx_fga_policies_risk ON fga_policies (risk_level) WHERE risk_level = 'HIGH';
