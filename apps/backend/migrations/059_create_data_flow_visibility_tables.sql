-- Migration: Data Flow Visibility - Track where sensitive data flows
-- This enables source-based classification (zero false positives) and policy enforcement
-- for PII, PHI, PCI, and other sensitive data types

-- ============================================================================
-- DATA CLASSIFICATION TAXONOMY
-- ============================================================================
-- Defines the classification categories and data types with their risk levels

CREATE TABLE IF NOT EXISTS data_classification_categories (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(50) NOT NULL UNIQUE,           -- PII, PHI, PCI, SENSITIVE, INTERNAL, PUBLIC
    display_name VARCHAR(100) NOT NULL,
    description TEXT,
    risk_level VARCHAR(20) NOT NULL DEFAULT 'medium',  -- low, medium, high, critical
    regulatory_frameworks TEXT[] DEFAULT '{}',  -- GDPR, HIPAA, PCI-DSS, CCPA, etc.
    color VARCHAR(7) DEFAULT '#808080',         -- Hex color for UI display
    icon VARCHAR(50),                           -- Icon name for UI
    sort_order INTEGER DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS data_classification_types (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    category_id UUID NOT NULL REFERENCES data_classification_categories(id) ON DELETE CASCADE,
    name VARCHAR(100) NOT NULL,                 -- ssn, credit_card, email, medical_record, etc.
    display_name VARCHAR(100) NOT NULL,
    description TEXT,

    -- Schema inference patterns (for Tier 2 classification)
    column_patterns TEXT[] DEFAULT '{}',        -- Regex patterns for column names: ['ssn', 'social_security.*']
    table_patterns TEXT[] DEFAULT '{}',         -- Regex patterns for table names: ['patient.*', 'medical.*']

    -- Content detection patterns (for Tier 4 fallback - use with caution)
    content_patterns TEXT[] DEFAULT '{}',       -- Regex patterns for content: ['\d{3}-\d{2}-\d{4}']
    content_pattern_confidence DOUBLE PRECISION DEFAULT 0.6,  -- Confidence when matched by content

    -- Metadata
    examples TEXT[] DEFAULT '{}',               -- Example values for documentation
    is_builtin BOOLEAN DEFAULT FALSE,           -- True for system-defined types
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    UNIQUE(category_id, name)
);

-- ============================================================================
-- DATA SOURCES - Known origins of data with classification
-- ============================================================================
-- This is where the magic happens: classification at SOURCE, not at exit

CREATE TABLE IF NOT EXISTS data_sources (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,

    -- Source identification
    source_type VARCHAR(50) NOT NULL,           -- database, api, file, environment, manual
    source_name VARCHAR(255) NOT NULL,          -- customers_db, stripe_api, config.json
    source_identifier VARCHAR(500) NOT NULL,   -- Full path/connection string (hashed for security)

    -- For database sources
    database_type VARCHAR(50),                  -- postgresql, mysql, mongodb, etc.
    schema_name VARCHAR(255),
    table_name VARCHAR(255),
    column_name VARCHAR(255),

    -- For API sources
    api_endpoint VARCHAR(500),
    api_method VARCHAR(10),
    response_path VARCHAR(500),                 -- JSONPath to the data field

    -- Classification (Tier 1 = explicit, Tier 2 = schema-inferred)
    classification_tier INTEGER NOT NULL DEFAULT 2,  -- 1=explicit, 2=schema, 3=heuristic, 4=content
    classification_type_id UUID REFERENCES data_classification_types(id),
    classification_confidence DOUBLE PRECISION NOT NULL DEFAULT 1.0,

    -- Override flags
    is_manually_classified BOOLEAN DEFAULT FALSE,
    classified_by UUID REFERENCES users(id),
    classified_at TIMESTAMPTZ,

    -- Metadata
    description TEXT,
    metadata JSONB DEFAULT '{}',
    is_active BOOLEAN DEFAULT TRUE,
    discovered_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- Prevent duplicate sources
    UNIQUE(organization_id, source_type, source_identifier)
);

-- Indexes for data source lookups
CREATE INDEX IF NOT EXISTS idx_data_sources_org_id ON data_sources(organization_id);
CREATE INDEX IF NOT EXISTS idx_data_sources_source_type ON data_sources(source_type);
CREATE INDEX IF NOT EXISTS idx_data_sources_classification ON data_sources(classification_type_id);
CREATE INDEX IF NOT EXISTS idx_data_sources_tier ON data_sources(classification_tier);
CREATE INDEX IF NOT EXISTS idx_data_sources_active ON data_sources(is_active);

-- ============================================================================
-- DATA FLOW EVENTS - High-volume time-series table
-- ============================================================================
-- Records every data movement through agents for complete visibility

CREATE TABLE IF NOT EXISTS data_flows (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,

    -- Agent context
    agent_id UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    agent_name VARCHAR(255) NOT NULL,           -- Denormalized for query performance

    -- Flow identification
    flow_id VARCHAR(100) NOT NULL,              -- Unique ID for this flow chain (for tracking across hops)
    parent_flow_id VARCHAR(100),                -- Parent flow if this is a hop
    hop_number INTEGER DEFAULT 1,               -- Position in flow chain

    -- Source (where data came from)
    source_id UUID REFERENCES data_sources(id),
    source_type VARCHAR(50) NOT NULL,           -- database, api, file, user_input, mcp, agent
    source_name VARCHAR(255) NOT NULL,
    source_details JSONB DEFAULT '{}',

    -- Destination (where data is going)
    destination_type VARCHAR(50) NOT NULL,      -- api, mcp, llm, file, database, response
    destination_name VARCHAR(255) NOT NULL,
    destination_endpoint VARCHAR(500),          -- API endpoint, MCP tool, etc.
    destination_details JSONB DEFAULT '{}',

    -- Data classification (inherited from source)
    classification_category VARCHAR(50),        -- PII, PHI, PCI, etc.
    classification_type VARCHAR(100),           -- ssn, credit_card, email, etc.
    classification_tier INTEGER NOT NULL DEFAULT 4,  -- How the classification was determined
    classification_confidence DOUBLE PRECISION NOT NULL DEFAULT 1.0,

    -- Data metrics (no actual data stored - just metadata)
    data_size_bytes BIGINT,
    record_count INTEGER,
    field_count INTEGER,
    fields_detected TEXT[],                     -- List of sensitive field names detected

    -- Policy evaluation result
    policy_id UUID,                             -- Policy that was evaluated
    policy_action VARCHAR(20) NOT NULL DEFAULT 'allow',  -- allow, block, alert, redact
    policy_reason TEXT,

    -- Request context
    request_id VARCHAR(100),                    -- Correlation ID from SDK
    session_id VARCHAR(100),
    user_id VARCHAR(255),                       -- End user context if available
    client_ip VARCHAR(45),

    -- Verification context (links to verification_events)
    verification_event_id UUID,
    capability VARCHAR(100),

    -- Timing
    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ,
    duration_ms INTEGER,

    -- Status
    status VARCHAR(20) NOT NULL DEFAULT 'in_progress',  -- in_progress, completed, blocked, failed
    error_message TEXT,

    -- Metadata
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Indexes optimized for time-series queries and dashboard aggregations
CREATE INDEX IF NOT EXISTS idx_data_flows_org_time ON data_flows(organization_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_data_flows_agent_time ON data_flows(agent_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_data_flows_classification ON data_flows(classification_category, classification_type);
CREATE INDEX IF NOT EXISTS idx_data_flows_destination ON data_flows(destination_type, destination_name);
CREATE INDEX IF NOT EXISTS idx_data_flows_policy_action ON data_flows(policy_action, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_data_flows_flow_chain ON data_flows(flow_id, hop_number);
CREATE INDEX IF NOT EXISTS idx_data_flows_status ON data_flows(status);

-- Composite index for common dashboard queries
CREATE INDEX IF NOT EXISTS idx_data_flows_org_category_time
    ON data_flows(organization_id, classification_category, created_at DESC);

-- ============================================================================
-- DATA FLOW POLICIES - Rules for data movement
-- ============================================================================
-- Define what data can go where, evaluated in priority order

CREATE TABLE IF NOT EXISTS data_flow_policies (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,

    -- Policy identity
    name VARCHAR(255) NOT NULL,
    description TEXT,

    -- Priority (lower = higher priority, first match wins)
    priority INTEGER NOT NULL DEFAULT 100,

    -- Conditions (all must match for policy to apply)
    -- Classification conditions
    classification_categories TEXT[] DEFAULT '{}',  -- Match these categories (empty = any)
    classification_types TEXT[] DEFAULT '{}',       -- Match these types (empty = any)
    min_classification_tier INTEGER,                -- Only apply to tier >= this

    -- Source conditions
    source_types TEXT[] DEFAULT '{}',               -- Match these source types
    source_patterns TEXT[] DEFAULT '{}',            -- Regex patterns for source names

    -- Destination conditions
    destination_types TEXT[] DEFAULT '{}',          -- Match these destination types
    destination_patterns TEXT[] DEFAULT '{}',       -- Regex patterns for destination names
    blocked_destinations TEXT[] DEFAULT '{}',       -- Explicit block list
    allowed_destinations TEXT[] DEFAULT '{}',       -- Explicit allow list (if set, only these allowed)

    -- Agent conditions
    agent_ids UUID[] DEFAULT '{}',                  -- Apply to specific agents (empty = all)
    agent_types TEXT[] DEFAULT '{}',                -- Apply to agent types

    -- Action when policy matches
    action VARCHAR(20) NOT NULL DEFAULT 'alert',    -- allow, block, alert, redact

    -- Alert configuration
    alert_severity VARCHAR(20) DEFAULT 'medium',    -- low, medium, high, critical
    alert_message TEXT,
    notify_channels TEXT[] DEFAULT '{}',            -- slack, email, webhook

    -- Redaction configuration (for action = 'redact')
    redaction_strategy VARCHAR(50),                 -- mask, hash, tokenize, truncate
    redaction_config JSONB DEFAULT '{}',

    -- Metadata
    is_enabled BOOLEAN DEFAULT TRUE,
    is_builtin BOOLEAN DEFAULT FALSE,               -- True for system default policies
    created_by UUID REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    UNIQUE(organization_id, name)
);

-- Index for policy evaluation (priority order)
CREATE INDEX IF NOT EXISTS idx_data_flow_policies_org_priority
    ON data_flow_policies(organization_id, priority, is_enabled);
CREATE INDEX IF NOT EXISTS idx_data_flow_policies_enabled
    ON data_flow_policies(organization_id, is_enabled) WHERE is_enabled = TRUE;

-- ============================================================================
-- DATA FLOW VIOLATIONS - Policy violations for audit
-- ============================================================================

CREATE TABLE IF NOT EXISTS data_flow_violations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,

    -- References
    data_flow_id UUID NOT NULL REFERENCES data_flows(id) ON DELETE CASCADE,
    policy_id UUID NOT NULL REFERENCES data_flow_policies(id) ON DELETE CASCADE,
    agent_id UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,

    -- Violation details
    violation_type VARCHAR(50) NOT NULL,            -- blocked, alerted, redacted
    severity VARCHAR(20) NOT NULL,

    -- What was attempted
    classification_category VARCHAR(50) NOT NULL,
    classification_type VARCHAR(100),
    source_name VARCHAR(255) NOT NULL,
    destination_name VARCHAR(255) NOT NULL,
    destination_endpoint VARCHAR(500),

    -- Action taken
    action_taken VARCHAR(20) NOT NULL,              -- blocked, alerted, redacted, allowed_with_warning
    action_details TEXT,

    -- Resolution (for review workflow)
    status VARCHAR(20) NOT NULL DEFAULT 'open',     -- open, acknowledged, resolved, false_positive
    resolved_by UUID REFERENCES users(id),
    resolved_at TIMESTAMPTZ,
    resolution_notes TEXT,

    -- Alert linkage
    alert_id UUID REFERENCES alerts(id),

    -- Metadata
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Indexes for violation queries
CREATE INDEX IF NOT EXISTS idx_data_flow_violations_org_time
    ON data_flow_violations(organization_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_data_flow_violations_agent ON data_flow_violations(agent_id);
CREATE INDEX IF NOT EXISTS idx_data_flow_violations_status ON data_flow_violations(status);
CREATE INDEX IF NOT EXISTS idx_data_flow_violations_severity ON data_flow_violations(severity, created_at DESC);

-- ============================================================================
-- DATA FLOW STATISTICS - Pre-aggregated metrics for dashboards
-- ============================================================================
-- Aggregated per hour for fast dashboard queries

CREATE TABLE IF NOT EXISTS data_flow_statistics (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,

    -- Time bucket (hourly aggregation)
    bucket_time TIMESTAMPTZ NOT NULL,

    -- Aggregation dimensions
    agent_id UUID REFERENCES agents(id) ON DELETE CASCADE,
    classification_category VARCHAR(50),
    destination_type VARCHAR(50),

    -- Metrics
    total_flows BIGINT DEFAULT 0,
    allowed_flows BIGINT DEFAULT 0,
    blocked_flows BIGINT DEFAULT 0,
    alerted_flows BIGINT DEFAULT 0,
    redacted_flows BIGINT DEFAULT 0,

    total_bytes BIGINT DEFAULT 0,
    total_records BIGINT DEFAULT 0,

    avg_duration_ms DOUBLE PRECISION DEFAULT 0,
    p95_duration_ms DOUBLE PRECISION DEFAULT 0,

    unique_destinations INTEGER DEFAULT 0,
    unique_sources INTEGER DEFAULT 0,

    -- Computed at aggregation time
    computed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    UNIQUE(organization_id, bucket_time, agent_id, classification_category, destination_type)
);

-- Index for dashboard time-series queries
CREATE INDEX IF NOT EXISTS idx_data_flow_stats_org_time
    ON data_flow_statistics(organization_id, bucket_time DESC);
CREATE INDEX IF NOT EXISTS idx_data_flow_stats_agent_time
    ON data_flow_statistics(agent_id, bucket_time DESC);

-- ============================================================================
-- SEED DATA: Default classification categories and types
-- ============================================================================

-- Insert default categories
INSERT INTO data_classification_categories (name, display_name, description, risk_level, regulatory_frameworks, color, icon, sort_order) VALUES
    ('PII', 'Personal Identifiable Information', 'Data that can identify an individual', 'high', ARRAY['GDPR', 'CCPA', 'PIPEDA'], '#E53E3E', 'user', 1),
    ('PHI', 'Protected Health Information', 'Health-related data protected under HIPAA', 'critical', ARRAY['HIPAA', 'HITECH'], '#DD6B20', 'heart', 2),
    ('PCI', 'Payment Card Industry Data', 'Credit card and payment information', 'critical', ARRAY['PCI-DSS'], '#D69E2E', 'credit-card', 3),
    ('SENSITIVE', 'Sensitive Business Data', 'Internal sensitive business information', 'high', ARRAY[]::TEXT[], '#805AD5', 'lock', 4),
    ('INTERNAL', 'Internal Only', 'Internal company data not for public disclosure', 'medium', ARRAY[]::TEXT[], '#3182CE', 'building', 5),
    ('PUBLIC', 'Public Data', 'Data that can be freely shared', 'low', ARRAY[]::TEXT[], '#38A169', 'globe', 6)
ON CONFLICT (name) DO NOTHING;

-- Insert default PII types
INSERT INTO data_classification_types (category_id, name, display_name, description, column_patterns, content_patterns, content_pattern_confidence, examples, is_builtin)
SELECT
    c.id,
    t.name,
    t.display_name,
    t.description,
    t.column_patterns,
    t.content_patterns,
    t.content_pattern_confidence,
    t.examples,
    TRUE
FROM data_classification_categories c
CROSS JOIN (VALUES
    ('ssn', 'Social Security Number', 'US Social Security Number',
     ARRAY['ssn', 'social_security', 'social_security_number', 'ss_number', 'socialsecuritynumber'],
     ARRAY['\d{3}-\d{2}-\d{4}', '\d{9}'],
     0.7, ARRAY['123-45-6789']),
    ('email', 'Email Address', 'Personal or business email address',
     ARRAY['email', 'email_address', 'user_email', 'contact_email', 'e_mail'],
     ARRAY['[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}'],
     0.9, ARRAY['user@example.com']),
    ('phone', 'Phone Number', 'Personal or business phone number',
     ARRAY['phone', 'phone_number', 'telephone', 'mobile', 'cell', 'contact_number'],
     ARRAY['\(\d{3}\)\s?\d{3}-\d{4}', '\d{3}-\d{3}-\d{4}', '\+1\d{10}'],
     0.7, ARRAY['(555) 123-4567']),
    ('address', 'Physical Address', 'Street address or mailing address',
     ARRAY['address', 'street_address', 'mailing_address', 'home_address', 'shipping_address'],
     ARRAY[]::TEXT[],
     0.5, ARRAY['123 Main St, City, ST 12345']),
    ('dob', 'Date of Birth', 'Person''s date of birth',
     ARRAY['dob', 'date_of_birth', 'birthdate', 'birth_date', 'birthday'],
     ARRAY['\d{4}-\d{2}-\d{2}', '\d{2}/\d{2}/\d{4}'],
     0.6, ARRAY['1990-01-15']),
    ('name', 'Full Name', 'Person''s full legal name',
     ARRAY['full_name', 'name', 'legal_name', 'customer_name', 'patient_name', 'first_name', 'last_name'],
     ARRAY[]::TEXT[],
     0.4, ARRAY['John Doe']),
    ('drivers_license', 'Driver''s License', 'Driver''s license number',
     ARRAY['drivers_license', 'license_number', 'dl_number', 'driver_license'],
     ARRAY[]::TEXT[],
     0.5, ARRAY['D1234567'])
) AS t(name, display_name, description, column_patterns, content_patterns, content_pattern_confidence, examples)
WHERE c.name = 'PII'
ON CONFLICT (category_id, name) DO NOTHING;

-- Insert default PHI types
INSERT INTO data_classification_types (category_id, name, display_name, description, column_patterns, content_patterns, content_pattern_confidence, examples, is_builtin)
SELECT
    c.id,
    t.name,
    t.display_name,
    t.description,
    t.column_patterns,
    t.content_patterns,
    t.content_pattern_confidence,
    t.examples,
    TRUE
FROM data_classification_categories c
CROSS JOIN (VALUES
    ('medical_record_number', 'Medical Record Number', 'Patient medical record number',
     ARRAY['mrn', 'medical_record_number', 'medical_record_id', 'patient_id', 'chart_number'],
     ARRAY[]::TEXT[],
     0.5, ARRAY['MRN-12345678']),
    ('diagnosis', 'Medical Diagnosis', 'ICD codes or diagnosis descriptions',
     ARRAY['diagnosis', 'diagnosis_code', 'icd_code', 'condition', 'medical_condition'],
     ARRAY['[A-Z]\d{2}(\.\d+)?'],
     0.6, ARRAY['J06.9', 'Common cold']),
    ('prescription', 'Prescription Information', 'Medication and prescription details',
     ARRAY['prescription', 'medication', 'rx', 'drug', 'medicine', 'rx_number'],
     ARRAY[]::TEXT[],
     0.5, ARRAY['Amoxicillin 500mg']),
    ('lab_result', 'Laboratory Results', 'Medical test results',
     ARRAY['lab_result', 'test_result', 'lab_value', 'blood_test', 'specimen'],
     ARRAY[]::TEXT[],
     0.5, ARRAY['Glucose: 95 mg/dL']),
    ('insurance_id', 'Health Insurance ID', 'Health insurance policy or member ID',
     ARRAY['insurance_id', 'member_id', 'policy_number', 'subscriber_id', 'health_plan_id'],
     ARRAY[]::TEXT[],
     0.6, ARRAY['ABC123456789'])
) AS t(name, display_name, description, column_patterns, content_patterns, content_pattern_confidence, examples)
WHERE c.name = 'PHI'
ON CONFLICT (category_id, name) DO NOTHING;

-- Insert default PCI types
INSERT INTO data_classification_types (category_id, name, display_name, description, column_patterns, content_patterns, content_pattern_confidence, examples, is_builtin)
SELECT
    c.id,
    t.name,
    t.display_name,
    t.description,
    t.column_patterns,
    t.content_patterns,
    t.content_pattern_confidence,
    t.examples,
    TRUE
FROM data_classification_categories c
CROSS JOIN (VALUES
    ('credit_card', 'Credit Card Number', 'Full credit card number (PAN)',
     ARRAY['credit_card', 'card_number', 'cc_number', 'pan', 'payment_card'],
     ARRAY['\d{4}[\s-]?\d{4}[\s-]?\d{4}[\s-]?\d{4}', '\d{16}'],
     0.85, ARRAY['4111-1111-1111-1111']),
    ('cvv', 'Card Verification Value', 'CVV/CVC security code',
     ARRAY['cvv', 'cvc', 'cvv2', 'security_code', 'card_security_code'],
     ARRAY['\d{3,4}'],
     0.3, ARRAY['123']),
    ('expiry_date', 'Card Expiration Date', 'Credit card expiration date',
     ARRAY['expiry', 'expiration', 'exp_date', 'card_expiry', 'expiration_date'],
     ARRAY['\d{2}/\d{2}', '\d{2}/\d{4}'],
     0.5, ARRAY['12/25']),
    ('cardholder_name', 'Cardholder Name', 'Name on the credit card',
     ARRAY['cardholder', 'cardholder_name', 'name_on_card', 'card_holder'],
     ARRAY[]::TEXT[],
     0.4, ARRAY['JOHN DOE']),
    ('bank_account', 'Bank Account Number', 'Bank account number',
     ARRAY['bank_account', 'account_number', 'iban', 'routing_number', 'bank_routing'],
     ARRAY['\d{8,17}', '[A-Z]{2}\d{2}[A-Z0-9]{11,30}'],
     0.6, ARRAY['1234567890'])
) AS t(name, display_name, description, column_patterns, content_patterns, content_pattern_confidence, examples)
WHERE c.name = 'PCI'
ON CONFLICT (category_id, name) DO NOTHING;

-- ============================================================================
-- DEFAULT POLICIES - Enterprise security policies
-- ============================================================================

-- This function creates default policies for an organization when they enable Data Flow Visibility
CREATE OR REPLACE FUNCTION create_default_data_flow_policies(org_id UUID)
RETURNS VOID AS $$
BEGIN
    -- Policy 1: Block PCI data to external APIs (highest priority)
    INSERT INTO data_flow_policies (
        organization_id, name, description, priority,
        classification_categories, destination_types, blocked_destinations,
        action, alert_severity, alert_message, is_builtin
    ) VALUES (
        org_id,
        'Block PCI to External APIs',
        'Prevent payment card data from being sent to external APIs',
        10,
        ARRAY['PCI'],
        ARRAY['api', 'llm'],
        ARRAY['*'],
        'block',
        'critical',
        'Attempted to send payment card data to external API. This is a PCI-DSS violation.',
        TRUE
    ) ON CONFLICT (organization_id, name) DO NOTHING;

    -- Policy 2: Alert on PHI to non-approved destinations
    INSERT INTO data_flow_policies (
        organization_id, name, description, priority,
        classification_categories, destination_types,
        action, alert_severity, alert_message, is_builtin
    ) VALUES (
        org_id,
        'Alert on PHI External Transfer',
        'Alert when protected health information is sent externally',
        20,
        ARRAY['PHI'],
        ARRAY['api', 'llm', 'mcp'],
        'alert',
        'high',
        'Protected health information (PHI) was sent to external service. Review for HIPAA compliance.',
        TRUE
    ) ON CONFLICT (organization_id, name) DO NOTHING;

    -- Policy 3: Alert on PII to LLMs
    INSERT INTO data_flow_policies (
        organization_id, name, description, priority,
        classification_categories, destination_types,
        action, alert_severity, alert_message, is_builtin
    ) VALUES (
        org_id,
        'Alert on PII to LLM',
        'Alert when PII is included in LLM prompts',
        30,
        ARRAY['PII'],
        ARRAY['llm'],
        'alert',
        'medium',
        'Personal identifiable information was included in an LLM prompt. Consider if this is necessary.',
        TRUE
    ) ON CONFLICT (organization_id, name) DO NOTHING;

    -- Policy 4: Allow all internal flows
    INSERT INTO data_flow_policies (
        organization_id, name, description, priority,
        destination_types,
        action, is_builtin
    ) VALUES (
        org_id,
        'Allow Internal Flows',
        'Allow all data flows to internal destinations',
        1000,
        ARRAY['database', 'file', 'response'],
        'allow',
        TRUE
    ) ON CONFLICT (organization_id, name) DO NOTHING;
END;
$$ LANGUAGE plpgsql;

-- ============================================================================
-- COMMENTS
-- ============================================================================

COMMENT ON TABLE data_classification_categories IS 'Classification categories (PII, PHI, PCI) with regulatory mappings';
COMMENT ON TABLE data_classification_types IS 'Specific data types within each category with detection patterns';
COMMENT ON TABLE data_sources IS 'Known data sources with source-based classification (Tier 1-4)';
COMMENT ON TABLE data_flows IS 'High-volume time-series table tracking all data movements through agents';
COMMENT ON TABLE data_flow_policies IS 'Rules defining what data can flow where, evaluated in priority order';
COMMENT ON TABLE data_flow_violations IS 'Audit log of policy violations for compliance and investigation';
COMMENT ON TABLE data_flow_statistics IS 'Pre-aggregated hourly statistics for fast dashboard queries';

COMMENT ON COLUMN data_sources.classification_tier IS '1=explicit tag, 2=schema inference, 3=heuristic, 4=content analysis. Lower tier = higher confidence.';
COMMENT ON COLUMN data_flows.flow_id IS 'Unique ID for tracking data across multiple hops in a flow chain';
COMMENT ON COLUMN data_flow_policies.priority IS 'Lower number = higher priority. First matching policy wins.';
