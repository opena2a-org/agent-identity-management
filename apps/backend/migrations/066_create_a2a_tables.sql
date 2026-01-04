-- A2A (Agent-to-Agent) Core Support Tables
-- Migration 065: Creates tables for A2A protocol support with AIM security enhancements
-- See design doc: aim-roadmap/A2A_CORE_SUPPORT.md

-- ============================================================================
-- A2A Agent Cards
-- Standard A2A Agent Cards with AIM attestation and identity binding
-- ============================================================================
CREATE TABLE IF NOT EXISTS a2a_agent_cards (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,

    -- A2A Agent Card URL and data
    card_url VARCHAR(500) NOT NULL,
    card_data JSONB NOT NULL,
    card_hash VARCHAR(64) NOT NULL,  -- SHA256 of card for integrity verification

    -- A2A protocol version
    protocol_version VARCHAR(20) DEFAULT '1.0.0',

    -- AIM attestation
    attestation_signature TEXT,
    attestation_issued_at TIMESTAMPTZ,
    attestation_expires_at TIMESTAMPTZ,

    -- Card validation status
    is_valid BOOLEAN DEFAULT TRUE,
    validation_error TEXT,

    -- Metadata
    last_fetched_at TIMESTAMPTZ,
    fetch_count INT DEFAULT 0,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),

    CONSTRAINT unique_agent_card UNIQUE(agent_id)
);

CREATE INDEX idx_a2a_agent_cards_valid ON a2a_agent_cards(agent_id) WHERE is_valid = TRUE;
CREATE INDEX idx_a2a_agent_cards_expiry ON a2a_agent_cards(attestation_expires_at) WHERE attestation_expires_at IS NOT NULL;

-- ============================================================================
-- A2A Skills
-- Skills parsed from A2A Agent Cards for efficient querying
-- ============================================================================
CREATE TABLE IF NOT EXISTS a2a_skills (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,

    -- A2A skill definition
    skill_id VARCHAR(100) NOT NULL,
    name VARCHAR(200) NOT NULL,
    description TEXT,

    -- Skill metadata
    tags JSONB DEFAULT '[]'::jsonb,
    input_modes JSONB DEFAULT '[]'::jsonb,
    output_modes JSONB DEFAULT '[]'::jsonb,
    examples JSONB DEFAULT '[]'::jsonb,

    -- Usage tracking
    invocation_count INT DEFAULT 0,
    success_count INT DEFAULT 0,
    failure_count INT DEFAULT 0,
    avg_duration_ms INT,

    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),

    CONSTRAINT unique_agent_skill UNIQUE(agent_id, skill_id)
);

CREATE INDEX idx_a2a_skills_agent ON a2a_skills(agent_id);
CREATE INDEX idx_a2a_skills_name ON a2a_skills(name);
CREATE INDEX idx_a2a_skills_tags ON a2a_skills USING GIN(tags);

-- ============================================================================
-- A2A Tasks
-- Audit log of all A2A tasks between agents
-- ============================================================================
CREATE TABLE IF NOT EXISTS a2a_tasks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    external_task_id VARCHAR(100) NOT NULL,
    context_id VARCHAR(100),

    -- Participants
    client_agent_id UUID NOT NULL REFERENCES agents(id),
    remote_agent_id UUID NOT NULL REFERENCES agents(id),

    -- Task info
    skill_id VARCHAR(100),
    state VARCHAR(50) NOT NULL DEFAULT 'SUBMITTED',  -- SUBMITTED, WORKING, INPUT_NEEDED, COMPLETED, FAILED, CANCELLED

    -- Timing
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,

    -- Duration tracking
    duration_ms INT,

    -- AIM policy decision
    policy_decision JSONB,
    policy_evaluated_at TIMESTAMPTZ,

    -- Trust scores at time of task
    client_trust_score_snapshot FLOAT,
    remote_trust_score_snapshot FLOAT,

    -- Messages count
    message_count INT DEFAULT 0,

    -- Error info
    error_code VARCHAR(50),
    error_message TEXT,

    CONSTRAINT valid_task_state CHECK (state IN ('SUBMITTED', 'WORKING', 'INPUT_NEEDED', 'COMPLETED', 'FAILED', 'CANCELLED'))
);

CREATE INDEX idx_a2a_tasks_client ON a2a_tasks(client_agent_id, created_at DESC);
CREATE INDEX idx_a2a_tasks_remote ON a2a_tasks(remote_agent_id, created_at DESC);
CREATE INDEX idx_a2a_tasks_external ON a2a_tasks(external_task_id);
CREATE INDEX idx_a2a_tasks_context ON a2a_tasks(context_id) WHERE context_id IS NOT NULL;
CREATE INDEX idx_a2a_tasks_state ON a2a_tasks(state, created_at DESC);

