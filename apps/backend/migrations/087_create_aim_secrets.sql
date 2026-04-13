-- AIM Secrets: identity-native credential management for AI agents
-- Phase 1b: secret namespaces, encrypted credentials, audit log, backend configs

CREATE TABLE IF NOT EXISTS secret_namespaces (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    namespace VARCHAR(255) NOT NULL,
    backend_type VARCHAR(50) NOT NULL DEFAULT 'aim_native',
    operations TEXT[] NOT NULL DEFAULT '{}',
    url_patterns TEXT[] NOT NULL DEFAULT '{}',
    status VARCHAR(20) NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by UUID NOT NULL,
    UNIQUE(agent_id, namespace)
);

CREATE INDEX IF NOT EXISTS idx_secret_ns_agent ON secret_namespaces(agent_id);
CREATE INDEX IF NOT EXISTS idx_secret_ns_status ON secret_namespaces(status);

CREATE TABLE IF NOT EXISTS secret_credentials (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    namespace_id UUID NOT NULL REFERENCES secret_namespaces(id) ON DELETE CASCADE,
    encrypted_blob BYTEA NOT NULL,
    encryption_alg VARCHAR(50) NOT NULL DEFAULT 'X25519-XSalsa20-Poly1305',
    ephemeral_pubkey BYTEA,
    version INTEGER NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_secret_cred_ns ON secret_credentials(namespace_id);
CREATE INDEX IF NOT EXISTS idx_secret_cred_ns_version ON secret_credentials(namespace_id, version DESC);

CREATE TABLE IF NOT EXISTS secret_audit_log (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    namespace_id UUID NOT NULL REFERENCES secret_namespaces(id) ON DELETE CASCADE,
    agent_id UUID NOT NULL,
    operation VARCHAR(50) NOT NULL,
    result VARCHAR(20) NOT NULL,
    deny_reason VARCHAR(500),
    atc_id VARCHAR(255),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_secret_audit_ns ON secret_audit_log(namespace_id);
CREATE INDEX IF NOT EXISTS idx_secret_audit_agent ON secret_audit_log(agent_id);
CREATE INDEX IF NOT EXISTS idx_secret_audit_created ON secret_audit_log(created_at);

CREATE TABLE IF NOT EXISTS secret_backend_configs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    backend_type VARCHAR(50) NOT NULL,
    config_json JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(agent_id, backend_type)
);

CREATE INDEX IF NOT EXISTS idx_secret_backend_agent ON secret_backend_configs(agent_id);
