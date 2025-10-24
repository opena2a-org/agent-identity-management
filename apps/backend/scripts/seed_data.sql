-- Seed Data for AIM Testing
-- Run with: export PGPASSWORD=postgres && psql -h localhost -U postgres -d identity -f scripts/seed_data.sql

-- Create test organization
INSERT INTO organizations (id, name, slug, created_at, updated_at)
VALUES ('11111111-1111-1111-1111-111111111111', 'Test Organization', 'test-org', NOW(), NOW())
ON CONFLICT (id) DO NOTHING;

-- Create test users with different roles
INSERT INTO users (id, organization_id, email, name, role, is_active, created_at, updated_at)
VALUES
  ('22222222-2222-2222-2222-222222222222', '11111111-1111-1111-1111-111111111111',
   'admin@aim.test', 'Admin User', 'admin', true, NOW(), NOW()),

  ('33333333-3333-3333-3333-333333333333', '11111111-1111-1111-1111-111111111111',
   'manager@aim.test', 'Manager User', 'manager', true, NOW(), NOW()),

  ('44444444-4444-4444-4444-444444444444', '11111111-1111-1111-1111-111111111111',
   'member@aim.test', 'Member User', 'member', true, NOW(), NOW()),

  ('55555555-5555-5555-5555-555555555555', '11111111-1111-1111-1111-111111111111',
   'viewer@aim.test', 'Viewer User', 'viewer', true, NOW(), NOW())
ON CONFLICT (id) DO NOTHING;

-- Create test agents
INSERT INTO agents (id, organization_id, name, agent_type, description, is_verified, is_active, created_at, updated_at)
VALUES
  ('66666666-6666-6666-6666-666666666666', '11111111-1111-1111-1111-111111111111',
   'Test AI Agent', 'ai_agent', 'A test AI agent for demonstration', true, true, NOW(), NOW()),

  ('77777777-7777-7777-7777-777777777777', '11111111-1111-1111-1111-111111111111',
   'Test MCP Server', 'mcp_server', 'A test MCP server for demonstration', true, true, NOW(), NOW())
ON CONFLICT (id) DO NOTHING;

-- Create test MCP servers
INSERT INTO mcp_servers (id, organization_id, name, description, url, is_verified, created_at, updated_at)
VALUES
  ('88888888-8888-8888-8888-888888888888', '11111111-1111-1111-1111-111111111111',
   'Test MCP Server 1', 'Test MCP server for API testing', 'http://localhost:9000', true, NOW(), NOW())
ON CONFLICT (id) DO NOTHING;

-- Create test trust scores
INSERT INTO trust_scores (agent_id, score, calculated_at, factors)
VALUES
  ('66666666-6666-6666-6666-666666666666', 85.5, NOW(), '{"verification": 30, "security": 20, "community": 15, "uptime": 15, "incidents": 10, "compliance": 5, "activity": 3, "verified_date": 2}'::jsonb),

  ('77777777-7777-7777-7777-777777777777', 92.0, NOW(), '{"verification": 30, "security": 20, "community": 15, "uptime": 15, "incidents": 10, "compliance": 5, "activity": 3, "verified_date": 2}'::jsonb)
ON CONFLICT (agent_id, calculated_at) DO NOTHING;

SELECT '✅ Seed data inserted successfully' as status;
SELECT '' as blank;
SELECT 'Test Users:' as info;
SELECT '  - admin@aim.test (Admin)' as user;
SELECT '  - manager@aim.test (Manager)' as user;
SELECT '  - member@aim.test (Member)' as user;
SELECT '  - viewer@aim.test (Viewer)' as user;
SELECT '' as blank;
SELECT 'Test Agents:' as info;
SELECT '  - Test AI Agent (66666666-6666-6666-6666-666666666666)' as agent;
SELECT '  - Test MCP Server (77777777-7777-7777-7777-777777777777)' as agent;
