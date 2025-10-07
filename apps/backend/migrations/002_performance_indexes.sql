-- Performance optimization indexes
-- Add indexes for common query patterns

-- Agents: frequently queried by organization and status
CREATE INDEX IF NOT EXISTS idx_agents_org_status
  ON agents(organization_id, status);

CREATE INDEX IF NOT EXISTS idx_agents_trust_score
  ON agents(trust_score DESC)
  WHERE status = 'verified';

-- Users: lookup by provider for SSO
CREATE INDEX IF NOT EXISTS idx_users_provider
  ON users(provider, provider_id);

CREATE INDEX IF NOT EXISTS idx_users_org
  ON users(organization_id);

-- API Keys: lookup by hash for authentication
CREATE INDEX IF NOT EXISTS idx_api_keys_hash
  ON api_keys(key_hash);

CREATE INDEX IF NOT EXISTS idx_api_keys_agent_active
  ON api_keys(agent_id, is_active);

-- Trust scores: time-series queries
CREATE INDEX IF NOT EXISTS idx_trust_scores_agent_time
  ON trust_scores(agent_id, created_at DESC);

-- Audit logs: common filters
CREATE INDEX IF NOT EXISTS idx_audit_logs_org_time
  ON audit_logs(organization_id, timestamp DESC);

CREATE INDEX IF NOT EXISTS idx_audit_logs_user
  ON audit_logs(user_id, timestamp DESC);

CREATE INDEX IF NOT EXISTS idx_audit_logs_resource
  ON audit_logs(resource_type, resource_id);

CREATE INDEX IF NOT EXISTS idx_audit_logs_action
  ON audit_logs(action, timestamp DESC);

-- Alerts: unacknowledged alerts are frequently queried
CREATE INDEX IF NOT EXISTS idx_alerts_org_unack
  ON alerts(organization_id, is_acknowledged)
  WHERE is_acknowledged = false;

CREATE INDEX IF NOT EXISTS idx_alerts_severity
  ON alerts(severity, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_alerts_resource
  ON alerts(resource_type, resource_id);

-- Organizations: domain lookup for auto-provisioning
CREATE INDEX IF NOT EXISTS idx_organizations_domain
  ON organizations(domain);

-- Add partial index for active API keys
CREATE INDEX IF NOT EXISTS idx_api_keys_active_only
  ON api_keys(agent_id)
  WHERE is_active = true AND (expires_at IS NULL OR expires_at > CURRENT_TIMESTAMP);

-- Add GIN index for JSONB metadata in audit logs (for full-text search)
CREATE INDEX IF NOT EXISTS idx_audit_logs_metadata
  ON audit_logs USING gin(metadata);

-- Add composite index for compliance reporting queries
CREATE INDEX IF NOT EXISTS idx_agents_compliance
  ON agents(organization_id, status, trust_score);

-- Analyze tables to update statistics for query planner
ANALYZE organizations;
ANALYZE users;
ANALYZE agents;
ANALYZE api_keys;
ANALYZE trust_scores;
ANALYZE audit_logs;
ANALYZE alerts;
