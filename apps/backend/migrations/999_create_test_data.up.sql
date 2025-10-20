-- =============================================================================
-- TEST DATA MIGRATION - Creates comprehensive test data for endpoint testing
-- =============================================================================
-- This migration creates test agents, MCP servers, and related data needed
-- for comprehensive endpoint testing with predictable IDs.
-- =============================================================================

-- ===== 1. CREATE TEST AGENTS =====
-- Note: trust_score is numeric(4,3) so max value is 9.999 (not 100)
-- We use 0-9.999 scale where 9.999 = perfect score
INSERT INTO agents (
  id,
  organization_id,
  name,
  display_name,
  description,
  agent_type,
  status,
  version,
  trust_score,
  capabilities,
  created_by,
  created_at,
  updated_at
)
VALUES
  (
    'b0000000-0000-0000-0000-000000000001',
    'a0000000-0000-0000-0000-000000000001',
    'test-agent-1-verified',
    'Test Agent 1 - Verified',
    'Test agent for endpoint validation - Verified status',
    'ai_agent',
    'verified',
    '1.0.0', -- Version
    8.550, -- High trust score (85.5 on 0-100 scale = 8.550 on 0-10 scale)
    '["read", "write", "execute"]'::jsonb,
    'a0000000-0000-0000-0000-000000000002',
    NOW() - INTERVAL '30 days',
    NOW() - INTERVAL '1 day'
  ),
  (
    'b0000000-0000-0000-0000-000000000002',
    'a0000000-0000-0000-0000-000000000001',
    'test-agent-2-pending',
    'Test Agent 2 - Pending',
    'Test agent for endpoint validation - Pending status',
    'automation_agent',
    'pending',
    '0.1.0', -- Version
    0.000, -- No trust score yet
    '["read", "write"]'::jsonb,
    'a0000000-0000-0000-0000-000000000002',
    NOW() - INTERVAL '7 days',
    NOW() - INTERVAL '1 hour'
  ),
  (
    'b0000000-0000-0000-0000-000000000003',
    'a0000000-0000-0000-0000-000000000001',
    'test-agent-3-suspended',
    'Test Agent 3 - Suspended',
    'Test agent for endpoint validation - Suspended status',
    'ai_agent',
    'suspended',
    '2.1.0', -- Version
    4.520, -- Medium trust score (45.2 on 0-100 scale = 4.520 on 0-10 scale)
    '["read"]'::jsonb,
    'a0000000-0000-0000-0000-000000000002',
    NOW() - INTERVAL '60 days',
    NOW() - INTERVAL '5 days'
  )
ON CONFLICT (id) DO NOTHING;

-- ===== 2. CREATE TEST MCP SERVERS =====
INSERT INTO mcp_servers (
  id,
  organization_id,
  name,
  description,
  status,
  url,
  public_key,
  capabilities,
  created_at,
  updated_at
)
VALUES
  (
    'c0000000-0000-0000-0000-000000000001',
    'a0000000-0000-0000-0000-000000000001',
    'Test MCP Server 1 - Verified',
    'Test MCP server for endpoint validation - Verified status',
    'verified',
    'https://mcp1.test.example.com',
    'test_mcp_public_key_1_base64_encoded_placeholder',
    '["tools", "resources", "prompts"]'::jsonb,
    NOW() - INTERVAL '45 days',
    NOW() - INTERVAL '2 days'
  ),
  (
    'c0000000-0000-0000-0000-000000000002',
    'a0000000-0000-0000-0000-000000000001',
    'Test MCP Server 2 - Pending',
    'Test MCP server for endpoint validation - Pending status',
    'pending',
    'https://mcp2.test.example.com',
    'test_mcp_public_key_2_base64_encoded_placeholder',
    '["tools", "resources"]'::jsonb,
    NOW() - INTERVAL '10 days',
    NOW() - INTERVAL '3 hours'
  ),
  (
    'c0000000-0000-0000-0000-000000000003',
    'a0000000-0000-0000-0000-000000000001',
    'Test MCP Server 3 - Suspended',
    'Test MCP server for endpoint validation - Suspended status',
    'suspended',
    'https://mcp3.test.example.com',
    'test_mcp_public_key_3_base64_encoded_placeholder',
    '["tools"]'::jsonb,
    NOW() - INTERVAL '90 days',
    NOW() - INTERVAL '10 days'
  )
