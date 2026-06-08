-- Migration 100: declared purpose (ATX declaredPurpose field)
--
-- Adds the optional, structured "what is this agent FOR" declaration to agents,
-- plus the two governed vocabulary registry tables that back the category and
-- taskScope vocabularies. Mirrors the capability registry governance pattern
-- (capability_definitions): closed core (organization_id NULL) plus org-custom,
-- versioned so a later vocabulary revision never silently re-interprets an old
-- credential.
--
-- declaredPurpose is identity/attestation + an OFFLINE detection signal only. It
-- is NEVER an authorization input: nothing in the verification or enforcement
-- path reads this column. Optional/additive — NULL means no purpose declared,
-- which degrades the detection signal but never fails verification.

-- 1. The declared purpose on an agent (nullable; NULL = not declared).
ALTER TABLE agents ADD COLUMN IF NOT EXISTS declared_purpose JSONB;

COMMENT ON COLUMN agents.declared_purpose IS
    'Optional structured declaredPurpose object (atx-spec core.md §1.5): what the agent is for. Identity/attestation + offline detection signal only; never an authorization input.';

-- 2. Purpose category vocabulary. Closed core (organization_id NULL) plus
--    org-custom (<org>.<name>). breadth_class / sensitivity_class drive the
--    breadth measure and verifier-policy defaults.
CREATE TABLE IF NOT EXISTS purpose_categories (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    key              TEXT NOT NULL,
    display_name     TEXT NOT NULL,
    description      TEXT NOT NULL DEFAULT '',
    breadth_class    TEXT NOT NULL CHECK (breadth_class IN ('narrow', 'moderate', 'broad')),
    sensitivity_class TEXT NOT NULL CHECK (sensitivity_class IN ('standard', 'elevated', 'regulated')),
    category_type    TEXT NOT NULL CHECK (category_type IN ('core', 'custom')),
    organization_id  UUID REFERENCES organizations(id) ON DELETE CASCADE,
    vocab_version    TEXT NOT NULL DEFAULT '1',
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- Core categories are globally unique by key; custom categories are unique
    -- per organization. NULLS NOT DISTINCT so two core rows with the same key
    -- collide (PG15+).
    CONSTRAINT purpose_categories_key_org_unique UNIQUE NULLS NOT DISTINCT (key, organization_id),
    -- A core category has no organization; a custom category must have one.
    CONSTRAINT purpose_categories_core_no_org CHECK (
        (category_type = 'core' AND organization_id IS NULL) OR
        (category_type = 'custom' AND organization_id IS NOT NULL)
    )
);

CREATE INDEX IF NOT EXISTS idx_purpose_categories_org ON purpose_categories (organization_id);

-- 3. TaskScope definitions (namespace:objective). Reserved core namespaces are
--    enforced in application code (ReservedTaskScopeNamespaces); this table holds
--    concrete objective definitions for listing and org-custom extension.
CREATE TABLE IF NOT EXISTS task_scope_definitions (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    namespace        TEXT NOT NULL,
    objective        TEXT NOT NULL,
    scope_type       TEXT NOT NULL CHECK (scope_type IN ('core', 'custom')),
    organization_id  UUID REFERENCES organizations(id) ON DELETE CASCADE,
    description      TEXT NOT NULL DEFAULT '',
    vocab_version    TEXT NOT NULL DEFAULT '1',
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT task_scope_namespace_objective_org_unique UNIQUE NULLS NOT DISTINCT (namespace, objective, organization_id),
    CONSTRAINT task_scope_core_no_org CHECK (
        (scope_type = 'core' AND organization_id IS NULL) OR
        (scope_type = 'custom' AND organization_id IS NOT NULL)
    )
);

CREATE INDEX IF NOT EXISTS idx_task_scope_definitions_org ON task_scope_definitions (organization_id);

-- 4. Seed the closed core category vocabulary (vocab v1). No catch-all.
INSERT INTO purpose_categories (key, display_name, breadth_class, sensitivity_class, category_type, vocab_version) VALUES
    ('customer-support',     'Customer support',      'narrow',   'standard',  'core', '1'),
    ('financial-operations', 'Financial operations',  'moderate', 'regulated', 'core', '1'),
    ('data-analysis',        'Data analysis',         'narrow',   'standard',  'core', '1'),
    ('data-engineering',     'Data engineering',      'moderate', 'elevated',  'core', '1'),
    ('software-development', 'Software development',   'moderate', 'standard',  'core', '1'),
    ('devops-automation',    'DevOps automation',     'broad',    'elevated',  'core', '1'),
    ('security-operations',  'Security operations',   'broad',    'elevated',  'core', '1'),
    ('content-generation',   'Content generation',    'narrow',   'standard',  'core', '1'),
    ('research-assistant',   'Research assistant',    'moderate', 'standard',  'core', '1'),
    ('sales-marketing',      'Sales and marketing',   'moderate', 'elevated',  'core', '1'),
    ('hr-people-ops',        'HR / people ops',       'moderate', 'regulated', 'core', '1'),
    ('legal-compliance',     'Legal and compliance',  'narrow',   'regulated', 'core', '1'),
    ('healthcare-clinical',  'Healthcare / clinical', 'moderate', 'regulated', 'core', '1'),
    ('device-control',       'Device control',        'moderate', 'elevated',  'core', '1'),
    ('agent-orchestration',  'Agent orchestration',   'broad',    'elevated',  'core', '1')
ON CONFLICT DO NOTHING;

-- 5. Seed the example core taskScope objectives named in the vocabulary (vocab v1).
--    The reserved core NAMESPACES are governed in application code; orgs and CA
--    add further objectives over time.
INSERT INTO task_scope_definitions (namespace, objective, scope_type, vocab_version, description) VALUES
    ('billing',     'inquiry',   'core', '1', 'Handle a customer billing inquiry'),
    ('billing',     'refund',    'core', '1', 'Issue a customer refund'),
    ('support',     'triage',    'core', '1', 'Triage an inbound support request'),
    ('dataops',     'transform', 'core', '1', 'Transform / reshape data between systems'),
    ('secops',      'scan',      'core', '1', 'Run a security scan'),
    ('orchestrate', 'route',     'core', '1', 'Route or supervise other agents')
ON CONFLICT DO NOTHING;