-- ============================================================================
-- A2A Messages
-- Audit log of messages within A2A tasks (content may be redacted for privacy)
-- ============================================================================
CREATE TABLE IF NOT EXISTS a2a_messages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    task_id UUID REFERENCES a2a_tasks(id) ON DELETE CASCADE,
    external_message_id VARCHAR(100),

    -- Message info
    role VARCHAR(20) NOT NULL,  -- 'user' or 'agent'
    sender_agent_id UUID NOT NULL REFERENCES agents(id),

    -- Content summary (actual content not stored for privacy)
    parts_summary JSONB,  -- [{type, size, mime_type}]
    contains_pii BOOLEAN DEFAULT FALSE,
    pii_types JSONB DEFAULT '[]'::jsonb,  -- ['email', 'ssn', 'phone', etc.]

    -- Signature verification
    aim_signature VARCHAR(200),
    signature_valid BOOLEAN,
    signature_verified_at TIMESTAMPTZ,

    -- Timing
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT valid_message_role CHECK (role IN ('user', 'agent'))
);

CREATE INDEX idx_a2a_messages_task ON a2a_messages(task_id, created_at);
CREATE INDEX idx_a2a_messages_sender ON a2a_messages(sender_agent_id, created_at DESC);

-- ============================================================================
-- A2A Peer Trust
-- Trust relationships between agent pairs based on A2A interactions
-- ============================================================================
CREATE TABLE IF NOT EXISTS a2a_peer_trust (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    peer_agent_id UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,

    -- Interaction stats as client
    tasks_initiated INT DEFAULT 0,
    tasks_initiated_completed INT DEFAULT 0,
    tasks_initiated_failed INT DEFAULT 0,

    -- Interaction stats as server
    tasks_received INT DEFAULT 0,
    tasks_received_completed INT DEFAULT 0,
    tasks_received_failed INT DEFAULT 0,

    -- Performance metrics
    success_rate FLOAT,
    avg_response_time_ms INT,
    p95_response_time_ms INT,

    -- Trust metrics
    peer_trust_score FLOAT,
    trust_computed_at TIMESTAMPTZ,
    trust_data_points INT DEFAULT 0,

    -- Interaction tracking
    first_interaction_at TIMESTAMPTZ,
    last_interaction_at TIMESTAMPTZ,

    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),

    CONSTRAINT unique_peer_trust UNIQUE(agent_id, peer_agent_id),
    CONSTRAINT no_self_trust CHECK (agent_id != peer_agent_id)
);

CREATE INDEX idx_a2a_peer_trust_agent ON a2a_peer_trust(agent_id, peer_trust_score DESC);
CREATE INDEX idx_a2a_peer_trust_peer ON a2a_peer_trust(peer_agent_id);

-- ============================================================================
-- A2A Consent Records
-- User consent for cross-agent data sharing (GDPR/PSD2 compliance)
-- ============================================================================
CREATE TABLE IF NOT EXISTS a2a_consent_records (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id VARCHAR(200) NOT NULL,
    organization_id UUID REFERENCES organizations(id),

    -- Agents involved
    grantor_agent_id UUID NOT NULL REFERENCES agents(id),
    recipient_agent_id UUID NOT NULL REFERENCES agents(id),

    -- Consent details
    scope JSONB NOT NULL,  -- ['pii', 'payment', 'health', etc.]
    purpose TEXT NOT NULL,
    data_types JSONB NOT NULL,  -- ['email', 'address', 'ssn', etc.]

    -- Validity
    granted_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ,
    revoked BOOLEAN DEFAULT FALSE,
    revoked_at TIMESTAMPTZ,
    revoked_reason TEXT,

    -- Audit
    consent_method VARCHAR(50) NOT NULL,  -- 'explicit_click', 'api', 'delegated'
    evidence TEXT,  -- Signature or token
    ip_address INET,
    user_agent TEXT,

    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),

    CONSTRAINT valid_consent_method CHECK (consent_method IN ('explicit_click', 'api', 'delegated', 'inherited'))
);

CREATE INDEX idx_a2a_consent_user ON a2a_consent_records(user_id, revoked);
CREATE INDEX idx_a2a_consent_agents ON a2a_consent_records(grantor_agent_id, recipient_agent_id);
CREATE INDEX idx_a2a_consent_org ON a2a_consent_records(organization_id) WHERE organization_id IS NOT NULL;
CREATE INDEX idx_a2a_consent_active ON a2a_consent_records(user_id, grantor_agent_id, recipient_agent_id)
    WHERE revoked = FALSE;