ON CONFLICT (id) DO NOTHING;

-- ===== 3. CREATE VERIFICATION EVENTS =====
INSERT INTO verification_events (
  id,
  organization_id,
  agent_id,
  mcp_server_id,
  verification_type,
  status,
  duration_ms,
  metadata,
  created_at
)
VALUES
  (
    'd0000000-0000-0000-0000-000000000001',
    'a0000000-0000-0000-0000-000000000001',
    'b0000000-0000-0000-0000-000000000001',
    NULL,
    'agent_verification',
    'success',
    150,
    '{"method": "cryptographic"}'::jsonb,
    NOW() - INTERVAL '1 day'
  ),
  (
    'd0000000-0000-0000-0000-000000000002',
    'a0000000-0000-0000-0000-000000000001',
    'b0000000-0000-0000-0000-000000000002',
    NULL,
    'agent_verification',
    'pending',
    NULL,
    '{"method": "cryptographic"}'::jsonb,
    NOW() - INTERVAL '1 hour'
  ),
  (
    'd0000000-0000-0000-0000-000000000003',
    'a0000000-0000-0000-0000-000000000001',
    NULL,
    'c0000000-0000-0000-0000-000000000001',
    'mcp_verification',
    'success',
    200,
    '{"method": "signature"}'::jsonb,
    NOW() - INTERVAL '2 days'
  )
ON CONFLICT (id) DO NOTHING;

-- ===== 4. CREATE API KEYS FOR TEST AGENTS =====
INSERT INTO api_keys (
  id,
  agent_id,
  name,
  key_hash,
  prefix,
  expires_at,
  last_used_at,
  created_at,
  updated_at
)
VALUES
  (
    'e0000000-0000-0000-0000-000000000001',
    'b0000000-0000-0000-0000-000000000001',
    'Test API Key 1',
    'sha256_hash_placeholder_test_key_1',
    'aim_',
    NOW() + INTERVAL '90 days',
    NOW() - INTERVAL '1 hour',
    NOW() - INTERVAL '30 days',
    NOW() - INTERVAL '1 hour'
  ),
  (
    'e0000000-0000-0000-0000-000000000002',
    'b0000000-0000-0000-0000-000000000001',
    'Test API Key 2',
    'sha256_hash_placeholder_test_key_2',
    'aim_',
    NOW() + INTERVAL '30 days',
    NOW() - INTERVAL '10 minutes',
    NOW() - INTERVAL '15 days',
    NOW() - INTERVAL '10 minutes'
  )
ON CONFLICT (id) DO NOTHING;

-- ===== 5. CREATE AUDIT LOG ENTRIES =====
INSERT INTO audit_logs (
  id,
  organization_id,
  user_id,
  action,
  resource_type,
  resource_id,
  status,
  details,
  ip_address,
  user_agent,
  created_at
)
VALUES
  (
    'a1000000-0000-0000-0000-000000000001',
    'a0000000-0000-0000-0000-000000000001',
    'a0000000-0000-0000-0000-000000000002',
    'agent.create',
    'agent',
    'b0000000-0000-0000-0000-000000000001',
    'success',
    '{"agent_name": "Test Agent 1 - Verified"}'::jsonb,
    '127.0.0.1',
    'Test User Agent',
    NOW() - INTERVAL '30 days'
  ),
  (
    'a1000000-0000-0000-0000-000000000002',
    'a0000000-0000-0000-0000-000000000001',
    'a0000000-0000-0000-0000-000000000002',
    'agent.verify',
    'agent',
    'b0000000-0000-0000-0000-000000000001',
    'success',
    '{"verification_method": "cryptographic"}'::jsonb,
    '127.0.0.1',
    'Test User Agent',
    NOW() - INTERVAL '1 day'
  ),
  (
    'a1000000-0000-0000-0000-000000000003',
    'a0000000-0000-0000-0000-000000000001',
    'a0000000-0000-0000-0000-000000000002',
    'mcp.create',
    'mcp_server',
    'c0000000-0000-0000-0000-000000000001',
    'success',
    '{"mcp_name": "Test MCP Server 1 - Verified"}'::jsonb,
    '127.0.0.1',
    'Test User Agent',
    NOW() - INTERVAL '45 days'
  )
