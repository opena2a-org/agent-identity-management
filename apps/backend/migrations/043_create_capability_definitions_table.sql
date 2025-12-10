-- Migration: Create capability_definitions table
-- Purpose: Central registry for all capability types with metadata

-- 1. Create capability definitions table (source of truth)
CREATE TABLE IF NOT EXISTS capability_definitions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    namespace VARCHAR(50) NOT NULL,
    action VARCHAR(100) NOT NULL,
    capability_type VARCHAR(20) NOT NULL DEFAULT 'core', -- 'core' or 'custom'
    risk_level VARCHAR(20) NOT NULL DEFAULT 'medium', -- 'low', 'medium', 'high', 'critical'
    display_name VARCHAR(200) NOT NULL,
    description TEXT,
    category VARCHAR(50),
    organization_id UUID REFERENCES organizations(id) ON DELETE CASCADE, -- NULL for core capabilities
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- Unique constraint: namespace+action must be unique per org
    -- For PostgreSQL 14 compatibility, we use a partial unique index for NULL org_id
    CONSTRAINT uq_capability_definition_with_org UNIQUE (namespace, action, organization_id)
);

-- For core capabilities (NULL organization_id), use a partial unique index
CREATE UNIQUE INDEX IF NOT EXISTS idx_capability_definitions_unique_core
    ON capability_definitions (namespace, action)
    WHERE organization_id IS NULL;

-- Indexes for performance
CREATE INDEX IF NOT EXISTS idx_capability_definitions_namespace ON capability_definitions(namespace);
CREATE INDEX IF NOT EXISTS idx_capability_definitions_type ON capability_definitions(capability_type);
CREATE INDEX IF NOT EXISTS idx_capability_definitions_org ON capability_definitions(organization_id) WHERE organization_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_capability_definitions_lookup ON capability_definitions(namespace, action);

-- Add updated_at trigger
CREATE TRIGGER update_capability_definitions_updated_at
    BEFORE UPDATE ON capability_definitions
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- 2. Seed core capabilities (organization_id = NULL means global/core)
INSERT INTO capability_definitions (namespace, action, capability_type, risk_level, display_name, description, category) VALUES
-- File System
('file', 'read', 'core', 'low', 'File Read', 'Read files from filesystem', 'filesystem'),
('file', 'write', 'core', 'medium', 'File Write', 'Write/create files', 'filesystem'),
('file', 'delete', 'core', 'high', 'File Delete', 'Delete files', 'filesystem'),
('file', 'execute', 'core', 'critical', 'File Execute', 'Execute files/scripts', 'filesystem'),
-- Database
('db', 'read', 'core', 'low', 'Database Read', 'Query database (SELECT)', 'database'),
('db', 'write', 'core', 'medium', 'Database Write', 'Modify database (INSERT/UPDATE)', 'database'),
('db', 'admin', 'core', 'critical', 'Database Admin', 'Schema changes, DROP, admin ops', 'database'),
-- API/Network
('api', 'call', 'core', 'low', 'API Call', 'Make outbound API requests', 'network'),
('api', 'webhook', 'core', 'medium', 'API Webhook', 'Register/receive webhooks', 'network'),
('network', 'connect', 'core', 'low', 'Network Connect', 'Outbound network connections', 'network'),
('network', 'listen', 'core', 'high', 'Network Listen', 'Open listening ports', 'network'),
-- System
('system', 'exec', 'core', 'critical', 'System Execute', 'Execute system commands', 'system'),
('system', 'admin', 'core', 'critical', 'System Admin', 'System administration', 'system'),
-- User
('user', 'read', 'core', 'low', 'User Read', 'Read user data', 'user'),
('user', 'write', 'core', 'medium', 'User Write', 'Modify user data', 'user'),
('user', 'impersonate', 'core', 'critical', 'User Impersonate', 'Act as another user', 'user'),
-- Data
('data', 'export', 'core', 'medium', 'Data Export', 'Export data externally', 'data'),
('data', 'encrypt', 'core', 'low', 'Data Encrypt', 'Encrypt/decrypt data', 'data'),
-- MCP
('mcp', 'tool_use', 'core', 'medium', 'MCP Tool Use', 'Use MCP server tools', 'mcp'),
('mcp', 'resource_read', 'core', 'low', 'MCP Resource Read', 'Read MCP resources', 'mcp')
ON CONFLICT DO NOTHING;

-- 3. Create mapping table for old format -> new format migration
CREATE TABLE IF NOT EXISTS capability_format_migration (
    old_format VARCHAR(100) PRIMARY KEY,
    new_format VARCHAR(100) NOT NULL
);

INSERT INTO capability_format_migration (old_format, new_format) VALUES
('read_files', 'file:read'),
('write_files', 'file:write'),
('execute_code', 'system:exec'),
('network_access', 'network:connect'),
('database_access', 'db:read'),
('api_calls', 'api:call'),
('user_interaction', 'user:read'),
('data_processing', 'data:export'),
-- Also handle any other formats that might exist
('file_read', 'file:read'),
('file_write', 'file:write'),
('file_delete', 'file:delete'),
('db_query', 'db:read'),
('db_write', 'db:write'),
('api_call', 'api:call')
ON CONFLICT DO NOTHING;

-- 4. Migrate existing agent_capabilities to new format
UPDATE agent_capabilities ac
SET capability_type = cfm.new_format
FROM capability_format_migration cfm
WHERE ac.capability_type = cfm.old_format;

-- 5. Clean up migration table (optional - keep for reference)
-- DROP TABLE capability_format_migration;