-- ============================================================================
-- A2A Trust Scores
-- A2A-specific trust score components (aggregated from peer trust)
-- ============================================================================
CREATE TABLE IF NOT EXISTS a2a_trust_scores (
    agent_id UUID PRIMARY KEY REFERENCES agents(id) ON DELETE CASCADE,

    -- Task statistics
    total_tasks_as_client INT DEFAULT 0,
    total_tasks_as_remote INT DEFAULT 0,
    tasks_completed INT DEFAULT 0,
    tasks_failed INT DEFAULT 0,
    tasks_cancelled INT DEFAULT 0,

    -- Performance metrics
    avg_response_time_ms INT,
    p95_response_time_ms INT,
    avg_task_duration_ms INT,

    -- Trust components
    a2a_trust_score FLOAT,
    peer_trust_average FLOAT,
    unique_peers_count INT DEFAULT 0,

    -- Reputation from peers
    positive_feedback_count INT DEFAULT 0,
    negative_feedback_count INT DEFAULT 0,

    -- Metadata
    computed_at TIMESTAMPTZ,
    data_points INT DEFAULT 0,

    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- ============================================================================
-- A2A Request Nonces
-- Anti-replay protection for A2A request signatures
-- ============================================================================
CREATE TABLE IF NOT EXISTS a2a_request_nonces (
    nonce VARCHAR(64) PRIMARY KEY,
    agent_id UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    used_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ NOT NULL,
    request_hash VARCHAR(64)  -- SHA256 of the request body
);

CREATE INDEX idx_a2a_nonces_agent ON a2a_request_nonces(agent_id, used_at DESC);
CREATE INDEX idx_a2a_nonces_expiry ON a2a_request_nonces(expires_at);

-- Cleanup expired nonces (run periodically)
-- DELETE FROM a2a_request_nonces WHERE expires_at < NOW();

-- ============================================================================
-- A2A Policies
-- Cross-agent policies for A2A interactions
-- ============================================================================
CREATE TABLE IF NOT EXISTS a2a_policies (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID REFERENCES organizations(id) ON DELETE CASCADE,

    -- Policy definition
    name VARCHAR(200) NOT NULL,
    description TEXT,
    policy_yaml TEXT NOT NULL,

    -- Targeting
    applies_to_agents JSONB DEFAULT '[]'::jsonb,  -- Empty = all agents
    applies_to_skills JSONB DEFAULT '[]'::jsonb,  -- Empty = all skills

    -- Status
    is_active BOOLEAN DEFAULT TRUE,
    priority INT DEFAULT 100,  -- Lower = higher priority

    -- Audit
    created_by UUID REFERENCES users(id),
    updated_by UUID REFERENCES users(id),
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_a2a_policies_org ON a2a_policies(organization_id, is_active);
CREATE INDEX idx_a2a_policies_active ON a2a_policies(is_active, priority);

-- ============================================================================
-- Triggers for updated_at
-- ============================================================================

CREATE OR REPLACE FUNCTION update_a2a_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trigger_a2a_agent_cards_updated_at
    BEFORE UPDATE ON a2a_agent_cards
    FOR EACH ROW EXECUTE FUNCTION update_a2a_updated_at();

CREATE TRIGGER trigger_a2a_skills_updated_at
    BEFORE UPDATE ON a2a_skills
    FOR EACH ROW EXECUTE FUNCTION update_a2a_updated_at();

CREATE TRIGGER trigger_a2a_peer_trust_updated_at
    BEFORE UPDATE ON a2a_peer_trust
    FOR EACH ROW EXECUTE FUNCTION update_a2a_updated_at();

CREATE TRIGGER trigger_a2a_consent_records_updated_at
    BEFORE UPDATE ON a2a_consent_records
    FOR EACH ROW EXECUTE FUNCTION update_a2a_updated_at();

CREATE TRIGGER trigger_a2a_trust_scores_updated_at
    BEFORE UPDATE ON a2a_trust_scores
    FOR EACH ROW EXECUTE FUNCTION update_a2a_updated_at();

CREATE TRIGGER trigger_a2a_policies_updated_at
    BEFORE UPDATE ON a2a_policies
    FOR EACH ROW EXECUTE FUNCTION update_a2a_updated_at();

-- ============================================================================
-- Comments for documentation
-- ============================================================================

COMMENT ON TABLE a2a_agent_cards IS 'A2A Agent Cards with AIM attestation for secure agent-to-agent communication';
COMMENT ON TABLE a2a_skills IS 'A2A skills parsed from Agent Cards for efficient discovery and routing';
COMMENT ON TABLE a2a_tasks IS 'Audit log of all A2A tasks for compliance and debugging';
COMMENT ON TABLE a2a_messages IS 'Audit log of A2A messages with PII tracking (content redacted)';
COMMENT ON TABLE a2a_peer_trust IS 'Peer-to-peer trust relationships based on A2A interaction history';
COMMENT ON TABLE a2a_consent_records IS 'User consent records for cross-agent data sharing (GDPR/PSD2)';
COMMENT ON TABLE a2a_trust_scores IS 'Aggregated A2A trust scores for each agent';
COMMENT ON TABLE a2a_request_nonces IS 'Nonces for anti-replay protection on signed A2A requests';
COMMENT ON TABLE a2a_policies IS 'Cross-agent policies governing A2A interactions';
