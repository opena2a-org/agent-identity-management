-- Complete Seed Data for AIM Testing (Matches Actual Schema)
-- Run with: export PGPASSWORD=postgres && psql -h localhost -U postgres -d identity -f scripts/seed_complete.sql

-- Create test organization (all required fields)
INSERT INTO organizations (id, name, domain, plan_type, max_agents, max_users, is_active, created_at, updated_at)
VALUES ('11111111-1111-1111-1111-111111111111', 'Test Organization', 'test.aim.local', 'free', 100, 10, true, NOW(), NOW())
ON CONFLICT (id) DO NOTHING;

-- Create test users with different roles
INSERT INTO users (id, organization_id, email, name, role, provider, provider_id, created_at, updated_at)
VALUES
  ('22222222-2222-2222-2222-222222222222', '11111111-1111-1111-1111-111111111111',
   'admin@aim.test', 'Admin User', 'admin', 'test', 'admin-001', NOW(), NOW()),

  ('33333333-3333-3333-3333-333333333333', '11111111-1111-1111-1111-111111111111',
   'manager@aim.test', 'Manager User', 'manager', 'test', 'manager-001', NOW(), NOW()),

  ('44444444-4444-4444-4444-444444444444', '11111111-1111-1111-1111-111111111111',
   'member@aim.test', 'Member User', 'member', 'test', 'member-001', NOW(), NOW())
ON CONFLICT (id) DO NOTHING;

-- Create test agents (all required fields: display_name, created_by)
INSERT INTO agents (id, organization_id, name, display_name, description, agent_type, status, trust_score, created_by, created_at, updated_at)
VALUES
  ('66666666-6666-6666-6666-666666666666', '11111111-1111-1111-1111-111111111111',
   'test-ai-agent', 'Test AI Agent', 'A test AI agent for demonstration purposes', 'ai_agent', 'active', 0.855, '44444444-4444-4444-4444-444444444444', NOW(), NOW()),

  ('77777777-7777-7777-7777-777777777777', '11111111-1111-1111-1111-111111111111',
   'test-mcp-server', 'Test MCP Server', 'A test MCP server agent', 'mcp_server', 'active', 0.920, '44444444-4444-4444-4444-444444444444', NOW(), NOW())
ON CONFLICT (id) DO NOTHING;

-- Create test trust scores (all 8 factors required)
INSERT INTO trust_scores (agent_id, score, verification_status, certificate_validity, repository_quality, documentation_score, community_trust, security_audit, update_frequency, age_score, confidence, last_calculated, created_at)
VALUES
  ('66666666-6666-6666-6666-666666666666', 0.855, 0.900, 0.850, 0.800, 0.750, 0.900, 0.950, 0.700, 0.600, 0.850, NOW(), NOW()),

  ('77777777-7777-7777-7777-777777777777', 0.920, 0.950, 0.900, 0.850, 0.800, 0.950, 0.980, 0.850, 0.700, 0.900, NOW(), NOW())
ON CONFLICT (agent_id) DO NOTHING;

SELECT '✅ Seed data inserted successfully' as status;
SELECT '' as blank;
SELECT 'Test Organization: Test Organization (test.aim.local)' as info;
SELECT 'Test Users:' as info;
SELECT '  - admin@aim.test (Admin) - ID: 22222222-2222-2222-2222-222222222222' as user;
SELECT '  - manager@aim.test (Manager) - ID: 33333333-3333-3333-3333-333333333333' as user;
SELECT '  - member@aim.test (Member) - ID: 44444444-4444-4444-4444-444444444444' as user;
SELECT '' as blank;
SELECT 'Test Agents:' as info;
SELECT '  - test-ai-agent (AI Agent) - ID: 66666666-6666-6666-6666-666666666666 - Trust Score: 85.5%' as agent;
SELECT '  - test-mcp-server (MCP Server) - ID: 77777777-7777-7777-7777-777777777777 - Trust Score: 92.0%' as agent;