ON CONFLICT (id) DO NOTHING;

-- ===== 6. CREATE TAG ASSIGNMENTS =====
INSERT INTO agent_tags (agent_id, tag_id)
SELECT 'b0000000-0000-0000-0000-000000000001', id FROM tags WHERE name = 'production' LIMIT 1
ON CONFLICT (agent_id, tag_id) DO NOTHING;

INSERT INTO agent_tags (agent_id, tag_id)
SELECT 'b0000000-0000-0000-0000-000000000001', id FROM tags WHERE name = 'ai' LIMIT 1
ON CONFLICT (agent_id, tag_id) DO NOTHING;

INSERT INTO agent_tags (agent_id, tag_id)
SELECT 'b0000000-0000-0000-0000-000000000002', id FROM tags WHERE name = 'development' LIMIT 1
ON CONFLICT (agent_id, tag_id) DO NOTHING;

INSERT INTO mcp_server_tags (mcp_server_id, tag_id)
SELECT 'c0000000-0000-0000-0000-000000000001', id FROM tags WHERE name = 'production' LIMIT 1
ON CONFLICT (mcp_server_id, tag_id) DO NOTHING;

INSERT INTO mcp_server_tags (mcp_server_id, tag_id)
SELECT 'c0000000-0000-0000-0000-000000000001', id FROM tags WHERE name = 'tools' LIMIT 1
ON CONFLICT (mcp_server_id, tag_id) DO NOTHING;

-- ===== 7. CREATE CAPABILITY REQUESTS =====
INSERT INTO capability_requests (
  id,
  agent_id,
  capability_name,
  justification,
  status,
  requested_by,
  reviewed_by,
  created_at,
  updated_at
)
VALUES
  (
    'g0000000-0000-0000-0000-000000000001',
    'b0000000-0000-0000-0000-000000000001',
    'admin_access',
    'Test capability request for admin access',
    'pending',
    'a0000000-0000-0000-0000-000000000002',
    NULL,
    NOW() - INTERVAL '1 day',
    NOW() - INTERVAL '1 day'
  ),
  (
    'g0000000-0000-0000-0000-000000000002',
    'b0000000-0000-0000-0000-000000000001',
    'data_export',
    'Test capability request for data export',
    'approved',
    'a0000000-0000-0000-0000-000000000002',
    'a0000000-0000-0000-0000-000000000002',
    NOW() - INTERVAL '10 days',
    NOW() - INTERVAL '5 days'
  )
ON CONFLICT (id) DO NOTHING;

-- ===== 8. UPDATE STATISTICS =====
ANALYZE agents;
ANALYZE mcp_servers;
ANALYZE verification_events;
ANALYZE api_keys;
ANALYZE audit_logs;
ANALYZE agent_tags;
ANALYZE mcp_server_tags;
ANALYZE capability_requests;

-- =============================================================================
-- TEST DATA MIGRATION COMPLETE
-- =============================================================================
-- Created:
-- - 3 test agents (verified, pending, suspended)
-- - 3 test MCP servers (verified, pending, suspended)
-- - 3 verification events
-- - 2 API keys
-- - 3 audit log entries
-- - 5 tag assignments
-- - 2 capability requests
-- =============================================================================
