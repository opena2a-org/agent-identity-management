-- A2A Supply Chain Tracking
-- Migration 069: Track agent-to-agent call chains for audit and traceability

-- ============================================================================
-- A2A Call Chain / Supply Chain
-- Tracks the chain of agent calls for end-to-end visibility
-- ============================================================================
CREATE TABLE IF NOT EXISTS a2a_call_chain (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    -- Chain identification
    chain_id UUID NOT NULL,  -- Groups related calls into a single chain
    sequence_number INT NOT NULL DEFAULT 0,  -- Order in the chain (0 = initiator)
    parent_call_id UUID REFERENCES a2a_call_chain(id),  -- Previous call in chain

    -- Participants
    caller_agent_id UUID REFERENCES agents(id) ON DELETE SET NULL,
    callee_agent_id UUID REFERENCES agents(id) ON DELETE SET NULL,
    skill_id VARCHAR(100),
    task_id UUID REFERENCES a2a_tasks(id) ON DELETE SET NULL,

    -- Chain origin
    is_human_initiated BOOLEAN DEFAULT TRUE,  -- True if chain started by human
    origin_user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    origin_organization_id UUID REFERENCES organizations(id) ON DELETE SET NULL,

    -- Timing
    call_started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    call_ended_at TIMESTAMPTZ,
    duration_ms INT,

    -- Status
    status VARCHAR(20) NOT NULL DEFAULT 'ACTIVE',
    -- 'ACTIVE', 'COMPLETED', 'FAILED', 'TIMEOUT'
    error_message TEXT,

    -- Data flow tracking
    data_input_size_bytes INT,
    data_output_size_bytes INT,
    pii_detected BOOLEAN DEFAULT FALSE,

    -- Metadata
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_a2a_call_chain_chain ON a2a_call_chain(chain_id, sequence_number);
CREATE INDEX IF NOT EXISTS idx_a2a_call_chain_caller ON a2a_call_chain(caller_agent_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_a2a_call_chain_callee ON a2a_call_chain(callee_agent_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_a2a_call_chain_org ON a2a_call_chain(origin_organization_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_a2a_call_chain_parent ON a2a_call_chain(parent_call_id);

-- ============================================================================
-- A2A Delegation Records
-- Tracks which agents delegate to which other agents
-- ============================================================================
CREATE TABLE IF NOT EXISTS a2a_delegation (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,

    -- Delegation parties
    delegator_agent_id UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    delegate_agent_id UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,

    -- Scope
    skill_ids JSONB DEFAULT '[]'::jsonb,  -- Empty = all skills
    max_chain_depth INT DEFAULT 3,         -- Maximum delegation depth

    -- Permissions
    can_delegate_further BOOLEAN DEFAULT FALSE,
    requires_consent BOOLEAN DEFAULT TRUE,

    -- Validity
    is_active BOOLEAN DEFAULT TRUE,
    valid_from TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    valid_until TIMESTAMPTZ,

    -- Audit
    created_by UUID REFERENCES users(id),
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),

    CONSTRAINT no_self_delegation CHECK (delegator_agent_id != delegate_agent_id)
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_a2a_delegation_unique ON a2a_delegation(delegator_agent_id, delegate_agent_id, organization_id) WHERE is_active = TRUE;
CREATE INDEX IF NOT EXISTS idx_a2a_delegation_delegator ON a2a_delegation(delegator_agent_id, is_active);
CREATE INDEX IF NOT EXISTS idx_a2a_delegation_delegate ON a2a_delegation(delegate_agent_id, is_active);

-- ============================================================================
-- A2A Agent Dependencies
-- Tracks which agents depend on which other agents
-- ============================================================================
CREATE TABLE IF NOT EXISTS a2a_agent_dependencies (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    -- Dependency relationship
    agent_id UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    depends_on_agent_id UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,

    -- Dependency type
    dependency_type VARCHAR(30) NOT NULL DEFAULT 'SKILL',
    -- 'SKILL' - Agent needs another agent's skill
    -- 'DATA' - Agent needs data from another agent
    -- 'ORCHESTRATION' - Agent is orchestrated by another

    -- Dependency details
    skill_ids JSONB DEFAULT '[]'::jsonb,  -- Which skills are depended on
    is_critical BOOLEAN DEFAULT FALSE,     -- Failure of dependency = failure of agent

    -- Statistics
    call_count INT DEFAULT 0,
    success_count INT DEFAULT 0,
    failure_count INT DEFAULT 0,
    avg_latency_ms INT,

    -- Metadata
    first_seen_at TIMESTAMPTZ DEFAULT NOW(),
    last_seen_at TIMESTAMPTZ DEFAULT NOW(),

    CONSTRAINT no_self_dependency CHECK (agent_id != depends_on_agent_id),
    CONSTRAINT unique_dependency UNIQUE (agent_id, depends_on_agent_id, dependency_type)
);

CREATE INDEX IF NOT EXISTS idx_a2a_dependencies_agent ON a2a_agent_dependencies(agent_id);
CREATE INDEX IF NOT EXISTS idx_a2a_dependencies_depends_on ON a2a_agent_dependencies(depends_on_agent_id);
CREATE INDEX IF NOT EXISTS idx_a2a_dependencies_critical ON a2a_agent_dependencies(is_critical) WHERE is_critical = TRUE;

-- ============================================================================
-- Trigger for updated_at
-- ============================================================================
DROP TRIGGER IF EXISTS trigger_a2a_delegation_updated_at ON a2a_delegation;
CREATE TRIGGER trigger_a2a_delegation_updated_at
    BEFORE UPDATE ON a2a_delegation
    FOR EACH ROW EXECUTE FUNCTION update_a2a_updated_at();

-- ============================================================================
-- Comments
-- ============================================================================
COMMENT ON TABLE a2a_call_chain IS 'Tracks agent-to-agent call chains for supply chain visibility';
COMMENT ON TABLE a2a_delegation IS 'Records of which agents are authorized to delegate to other agents';
COMMENT ON TABLE a2a_agent_dependencies IS 'Graph of agent dependencies for impact analysis';
COMMENT ON COLUMN a2a_call_chain.chain_id IS 'Groups all calls in a single end-to-end chain';
COMMENT ON COLUMN a2a_call_chain.sequence_number IS 'Order of call in chain, 0 = initial human-to-agent call';
